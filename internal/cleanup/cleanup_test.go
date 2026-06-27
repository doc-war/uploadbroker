package cleanup

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"uploadBroker/internal/config"
	"uploadBroker/internal/metadata"
	"uploadBroker/internal/storage"
)

func TestCleanupExpiredRecords(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "cleanup.db")

	store, err := metadata.Open(dbPath)
	if err != nil {
		t.Fatalf("metadata: %v", err)
	}
	defer store.Close()

	drivers := map[string]storage.Storage{
		"local": storage.NewLocalDriver(filepath.Join(dir, "objects")),
	}

	now := time.Now().Unix()

	notExpired := &metadata.Record{
		Key:        "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		MIMEType:   "image/png",
		Size:       100,
		Backend:    "local",
		StorageKey: "",
		CreatedAt:  now,
		ExpireAt:   now + 3600,
	}

	expired := &metadata.Record{
		Key:        "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		MIMEType:   "image/png",
		Size:       200,
		Backend:    "local",
		StorageKey: "",
		CreatedAt:  now - 7200,
		ExpireAt:   now - 3600,
	}

	obj1 := &storage.Object{
		Key:       notExpired.Key,
		Data:      []byte("not expired"),
		MIMEType:  "text/plain",
		Extension: ".txt",
	}
	r1, _ := drivers["local"].Put(context.Background(), obj1)
	notExpired.StorageKey = r1.StorageKey
	store.Insert(notExpired)

	obj2 := &storage.Object{
		Key:       expired.Key,
		Data:      []byte("expired data"),
		MIMEType:  "text/plain",
		Extension: ".txt",
	}
	r2, _ := drivers["local"].Put(context.Background(), obj2)
	expired.StorageKey = r2.StorageKey
	store.Insert(expired)

	cfg := &config.Config{
		CleanupInterval: 100 * time.Millisecond,
	}

	task := New(cfg, store, drivers)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	go task.Start(ctx)

	time.Sleep(500 * time.Millisecond)

	got, _ := store.Get(expired.Key)
	if got != nil {
		t.Fatal("expired record should have been deleted")
	}

	got2, _ := store.Get(notExpired.Key)
	if got2 == nil {
		t.Fatal("non-expired record should still exist")
	}
}

func TestCleanupNoExpired(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "noop.db")

	store, err := metadata.Open(dbPath)
	if err != nil {
		t.Fatalf("metadata: %v", err)
	}
	defer store.Close()

	drivers := map[string]storage.Storage{
		"local": storage.NewLocalDriver(filepath.Join(dir, "objects")),
	}

	cfg := &config.Config{
		CleanupInterval: 24 * time.Hour,
	}

	task := New(cfg, store, drivers)

	task.runOnce()
}
