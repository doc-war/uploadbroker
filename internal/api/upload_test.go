package api

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"uploadBroker/internal/config"
	"uploadBroker/internal/hash"
	"uploadBroker/internal/metadata"
	"uploadBroker/internal/storage"
)

func newTestFixture(t *testing.T) (*config.Config, *metadata.Store, map[string]storage.Storage, string) {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	store, err := metadata.Open(dbPath)
	if err != nil {
		t.Fatalf("metadata: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	drivers := map[string]storage.Storage{
		"local": storage.NewLocalDriver(filepath.Join(dir, "objects")),
	}

	cfg := &config.Config{
		Listen:          "127.0.0.1:0",
		BaseURL:         "https://upload.example.com",
		URLBlake2bSalts: []string{"test-salt"},
		URLPrefix:       "tmp",
		MetadataDB:      dbPath,
		DefaultTTL:      86400000000000,
		Limits: config.Limits{
			Image: config.SizeBytes(2 << 20),
			Audio: config.SizeBytes(3 << 20),
			Video: config.SizeBytes(10 << 20),
		},
		Version: "1.0.0",
		Storage: config.StorageConfig{
			UploadDriver: "local",
		},
	}

	return cfg, store, drivers, dir
}

func pngData() []byte {
	return []byte{137, 80, 78, 71, 13, 10, 26, 10, 0, 0, 0, 13, 73, 72, 68, 82, 0, 0, 0, 1, 0, 0, 0, 1, 8, 2, 0, 0, 0, 144, 119, 83, 222, 0, 0, 0, 12, 73, 68, 65, 84, 8, 215, 99, 248, 207, 192, 0, 0, 0, 2, 0, 1, 226, 33, 58, 85, 0, 0, 0, 0, 73, 69, 78, 68, 174, 66, 96, 130}
}

func multipartUpload(t *testing.T, data []byte, filename string, expires string) *http.Request {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)

	fw, err := w.CreateFormFile("file", filename)
	if err != nil {
		t.Fatalf("CreateFormFile: %v", err)
	}
	if _, err := fw.Write(data); err != nil {
		t.Fatalf("Write file: %v", err)
	}

	if expires != "" {
		w.WriteField("expires", expires)
	}
	w.Close()

	r := httptest.NewRequest("POST", "/v1/upload", &buf)
	r.Header.Set("Content-Type", w.FormDataContentType())
	return r
}

