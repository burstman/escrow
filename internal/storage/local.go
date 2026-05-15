package storage

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

type LocalStorage struct {
	baseDir string
}

func NewLocalStorage(baseDir string) (*LocalStorage, error) {
	abs, err := filepath.Abs(baseDir)
	if err != nil {
		return nil, fmt.Errorf("resolve upload dir: %w", err)
	}
	if err := os.MkdirAll(abs, 0755); err != nil {
		return nil, fmt.Errorf("create upload dir: %w", err)
	}
	return &LocalStorage{baseDir: abs}, nil
}

func (s *LocalStorage) Upload(ctx context.Context, key string, r io.Reader) (string, error) {
	path := filepath.Join(s.baseDir, key)
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("create dir: %w", err)
	}

	f, err := os.Create(path)
	if err != nil {
		return "", fmt.Errorf("create file: %w", err)
	}
	defer f.Close()

	h := sha256.New()
	tr := io.TeeReader(r, h)
	if _, err := io.Copy(f, tr); err != nil {
		return "", fmt.Errorf("write file: %w", err)
	}

	hash := fmt.Sprintf("%x", h.Sum(nil))
	return hash, nil
}

func (s *LocalStorage) Download(ctx context.Context, key string) (io.ReadCloser, error) {
	path := filepath.Join(s.baseDir, key)
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open file: %w", err)
	}
	return f, nil
}

func (s *LocalStorage) Delete(ctx context.Context, key string) error {
	path := filepath.Join(s.baseDir, key)
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("delete file: %w", err)
	}
	return nil
}

func (s *LocalStorage) URL(key string) string {
	return "/files/storage/" + key
}
