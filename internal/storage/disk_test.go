package storage

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"
)

func TestDisk_PutGetList(t *testing.T) {
	dir := t.TempDir()
	d, err := NewDisk(dir)
	if err != nil {
		t.Fatalf("NewDisk: %v", err)
	}
	ctx := context.Background()

	if err := d.Put(ctx, "a.txt", bytes.NewReader([]byte("aaa")), 3); err != nil {
		t.Fatalf("Put a: %v", err)
	}
	if err := d.Put(ctx, "b.txt", bytes.NewReader([]byte("bbbb")), 4); err != nil {
		t.Fatalf("Put b: %v", err)
	}

	rc, obj, err := d.Get(ctx, "a.txt")
	if err != nil {
		t.Fatalf("Get a.txt: %v", err)
	}
	defer rc.Close()
	got, _ := io.ReadAll(rc)
	if string(got) != "aaa" {
		t.Errorf("Get content = %q, want aaa", got)
	}
	if obj.Size != 3 {
		t.Errorf("Get size = %d, want 3", obj.Size)
	}

	list, err := d.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 2 {
		t.Errorf("List len = %d, want 2", len(list))
	}
}

func TestDisk_GetMissingReturnsErrNotFound(t *testing.T) {
	d, err := NewDisk(t.TempDir())
	if err != nil {
		t.Fatalf("NewDisk: %v", err)
	}
	_, _, err = d.Get(context.Background(), "nope.txt")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestDisk_ListEmpty(t *testing.T) {
	d, err := NewDisk(t.TempDir())
	if err != nil {
		t.Fatalf("NewDisk: %v", err)
	}
	list, err := d.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 0 {
		t.Errorf("len = %d, want 0", len(list))
	}
}
