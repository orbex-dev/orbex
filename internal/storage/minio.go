package storage

import (
	"context"
	"fmt"
	"io"
	"log"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// ObjectInfo represents metadata about a stored object.
type ObjectInfo struct {
	Key          string
	Size         int64
	ContentType  string
	LastModified string
}

// Client wraps the MinIO client for object storage operations.
type Client struct {
	minio  *minio.Client
	bucket string
}

// New creates a new MinIO storage client and ensures the bucket exists.
func New(endpoint, accessKey, secretKey, bucket string, useSSL bool) (*Client, error) {
	mc, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: useSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("minio client: %w", err)
	}

	c := &Client{minio: mc, bucket: bucket}

	if err := c.EnsureBucket(context.Background()); err != nil {
		return nil, fmt.Errorf("minio bucket: %w", err)
	}

	return c, nil
}

// EnsureBucket creates the bucket if it doesn't exist.
func (c *Client) EnsureBucket(ctx context.Context) error {
	exists, err := c.minio.BucketExists(ctx, c.bucket)
	if err != nil {
		return fmt.Errorf("check bucket: %w", err)
	}
	if !exists {
		if err := c.minio.MakeBucket(ctx, c.bucket, minio.MakeBucketOptions{}); err != nil {
			return fmt.Errorf("create bucket: %w", err)
		}
		log.Printf("Created bucket: %s", c.bucket)
	}
	return nil
}

// Upload stores an object in MinIO.
func (c *Client) Upload(ctx context.Context, key string, reader io.Reader, size int64, contentType string) error {
	opts := minio.PutObjectOptions{}
	if contentType != "" {
		opts.ContentType = contentType
	}
	_, err := c.minio.PutObject(ctx, c.bucket, key, reader, size, opts)
	if err != nil {
		return fmt.Errorf("upload %s: %w", key, err)
	}
	return nil
}

// Download retrieves an object from MinIO.
func (c *Client) Download(ctx context.Context, key string) (io.ReadCloser, error) {
	obj, err := c.minio.GetObject(ctx, c.bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("download %s: %w", key, err)
	}
	return obj, nil
}

// List returns all objects under a given prefix.
func (c *Client) List(ctx context.Context, prefix string) ([]ObjectInfo, error) {
	var objects []ObjectInfo
	for obj := range c.minio.ListObjects(ctx, c.bucket, minio.ListObjectsOptions{
		Prefix:    prefix,
		Recursive: true,
	}) {
		if obj.Err != nil {
			return nil, fmt.Errorf("list %s: %w", prefix, obj.Err)
		}
		objects = append(objects, ObjectInfo{
			Key:          obj.Key,
			Size:         obj.Size,
			ContentType:  obj.ContentType,
			LastModified: obj.LastModified.Format("2006-01-02T15:04:05Z"),
		})
	}
	return objects, nil
}

// Delete removes an object from MinIO.
func (c *Client) Delete(ctx context.Context, key string) error {
	err := c.minio.RemoveObject(ctx, c.bucket, key, minio.RemoveObjectOptions{})
	if err != nil {
		return fmt.Errorf("delete %s: %w", key, err)
	}
	return nil
}

// DeletePrefix removes all objects under a given prefix.
func (c *Client) DeletePrefix(ctx context.Context, prefix string) error {
	objects, err := c.List(ctx, prefix)
	if err != nil {
		return err
	}
	for _, obj := range objects {
		if err := c.Delete(ctx, obj.Key); err != nil {
			return err
		}
	}
	return nil
}
