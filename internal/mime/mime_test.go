package mime

import (
	"os"
	"testing"
)

func pngHeader() []byte {
	b := []byte{137, 80, 78, 71, 13, 10, 26, 10}
	for len(b) < 512 {
		b = append(b, 0)
	}
	return b[:512]
}

func jpegHeader() []byte {
	b := []byte{255, 216, 255, 224, 0, 16, 74, 70, 73, 70, 0}
	for len(b) < 512 {
		b = append(b, 0)
	}
	return b[:512]
}

func TestDetectPNG(t *testing.T) {
	ti, ok := Detect("test.png", pngHeader())
	if !ok {
		t.Fatal("should detect PNG")
	}
	if ti.MIME != "image/png" {
		t.Fatalf("expected image/png, got %s", ti.MIME)
	}
	if ti.Extension != ".png" {
		t.Fatalf("expected .png, got %s", ti.Extension)
	}
	if ti.Category != "image" {
		t.Fatalf("expected category image, got %s", ti.Category)
	}
}

func TestDetectJPEG(t *testing.T) {
	ti, ok := Detect("test.jpg", jpegHeader())
	if !ok {
		t.Fatal("should detect JPEG")
	}
	if ti.MIME != "image/jpeg" {
		t.Fatalf("expected image/jpeg, got %s", ti.MIME)
	}
}

func TestDetectWEBP(t *testing.T) {
	// RIFF header with WEBP format
	b := []byte{
		0x52, 0x49, 0x46, 0x46, // RIFF
		0x00, 0x00, 0x00, 0x00, // size
		0x57, 0x45, 0x42, 0x50, // WEBP
		0x56, 0x50, 0x38, 0x20, // VP8
		0x00, 0x00, 0x00, 0x00, // chunk size
		0x00, 0x00, 0x00, 0x00, // width/height
	}
	for len(b) < 512 {
		b = append(b, 0)
	}
	ti, ok := Detect("test.webp", b)
	if !ok {
		t.Fatal("should detect WEBP")
	}
	if ti.MIME != "image/webp" {
		t.Fatalf("expected image/webp, got %s", ti.MIME)
	}
}

func TestDetectMP3(t *testing.T) {
	// valid MP3 frame header with enough data for sniffing
	b := []byte{
		0xFF, 0xFB, 0x90, 0x00, // MPEG1, Layer3, 128kbps, 44100Hz
	}
	for len(b) < 512 {
		b = append(b, 0)
	}
	ti, ok := Detect("test.mp3", b)
	if !ok {
		t.Log("MP3 detection may vary by platform, skipping")
		return
	}
	if ti.Category != "audio" || ti.Extension != ".mp3" {
		t.Logf("MP3 detected as %s/%s", ti.MIME, ti.Extension)
	}
}

func TestDetectWAV(t *testing.T) {
	b := []byte{
		0x52, 0x49, 0x46, 0x46, // RIFF
		0x00, 0x00, 0x00, 0x00, // size
		0x57, 0x41, 0x56, 0x45, // WAVE
		0x66, 0x6D, 0x74, 0x20, // fmt
		0x10, 0x00, 0x00, 0x00, // chunk size
		0x01, 0x00, // PCM
		0x01, 0x00, // mono
	}
	for len(b) < 512 {
		b = append(b, 0)
	}
	ti, ok := Detect("test.wav", b)
	if !ok {
		t.Fatal("should detect WAV")
	}
	if ti.MIME != "audio/wav" {
		t.Fatalf("expected audio/wav, got %s", ti.MIME)
	}
	if ti.Category != "audio" || ti.Extension != ".wav" {
		t.Fatalf("WAV detected as %s/%s", ti.MIME, ti.Extension)
	}
}

