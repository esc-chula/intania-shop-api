// Package storage implements external object storage adapters.
package storage

import (
	"context"
	"fmt"
	"io"
	"path"
	"strings"
	"time"

	"cloud.google.com/go/storage"
)

// Object is a successfully uploaded object.
type Object struct {
	URL        string `json:"url"`
	ObjectName string `json:"object_name"`
}

// Uploader stores one object under a configured logical folder.
type Uploader interface {
	Upload(context.Context, string, string, string, io.Reader) (Object, error)
}

// GCSUploader stores objects in Google Cloud Storage using application-default credentials.
type GCSUploader struct {
	client *storage.Client
	bucket string
}

// NewGCSUploader constructs a GCS adapter and validates the bucket name.
func NewGCSUploader(ctx context.Context, bucket string) (*GCSUploader, error) {
	if strings.TrimSpace(bucket) == "" {
		return nil, fmt.Errorf("GCS bucket is required")
	}
	client, err := storage.NewClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("create GCS client: %w", err)
	}
	return &GCSUploader{client: client, bucket: bucket}, nil
}

// Close releases underlying GCS resources.
func (u *GCSUploader) Close() error { return u.client.Close() }

// Upload streams an object to the configured bucket.
func (u *GCSUploader) Upload(ctx context.Context, folder, filename, contentType string, source io.Reader) (Object, error) {
	extension := path.Ext(filename)
	name := fmt.Sprintf("%s/%d-%s%s", strings.Trim(folder, "/"), time.Now().UTC().UnixNano(), sanitize(filename), extension)
	writer := u.client.Bucket(u.bucket).Object(name).NewWriter(ctx)
	writer.ContentType = contentType
	if _, err := io.Copy(writer, source); err != nil {
		_ = writer.Close()
		return Object{}, fmt.Errorf("write object: %w", err)
	}
	if err := writer.Close(); err != nil {
		return Object{}, fmt.Errorf("close object: %w", err)
	}
	return Object{URL: "https://storage.googleapis.com/" + u.bucket + "/" + name, ObjectName: name}, nil
}
func sanitize(value string) string {
	base := strings.TrimSuffix(path.Base(value), path.Ext(value))
	return strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_' {
			return r
		}
		return '_'
	}, base)
}
