package mime

import (
	"mime"
	"net/http"
	"path/filepath"
	"strings"
)

type TypeInfo struct {
	MIME      string
	Extension string
	Category  string
}

// allowed 二进制魔数类型 + 文本类型（文本类型由 init 从 textExtToMIME 补全）。
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
	"application/pdf":              {"application/pdf", ".pdf", "document"},
	"application/zip":              {"application/zip", ".zip", "archive"},
	"application/x-7z-compressed":  {"application/x-7z-compressed", ".7z", "archive"},
	"application/x-rar-compressed": {"application/x-rar-compressed", ".rar", "archive"},
	"text/plain":                   {"text/plain", ".txt", "document"},
}

// textExtToMIME 文本/源码/配置文件白名单：每个扩展名映射独立 MIME，
// 保证 URL 扩展名与原文件名扩展名一致（一扩展名一 MIME）。
var textExtToMIME = map[string]string{
	".txt":   "text/plain",
	".md":    "text/markdown",
	".json":  "application/json",
	".yaml":  "application/yaml",
	".yml":   "text/yaml",
	".toml":  "application/toml",
	".ini":   "text/x-ini",
	".conf":  "text/x-config",
	".sh":    "text/x-shellscript",
	".bash":  "text/x-bash",
	".go":    "text/x-go",
	".js":    "text/javascript",
	".ts":    "text/typescript",
	".py":    "text/x-python",
	".java":  "text/x-java",
	".c":     "text/x-c",
	".h":     "text/x-ch",
	".cpp":   "text/x-c++",
	".hpp":   "text/x-cpp-header",
	".rs":    "text/x-rust",
	".rb":    "text/x-ruby",
	".php":   "text/x-php",
	".sql":   "text/x-sql",
	".csv":   "text/csv",
	".css":   "text/css",
}

// dangerousTextExts 检测为纯文本但禁止原样保留的扩展名（防存储型 XSS，
// HTML/SVG/XML 可内联执行脚本，浏览器解析同源 URL 时会渲染执行）。
var dangerousTextExts = map[string]bool{
	".html": true,
	".htm":  true,
	".svg":  true,
	".xml":  true,
}

var extToMIME = map[string]string{}

func init() {
	for ext, mt := range textExtToMIME {
		allowed[mt] = TypeInfo{MIME: mt, Extension: ext, Category: "document"}
		extToMIME[ext] = mt
	}
	for _, ti := range allowed {
		extToMIME[ti.Extension] = ti.MIME
	}
}

func Detect(filename string, data []byte) (*TypeInfo, bool) {
	if len(data) == 0 {
		return nil, false
	}
	// 手动魔数：WAV / 7z / RAR（Go 标准库 sniff 不识别或识别不准确）
	if len(data) >= 12 && string(data[0:4]) == "RIFF" && string(data[8:12]) == "WAVE" {
		ti := allowed["audio/wav"]
		return &ti, true
	}
	if len(data) >= 6 && string(data[0:6]) == "7z\xbc\xaf\x27\x1c" {
		ti := allowed["application/x-7z-compressed"]
		return &ti, true
	}
	if len(data) >= 6 && string(data[0:4]) == "Rar!" && data[4] == 0x1a && data[5] == 0x07 {
		ti := allowed["application/x-rar-compressed"]
		return &ti, true
	}

	raw := http.DetectContentType(data)
	mediatype, _, err := mime.ParseMediaType(raw)
	if err != nil {
		return nil, false
	}
	mediatype = strings.ToLower(mediatype)

	if mediatype == "text/plain" {
		return detectText(filename, data)
	}

	ti, ok := allowed[mediatype]
	if !ok {
		return nil, false
	}
	return &ti, true
}

// detectText 纯文本内容按原始扩展名识别；危险扩展名拒绝，未知扩展名回落 .txt。
func detectText(filename string, data []byte) (*TypeInfo, bool) {
	ext := strings.ToLower(filepath.Ext(filename))
	if dangerousTextExts[ext] {
		return nil, false
	}
	if mt, ok := textExtToMIME[ext]; ok {
		ti := TypeInfo{MIME: mt, Extension: ext, Category: "document"}
		return &ti, true
	}
	_ = data // 保留参数以兼容未来启发式检测
	ti := allowed["text/plain"]
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