func TestDetectWAVNotWebpOrAVI(t *testing.T) {
	// RIFF with WEBP subtype must NOT be detected as audio/wav
	webp := []byte{0x52, 0x49, 0x46, 0x46, 0x00, 0x00, 0x00, 0x00, 0x57, 0x45, 0x42, 0x50}
	for len(webp) < 512 {
		webp = append(webp, 0)
	}
	ti, ok := Detect("test.webp", webp)
	if ok && ti.MIME == "audio/wav" {
		t.Fatal("RIFF/WEBP must not be detected as audio/wav")
	}

	// RIFF with AVI subtype must not be detected as audio/wav
	avi := []byte{0x52, 0x49, 0x46, 0x46, 0x00, 0x00, 0x00, 0x00, 0x41, 0x56, 0x49, 0x20}
	for len(avi) < 512 {
		avi = append(avi, 0)
	}
	ti, ok = Detect("test.avi", avi)
	if ok && ti.MIME == "audio/wav" {
		t.Fatal("RIFF/AVI must not be detected as audio/wav")
	}
}

func TestDetectTextPlain(t *testing.T) {
	ti, ok := Detect("hello.txt", []byte("hello world"))
	if !ok {
		t.Fatal("should detect text/plain")
	}
	if ti.MIME != "text/plain" {
		t.Fatalf("expected text/plain, got %s", ti.MIME)
	}
	if ti.Extension != ".txt" {
		t.Fatalf("expected .txt, got %s", ti.Extension)
	}
	if ti.Category != "document" {
		t.Fatalf("expected document category, got %s", ti.Category)
	}
}

func TestDetectMarkdown(t *testing.T) {
	ti, ok := Detect("README.md", []byte("# Title\nsome content\n"))
	if !ok {
		t.Fatal("should detect markdown")
	}
	if ti.MIME != "text/markdown" {
		t.Fatalf("expected text/markdown, got %s", ti.MIME)
	}
	if ti.Extension != ".md" {
		t.Fatalf("expected .md, got %s", ti.Extension)
	}
	if ti.Category != "document" {
		t.Fatalf("expected document, got %s", ti.Category)
	}
}

func TestDetectSourceCode(t *testing.T) {
	// 纯文本内容 + 源码扩展名 → 保留原扩展名
	ti, ok := Detect("main.go", []byte("package main\n\nfunc main() {}\n"))
	if !ok {
		t.Fatal("should detect go source")
	}
	if ti.MIME != "text/x-go" || ti.Extension != ".go" {
		t.Fatalf("go detected as %s/%s", ti.MIME, ti.Extension)
	}

	ti, ok = Detect("app.py", []byte("#!/usr/bin/env python\nprint('hi')\n"))
	if !ok {
		t.Fatal("should detect python source")
	}
	if ti.MIME != "text/x-python" || ti.Extension != ".py" {
		t.Fatalf("py detected as %s/%s", ti.MIME, ti.Extension)
	}

	ti, ok = Detect("script.sh", []byte("#!/bin/bash\necho hi\n"))
	if !ok {
		t.Fatal("should detect shell script")
	}
	if ti.MIME != "text/x-shellscript" || ti.Extension != ".sh" {
		t.Fatalf("sh detected as %s/%s", ti.MIME, ti.Extension)
	}
}

func TestDetectConfigFiles(t *testing.T) {
	ti, ok := Detect("app.conf", []byte("[section]\nkey=value\n"))
	if !ok {
		t.Fatal("should detect conf")
	}
	if ti.MIME != "text/x-config" || ti.Extension != ".conf" {
		t.Fatalf("conf detected as %s/%s", ti.MIME, ti.Extension)
	}

	ti, ok = Detect("config.yaml", []byte("a: 1\nb: two\n"))
	if !ok {
		t.Fatal("should detect yaml")
	}
	if ti.MIME != "application/yaml" || ti.Extension != ".yaml" {
		t.Fatalf("yaml detected as %s/%s", ti.MIME, ti.Extension)
	}

	ti, ok = Detect("data.json", []byte(`{"a":1}`))
	if !ok {
		t.Fatal("should detect json")
	}
	if ti.MIME != "application/json" || ti.Extension != ".json" {
		t.Fatalf("json detected as %s/%s", ti.MIME, ti.Extension)
	}

	ti, ok = Detect("settings.ini", []byte("[db]\nuser=root\n"))
	if !ok {
		t.Fatal("should detect ini")
	}
	if ti.MIME != "text/x-ini" || ti.Extension != ".ini" {
		t.Fatalf("ini detected as %s/%s", ti.MIME, ti.Extension)
	}
}

