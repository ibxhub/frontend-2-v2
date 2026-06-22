package handlers

import (
	"context"
	"errors"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"frontend-2-v2/internal/auth"
	"frontend-2-v2/internal/storage"
)

const maxUploadBytes = 25 << 20

type fileEntry struct {
	Name    string
	Size    string
	ModTime string
}

type pageData struct {
	Title   string
	User    string
	Files   []fileEntry
	Message string
	Error   string
}

type Files struct {
	Store storage.Store
	Tmpl  *template.Template
}

func New(store storage.Store, tmpl *template.Template) *Files {
	return &Files{Store: store, Tmpl: tmpl}
}

func (h *Files) Home(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	h.render(w, r, "")
}

func (h *Files) Upload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxUploadBytes)
	if err := r.ParseMultipartForm(maxUploadBytes); err != nil {
		h.respond(w, r, "", "Upload too large or malformed.")
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		h.respond(w, r, "", "Please choose a file to upload.")
		return
	}
	defer file.Close()

	name, err := SafeName(header.Filename)
	if err != nil {
		h.respond(w, r, "", "Invalid filename.")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()
	if err := h.Store.Put(ctx, name, file, header.Size); err != nil {
		http.Error(w, "could not save file", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/?ok="+name, http.StatusSeeOther)
}

func (h *Files) Download(w http.ResponseWriter, r *http.Request) {
	requested := strings.TrimPrefix(r.URL.Path, "/download/")
	name, err := SafeName(requested)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()
	body, obj, err := h.Store.Get(ctx, name)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			http.NotFound(w, r)
			return
		}
		http.Error(w, "could not read file", http.StatusInternalServerError)
		return
	}
	defer body.Close()

	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", name))
	w.Header().Set("Content-Type", "application/octet-stream")
	if obj != nil && obj.Size > 0 {
		w.Header().Set("Content-Length", fmt.Sprintf("%d", obj.Size))
	}
	if _, err := io.Copy(w, body); err != nil {
		return
	}
}

func (h *Files) render(w http.ResponseWriter, r *http.Request, errMsg string) {
	h.respond(w, r, r.URL.Query().Get("ok"), errMsg)
}

func (h *Files) respond(w http.ResponseWriter, r *http.Request, okName, errMsg string) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	objs, listErr := h.Store.List(ctx)
	if listErr != nil && errMsg == "" {
		errMsg = "Could not list files."
	}

	files := make([]fileEntry, 0, len(objs))
	for _, o := range objs {
		files = append(files, fileEntry{
			Name:    o.Name,
			Size:    HumanSize(o.Size),
			ModTime: o.ModTime.Format(time.RFC3339),
		})
	}

	message := ""
	if okName != "" {
		message = fmt.Sprintf("Uploaded %s.", okName)
	}

	data := pageData{
		Title:   "frontend-2-v2 — file upload & download",
		User:    userLabel(r),
		Files:   files,
		Message: message,
		Error:   errMsg,
	}
	if err := h.Tmpl.ExecuteTemplate(w, "base.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func userLabel(r *http.Request) string {
	if c := auth.ClaimsFrom(r); c != nil {
		if c.Name != "" {
			return c.Name
		}
		return c.Email
	}
	return ""
}

func SafeName(raw string) (string, error) {
	name := filepath.Base(raw)
	if name == "" || name == "." || name == ".." || strings.ContainsAny(name, "/\\") {
		return "", errors.New("invalid filename")
	}
	return name, nil
}

func HumanSize(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}
