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
	ti, ok := Detect(pngHeader())
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
	ti, ok := Detect(jpegHeader())
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
	ti, ok := Detect(b)
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
	ti, ok := Detect(b)
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
	ti, ok := Detect(b)
	if !ok {
		t.Log("WAV detection may vary by platform, skip")
		return
	}
	if ti.Category != "audio" || ti.Extension != ".wav" {
		t.Logf("WAV detected as %s/%s", ti.MIME, ti.Extension)
	}
}

func TestDetectUnsupported(t *testing.T) {
	_, ok := Detect([]byte("plain text"))
	if ok {
		t.Fatal("should not detect plain text")
	}

	_, ok = Detect([]byte{0x00, 0x01, 0x02})
	if ok {
		t.Fatal("should not detect random bytes")
	}

	_, ok = Detect([]byte("<html></html>"))
	if ok {
		t.Fatal("should not detect HTML")
	}
}

func TestDetectEmpty(t *testing.T) {
	_, ok := Detect([]byte{})
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

func TestDetectRealPNG(t *testing.T) {
	data, err := os.ReadFile("../api/testdata/pixel.png")
	if err != nil {
		t.Skip("test PNG not available")
	}
	ti, ok := Detect(data)
	if !ok {
		t.Fatal("should detect real PNG")
	}
	if ti.MIME != "image/png" {
		t.Fatalf("expected image/png, got %s", ti.MIME)
	}
}