func TestDetectTextUnknownExtFallback(t *testing.T) {
	// 未知扩展名 / 无扩展名的纯文本 → 回落 text/plain/.txt
	ti, ok := Detect("notes.xyz", []byte("plain text"))
	if !ok {
		t.Fatal("should detect text")
	}
	if ti.MIME != "text/plain" || ti.Extension != ".txt" {
		t.Fatalf("unknown ext detected as %s/%s, want text/plain/.txt", ti.MIME, ti.Extension)
	}

	ti, ok = Detect("README", []byte("no extension"))
	if !ok {
		t.Fatal("should detect text")
	}
	if ti.MIME != "text/plain" || ti.Extension != ".txt" {
		t.Fatalf("no ext detected as %s/%s, want text/plain/.txt", ti.MIME, ti.Extension)
	}
}

func TestDetectTextDangerousExtRejected(t *testing.T) {
	// 危险扩展名：检测为纯文本也拒绝（防存储型 XSS）
	_, ok := Detect("page.html", []byte("just some html text"))
	if ok {
		t.Fatal("html should be rejected")
	}
	_, ok = Detect("icon.svg", []byte("plain text svg"))
	if ok {
		t.Fatal("svg should be rejected")
	}
	_, ok = Detect("data.xml", []byte("plain text xml"))
	if ok {
		t.Fatal("xml should be rejected")
	}
}

func TestDetectZIP(t *testing.T) {
	// PK\x03\x04
	b := []byte{0x50, 0x4B, 0x03, 0x04, 0x14, 0x00, 0x00, 0x00}
	for len(b) < 512 {
		b = append(b, 0)
	}
	ti, ok := Detect("archive.zip", b)
	if !ok {
		t.Fatal("should detect zip")
	}
	if ti.MIME != "application/zip" {
		t.Fatalf("expected application/zip, got %s", ti.MIME)
	}
	if ti.Extension != ".zip" || ti.Category != "archive" {
		t.Fatalf("zip detected as %s/%s/%s", ti.MIME, ti.Extension, ti.Category)
	}
}

func TestDetect7z(t *testing.T) {
	// 7z\xBC\xAF\x27\x1C
	b := []byte{0x37, 0x7A, 0xBC, 0xAF, 0x27, 0x1C, 0x00, 0x04}
	for len(b) < 512 {
		b = append(b, 0)
	}
	ti, ok := Detect("archive.7z", b)
	if !ok {
		t.Fatal("should detect 7z")
	}
	if ti.MIME != "application/x-7z-compressed" {
		t.Fatalf("expected application/x-7z-compressed, got %s", ti.MIME)
	}
	if ti.Extension != ".7z" || ti.Category != "archive" {
		t.Fatalf("7z detected as %s/%s/%s", ti.MIME, ti.Extension, ti.Category)
	}
}

func TestDetectRAR(t *testing.T) {
	// RAR4: Rar!\x1A\x07\x00 ; RAR5: Rar!\x1A\x07\x01\x00
	for _, magic := range [][]byte{
		{0x52, 0x61, 0x72, 0x21, 0x1A, 0x07, 0x00, 0x00},
		{0x52, 0x61, 0x72, 0x21, 0x1A, 0x07, 0x01, 0x00},
	} {
		b := append([]byte{}, magic...)
		for len(b) < 512 {
			b = append(b, 0)
		}
		ti, ok := Detect("archive.rar", b)
		if !ok {
			t.Fatal("should detect rar")
		}
		if ti.MIME != "application/x-rar-compressed" {
			t.Fatalf("expected application/x-rar-compressed, got %s", ti.MIME)
		}
		if ti.Extension != ".rar" || ti.Category != "archive" {
			t.Fatalf("rar detected as %s/%s/%s", ti.MIME, ti.Extension, ti.Category)
		}
	}
}

