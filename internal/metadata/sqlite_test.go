package metadata

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "test.db")
	s, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { s.Close(); os.Remove(path) })
	return s
}

func makeRecord(key string, expireOffset int64) *Record {
	now := time.Now().Unix()
	return &Record{
		Key:        key,
		MIMEType:   "image/png",
		Size:       1024,
		Backend:    "local",
		StorageKey: "20260627/4d/" + key + ".png",
		CreatedAt:  now,
		ExpireAt:   now + expireOffset,
	}
}

func TestOpenClose(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.db")
	s, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

func TestInsertAndGet(t *testing.T) {
	s := newTestStore(t)

	rec := makeRecord("key1", 3600)
	if err := s.Insert(rec); err != nil {
		t.Fatalf("Insert: %v", err)
	}

	got, err := s.Get("key1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got == nil {
		t.Fatal("Get returned nil")
	}
	if got.Key != rec.Key {
		t.Fatalf("Key = %s, want %s", got.Key, rec.Key)
	}
	if got.MIMEType != rec.MIMEType {
		t.Fatalf("MIMEType = %s, want %s", got.MIMEType, rec.MIMEType)
	}
	if got.Size != rec.Size {
		t.Fatalf("Size = %d, want %d", got.Size, rec.Size)
	}
	if got.Backend != rec.Backend {
		t.Fatalf("Backend = %s, want %s", got.Backend, rec.Backend)
	}
	if got.StorageKey != rec.StorageKey {
		t.Fatalf("StorageKey = %s, want %s", got.StorageKey, rec.StorageKey)
	}
}

func TestGetNotFound(t *testing.T) {
	s := newTestStore(t)
	got, err := s.Get("nonexistent")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got != nil {
		t.Fatal("expected nil for nonexistent key")
	}
}

func TestInsertReplace(t *testing.T) {
	s := newTestStore(t)

	rec1 := makeRecord("key_replace", 3600)
	rec1.Size = 100
	if err := s.Insert(rec1); err != nil {
		t.Fatalf("Insert first: %v", err)
	}

	rec2 := makeRecord("key_replace", 7200)
	rec2.Size = 200
	if err := s.Insert(rec2); err != nil {
		t.Fatalf("Insert second: %v", err)
	}

	got, err := s.Get("key_replace")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Size != 200 {
		t.Fatalf("Size = %d, want 200 (replaced)", got.Size)
	}
}

func TestDelete(t *testing.T) {
	s := newTestStore(t)
	rec := makeRecord("delete_me", 3600)
	if err := s.Insert(rec); err != nil {
		t.Fatalf("Insert: %v", err)
	}
	if err := s.Delete("delete_me"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	got, _ := s.Get("delete_me")
	if got != nil {
		t.Fatal("should be deleted")
	}
}

func TestListExpired(t *testing.T) {
	s := newTestStore(t)

	records := []*Record{
		makeRecord("expired1", -100),
		makeRecord("expired2", -50),
		makeRecord("valid1", 3600),
		makeRecord("valid2", 7200),
	}

	for _, r := range records {
		if err := s.Insert(r); err != nil {
			t.Fatalf("Insert: %v", err)
		}
	}

	now := time.Now().Unix()
	expired, err := s.ListExpired(now)
	if err != nil {
		t.Fatalf("ListExpired: %v", err)
	}

	if len(expired) != 2 {
		t.Fatalf("expected 2 expired, got %d", len(expired))
	}
}

func TestDeleteByStorageKeyPrefix(t *testing.T) {
	s := newTestStore(t)

	recs := []*Record{
		makeRecord("a1", 3600),
		makeRecord("a2", 3600),
		makeRecord("b1", 3600),
	}
	recs[0].StorageKey = "20260627/aa/file1.png"
	recs[1].StorageKey = "20260627/aa/file2.png"
	recs[2].StorageKey = "20260628/bb/file3.png"

	for _, r := range recs {
		if err := s.Insert(r); err != nil {
			t.Fatalf("Insert: %v", err)
		}
	}

	if err := s.DeleteByStorageKeyPrefix("local", "20260627/"); err != nil {
		t.Fatalf("DeleteByStorageKeyPrefix: %v", err)
	}

	got1, _ := s.Get("a1")
	got2, _ := s.Get("a2")
	got3, _ := s.Get("b1")

	if got1 != nil {
		t.Fatal("a1 should be deleted")
	}
	if got2 != nil {
		t.Fatal("a2 should be deleted")
	}
	if got3 == nil {
		t.Fatal("b1 should still exist")
	}
}

func TestGetKeyTotalSize(t *testing.T) {
	s := newTestStore(t)

	recs := []*Record{
		makeRecord("s1", 3600),
		makeRecord("s2", 3600),
		makeRecord("s3", 3600),
	}
	recs[0].Size = 1000
	recs[1].Size = 2000
	recs[2].Size = 3000

	for _, r := range recs {
		if err := s.Insert(r); err != nil {
			t.Fatalf("Insert: %v", err)
		}
	}

	total, err := s.GetKeyTotalSize()
	if err != nil {
		t.Fatalf("GetKeyTotalSize: %v", err)
	}
	if total != 6000 {
		t.Fatalf("expected total 6000, got %d", total)
	}
}

func TestGetRecordCount(t *testing.T) {
	s := newTestStore(t)

	for i := 0; i < 5; i++ {
		key := "cnt_" + string(rune('a'+i))
		rec := makeRecord(key, 3600)
		if err := s.Insert(rec); err != nil {
			t.Fatalf("Insert: %v", err)
		}
	}

	count, err := s.GetRecordCount()
	if err != nil {
		t.Fatalf("GetRecordCount: %v", err)
	}
	if count != 5 {
		t.Fatalf("expected 5, got %d", count)
	}
}

func TestHealth(t *testing.T) {
	s := newTestStore(t)
	if err := s.Health(); err != nil {
		t.Fatalf("Health: %v", err)
	}
}
