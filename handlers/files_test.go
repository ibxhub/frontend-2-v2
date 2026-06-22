package handlers

import (
	"bytes"
	"context"
	"errors"
	"html/template"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"frontend-2-v2/internal/storage"
)

func testTmpl(t *testing.T) *template.Template {
	t.Helper()
	return template.Must(template.New("base.html").Parse(
		`{{.Title}}|files={{range .Files}}{{.Name}},{{end}}|msg={{.Message}}|err={{.Error}}`,
	))
}

type memStore struct {
	mu      map[string][]byte
	getErr  error
	putErr  error
	listErr error
}

func newMemStore() *memStore { return &memStore{mu: map[string][]byte{}} }

func (m *memStore) Put(_ context.Context, name string, r io.Reader, _ int64) error {
	if m.putErr != nil {
		return m.putErr
	}
	b, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	m.mu[name] = b
	return nil
}

func (m *memStore) Get(_ context.Context, name string) (io.ReadCloser, *storage.Object, error) {
	if m.getErr != nil {
		return nil, nil, m.getErr
	}
	b, ok := m.mu[name]
	if !ok {
		return nil, nil, storage.ErrNotFound
	}
	return io.NopCloser(bytes.NewReader(b)), &storage.Object{Name: name, Size: int64(len(b)), ModTime: time.Now()}, nil
}

func (m *memStore) List(_ context.Context) ([]storage.Object, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	out := make([]storage.Object, 0, len(m.mu))
	for name, b := range m.mu {
		out = append(out, storage.Object{Name: name, Size: int64(len(b)), ModTime: time.Now()})
	}
	return out, nil
}


func TestSafeName(t *testing.T) {
	cases := []struct {
		in      string
		want    string
		wantErr bool
	}{
		{"file.txt", "file.txt", false},
		{"some/path/file.txt", "file.txt", false},
		{"", "", true},
		{"..", "", true},
		{".", "", true},
		{"a/../b", "b", false},
		{"a\\b.txt", "", true},
	}
	for _, c := range cases {
		got, err := SafeName(c.in)
		if c.wantErr {
			if err == nil {
				t.Errorf("SafeName(%q): expected error, got %q", c.in, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("SafeName(%q): unexpected error: %v", c.in, err)
			continue
		}
		if got != c.want {
			t.Errorf("SafeName(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestHumanSize(t *testing.T) {
	cases := []struct {
		in   int64
		want string
	}{
		{0, "0 B"},
		{512, "512 B"},
		{1024, "1.0 KB"},
		{1536, "1.5 KB"},
		{1024 * 1024, "1.0 MB"},
		{1024 * 1024 * 1024, "1.0 GB"},
	}
	for _, c := range cases {
		if got := HumanSize(c.in); got != c.want {
			t.Errorf("HumanSize(%d) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestUpload_HappyPath(t *testing.T) {

	store := newMemStore()
	h := New(store, testTmpl(t))

	body := &bytes.Buffer{}
	mw := multipart.NewWriter(body)
	fw, err := mw.CreateFormFile("file", "hello.txt")
	if err != nil {
		t.Fatalf("CreateFormFile: %v", err)
	}
	_, _ = fw.Write([]byte("hello world"))
	mw.Close()

	req := httptest.NewRequest(http.MethodPost, "/upload", body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	rec := httptest.NewRecorder()
	h.Upload(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303 (body: %s)", rec.Code, rec.Body.String())
	}
	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "/?ok=hello.txt") {
		t.Errorf("Location = %q, want contains /?ok=hello.txt", loc)
	}
	if string(store.mu["hello.txt"]) != "hello world" {
		t.Errorf("stored content = %q, want %q", store.mu["hello.txt"], "hello world")
	}
}

func TestUpload_RejectsWrongMethod(t *testing.T) {
	h := New(newMemStore(), testTmpl(t))

	req := httptest.NewRequest(http.MethodGet, "/upload", nil)
	rec := httptest.NewRecorder()
	h.Upload(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want 405", rec.Code)
	}
}

func TestDownload_HappyPath(t *testing.T) {
	store := newMemStore()
	store.mu["hello.txt"] = []byte("hello")
	h := New(store, testTmpl(t))

	req := httptest.NewRequest(http.MethodGet, "/download/hello.txt", nil)
	rec := httptest.NewRecorder()
	h.Download(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
	if got := rec.Body.String(); got != "hello" {
		t.Errorf("body = %q, want %q", got, "hello")
	}
	if cd := rec.Header().Get("Content-Disposition"); !strings.Contains(cd, "hello.txt") {
		t.Errorf("Content-Disposition = %q, want contains hello.txt", cd)
	}
}

func TestDownload_NotFound(t *testing.T) {
	h := New(newMemStore(), testTmpl(t))

	req := httptest.NewRequest(http.MethodGet, "/download/missing.txt", nil)
	rec := httptest.NewRecorder()
	h.Download(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

func TestDownload_BlocksPathTraversal(t *testing.T) {
	h := New(newMemStore(), testTmpl(t))

	req := httptest.NewRequest(http.MethodGet, "/download/..%2Fpasswd", nil)
	rec := httptest.NewRecorder()
	h.Download(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404 for traversal attempt", rec.Code)
	}
}

func TestHome_ListsFiles(t *testing.T) {
	store := newMemStore()
	store.mu["one.txt"] = []byte("1")
	store.mu["two.txt"] = []byte("22")
	h := New(store, testTmpl(t))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	h.Home(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "one.txt") || !strings.Contains(body, "two.txt") {
		t.Errorf("body missing files: %q", body)
	}
}

func TestHome_StoreErrorShowsBanner(t *testing.T) {
	store := newMemStore()
	store.listErr = errors.New("boom")
	h := New(store, testTmpl(t))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	h.Home(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Could not list files") {
		t.Errorf("expected error banner in body, got %q", rec.Body.String())
	}
}