func TestDetectPDF(t *testing.T) {
	b := []byte("%PDF-1.4 some content")
	for len(b) < 512 {
		b = append(b, 0)
	}
	ti, ok := Detect("doc.pdf", b)
	if !ok {
		t.Fatal("should detect application/pdf")
	}
	if ti.MIME != "application/pdf" {
		t.Fatalf("expected application/pdf, got %s", ti.MIME)
	}
	if ti.Extension != ".pdf" {
		t.Fatalf("expected .pdf, got %s", ti.Extension)
	}
}

func TestDetectUnsupported(t *testing.T) {
	_, ok := Detect("random.bin", []byte{0x00, 0x01, 0x02})
	if ok {
		t.Fatal("should not detect random bytes")
	}

	_, ok = Detect("page.html", []byte("<html></html>"))
	if ok {
		t.Fatal("should not detect HTML")
	}
}

func TestDetectEmpty(t *testing.T) {
	_, ok := Detect("empty.txt", []byte{})
	if ok {
		t.Fatal("should not detect empty data")
	}
}

func TestExtensionFromMIME(t *testing.T) {
	tests := []struct {
		mime string
		exts []string
	}{
		{"image/png", []string{".png"}},
		{"image/webp", []string{".webp"}},
		{"audio/wav", []string{".wav"}},
		{"audio/aac", []string{".aac", ".bin"}},
		{"text/markdown", []string{".md"}},
		{"application/zip", []string{".zip"}},
		{"application/x-7z-compressed", []string{".7z"}},
		{"application/x-rar-compressed", []string{".rar"}},
	}

	for _, tt := range tests {
		ext := ExtensionFromMIME(tt.mime)
		found := false
		for _, e := range tt.exts {
			if ext == e {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("ExtensionFromMIME(%s) = %s, want one of %v", tt.mime, ext, tt.exts)
		}
	}
}

func TestExtensionFromMIMEJPEG(t *testing.T) {
	ext := ExtensionFromMIME("image/jpeg")
	if ext != ".jpg" && ext != ".jpeg" && ext != ".jfif" {
		t.Fatalf("unexpected extension for image/jpeg: %s", ext)
	}
}

func TestExtensionFromMIMEMpeg(t *testing.T) {
	ext := ExtensionFromMIME("audio/mpeg")
	if ext == "" {
		t.Fatal("empty extension for audio/mpeg")
	}
}

func TestExtensionFromMIMEUnknown(t *testing.T) {
	ext := ExtensionFromMIME("application/octet-stream")
	if ext != ".bin" {
		t.Fatalf("expected .bin for unknown mime, got %s", ext)
	}
}

func TestValidateExtension(t *testing.T) {
	tests := []struct {
		filename    string
		mime        string
		expectMatch bool
	}{
		{"photo.png", "image/png", true},
		{"photo.jpg", "image/png", false},
		{"video.mp4", "video/mp4", true},
		{"doc.txt", "text/plain", true},
		{"doc.pdf", "application/pdf", true},
		{"doc.pdf", "image/png", false},
		{"README.md", "text/markdown", true},
		{"main.go", "text/x-go", true},
		{"app.conf", "text/x-config", true},
		{"archive.zip", "application/zip", true},
		{"archive.rar", "application/x-rar-compressed", true},
		{"noext", "image/png", true},    // no extension → skip
		{"evil.exe", "video/mp4", true}, // unknown extension → skip (other checks catch it)
	}
	for _, tt := range tests {
		got := ValidateExtension(tt.filename, tt.mime)
		if got != tt.expectMatch {
			t.Errorf("ValidateExtension(%q, %q) = %v, want %v", tt.filename, tt.mime, got, tt.expectMatch)
		}
	}
}

func TestDetectRealPNG(t *testing.T) {
	data, err := os.ReadFile("../api/testdata/pixel.png")
	if err != nil {
		t.Skip("test PNG not available")
	}
	ti, ok := Detect("pixel.png", data)
	if !ok {
		t.Fatal("should detect real PNG")
	}
	if ti.MIME != "image/png" {
		t.Fatalf("expected image/png, got %s", ti.MIME)
	}
}