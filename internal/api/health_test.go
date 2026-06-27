package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"uploadBroker/internal/metadata"
	"uploadBroker/internal/storage"
)

var errStorageDown = errors.New("storage is down")

type mockDriver struct {
	storage.Storage
	healthErr error
}

func (m *mockDriver) Health(ctx context.Context) error {
	return m.healthErr
}

func TestHealthOK(t *testing.T) {
	dir := t.TempDir()
	store, err := metadata.Open(dir + "/health.db")
	if err != nil {
		t.Fatalf("metadata: %v", err)
	}
	defer store.Close()

	drivers := map[string]storage.Storage{
		"local": &mockDriver{},
	}

	h := NewHealthHandler("1.0.0", store, drivers)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/v1/health", nil)
	h.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	var body map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if body["status"] != "ok" {
		t.Fatalf("status = %v, want ok", body["status"])
	}
	if body["version"] != "1.0.0" {
		t.Fatalf("version = %v, want 1.0.0", body["version"])
	}
	if body["storage"] != "ok" {
		t.Fatalf("storage = %v, want ok", body["storage"])
	}
	if body["sqlite"] != "ok" {
		t.Fatalf("sqlite = %v, want ok", body["sqlite"])
	}
	if _, ok := body["time"]; !ok {
		t.Fatal("time field missing")
	}
}

func TestHealthDegraded(t *testing.T) {
	dir := t.TempDir()
	store, err := metadata.Open(dir + "/degraded.db")
	if err != nil {
		t.Fatalf("metadata: %v", err)
	}
	defer store.Close()

	drivers := map[string]storage.Storage{
		"local": &mockDriver{healthErr: errStorageDown},
	}

	h := NewHealthHandler("1.0.0", store, drivers)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/v1/health", nil)
	h.ServeHTTP(w, r)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", w.Code)
	}

	var body map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &body)
	if body["status"] != "degraded" {
		t.Fatalf("status = %v, want degraded", body["status"])
	}
	if body["storage"] != "local: storage is down" {
		t.Fatalf("storage = %v, want 'local: storage is down'", body["storage"])
	}
}
