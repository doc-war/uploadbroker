package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWriteJSON(t *testing.T) {
	w := httptest.NewRecorder()
	resp := &Response{Code: 0, Data: map[string]string{"key": "val"}}
	writeJSON(w, http.StatusOK, resp)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	ct := w.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Fatalf("Content-Type = %s, want application/json", ct)
	}

	var decoded Response
	if err := json.Unmarshal(w.Body.Bytes(), &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if decoded.Code != 0 {
		t.Fatalf("code = %d, want 0", decoded.Code)
	}
}

func TestWriteError(t *testing.T) {
	w := httptest.NewRecorder()
	writeError(w, 40003, "unsupported type")

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}

	var resp Response
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Code != 40003 {
		t.Fatalf("code = %d, want 40003", resp.Code)
	}
	if resp.Msg != "unsupported type" {
		t.Fatalf("msg = %s, want 'unsupported type'", resp.Msg)
	}
}

func TestWriteError500(t *testing.T) {
	w := httptest.NewRecorder()
	writeError(w, 50000, "internal error")

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", w.Code)
	}
}

func TestWriteError404(t *testing.T) {
	w := httptest.NewRecorder()
	writeError(w, 40401, "not found")

	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", w.Code)
	}

	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if resp.Code != 40401 {
		t.Fatalf("code = %d, want 40401", resp.Code)
	}
}

func TestWriteSuccess(t *testing.T) {
	w := httptest.NewRecorder()
	data := map[string]string{"key": "value"}
	writeSuccess(w, data)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	var resp Response
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Code != 0 {
		t.Fatalf("code = %d, want 0", resp.Code)
	}
}

func TestLoggingMiddleware(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("not found"))
	})

	wrapped := LoggingMiddleware(handler)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/test", nil)
	wrapped.ServeHTTP(w, r)

	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", w.Code)
	}
}

func TestLoggingResponseWriter(t *testing.T) {
	lrw := &loggingResponseWriter{
		ResponseWriter: httptest.NewRecorder(),
		statusCode:     http.StatusOK,
	}
	lrw.WriteHeader(http.StatusTeapot)

	if lrw.statusCode != http.StatusTeapot {
		t.Fatalf("statusCode = %d, want 418", lrw.statusCode)
	}
}
