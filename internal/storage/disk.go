package storage

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sort"
)

type Disk struct {
	Dir string
}

func NewDisk(dir string) (*Disk, error) {
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, err
	}
	return &Disk{Dir: dir}, nil
}

func (d *Disk) Put(_ context.Context, name string, r io.Reader, _ int64) error {
	dst, err := os.OpenFile(filepath.Join(d.Dir, name), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o640)
	if err != nil {
		return err
	}
	defer dst.Close()
	_, err = io.Copy(dst, r)
	return err
}

func (d *Disk) Get(_ context.Context, name string) (io.ReadCloser, *Object, error) {
	path := filepath.Join(d.Dir, name)
	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil, ErrNotFound
		}
		return nil, nil, err
	}
	info, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, nil, err
	}
	return f, &Object{Name: name, Size: info.Size(), ModTime: info.ModTime()}, nil
}

func (d *Disk) List(_ context.Context) ([]Object, error) {
	entries, err := os.ReadDir(d.Dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	out := make([]Object, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		out = append(out, Object{Name: e.Name(), Size: info.Size(), ModTime: info.ModTime()})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ModTime.After(out[j].ModTime) })
	return out, nil
}
