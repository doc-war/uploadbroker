package storage

import (
	"bytes"
	"context"
	"fmt"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type S3Driver struct {
	name   string
	client *minio.Client
	bucket string
}

func NewS3Driver(name, endpoint, bucket, region, accessKey, secretKey string, secure bool) (*S3Driver, error) {
	if endpoint == "" || bucket == "" || accessKey == "" || secretKey == "" {
		return nil, fmt.Errorf("s3 driver %s: endpoint, bucket, access_key_id, secret_access_key are required", name)
	}
	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: secure,
		Region: region,
	})
	if err != nil {
		return nil, fmt.Errorf("s3 client %s: %w", name, err)
	}
	return &S3Driver{name: name, client: client, bucket: bucket}, nil
}

func (d *S3Driver) buildKey(obj *Object) string {
	dateDir := time.Now().UTC().Format("20060102")
	shard := obj.Key[:2]
	return fmt.Sprintf("%s/%s/%s%s", dateDir, shard, obj.Key, obj.Extension)
}

func (d *S3Driver) Put(ctx context.Context, obj *Object) (*PutResult, error) {
	storageKey := d.buildKey(obj)
	_, err := d.client.PutObject(ctx, d.bucket, storageKey, bytes.NewReader(obj.Data), int64(len(obj.Data)), minio.PutObjectOptions{
		ContentType:          obj.MIMEType,
		DisableContentSha256: true,
	})
	if err != nil {
		return nil, fmt.Errorf("s3 put %s/%s: %w", d.bucket, storageKey, err)
	}
	return &PutResult{
		Backend:    d.name,
		StorageKey: storageKey,
	}, nil
}

func (d *S3Driver) Get(ctx context.Context, storageKey string) (*GetResult, error) {
	stat, err := d.client.StatObject(ctx, d.bucket, storageKey, minio.StatObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("s3 stat %s/%s: %w", d.bucket, storageKey, err)
	}

	obj, err := d.client.GetObject(ctx, d.bucket, storageKey, minio.GetObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("s3 get %s/%s: %w", d.bucket, storageKey, err)
	}

	return &GetResult{
		Data: obj,
		Size: stat.Size,
	}, nil
}

func (d *S3Driver) Delete(ctx context.Context, storageKey string) error {
	err := d.client.RemoveObject(ctx, d.bucket, storageKey, minio.RemoveObjectOptions{})
	if err != nil {
		return fmt.Errorf("s3 delete %s/%s: %w", d.bucket, storageKey, err)
	}
	return nil
}

func (d *S3Driver) Health(ctx context.Context) error {
	exists, err := d.client.BucketExists(ctx, d.bucket)
	if err != nil {
		return fmt.Errorf("s3 health %s: %w", d.bucket, err)
	}
	if !exists {
		return fmt.Errorf("s3 health %s: bucket %q does not exist", d.bucket, d.bucket)
	}
	return nil
}
