package storage

import (
	"context"
	"io"
)

type FileStorage interface {
	Upload(ctx context.Context, key string, r io.Reader) (sha256hash string, err error)
	Download(ctx context.Context, key string) (io.ReadCloser, error)
	Delete(ctx context.Context, key string) error
	URL(key string) string
}
