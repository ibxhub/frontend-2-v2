package storage

import (
	"context"
	"errors"
	"io"
	"time"
)

var ErrNotFound = errors.New("not found")

type Object struct {
	Name    string
	Size    int64
	ModTime time.Time
}

type Store interface {
	Put(ctx context.Context, name string, r io.Reader, size int64) error
	Get(ctx context.Context, name string) (io.ReadCloser, *Object, error)
	List(ctx context.Context) ([]Object, error)
}
