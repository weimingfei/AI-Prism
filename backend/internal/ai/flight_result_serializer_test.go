package ai

import (
	"encoding/base64"
	"encoding/json"
	"strconv"
	"strings"
	"testing"

	"github.com/ai-prism/backend/internal/config"
)

func TestSerializeFlightResultCompressesAndVerifiesChecksum(t *testing.T) {
	resp := ChatResponse{
		Content: strings.Repeat("knowledge explanation ", 200),
		Raw:     []byte(strings.Repeat("raw", 200)),
	}
	stored, err := serializeFlightResult(resp, config.SingleFlightConfig{
		CompressionCodec:          "gzip",
		CompressionThresholdBytes: 32,
	})
	if err != nil {
		t.Fatalf("serializeFlightResult returned error: %v", err)
	}
	if !stored.Compressed || stored.Codec != "gzip" {
		t.Fatalf("expected gzip compressed result, got compressed=%v codec=%q", stored.Compressed, stored.Codec)
	}
	if stored.Payload == "" || stored.Checksum == "" || stored.RawSize <= stored.StoredSize {
		t.Fatalf("unexpected serialized metadata: %#v", stored)
	}

	decoded, err := deserializeFlightResult(map[string]string{
		"payload":     stored.Payload,
		"codec":       stored.Codec,
		"compressed":  boolFlag(stored.Compressed),
		"rawSize":     "0",
		"storedSize":  "0",
		"checksum":    stored.Checksum,
		"contentType": stored.ContentType,
	})
	if err == nil {
		t.Fatalf("expected raw size mismatch")
	}

	decoded, err = deserializeFlightResult(map[string]string{
		"payload":     stored.Payload,
		"codec":       stored.Codec,
		"compressed":  boolFlag(stored.Compressed),
		"rawSize":     intString(stored.RawSize),
		"storedSize":  intString(stored.StoredSize),
		"checksum":    stored.Checksum,
		"contentType": stored.ContentType,
	})
	if err != nil {
		t.Fatalf("deserializeFlightResult returned error: %v", err)
	}
	if decoded.Content != resp.Content || string(decoded.Raw) != string(resp.Raw) {
		t.Fatalf("decoded response mismatch")
	}
}

func TestSerializeFlightResultSkipsCompressionBelowThreshold(t *testing.T) {
	resp := ChatResponse{Content: "small"}
	stored, err := serializeFlightResult(resp, config.SingleFlightConfig{
		CompressionCodec:          "gzip",
		CompressionThresholdBytes: 4096,
	})
	if err != nil {
		t.Fatalf("serializeFlightResult returned error: %v", err)
	}
	if stored.Compressed || stored.Codec != "none" {
		t.Fatalf("expected uncompressed result, got compressed=%v codec=%q", stored.Compressed, stored.Codec)
	}
	if _, err := base64.StdEncoding.DecodeString(stored.Payload); err != nil {
		t.Fatalf("payload should be base64 encoded even without compression: %v", err)
	}
}

func TestDeserializeFlightResultRejectsChecksumMismatch(t *testing.T) {
	stored, err := serializeFlightResult(ChatResponse{Content: "stable"}, config.SingleFlightConfig{
		CompressionCodec:          "gzip",
		CompressionThresholdBytes: 1,
	})
	if err != nil {
		t.Fatalf("serializeFlightResult returned error: %v", err)
	}
	_, err = deserializeFlightResult(map[string]string{
		"payload":    stored.Payload,
		"codec":      stored.Codec,
		"compressed": boolFlag(stored.Compressed),
		"rawSize":    intString(stored.RawSize),
		"storedSize": intString(stored.StoredSize),
		"checksum":   "bad-checksum",
	})
	if err == nil || !strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatalf("expected checksum mismatch error, got %v", err)
	}
}

func TestDeserializeFlightResultSupportsLegacyPlainJSONPayload(t *testing.T) {
	raw, err := json.Marshal(flightStoredResult{Response: ChatResponse{Content: "legacy"}})
	if err != nil {
		t.Fatalf("marshal legacy payload: %v", err)
	}
	decoded, err := deserializeFlightResult(map[string]string{
		"payload":    string(raw),
		"codec":      "json",
		"compressed": "0",
		"rawSize":    intString(len(raw)),
		"storedSize": intString(len(raw)),
		"checksum":   checksum(raw),
	})
	if err != nil {
		t.Fatalf("deserializeFlightResult returned error: %v", err)
	}
	if decoded.Content != "legacy" {
		t.Fatalf("decoded legacy response mismatch: %#v", decoded)
	}
}

func intString(value int) string {
	return strconv.Itoa(value)
}
