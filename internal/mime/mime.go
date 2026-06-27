package mime

import (
	"mime"
	"net/http"
	"strings"
)

type TypeInfo struct {
	MIME      string
	Extension string
	Category  string
}

var allowed = map[string]TypeInfo{
	"image/png":  {"image/png", ".png", "image"},
	"image/jpeg": {"image/jpeg", ".jpg", "image"},
	"image/webp": {"image/webp", ".webp", "image"},
	"audio/mpeg": {"audio/mpeg", ".mp3", "audio"},
	"audio/mp3":  {"audio/mp3", ".mp3", "audio"},
	"audio/wav":  {"audio/wav", ".wav", "audio"},
	"audio/mp4":  {"audio/mp4", ".m4a", "audio"},
	"audio/aac":  {"audio/aac", ".aac", "audio"},
	"video/mp4":  {"video/mp4", ".mp4", "video"},
	"video/webm": {"video/webm", ".webm", "video"},
	"text/plain": {"text/plain", ".txt", "document"},
	"application/pdf":     {"application/pdf", ".pdf", "document"},
}

var extToMIME = map[string]string{}

func init() {
	for _, ti := range allowed {
		extToMIME[ti.Extension] = ti.MIME
	}
}

func Detect(data []byte) (*TypeInfo, bool) {
	if len(data) == 0 {
		return nil, false
	}
	raw := http.DetectContentType(data)
	mediatype, _, err := mime.ParseMediaType(raw)
	if err != nil {
		return nil, false
	}
	mediatype = strings.ToLower(mediatype)

	ti, ok := allowed[mediatype]
	if !ok {
		return nil, false
	}
	return &ti, true
}

func ExtensionFromMIME(mimeType string) string {
	if ti, ok := allowed[mimeType]; ok {
		return ti.Extension
	}
	exts, err := mime.ExtensionsByType(mimeType)
	if err != nil || len(exts) == 0 {
		return ".bin"
	}
	return exts[0]
}

// MIMEFromExtension looks up the expected MIME type for a file extension.
// Returns empty string if the extension is not in the allowed map.
func MIMEFromExtension(ext string) string {
	return extToMIME[ext]
}

// ValidateExtension checks whether the filename's extension matches the
// detected MIME type. Returns true if the extension is unknown or matches.
func ValidateExtension(filename, detectedMIME string) bool {
	idx := strings.LastIndex(filename, ".")
	if idx < 0 {
		return true // no extension, skip check
	}
	ext := filename[idx:]
	expected := extToMIME[ext]
	if expected == "" {
		return true // unknown extension, skip check
	}
	return expected == detectedMIME
}
