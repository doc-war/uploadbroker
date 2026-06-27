package storage

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestLocalPutGet(t *testing.T) {
	dir := t.TempDir()
	drv := NewLocalDriver(dir)

	ctx := context.Background()
	obj := &Object{
		Key:       "a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2",
		Data:      []byte("hello world"),
		MIMEType:  "text/plain",
		Extension: ".txt",
	}

	result, err := drv.Put(ctx, obj)
	if err != nil {
		t.Fatalf("Put: %v", err)
	}

	if result.Backend != "local" {
		t.Fatalf("expected backend local, got %s", result.Backend)
	}

	if result.StorageKey == "" {
		t.Fatal("storage_key must not be empty")
	}

	gr, err := drv.Get(ctx, result.StorageKey)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	defer gr.Data.Close()

	data, err := io.ReadAll(gr.Data)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}

	if string(data) != "hello world" {
		t.Fatalf("expected 'hello world', got '%s'", string(data))
	}

	if gr.Size != 11 {
		t.Fatalf("expected size 11, got %d", gr.Size)
	}
}

func TestLocalPutCreatesDateDir(t *testing.T) {
	dir := t.TempDir()
	drv := NewLocalDriver(dir)

	ctx := context.Background()
	obj := &Object{
		Key:       "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		Data:      []byte("test"),
		MIMEType:  "text/plain",
		Extension: ".txt",
	}

	result, err := drv.Put(ctx, obj)
	if err != nil {
		t.Fatalf("Put: %v", err)
	}

	abs := filepath.Join(dir, result.StorageKey)
	if _, err := os.Stat(abs); os.IsNotExist(err) {
		t.Fatalf("file not created at %s", abs)
	}
}

func TestLocalGetNotFound(t *testing.T) {
	dir := t.TempDir()
	drv := NewLocalDriver(dir)

	ctx := context.Background()
	_, err := drv.Get(ctx, "nonexistent/20260627/ab/somekey.txt")
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}

func TestLocalGetPathTraversal(t *testing.T) {
	dir := t.TempDir()
	drv := NewLocalDriver(dir)

	ctx := context.Background()
	_, err := drv.Get(ctx, "../../etc/passwd")
	if err == nil {
		t.Fatal("expected error for path traversal")
	}
}

func TestLocalDelete(t *testing.T) {
	dir := t.TempDir()
	drv := NewLocalDriver(dir)

	ctx := context.Background()
	obj := &Object{
		Key:       "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc",
		Data:      []byte("delete me"),
		MIMEType:  "text/plain",
		Extension: ".txt",
	}

	result, err := drv.Put(ctx, obj)
	if err != nil {
		t.Fatalf("Put: %v", err)
	}

	if err := drv.Delete(ctx, result.StorageKey); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	abs := filepath.Join(dir, result.StorageKey)
	if _, err := os.Stat(abs); !os.IsNotExist(err) {
		t.Fatal("file should be deleted")
	}
}

func TestLocalDeleteNotFound(t *testing.T) {
	dir := t.TempDir()
	drv := NewLocalDriver(dir)
	if err := drv.Delete(context.Background(), "nonexistent/file.txt"); err != nil {
		t.Fatal("delete nonexistent should not error")
	}
}

func TestLocalDeletePathTraversal(t *testing.T) {
	dir := t.TempDir()
	drv := NewLocalDriver(dir)
	err := drv.Delete(context.Background(), "../../etc/passwd")
	if err == nil {
		t.Fatal("expected error for path traversal")
	}
}

func TestLocalHealth(t *testing.T) {
	dir := t.TempDir()
	drv := NewLocalDriver(filepath.Join(dir, "objects"))

	if err := drv.Health(context.Background()); err != nil {
		t.Fatalf("Health: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, "objects")); os.IsNotExist(err) {
		t.Fatal("Health should create root dir")
	}
}

func TestIsSafePath(t *testing.T) {
	root := "/data/objects"

	tests := []struct {
		abs  string
		safe bool
	}{
		{"/data/objects/20260627/ab/file.txt", true},
		{"/data/objects/20260627/ab/", true},
		{"/data/objects/../etc/passwd", false},
		{"/etc/passwd", false},
	}

	for _, tt := range tests {
		got := isSafePath(root, tt.abs)
		if got != tt.safe {
			t.Errorf("isSafePath(%q, %q) = %v, want %v", root, tt.abs, got, tt.safe)
		}
	}
}

func TestLocalPutGetLargeFile(t *testing.T) {
	dir := t.TempDir()
	drv := NewLocalDriver(dir)

	largeData := make([]byte, 1024*1024)
	for i := range largeData {
		largeData[i] = byte(i % 256)
	}

	key := "dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd"

	ctx := context.Background()
	result, err := drv.Put(ctx, &Object{
		Key:       key,
		Data:      largeData,
		MIMEType:  "application/octet-stream",
		Extension: ".bin",
	})
	if err != nil {
		t.Fatalf("Put large: %v", err)
	}

	gr, err := drv.Get(ctx, result.StorageKey)
	if err != nil {
		t.Fatalf("Get large: %v", err)
	}
	defer gr.Data.Close()

	read, err := io.ReadAll(gr.Data)
	if err != nil {
		t.Fatalf("ReadAll large: %v", err)
	}

	if int64(len(read)) != gr.Size {
		t.Fatalf("size mismatch: read %d, reported %d", len(read), gr.Size)
	}
}
