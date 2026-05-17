package filetype

import (
	"bytes"
	"errors"
	"path/filepath"
	"strings"
)

const (
	KindPDF   = "pdf"
	KindImage = "image"
	KindAudio = "audio"
	KindVideo = "video"
)

type DetectedFile struct {
	Kind        string
	ContentType string
	Extension   string
	MaxSize     int64
}

var (
	ErrEmptyFile         = errors.New("uploaded file cannot be empty")
	ErrUnsupportedType   = errors.New("unsupported file type")
	ErrFileTooLarge      = errors.New("uploaded file exceeds size limit")
	ErrExtensionMismatch = errors.New("file extension does not match detected file type")
)

type signature struct {
	kind        string
	contentType string
	extension   string
	maxSize     int64
	prefixes    [][]byte
	matcher     func([]byte) bool
}

var signatures = []signature{
	{
		kind:        KindPDF,
		contentType: "application/pdf",
		extension:   ".pdf",
		maxSize:     20 * 1024 * 1024,
		prefixes:    [][]byte{[]byte("%PDF-")},
	},
	{
		kind:        KindImage,
		contentType: "image/jpeg",
		extension:   ".jpg",
		maxSize:     10 * 1024 * 1024,
		prefixes:    [][]byte{{0xFF, 0xD8, 0xFF}},
	},
	{
		kind:        KindImage,
		contentType: "image/png",
		extension:   ".png",
		maxSize:     10 * 1024 * 1024,
		prefixes:    [][]byte{{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}},
	},
	{
		kind:        KindImage,
		contentType: "image/gif",
		extension:   ".gif",
		maxSize:     10 * 1024 * 1024,
		prefixes:    [][]byte{[]byte("GIF87a"), []byte("GIF89a")},
	},
	{
		kind:        KindImage,
		contentType: "image/bmp",
		extension:   ".bmp",
		maxSize:     10 * 1024 * 1024,
		prefixes:    [][]byte{[]byte("BM")},
	},
	{
		kind:        KindAudio,
		contentType: "audio/mpeg",
		extension:   ".mp3",
		maxSize:     50 * 1024 * 1024,
		matcher: func(data []byte) bool {
			return bytes.HasPrefix(data, []byte("ID3")) ||
				(len(data) >= 2 && data[0] == 0xFF && data[1]&0xE0 == 0xE0)
		},
	},
	{
		kind:        KindAudio,
		contentType: "audio/wav",
		extension:   ".wav",
		maxSize:     50 * 1024 * 1024,
		matcher: func(data []byte) bool {
			return len(data) >= 12 &&
				bytes.Equal(data[0:4], []byte("RIFF")) &&
				bytes.Equal(data[8:12], []byte("WAVE"))
		},
	},
	{
		kind:        KindAudio,
		contentType: "audio/mp4",
		extension:   ".m4a",
		maxSize:     50 * 1024 * 1024,
		matcher: func(data []byte) bool {
			return isISOBaseMedia(data, "M4A") || isISOBaseMedia(data, "mp42") || isISOBaseMedia(data, "isom")
		},
	},
	{
		kind:        KindVideo,
		contentType: "video/mp4",
		extension:   ".mp4",
		maxSize:     100 * 1024 * 1024,
		matcher: func(data []byte) bool {
			return isISOBaseMedia(data, "mp42") || isISOBaseMedia(data, "isom") || isISOBaseMedia(data, "avc1")
		},
	},
	{
		kind:        KindVideo,
		contentType: "video/x-msvideo",
		extension:   ".avi",
		maxSize:     100 * 1024 * 1024,
		matcher: func(data []byte) bool {
			return len(data) >= 12 &&
				bytes.Equal(data[0:4], []byte("RIFF")) &&
				bytes.Equal(data[8:12], []byte("AVI "))
		},
	},
}

var allowedExtensionsByKind = map[string]map[string]bool{
	KindPDF: {
		".pdf": true,
	},
	KindImage: {
		".jpg": true, ".jpeg": true, ".png": true, ".gif": true, ".bmp": true,
	},
	KindAudio: {
		".mp3": true, ".wav": true, ".pcm": true, ".m4a": true,
	},
	KindVideo: {
		".mp4": true, ".avi": true, ".mov": true, ".wmv": true,
	},
}

// Detect 只根据文件头魔数判断真实类型，不信任前端传来的 Content-Type。
func Detect(content []byte) (DetectedFile, error) {
	if len(content) == 0 {
		return DetectedFile{}, ErrEmptyFile
	}
	for _, sig := range signatures {
		if sig.matches(content) {
			return DetectedFile{
				Kind:        sig.kind,
				ContentType: sig.contentType,
				Extension:   sig.extension,
				MaxSize:     sig.maxSize,
			}, nil
		}
	}
	if looksLikePCM(content) {
		return DetectedFile{
			Kind:        KindAudio,
			ContentType: "audio/pcm",
			Extension:   ".pcm",
			MaxSize:     50 * 1024 * 1024,
		}, nil
	}
	return DetectedFile{}, ErrUnsupportedType
}

// Validate 在魔数检测后再校验大小和扩展名，避免把伪装成 PDF 的脚本或图片放进解析链路。
func Validate(content []byte, fileName string, allowedKinds ...string) (DetectedFile, error) {
	detected, err := Detect(content)
	if err != nil {
		return DetectedFile{}, err
	}
	if len(allowedKinds) > 0 && !containsKind(allowedKinds, detected.Kind) {
		return DetectedFile{}, ErrUnsupportedType
	}
	if int64(len(content)) > detected.MaxSize {
		return DetectedFile{}, ErrFileTooLarge
	}
	extension := strings.ToLower(filepath.Ext(fileName))
	if extension != "" {
		allowed := allowedExtensionsByKind[detected.Kind]
		if !allowed[extension] {
			return DetectedFile{}, ErrExtensionMismatch
		}
	}
	return detected, nil
}

func (sig signature) matches(content []byte) bool {
	if sig.matcher != nil {
		return sig.matcher(content)
	}
	for _, prefix := range sig.prefixes {
		if bytes.HasPrefix(content, prefix) {
			return true
		}
	}
	return false
}

func containsKind(kinds []string, kind string) bool {
	for _, allowed := range kinds {
		if allowed == kind {
			return true
		}
	}
	return false
}

func isISOBaseMedia(data []byte, brand string) bool {
	if len(data) < 12 || !bytes.Equal(data[4:8], []byte("ftyp")) {
		return false
	}
	return bytes.Contains(data[8:min(len(data), 64)], []byte(brand))
}

func looksLikePCM(data []byte) bool {
	if len(data) < 32 {
		return false
	}
	zeroCount := 0
	for _, value := range data[:min(len(data), 256)] {
		if value == 0 {
			zeroCount++
		}
	}
	return zeroCount < 240
}
