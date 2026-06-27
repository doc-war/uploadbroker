package storage

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type LocalDriver struct {
	name string
	root string
}

func NewLocalDriver(name, root string) *LocalDriver {
	return &LocalDriver{name: name, root: root}
}

func (d *LocalDriver) Put(ctx context.Context, obj *Object) (*PutResult, error) {
	dateDir := time.Now().UTC().Format("20060102")
	shard := obj.Key[:2]
	storageRel := fmt.Sprintf("%s/%s/%s%s", dateDir, shard, obj.Key, obj.Extension)
	storageAbs := filepath.Join(d.root, storageRel)

	if err := os.MkdirAll(filepath.Dir(storageAbs), 0755); err != nil {
		return nil, fmt.Errorf("mkdir: %w", err)
	}
	if err := os.WriteFile(storageAbs, obj.Data, 0644); err != nil {
		return nil, fmt.Errorf("write: %w", err)
	}

	return &PutResult{
		Backend:    d.name,
		StorageKey: storageRel,
	}, nil
}

func (d *LocalDriver) Get(ctx context.Context, storageKey string) (*GetResult, error) {
	abs := filepath.Join(d.root, storageKey)

	if !isSafePath(d.root, abs) {
		return nil, fmt.Errorf("path traversal detected: %s", storageKey)
	}

	f, err := os.Open(abs)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("not found: %s", storageKey)
		}
		return nil, fmt.Errorf("open: %w", err)
	}

	info, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, fmt.Errorf("stat: %w", err)
	}

	return &GetResult{
		Data: f,
		Size: info.Size(),
	}, nil
}

func (d *LocalDriver) Delete(ctx context.Context, storageKey string) error {
	abs := filepath.Join(d.root, storageKey)
	if !isSafePath(d.root, abs) {
		return fmt.Errorf("path traversal detected: %s", storageKey)
	}
	if err := os.Remove(abs); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("delete: %w", err)
	}
	return nil
}

func (d *LocalDriver) Health(ctx context.Context) error {
	if err := os.MkdirAll(d.root, 0755); err != nil {
		return fmt.Errorf("root: %w", err)
	}
	return nil
}

func isSafePath(root, abs string) bool {
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return false
	}
	fileAbs, err := filepath.Abs(abs)
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(rootAbs, fileAbs)
	if err != nil {
		return false
	}
	return len(rel) >= 2 && rel[:2] != ".."
}
