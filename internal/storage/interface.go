package storage

import (
	"context"
	"io"
)

type Object struct {
	Key       string
	Data      []byte
	MIMEType  string
	Extension string
}

type PutResult struct {
	Backend    string
	StorageKey string
}

type GetResult struct {
	Data      io.ReadCloser
	MIMEType  string
	Size      int64
}

type Storage interface {
	Put(ctx context.Context, obj *Object) (*PutResult, error)
	Get(ctx context.Context, storageKey string) (*GetResult, error)
	Delete(ctx context.Context, storageKey string) error
	Health(ctx context.Context) error
}