func TestUploadSuccess(t *testing.T) {
	cfg, store, drv, _ := newTestFixture(t)
	h := NewUploadHandler(cfg, store, drv)

	r := multipartUpload(t, pngData(), "test.png", "")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Code int `json:"code"`
		Data struct {
			URL      string `json:"url"`
			MimeType string `json:"mimeType"`
			Size     int    `json:"size"`
			ExpireAt string `json:"expireAt"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if resp.Code != 0 {
		t.Fatalf("code = %d, want 0", resp.Code)
	}
	if resp.Data.MimeType != "image/png" {
		t.Fatalf("mimeType = %s, want image/png", resp.Data.MimeType)
	}
	if resp.Data.URL == "" {
		t.Fatal("url is empty")
	}
	if resp.Data.Size != len(pngData()) {
		t.Fatalf("size = %d, want %d", resp.Data.Size, len(pngData()))
	}
	if resp.Data.ExpireAt == "" {
		t.Fatal("expireAt is empty")
	}
}

func TestUploadIdempotent(t *testing.T) {
	cfg, store, drv, _ := newTestFixture(t)
	h := NewUploadHandler(cfg, store, drv)

	data := pngData()

	r1 := multipartUpload(t, data, "test.png", "")
	w1 := httptest.NewRecorder()
	h.ServeHTTP(w1, r1)

	var resp1 struct {
		Data struct {
			URL string `json:"url"`
		} `json:"data"`
	}
	json.Unmarshal(w1.Body.Bytes(), &resp1)

	r2 := multipartUpload(t, data, "test.png", "")
	w2 := httptest.NewRecorder()
	h.ServeHTTP(w2, r2)

	var resp2 struct {
		Data struct {
			URL string `json:"url"`
		} `json:"data"`
	}
	json.Unmarshal(w2.Body.Bytes(), &resp2)

	if resp1.Data.URL != resp2.Data.URL {
		t.Fatal("same content should get same URL")
	}
}

func TestUploadMissingFile(t *testing.T) {
	cfg, store, drv, _ := newTestFixture(t)
	h := NewUploadHandler(cfg, store, drv)

	r := httptest.NewRequest("POST", "/v1/upload", nil)
	r.Header.Set("Content-Type", "multipart/form-data; boundary=test")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	var resp struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Code != 40001 {
		t.Fatalf("code = %d, want 40001", resp.Code)
	}
}

func TestUploadEmptyFile(t *testing.T) {
	cfg, store, drv, _ := newTestFixture(t)
	h := NewUploadHandler(cfg, store, drv)

	r := multipartUpload(t, []byte{}, "empty.png", "")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	var resp struct {
		Code int `json:"code"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Code != 40002 {
		t.Fatalf("code = %d, want 40002", resp.Code)
	}
}

func TestUploadUnsupportedMIME(t *testing.T) {
	cfg, store, drv, _ := newTestFixture(t)
	h := NewUploadHandler(cfg, store, drv)

	r := multipartUpload(t, []byte("plain text"), "test.txt", "")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	var resp struct {
		Code int `json:"code"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Code != 40003 {
		t.Fatalf("code = %d, want 40003", resp.Code)
	}
}

func TestUploadFileTooLarge(t *testing.T) {
	cfg, store, drv, _ := newTestFixture(t)
	cfg.Limits.Image = config.SizeBytes(10)
	h := NewUploadHandler(cfg, store, drv)

	r := multipartUpload(t, pngData(), "test.png", "")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	var resp struct {
		Code int `json:"code"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Code != 40004 {
		t.Fatalf("code = %d, want 40004", resp.Code)
	}
}

func TestUploadInvalidExpires(t *testing.T) {
	cfg, store, drv, _ := newTestFixture(t)
	h := NewUploadHandler(cfg, store, drv)

	r := multipartUpload(t, pngData(), "test.png", "999")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	var resp struct {
		Code int `json:"code"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Code != 40005 {
		t.Fatalf("code = %d, want 40005", resp.Code)
	}
}

func TestUploadWithExpires(t *testing.T) {
	cfg, store, drv, _ := newTestFixture(t)
	h := NewUploadHandler(cfg, store, drv)

	r := multipartUpload(t, pngData(), "test.png", "2")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	var resp struct {
		Code int `json:"code"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Code != 0 {
		t.Fatalf("code = %d, want 0", resp.Code)
	}
}

func TestUploadMethodNotAllowed(t *testing.T) {
	cfg, store, drv, _ := newTestFixture(t)
	h := NewUploadHandler(cfg, store, drv)

	r := httptest.NewRequest("GET", "/v1/upload", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	var resp struct {
		Code int `json:"code"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Code != 40500 {
		t.Fatalf("code = %d, want 40500", resp.Code)
	}
}

func TestUploadHMACRequired(t *testing.T) {
	cfg, store, drv, _ := newTestFixture(t)
	cfg.HMACSecret = "my-secret"
	h := NewUploadHandler(cfg, store, drv)

	r := multipartUpload(t, pngData(), "test.png", "")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	var resp struct {
		Code int `json:"code"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Code != 50000 {
		t.Fatalf("code = %d, want 50000 (missing sign)", resp.Code)
	}
}

func TestUploadHMACValid(t *testing.T) {
	cfg, store, drv, _ := newTestFixture(t)
	cfg.HMACSecret = "my-secret"
	h := NewUploadHandler(cfg, store, drv)

	var buf bytes.Buffer
	bw := multipart.NewWriter(&buf)
	fw, _ := bw.CreateFormFile("file", "test.png")
	fw.Write(pngData())
	bw.WriteField("timestamp", "1782650347")
	bw.WriteField("sign", hash.HMACSign("my-secret", "1782650347"))
	bw.Close()

	r := httptest.NewRequest("POST", "/v1/upload", &buf)
	r.Header.Set("Content-Type", bw.FormDataContentType())
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	var resp struct {
		Code int `json:"code"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Code != 0 {
		t.Fatalf("code = %d, want 0, body: %s", resp.Code, w.Body.String())
	}
}

func TestUploadRealFile(t *testing.T) {
	cfg, store, drv, _ := newTestFixture(t)
	h := NewUploadHandler(cfg, store, drv)

	origPath := "../api/testdata/pixel.png"
	if _, err := os.Stat(origPath); os.IsNotExist(err) {
		os.MkdirAll(filepath.Dir(origPath), 0755)
		os.WriteFile(origPath, pngData(), 0644)
	}

	data, err := os.ReadFile(origPath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	r := multipartUpload(t, data, "pixel.png", "")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	var resp struct {
		Code int `json:"code"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Code != 0 {
		t.Fatalf("code = %d, want 0", resp.Code)
	}
}
