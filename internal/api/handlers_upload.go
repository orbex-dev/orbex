package api

import (
	"fmt"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/orbex-dev/orbex/internal/database"
	"github.com/orbex-dev/orbex/internal/models"
	"github.com/orbex-dev/orbex/internal/storage"
)

const maxUploadSize = 50 << 20 // 50MB

// UploadHandler handles file upload operations for jobs.
type UploadHandler struct {
	db      *database.DB
	storage *storage.Client
}

// NewUploadHandler creates a new UploadHandler.
func NewUploadHandler(db *database.DB, storageClient *storage.Client) *UploadHandler {
	return &UploadHandler{db: db, storage: storageClient}
}

// Upload handles multipart file upload for a job.
func (h *UploadHandler) Upload(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())
	jobID, err := uuid.Parse(chi.URLParam(r, "jobID"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, models.ErrorResponse{
			Error: "invalid_request", Message: "Invalid job ID",
		})
		return
	}

	// Verify job ownership
	var ownerID uuid.UUID
	err = h.db.Pool.QueryRow(r.Context(), `SELECT user_id FROM jobs WHERE id = $1`, jobID).Scan(&ownerID)
	if err != nil || ownerID != user.ID {
		writeJSON(w, http.StatusNotFound, models.ErrorResponse{
			Error: "not_found", Message: "Job not found",
		})
		return
	}

	// Parse multipart form
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadSize)
	if err := r.ParseMultipartForm(maxUploadSize); err != nil {
		writeJSON(w, http.StatusBadRequest, models.ErrorResponse{
			Error: "invalid_request", Message: "File too large (max 50MB)",
		})
		return
	}

	files := r.MultipartForm.File["files"]
	if len(files) == 0 {
		writeJSON(w, http.StatusBadRequest, models.ErrorResponse{
			Error: "invalid_request", Message: "No files provided",
		})
		return
	}

	var uploaded []map[string]interface{}

	for _, fh := range files {
		file, err := fh.Open()
		if err != nil {
			continue
		}
		defer file.Close()

		key := fmt.Sprintf("uploads/%s/%s/%s", user.ID, jobID, fh.Filename)
		contentType := fh.Header.Get("Content-Type")
		if contentType == "" {
			contentType = "application/octet-stream"
		}

		if err := h.storage.Upload(r.Context(), key, file, fh.Size, contentType); err != nil {
			writeJSON(w, http.StatusInternalServerError, models.ErrorResponse{
				Error: "upload_failed", Message: fmt.Sprintf("Failed to upload %s", fh.Filename),
			})
			return
		}

		uploaded = append(uploaded, map[string]interface{}{
			"filename": fh.Filename,
			"size":     fh.Size,
			"key":      key,
		})
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"uploaded": uploaded,
		"count":    len(uploaded),
	})
}

// ListFiles lists all uploaded files for a job.
func (h *UploadHandler) ListFiles(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())
	jobID, err := uuid.Parse(chi.URLParam(r, "jobID"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, models.ErrorResponse{
			Error: "invalid_request", Message: "Invalid job ID",
		})
		return
	}

	prefix := fmt.Sprintf("uploads/%s/%s/", user.ID, jobID)
	objects, err := h.storage.List(r.Context(), prefix)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, models.ErrorResponse{
			Error: "internal_error", Message: "Failed to list files",
		})
		return
	}

	// Transform to friendly response
	var files []map[string]interface{}
	for _, obj := range objects {
		filename := strings.TrimPrefix(obj.Key, prefix)
		files = append(files, map[string]interface{}{
			"filename":      filename,
			"size":          obj.Size,
			"last_modified": obj.LastModified,
		})
	}
	if files == nil {
		files = []map[string]interface{}{}
	}

	writeJSON(w, http.StatusOK, files)
}

// DeleteFile deletes a specific uploaded file.
func (h *UploadHandler) DeleteFile(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())
	jobID, err := uuid.Parse(chi.URLParam(r, "jobID"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, models.ErrorResponse{
			Error: "invalid_request", Message: "Invalid job ID",
		})
		return
	}

	filename := chi.URLParam(r, "filename")
	if filename == "" {
		writeJSON(w, http.StatusBadRequest, models.ErrorResponse{
			Error: "invalid_request", Message: "Filename is required",
		})
		return
	}

	key := fmt.Sprintf("uploads/%s/%s/%s", user.ID, jobID, filename)
	if err := h.storage.Delete(r.Context(), key); err != nil {
		writeJSON(w, http.StatusInternalServerError, models.ErrorResponse{
			Error: "internal_error", Message: "Failed to delete file",
		})
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// DetectImage returns a suggested Docker image based on file extensions.
func DetectImage(filenames []string) string {
	for _, name := range filenames {
		ext := strings.ToLower(filepath.Ext(name))
		switch ext {
		case ".py":
			return "python:3.12-slim"
		case ".js", ".ts", ".mjs":
			return "node:22-slim"
		case ".go":
			return "golang:1.22"
		case ".rb":
			return "ruby:3.3-slim"
		case ".sh", ".bash":
			return "alpine:latest"
		case ".rs":
			return "rust:1.77-slim"
		}
	}
	return ""
}
