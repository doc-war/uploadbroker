package api

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/doc-war/uploadbroker/internal/config"
	"github.com/doc-war/uploadbroker/internal/metadata"
	"github.com/doc-war/uploadbroker/internal/storage"
)

func TestCORSMiddlewareSetsHeaders(t *testing.T) {
	handler := CORSMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/v1/health", nil)
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Fatalf("Access-Control-Allow-Origin = %q, want *", got)
	}
	if got := w.Header().Get("Access-Control-Allow-Methods"); got != "GET, POST, OPTIONS" {
		t.Fatalf("Access-Control-Allow-Methods = %q, want GET, POST, OPTIONS", got)
	}
	if got := w.Header().Get("Access-Control-Allow-Headers"); got == "" {
		t.Fatal("Access-Control-Allow-Headers missing")
	}
}

func TestCORSMiddlewarePreflight(t *testing.T) {
	handler := CORSMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("preflight should not reach inner handler")
	}))

	w := httptest.NewRecorder()
	r := httptest.NewRequest("OPTIONS", "/v1/upload", nil)
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", w.Code)
	}
	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Fatalf("Access-Control-Allow-Origin = %q, want *", got)
	}
}

func TestServerCORSDefaultEnabled(t *testing.T) {
	dir := t.TempDir()
	store, err := metadata.Open(filepath.Join(dir, "cors-default.db"))
	if err != nil {
		t.Fatalf("metadata: %v", err)
	}
	defer store.Close()

	drivers := map[string]storage.Storage{"local": &mockDriver{}}
	cfg := &config.Config{
		Listen:  "127.0.0.1:0",
		Version: "1.0.0",
		Storage: config.StorageConfig{UploadDriver: "local"},
	}

	_, handler, err := StartServer(cfg, store, drivers)
	if err != nil {
		t.Fatalf("StartServer: %v", err)
	}

	// 未配置 cors → 默认允许跨域
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/v1/health", nil)
	handler.ServeHTTP(w, r)
	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Fatalf("CORS header = %q, want * (default enabled)", got)
	}
}

func TestServerCORSDisabled(t *testing.T) {
	dir := t.TempDir()
	store, err := metadata.Open(filepath.Join(dir, "cors-off.db"))
	if err != nil {
		t.Fatalf("metadata: %v", err)
	}
	defer store.Close()

	drivers := map[string]storage.Storage{"local": &mockDriver{}}
	disabled := false
	cfg := &config.Config{
		Listen:  "127.0.0.1:0",
		Version: "1.0.0",
		CORS:    &disabled,
		Storage: config.StorageConfig{UploadDriver: "local"},
	}

	_, handler, err := StartServer(cfg, store, drivers)
	if err != nil {
		t.Fatalf("StartServer: %v", err)
	}

	// cors: false → 不添加 CORS 头
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/v1/health", nil)
	handler.ServeHTTP(w, r)
	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("CORS header = %q, want empty (disabled)", got)
	}
}