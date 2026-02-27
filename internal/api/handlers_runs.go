package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/orbex-dev/orbex/internal/database"
	"github.com/orbex-dev/orbex/internal/docker"
	"github.com/orbex-dev/orbex/internal/models"
)

// RunHandler handles job run operations.
type RunHandler struct {
	db     *database.DB
	docker *docker.Client
}

// NewRunHandler creates a new RunHandler.
func NewRunHandler(db *database.DB, dockerClient *docker.Client) *RunHandler {
	return &RunHandler{db: db, docker: dockerClient}
}

// TriggerRun starts a new run for a job.
func (h *RunHandler) TriggerRun(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())
	jobID, err := uuid.Parse(chi.URLParam(r, "jobID"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, models.ErrorResponse{
			Error: "invalid_request", Message: "Invalid job ID",
		})
		return
	}

	// Fetch job definition
	var job models.Job
	var envJSON []byte
	err = h.db.Pool.QueryRow(r.Context(), `
		SELECT id, user_id, name, image, command, env, memory_mb, cpu_millicores, timeout_seconds
		FROM jobs
		WHERE id = $1 AND user_id = $2 AND is_active = true
	`, jobID, user.ID).Scan(
		&job.ID, &job.UserID, &job.Name, &job.Image, &job.Command,
		&envJSON, &job.MemoryMB, &job.CPUMillicores, &job.TimeoutSeconds,
	)
	if err != nil {
		writeJSON(w, http.StatusNotFound, models.ErrorResponse{
			Error: "not_found", Message: "Job not found or inactive",
		})
		return
	}
	_ = json.Unmarshal(envJSON, &job.Env)

	// Create job run record
	var run models.JobRun
	err = h.db.Pool.QueryRow(r.Context(), `
		INSERT INTO job_runs (job_id, user_id, status)
		VALUES ($1, $2, 'pending'::run_status)
		RETURNING id, job_id, user_id, status, created_at
	`, job.ID, user.ID).Scan(&run.ID, &run.JobID, &run.UserID, &run.Status, &run.CreatedAt)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, models.ErrorResponse{
			Error: "internal_error", Message: "Failed to create run",
		})
		return
	}

	// Enqueue the run
	_, err = h.db.Pool.Exec(r.Context(), `
		INSERT INTO job_queue (job_id, run_id)
		VALUES ($1, $2)
	`, job.ID, run.ID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, models.ErrorResponse{
			Error: "internal_error", Message: "Failed to enqueue run",
		})
		return
	}

	// Worker will pick this up via SKIP LOCKED polling

	writeJSON(w, http.StatusAccepted, run)
}

// ListRuns returns all runs for a job.
func (h *RunHandler) ListRuns(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())
	jobID, err := uuid.Parse(chi.URLParam(r, "jobID"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, models.ErrorResponse{
			Error: "invalid_request", Message: "Invalid job ID",
		})
		return
	}

	rows, err := h.db.Pool.Query(r.Context(), `
		SELECT id, job_id, user_id, status, container_id, exit_code, error_message,
		       started_at, finished_at, paused_at, duration_ms, created_at
		FROM job_runs
		WHERE job_id = $1 AND user_id = $2
		ORDER BY created_at DESC
		LIMIT 50
	`, jobID, user.ID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, models.ErrorResponse{
			Error: "internal_error", Message: "Failed to list runs",
		})
		return
	}
	defer rows.Close()

	var runs []models.JobRun
	for rows.Next() {
		var run models.JobRun
		if err := rows.Scan(
			&run.ID, &run.JobID, &run.UserID, &run.Status, &run.ContainerID,
			&run.ExitCode, &run.ErrorMessage, &run.StartedAt, &run.FinishedAt,
			&run.PausedAt, &run.DurationMs, &run.CreatedAt,
		); err != nil {
			continue
		}
		runs = append(runs, run)
	}

	if runs == nil {
		runs = []models.JobRun{}
	}
	writeJSON(w, http.StatusOK, runs)
}

