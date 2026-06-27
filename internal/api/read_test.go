package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"uploadBroker/internal/config"
	"uploadBroker/internal/hash"
	"uploadBroker/internal/metadata"
	"uploadBroker/internal/storage"
)

func setupReadTest(t *testing.T) (*config.Config, *metadata.Store, map[string]storage.Storage, *ReadHandler) {
	t.Helper()
	dir := t.TempDir()

	store, err := metadata.Open(dir + "/read.db")
	if err != nil {
		t.Fatalf("metadata: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	drivers := map[string]storage.Storage{
		"local": storage.NewLocalDriver("local", dir+"/objects"),
	}

	cfg := &config.Config{
		BaseURL:         "https://upload.example.com",
		URLBlake2bSalts: []string{"test-salt", "old-salt"},
		URLPrefix:       "tmp",
	}

	rec := &metadata.Record{
		Key:        "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		MIMEType:   "image/png",
		Size:       int64(len(pngData())),
		Backend:    "local",
		StorageKey: "",
		CreatedAt:  time.Now().Unix(),
		ExpireAt:   time.Now().Add(24 * time.Hour).Unix(),
	}

	obj := &storage.Object{
		Key:       rec.Key,
		Data:      pngData(),
		MIMEType:  rec.MIMEType,
		Extension: ".png",
	}

	result, err := drivers["local"].Put(context.Background(), obj)
	if err != nil {
		t.Fatalf("Put: %v", err)
	}
	rec.StorageKey = result.StorageKey

	if err := store.Insert(rec); err != nil {
		t.Fatalf("Insert: %v", err)
	}

	return cfg, store, drivers, NewReadHandler(cfg, store, drivers)
}

func makeReadURL(key, ext string, expireUnix int64, salt string) string {
	token := hash.SignToken(salt, key, expireUnix)
	shard := key[:2]
	return "/tmp/" + itoa(expireUnix) + "/" + shard + "/" + key + "/" + token + ext
}

func itoa(n int64) string {
	if n == 0 {
		return "0"
	}
	s := ""
	for n > 0 {
		s = string(rune('0'+n%10)) + s
		n /= 10
	}
	return s
}

func TestReadSuccess(t *testing.T) {
	_, _, _, h := setupReadTest(t)

	expire := time.Now().Add(24 * time.Hour).Unix()
	url := makeReadURL("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", ".png", expire, "test-salt")

	r := httptest.NewRequest("GET", url, nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body: %s", w.Code, w.Body.String())
	}

	ct := w.Header().Get("Content-Type")
	if ct != "image/png" {
		t.Fatalf("Content-Type = %s, want image/png", ct)
	}

	cl := w.Header().Get("Content-Length")
	if cl == "" {
		t.Fatal("Content-Length is empty")
	}

	if w.Body.Len() != len(pngData()) {
		t.Fatalf("body size = %d, want %d", w.Body.Len(), len(pngData()))
	}
}

func TestReadWithOldSalt(t *testing.T) {
	_, _, _, h := setupReadTest(t)

	expire := time.Now().Add(24 * time.Hour).Unix()
	url := makeReadURL("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", ".png", expire, "old-salt")

	r := httptest.NewRequest("GET", url, nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (old salt should work)", w.Code)
	}
}

func TestReadBadToken(t *testing.T) {
	_, _, _, h := setupReadTest(t)

	expire := time.Now().Add(24 * time.Hour).Unix()
	key := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	shard := key[:2]
	url := "/tmp/" + itoa(expire) + "/" + shard + "/" + key + "/badtoken12345678.png"

	r := httptest.NewRequest("GET", url, nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	var resp struct {
		Code int `json:"code"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Code != 40401 {
		t.Fatalf("code = %d, want 40401", resp.Code)
	}
}

func TestReadExpired(t *testing.T) {
	_, _, _, h := setupReadTest(t)

	expire := time.Now().Add(-1 * time.Hour).Unix()
	url := makeReadURL("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", ".png", expire, "test-salt")

	r := httptest.NewRequest("GET", url, nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	var resp struct {
		Code int `json:"code"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Code != 40402 {
		t.Fatalf("code = %d, want 40402", resp.Code)
	}
}

func TestReadInvalidURL(t *testing.T) {
	_, _, _, h := setupReadTest(t)

	tests := []string{
		"/tmp/abc",
		"/tmp/123/ab/key/token.jpg",
		"/tmp/123/ab/",
		"/invalid",
	}

	for _, url := range tests {
		r := httptest.NewRequest("GET", url, nil)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)

		var resp struct {
			Code int `json:"code"`
		}
		json.Unmarshal(w.Body.Bytes(), &resp)
		if resp.Code != 40401 {
			t.Fatalf("url=%s: code = %d, want 40401", url, resp.Code)
		}
	}
}

func TestReadMethodNotAllowed(t *testing.T) {
	_, _, _, h := setupReadTest(t)

	r := httptest.NewRequest("POST", "/tmp/123/ab/key/token.png", nil)
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

func TestReadNotFound(t *testing.T) {
	cfg, _, _, h := setupReadTest(t)

	key := "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	expire := time.Now().Add(24 * time.Hour).Unix()
	url := makeReadURL(key, ".png", expire, cfg.URLBlake2bSalts[0])

	r := httptest.NewRequest("GET", url, nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	var resp struct {
		Code int `json:"code"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Code != 40402 {
		t.Fatalf("code = %d, want 40402", resp.Code)
	}
}
