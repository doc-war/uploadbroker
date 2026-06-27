package storage

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// fakeS3Server serves a minimal S3-compatible API for testing.
type fakeS3Server struct {
	bucket  string
	objects map[string][]byte
}

func newFakeS3Server(bucket string) *fakeS3Server {
	return &fakeS3Server{
		bucket:  bucket,
		objects: make(map[string][]byte),
	}
}

func (f *fakeS3Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	key := r.URL.Path
	if len(key) > 0 && key[0] == '/' {
		key = key[1:]
	}
	// Strip trailing slash so "/bucket/" and "/bucket" both match.
	key = strings.TrimSuffix(key, "/")

	// Write a helper to return S3 errors
	s3Error := func(code, msg string, status int) {
		w.Header().Set("Content-Type", "application/xml")
		w.WriteHeader(status)
		fmt.Fprintf(w, `<?xml version="1.0" encoding="UTF-8"?><Error><Code>%s</Code><Message>%s</Message></Error>`, code, msg)
	}

	// GetBucketLocation: GET /{bucket}?location=
	if _, ok := r.URL.Query()["location"]; ok {
		if key == f.bucket {
			w.Header().Set("Content-Type", "application/xml")
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, `<?xml version="1.0" encoding="UTF-8"?><LocationConstraint xmlns="http://s3.amazonaws.com/doc/2006-03-01/"></LocationConstraint>`)
			return
		}
		s3Error("NoSuchBucket", "The specified bucket does not exist.", http.StatusNotFound)
		return
	}

	switch r.Method {
	case http.MethodHead:
		if key == f.bucket {
			w.WriteHeader(http.StatusOK)
			return
		}
		objKey := key
		if len(objKey) > len(f.bucket)+1 && objKey[:len(f.bucket)] == f.bucket && objKey[len(f.bucket)] == '/' {
			objKey = objKey[len(f.bucket)+1:]
		}
		if data, ok := f.objects[objKey]; ok {
			w.Header().Set("Content-Length", fmt.Sprintf("%d", len(data)))
			w.Header().Set("Content-Type", "application/octet-stream")
			w.Header().Set("Last-Modified", time.Now().UTC().Format(http.TimeFormat))
			w.WriteHeader(http.StatusOK)
			return
		}
		s3Error("NoSuchKey", "The specified key does not exist.", http.StatusNotFound)
		return

	case http.MethodGet:
		if len(key) > len(f.bucket)+1 && key[:len(f.bucket)] == f.bucket && key[len(f.bucket)] == '/' {
			objKey := key[len(f.bucket)+1:]
			data, ok := f.objects[objKey]
			if !ok {
				s3Error("NoSuchKey", "The specified key does not exist.", http.StatusNotFound)
				return
			}
			w.Header().Set("Content-Length", fmt.Sprintf("%d", len(data)))
			w.Header().Set("Content-Type", "application/octet-stream")
			w.Header().Set("ETag", fmt.Sprintf("\"%x\"", data))
			w.Header().Set("Last-Modified", time.Now().UTC().Format(http.TimeFormat))
			w.WriteHeader(http.StatusOK)
			w.Write(data)
			return
		}
		s3Error("NoSuchBucket", "The specified bucket does not exist.", http.StatusNotFound)

	case http.MethodPut:
		if len(key) > len(f.bucket)+1 && key[:len(f.bucket)] == f.bucket && key[len(f.bucket)] == '/' {
			objKey := key[len(f.bucket)+1:]
			data, err := io.ReadAll(r.Body)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			f.objects[objKey] = data
			w.Header().Set("ETag", fmt.Sprintf("\"%x\"", data))
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, `<UploadResult><ETag>"%x"</ETag></UploadResult>`, data)
			return
		}
		s3Error("NoSuchBucket", "The specified bucket does not exist.", http.StatusNotFound)

	case http.MethodDelete:
		if len(key) > len(f.bucket)+1 && key[:len(f.bucket)] == f.bucket && key[len(f.bucket)] == '/' {
			objKey := key[len(f.bucket)+1:]
			delete(f.objects, objKey)
			w.WriteHeader(http.StatusNoContent)
			return
		}
		s3Error("NoSuchBucket", "The specified bucket does not exist.", http.StatusNotFound)

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func TestS3Driver_PutGetDeleteHealth(t *testing.T) {
	fake := newFakeS3Server("testbucket")
	srv := httptest.NewServer(fake)
	defer srv.Close()

	drv, err := NewS3Driver("tests3", srv.Listener.Addr().String(), "testbucket", "", "minioadmin", "minioadmin", false)
	if err != nil {
		t.Fatalf("NewS3Driver: %v", err)
	}

	ctx := context.Background()

	// Health
	if err := drv.Health(ctx); err != nil {
		t.Fatalf("Health: %v", err)
	}

	// Put
	obj := &Object{
		Key:       "abc123def456abc123def456abc123def456abc123def456abc123def456ab",
		Data:      []byte("hello s3 world"),
		MIMEType:  "text/plain",
		Extension: ".txt",
	}
	result, err := drv.Put(ctx, obj)
	if err != nil {
		t.Fatalf("Put: %v", err)
	}
	if result.Backend != "tests3" {
		t.Fatalf("expected backend tests3, got %s", result.Backend)
	}
	if result.StorageKey == "" {
		t.Fatal("expected non-empty StorageKey")
	}

	// Get
	getResult, err := drv.Get(ctx, result.StorageKey)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	data, err := io.ReadAll(getResult.Data)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	getResult.Data.Close()
	if getResult.Size != int64(len(obj.Data)) {
		t.Fatalf("expected size %d, got %d", len(obj.Data), getResult.Size)
	}
	if string(data) != string(obj.Data) {
		t.Fatalf("expected %q, got %q", string(obj.Data), string(data))
	}

	// Delete
	if err := drv.Delete(ctx, result.StorageKey); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	// Get after delete
	_, err = drv.Get(ctx, result.StorageKey)
	if err == nil {
		t.Fatal("expected error after delete")
	}
}

func TestS3Driver_New_MissingConfig(t *testing.T) {
	_, err := NewS3Driver("bad", "", "", "", "", "", false)
	if err == nil {
		t.Fatal("expected error for missing config")
	}
}

func TestS3Driver_Health_MissingBucket(t *testing.T) {
	fake := newFakeS3Server("otherbucket")
	srv := httptest.NewServer(fake)
	defer srv.Close()

	drv, err := NewS3Driver("tests3", srv.Listener.Addr().String(), "missingbucket", "", "minioadmin", "minioadmin", false)
	if err != nil {
		t.Fatalf("NewS3Driver: %v", err)
	}

	ctx := context.Background()
	err = drv.Health(ctx)
	if err == nil {
		t.Fatal("expected health error for missing bucket")
	}
	if !strings.Contains(err.Error(), "does not exist") {
		t.Fatalf("unexpected health error: %v", err)
	}
}