// GetRun returns details of a specific run.
func (h *RunHandler) GetRun(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())
	runID, err := uuid.Parse(chi.URLParam(r, "runID"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, models.ErrorResponse{
			Error: "invalid_request", Message: "Invalid run ID",
		})
		return
	}

	var run models.JobRun
	err = h.db.Pool.QueryRow(r.Context(), `
		SELECT id, job_id, user_id, status, container_id, exit_code, error_message,
		       started_at, finished_at, paused_at, duration_ms, logs_tail, created_at
		FROM job_runs
		WHERE id = $1 AND user_id = $2
	`, runID, user.ID).Scan(
		&run.ID, &run.JobID, &run.UserID, &run.Status, &run.ContainerID,
		&run.ExitCode, &run.ErrorMessage, &run.StartedAt, &run.FinishedAt,
		&run.PausedAt, &run.DurationMs, &run.LogsTail, &run.CreatedAt,
	)
	if err != nil {
		writeJSON(w, http.StatusNotFound, models.ErrorResponse{
			Error: "not_found", Message: "Run not found",
		})
		return
	}

	writeJSON(w, http.StatusOK, run)
}

// PauseRun pauses a running job.
func (h *RunHandler) PauseRun(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())
	runID, err := uuid.Parse(chi.URLParam(r, "runID"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, models.ErrorResponse{
			Error: "invalid_request", Message: "Invalid run ID",
		})
		return
	}

	var containerID *string
	var status models.RunStatus
	err = h.db.Pool.QueryRow(r.Context(), `
		SELECT container_id, status FROM job_runs WHERE id = $1 AND user_id = $2
	`, runID, user.ID).Scan(&containerID, &status)
	if err != nil {
		writeJSON(w, http.StatusNotFound, models.ErrorResponse{
			Error: "not_found", Message: "Run not found",
		})
		return
	}

	if status != models.RunStatusRunning {
		writeJSON(w, http.StatusConflict, models.ErrorResponse{
			Error: "invalid_state", Message: "Can only pause running jobs",
		})
		return
	}

	if containerID == nil {
		writeJSON(w, http.StatusConflict, models.ErrorResponse{
			Error: "invalid_state", Message: "No container associated with this run",
		})
		return
	}

	if err := h.docker.PauseContainer(r.Context(), *containerID); err != nil {
		writeJSON(w, http.StatusInternalServerError, models.ErrorResponse{
			Error: "internal_error", Message: "Failed to pause container",
		})
		return
	}

	now := time.Now()
	_, _ = h.db.Pool.Exec(r.Context(), `
		UPDATE job_runs SET status = 'paused'::run_status, paused_at = $1 WHERE id = $2
	`, now, runID)

	writeJSON(w, http.StatusOK, map[string]string{
		"status":  "paused",
		"message": "Container paused. Use resume to continue or kill to terminate.",
	})
}

// ResumeRun resumes a paused job.
func (h *RunHandler) ResumeRun(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())
	runID, err := uuid.Parse(chi.URLParam(r, "runID"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, models.ErrorResponse{
			Error: "invalid_request", Message: "Invalid run ID",
		})
		return
	}

	var containerID *string
	var status models.RunStatus
	err = h.db.Pool.QueryRow(r.Context(), `
		SELECT container_id, status FROM job_runs WHERE id = $1 AND user_id = $2
	`, runID, user.ID).Scan(&containerID, &status)
	if err != nil {
		writeJSON(w, http.StatusNotFound, models.ErrorResponse{
			Error: "not_found", Message: "Run not found",
		})
		return
	}

	if status != models.RunStatusPaused {
		writeJSON(w, http.StatusConflict, models.ErrorResponse{
			Error: "invalid_state", Message: "Can only resume paused jobs",
		})
		return
	}

	if containerID == nil {
		writeJSON(w, http.StatusConflict, models.ErrorResponse{
			Error: "invalid_state", Message: "No container associated with this run",
		})
		return
	}

	if err := h.docker.UnpauseContainer(r.Context(), *containerID); err != nil {
		writeJSON(w, http.StatusInternalServerError, models.ErrorResponse{
			Error: "internal_error", Message: "Failed to resume container",
		})
		return
	}

	_, _ = h.db.Pool.Exec(r.Context(), `
		UPDATE job_runs SET status = 'running'::run_status, paused_at = NULL WHERE id = $1
	`, runID)

	writeJSON(w, http.StatusOK, map[string]string{
		"status":  "running",
		"message": "Container resumed from paused state.",
	})
}

