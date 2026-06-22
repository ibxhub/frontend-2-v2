package storage

import (
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

type fakeS3 struct {
	mu      sync.Mutex
	bucket  string
	objects map[string][]byte
	mod     map[string]time.Time
}

func newFakeS3(bucket string) *fakeS3 {
	return &fakeS3{bucket: bucket, objects: map[string][]byte{}, mod: map[string]time.Time{}}
}

func (f *fakeS3) handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()

		prefix := "/" + f.bucket
		if !strings.HasPrefix(r.URL.Path, prefix) {
			http.NotFound(w, r)
			return
		}
		key := strings.TrimPrefix(strings.TrimPrefix(r.URL.Path, prefix), "/")

		switch r.Method {
		case http.MethodPut:
			body, _ := io.ReadAll(r.Body)
			f.objects[key] = body
			f.mod[key] = time.Now()
			w.WriteHeader(http.StatusOK)
		case http.MethodGet:
			if r.URL.Query().Get("list-type") == "2" || key == "" {
				f.writeList(w)
				return
			}
			body, ok := f.objects[key]
			if !ok {
				w.WriteHeader(http.StatusNotFound)
				_, _ = w.Write([]byte(`<?xml version="1.0"?><Error><Code>NoSuchKey</Code></Error>`))
				return
			}
			w.Header().Set("Content-Length", fmt.Sprintf("%d", len(body)))
			w.Header().Set("Last-Modified", f.mod[key].UTC().Format(http.TimeFormat))
			_, _ = w.Write(body)
		default:
			http.Error(w, "not implemented", http.StatusNotImplemented)
		}
	})
}

type listEntry struct {
	XMLName      xml.Name `xml:"Contents"`
	Key          string   `xml:"Key"`
	Size         int64    `xml:"Size"`
	LastModified string   `xml:"LastModified"`
}

type listResp struct {
	XMLName  xml.Name    `xml:"ListBucketResult"`
	Name     string      `xml:"Name"`
	Contents []listEntry `xml:"Contents"`
}

func (f *fakeS3) writeList(w http.ResponseWriter) {
	resp := listResp{Name: f.bucket}
	for k, v := range f.objects {
		resp.Contents = append(resp.Contents, listEntry{
			Key:          k,
			Size:         int64(len(v)),
			LastModified: f.mod[k].UTC().Format(time.RFC3339),
		})
	}
	w.Header().Set("Content-Type", "application/xml")
	_, _ = w.Write([]byte(xml.Header))
	_ = xml.NewEncoder(w).Encode(resp)
}

func newTestS3(t *testing.T) (*S3, *fakeS3) {
	t.Helper()
	t.Setenv("AWS_ACCESS_KEY_ID", "test")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "test")
	t.Setenv("AWS_REGION", "us-east-1")

	fake := newFakeS3("test-bucket")
	srv := httptest.NewServer(fake.handler())
	t.Cleanup(srv.Close)

	endpoint := strings.TrimPrefix(srv.URL, "http://")
	s, err := NewS3WithEndpoint(context.Background(), "us-east-1", "test-bucket", "http://"+endpoint)
	if err != nil {
		t.Fatalf("NewS3WithEndpoint: %v", err)
	}
	s.disableKMS = true
	return s, fake
}

func TestS3_PutGet(t *testing.T) {
	s, _ := newTestS3(t)
	ctx := context.Background()

	if err := s.Put(ctx, "hello.txt", bytes.NewReader([]byte("hello s3")), int64(len("hello s3"))); err != nil {
		t.Fatalf("Put: %v", err)
	}
	rc, obj, err := s.Get(ctx, "hello.txt")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	defer rc.Close()
	got, _ := io.ReadAll(rc)
	if string(got) != "hello s3" {
		t.Errorf("body = %q, want hello s3", got)
	}
	if obj.Name != "hello.txt" {
		t.Errorf("Name = %q, want hello.txt", obj.Name)
	}
}

func TestS3_List(t *testing.T) {
	s, _ := newTestS3(t)
	ctx := context.Background()

	if err := s.Put(ctx, "a.txt", bytes.NewReader([]byte("a")), 1); err != nil {
		t.Fatalf("Put a: %v", err)
	}
	if err := s.Put(ctx, "b.txt", bytes.NewReader([]byte("bb")), 2); err != nil {
		t.Fatalf("Put b: %v", err)
	}

	list, err := s.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 2 {
		t.Errorf("List len = %d, want 2", len(list))
	}
}

func TestS3_GetMissing(t *testing.T) {
	s, _ := newTestS3(t)
	_, _, err := s.Get(context.Background(), "nope.txt")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestNewS3_RequiresBucket(t *testing.T) {
	_, err := NewS3(context.Background(), "us-east-1", "")
	if err == nil {
		t.Error("NewS3 with empty bucket should fail")
	}
}
