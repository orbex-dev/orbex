package api

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/orbex-dev/orbex/internal/database"
	"github.com/orbex-dev/orbex/internal/models"
)

// JobHandler handles job CRUD operations.
type JobHandler struct {
	db *database.DB
}

// NewJobHandler creates a new JobHandler.
func NewJobHandler(db *database.DB) *JobHandler {
	return &JobHandler{db: db}
}

// Create creates a new job definition.
func (h *JobHandler) Create(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())

	var req models.CreateJobRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, models.ErrorResponse{
			Error: "invalid_request", Message: "Invalid JSON body",
		})
		return
	}

	if req.Name == "" || req.Image == "" {
		writeJSON(w, http.StatusBadRequest, models.ErrorResponse{
			Error: "validation_error", Message: "Name and image are required",
		})
		return
	}

	// Apply defaults
	if req.MemoryMB == 0 {
		req.MemoryMB = 512
	}
	if req.CPUMillicores == 0 {
		req.CPUMillicores = 1000
	}
	if req.TimeoutSeconds == 0 {
		req.TimeoutSeconds = 3600
	}
	if req.Env == nil {
		req.Env = map[string]string{}
	}

	envJSON, _ := json.Marshal(req.Env)

	var job models.Job
	err := h.db.Pool.QueryRow(r.Context(), `
		INSERT INTO jobs (user_id, name, image, command, env, memory_mb, cpu_millicores, timeout_seconds, schedule)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id, user_id, name, image, command, env, memory_mb, cpu_millicores, timeout_seconds, schedule, is_active, created_at, updated_at
	`, user.ID, req.Name, req.Image, req.Command, envJSON,
		req.MemoryMB, req.CPUMillicores, req.TimeoutSeconds, req.Schedule,
	).Scan(
		&job.ID, &job.UserID, &job.Name, &job.Image, &job.Command,
		&envJSON, &job.MemoryMB, &job.CPUMillicores, &job.TimeoutSeconds,
		&job.Schedule, &job.IsActive, &job.CreatedAt, &job.UpdatedAt,
	)

	if err != nil {
		if isDuplicateError(err) {
			writeJSON(w, http.StatusConflict, models.ErrorResponse{
				Error: "conflict", Message: "A job with this name already exists",
			})
			return
		}
		writeJSON(w, http.StatusInternalServerError, models.ErrorResponse{
			Error: "internal_error", Message: "Failed to create job",
		})
		return
	}

	_ = json.Unmarshal(envJSON, &job.Env)
	writeJSON(w, http.StatusCreated, job)
}

// List returns all jobs for the authenticated user.
func (h *JobHandler) List(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())

	rows, err := h.db.Pool.Query(r.Context(), `
		SELECT id, user_id, name, image, command, env, memory_mb, cpu_millicores,
		       timeout_seconds, schedule, is_active, created_at, updated_at
		FROM jobs
		WHERE user_id = $1
		ORDER BY created_at DESC
	`, user.ID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, models.ErrorResponse{
			Error: "internal_error", Message: "Failed to list jobs",
		})
		return
	}
	defer rows.Close()

	var jobs []models.Job
	for rows.Next() {
		var job models.Job
		var envJSON []byte
		if err := rows.Scan(
			&job.ID, &job.UserID, &job.Name, &job.Image, &job.Command,
			&envJSON, &job.MemoryMB, &job.CPUMillicores, &job.TimeoutSeconds,
			&job.Schedule, &job.IsActive, &job.CreatedAt, &job.UpdatedAt,
		); err != nil {
			continue
		}
		_ = json.Unmarshal(envJSON, &job.Env)
		jobs = append(jobs, job)
	}

	if jobs == nil {
		jobs = []models.Job{}
	}
	writeJSON(w, http.StatusOK, jobs)
}

// Get returns a single job by ID.
func (h *JobHandler) Get(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())
	jobID, err := uuid.Parse(chi.URLParam(r, "jobID"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, models.ErrorResponse{
			Error: "invalid_request", Message: "Invalid job ID",
		})
		return
	}

	var job models.Job
	var envJSON []byte
	err = h.db.Pool.QueryRow(r.Context(), `
		SELECT id, user_id, name, image, command, env, memory_mb, cpu_millicores,
		       timeout_seconds, schedule, is_active, created_at, updated_at
		FROM jobs
		WHERE id = $1 AND user_id = $2
	`, jobID, user.ID).Scan(
		&job.ID, &job.UserID, &job.Name, &job.Image, &job.Command,
		&envJSON, &job.MemoryMB, &job.CPUMillicores, &job.TimeoutSeconds,
		&job.Schedule, &job.IsActive, &job.CreatedAt, &job.UpdatedAt,
	)

	if err != nil {
		writeJSON(w, http.StatusNotFound, models.ErrorResponse{
			Error: "not_found", Message: "Job not found",
		})
		return
	}

	_ = json.Unmarshal(envJSON, &job.Env)
	writeJSON(w, http.StatusOK, job)
}

// Delete removes a job definition.
func (h *JobHandler) Delete(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())
	jobID, err := uuid.Parse(chi.URLParam(r, "jobID"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, models.ErrorResponse{
			Error: "invalid_request", Message: "Invalid job ID",
		})
		return
	}

	tag, err := h.db.Pool.Exec(r.Context(), `
		DELETE FROM jobs WHERE id = $1 AND user_id = $2
	`, jobID, user.ID)

	if err != nil || tag.RowsAffected() == 0 {
		writeJSON(w, http.StatusNotFound, models.ErrorResponse{
			Error: "not_found", Message: "Job not found",
		})
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// GenerateWebhookToken creates or regenerates a webhook token for a job.
func (h *JobHandler) GenerateWebhookToken(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())
	jobID, err := uuid.Parse(chi.URLParam(r, "jobID"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, models.ErrorResponse{
			Error: "invalid_request", Message: "Invalid job ID",
		})
		return
	}

	// Generate random token
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		writeJSON(w, http.StatusInternalServerError, models.ErrorResponse{
			Error: "internal_error", Message: "Failed to generate token",
		})
		return
	}
	token := fmt.Sprintf("whk_%x", tokenBytes)

	// Update the job with the new token
	tag, err := h.db.Pool.Exec(r.Context(), `
		UPDATE jobs SET webhook_token = $1, updated_at = now()
		WHERE id = $2 AND user_id = $3
	`, token, jobID, user.ID)
	if err != nil || tag.RowsAffected() == 0 {
		writeJSON(w, http.StatusNotFound, models.ErrorResponse{
			Error: "not_found", Message: "Job not found",
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"webhook_token": token,
		"trigger_url":   fmt.Sprintf("/api/v1/webhooks/%s/trigger", token),
	})
}
