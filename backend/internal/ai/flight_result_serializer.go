package ai

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"strconv"
	"strings"

	"github.com/ai-prism/backend/internal/config"
)

type flightSerializedResult struct {
	Payload     string
	Codec       string
	Compressed  bool
	RawSize     int
	StoredSize  int
	Checksum    string
	ContentType string
}

func serializeFlightResult(resp ChatResponse, cfg config.SingleFlightConfig) (flightSerializedResult, error) {
	rawBytes, err := json.Marshal(flightStoredResult{Response: resp})
	if err != nil {
		return flightSerializedResult{}, err
	}

	codec := normalizeFlightCompressionCodec(cfg.CompressionCodec)
	threshold := cfg.CompressionThresholdBytes
	if threshold <= 0 {
		threshold = 4096
	}

	shouldCompress := codec == "gzip" && len(rawBytes) >= threshold
	storedBytes := rawBytes
	if shouldCompress {
		storedBytes, err = gzipBytes(rawBytes)
		if err != nil {
			return flightSerializedResult{}, err
		}
	}

	if !shouldCompress {
		codec = "none"
	}
	return flightSerializedResult{
		Payload:     base64.StdEncoding.EncodeToString(storedBytes),
		Codec:       codec,
		Compressed:  shouldCompress,
		RawSize:     len(rawBytes),
		StoredSize:  len(storedBytes),
		Checksum:    checksum(rawBytes),
		ContentType: "application/json",
	}, nil
}

func deserializeFlightResult(fields map[string]string) (ChatResponse, error) {
	payload := strings.TrimSpace(fields["payload"])
	if payload == "" {
		return ChatResponse{}, errors.New("flight result payload is empty")
	}
	storedBytes, err := base64.StdEncoding.DecodeString(payload)
	if err != nil {
		if strings.EqualFold(strings.TrimSpace(fields["codec"]), "json") && !parseStoredBool(fields["compressed"]) {
			storedBytes = []byte(payload)
		} else {
			return ChatResponse{}, err
		}
	}

	compressed := parseStoredBool(fields["compressed"])
	codec := normalizeFlightCompressionCodec(fields["codec"])
	rawBytes := storedBytes
	if compressed {
		if codec != "gzip" {
			return ChatResponse{}, errors.New("unsupported compressed flight result codec")
		}
		rawBytes, err = gunzipBytes(storedBytes)
		if err != nil {
			return ChatResponse{}, err
		}
	}

	if expected := strings.TrimSpace(fields["checksum"]); expected != "" && expected != checksum(rawBytes) {
		return ChatResponse{}, errors.New("flight result checksum mismatch")
	}
	if rawSize, err := strconv.Atoi(strings.TrimSpace(fields["rawSize"])); err == nil && rawSize >= 0 && rawSize != len(rawBytes) {
		return ChatResponse{}, errors.New("flight result raw size mismatch")
	}
	if storedSize, err := strconv.Atoi(strings.TrimSpace(fields["storedSize"])); err == nil && storedSize >= 0 && storedSize != len(storedBytes) {
		return ChatResponse{}, errors.New("flight result stored size mismatch")
	}

	var stored flightStoredResult
	if err := json.Unmarshal(rawBytes, &stored); err != nil {
		return ChatResponse{}, err
	}
	return stored.Response, nil
}

func normalizeFlightCompressionCodec(codec string) string {
	switch strings.ToLower(strings.TrimSpace(codec)) {
	case "", "gzip":
		return "gzip"
	default:
		return "none"
	}
}

func gzipBytes(rawBytes []byte) ([]byte, error) {
	var buffer bytes.Buffer
	writer := gzip.NewWriter(&buffer)
	if _, err := writer.Write(rawBytes); err != nil {
		_ = writer.Close()
		return nil, err
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

func gunzipBytes(storedBytes []byte) ([]byte, error) {
	reader, err := gzip.NewReader(bytes.NewReader(storedBytes))
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	return io.ReadAll(reader)
}

func parseStoredBool(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "y":
		return true
	default:
		return false
	}
}
