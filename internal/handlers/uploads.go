package handlers

import (
	"bytes"
	"fmt"
	"github.com/go-chi/chi/v5"
	"io"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/esc-chula/intania-shop-api/internal/storage"
)

type UploadHandler struct{ uploader storage.Uploader }

func NewUploadHandler(uploader storage.Uploader) *UploadHandler {
	return &UploadHandler{uploader: uploader}
}

func (h *UploadHandler) Register(router chi.Router) {
	router.Post("/upload/product-images", func(w http.ResponseWriter, r *http.Request) { h.uploadMany(w, r, "products/images", true) })
	router.Post("/upload/product-videos", func(w http.ResponseWriter, r *http.Request) { h.uploadMany(w, r, "products/videos", false) })
	router.Post("/stock/upload-proof-images", func(w http.ResponseWriter, r *http.Request) { h.uploadOne(w, r, "stock/proof", true) })
}
func (h *UploadHandler) uploadMany(w http.ResponseWriter, r *http.Request, folder string, imagesOnly bool) {
	r.Body = http.MaxBytesReader(w, r.Body, 100<<20)
	if e := r.ParseMultipartForm(100 << 20); e != nil {
		writeError(w, 413, "Upload exceeds 100 MB limit")
		return
	}
	files := r.MultipartForm.File["files"]
	if len(files) == 0 {
		for _, group := range r.MultipartForm.File {
			files = append(files, group...)
		}
	}
	if len(files) == 0 {
		writeError(w, 400, "No files provided")
		return
	}
	out := make([]storage.Object, 0, len(files))
	for _, file := range files {
		object, e := h.uploadFile(r, file, folder, imagesOnly)
		if e != nil {
			writeError(w, 400, e.Error())
			return
		}
		out = append(out, object)
	}
	writeSuccess(w, 200, map[string]any{"uploads": out})
}
func (h *UploadHandler) uploadOne(w http.ResponseWriter, r *http.Request, folder string, imagesOnly bool) {
	r.Body = http.MaxBytesReader(w, r.Body, 10<<20)
	if e := r.ParseMultipartForm(10 << 20); e != nil {
		writeError(w, 413, "Upload exceeds 10 MB limit")
		return
	}
	for _, group := range r.MultipartForm.File {
		if len(group) == 0 {
			continue
		}
		object, e := h.uploadFile(r, group[0], folder, imagesOnly)
		if e != nil {
			writeError(w, 400, e.Error())
			return
		}
		writeSuccess(w, 200, map[string]any{"image": object})
		return
	}
	writeError(w, 400, "No image file provided")
}
func (h *UploadHandler) uploadFile(r *http.Request, file *multipart.FileHeader, folder string, imagesOnly bool) (storage.Object, error) {
	extension := strings.ToLower(filepath.Ext(file.Filename))
	if imagesOnly && !isImageExtension(extension) {
		return storage.Object{}, fmt.Errorf("unsupported image type")
	}
	opened, e := file.Open()
	if e != nil {
		return storage.Object{}, fmt.Errorf("open upload: %w", e)
	}
	defer func() { _ = opened.Close() }()
	head := make([]byte, 512)
	n, _ := io.ReadFull(opened, head)
	contentType := http.DetectContentType(head[:n])
	if imagesOnly && !strings.HasPrefix(contentType, "image/") {
		return storage.Object{}, fmt.Errorf("uploaded content is not an image")
	}
	return h.uploader.Upload(r.Context(), folder, file.Filename, contentType, io.MultiReader(bytes.NewReader(head[:n]), opened))
}
func isImageExtension(value string) bool {
	switch value {
	case ".jpg", ".jpeg", ".png", ".gif", ".webp", ".bmp":
		return true
	}
	return false
}