// KillRun terminates a running or paused job.
func (h *RunHandler) KillRun(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())
	runID, err := uuid.Parse(chi.URLParam(r, "runID"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, models.ErrorResponse{
			Error: "invalid_request", Message: "Invalid run ID",
		})
		return
	}

	var containerID *string
	var status models.RunStatus
	var startedAt *time.Time
	err = h.db.Pool.QueryRow(r.Context(), `
		SELECT container_id, status, started_at FROM job_runs WHERE id = $1 AND user_id = $2
	`, runID, user.ID).Scan(&containerID, &status, &startedAt)
	if err != nil {
		writeJSON(w, http.StatusNotFound, models.ErrorResponse{
			Error: "not_found", Message: "Run not found",
		})
		return
	}

	if status != models.RunStatusRunning && status != models.RunStatusPaused {
		writeJSON(w, http.StatusConflict, models.ErrorResponse{
			Error: "invalid_state", Message: "Can only kill running or paused jobs",
		})
		return
	}

	if containerID != nil {
		if status == models.RunStatusPaused {
			_ = h.docker.UnpauseContainer(r.Context(), *containerID)
		}
		_ = h.docker.StopContainer(r.Context(), *containerID, 10)
		_ = h.docker.RemoveContainer(r.Context(), *containerID)
	}

	now := time.Now()
	var durationMs int64
	if startedAt != nil {
		durationMs = now.Sub(*startedAt).Milliseconds()
	}

	_, _ = h.db.Pool.Exec(r.Context(), `
		UPDATE job_runs
		SET status = 'cancelled'::run_status, finished_at = $1, duration_ms = $2, error_message = 'Killed by user'
		WHERE id = $3
	`, now, durationMs, runID)

	_, _ = h.db.Pool.Exec(r.Context(), `DELETE FROM job_queue WHERE run_id = $1`, runID)

	writeJSON(w, http.StatusOK, map[string]string{
		"status":  "cancelled",
		"message": "Job killed.",
	})
}

// GetRunLogs returns the logs for a run.
func (h *RunHandler) GetRunLogs(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())
	runID, err := uuid.Parse(chi.URLParam(r, "runID"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, models.ErrorResponse{
			Error: "invalid_request", Message: "Invalid run ID",
		})
		return
	}

	var containerID *string
	var logsTail *string
	var status models.RunStatus
	err = h.db.Pool.QueryRow(r.Context(), `
		SELECT container_id, logs_tail, status FROM job_runs WHERE id = $1 AND user_id = $2
	`, runID, user.ID).Scan(&containerID, &logsTail, &status)
	if err != nil {
		writeJSON(w, http.StatusNotFound, models.ErrorResponse{
			Error: "not_found", Message: "Run not found",
		})
		return
	}

	// If container is still alive, get live logs
	if containerID != nil && (status == models.RunStatusRunning || status == models.RunStatusPaused) {
		logs, err := h.docker.GetLogs(r.Context(), *containerID, "1000")
		if err == nil {
			writeJSON(w, http.StatusOK, map[string]string{"logs": logs})
			return
		}
	}

	// Otherwise return stored logs
	logs := ""
	if logsTail != nil {
		logs = *logsTail
	}
	writeJSON(w, http.StatusOK, map[string]string{"logs": logs})
}
