package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/orbex-dev/orbex/internal/database"
	"github.com/orbex-dev/orbex/internal/docker"
	"github.com/orbex-dev/orbex/internal/models"
)

// waitResult holds the result of a container wait operation.
type waitResult struct {
	exitCode int64
	err      error
}

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

	// Execute the run in background
	go h.executeRun(job, run)

	writeJSON(w, http.StatusAccepted, run)
}

// executeRun pulls the image, creates a container, runs it, and captures the result.
func (h *RunHandler) executeRun(job models.Job, run models.JobRun) {
	ctx := context.Background()
	now := time.Now()

	// Recover from panics so a run never gets stuck in "running" state
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("[executeRun] PANIC recovered for run %s: %v\n", run.ID, r)
			h.failRun(ctx, run.ID, now, fmt.Sprintf("Internal error (panic): %v", r))
		}
	}()

	fmt.Printf("[executeRun] Starting run %s for job %s (image: %s)\n", run.ID, job.Name, job.Image)
	fmt.Printf("[executeRun] Command: %v (len=%d)\n", job.Command, len(job.Command))

	// Mark as running
	if _, err := h.db.Pool.Exec(ctx, `
		UPDATE job_runs SET status = 'running'::run_status, started_at = $1 WHERE id = $2
	`, now, run.ID); err != nil {
		fmt.Printf("[executeRun] ERROR marking run as running: %v\n", err)
	}

	// Pull image
	if err := h.docker.PullImage(ctx, job.Image); err != nil {
		h.failRun(ctx, run.ID, now, fmt.Sprintf("Failed to pull image: %v", err))
		return
	}

	// Create container
	containerName := fmt.Sprintf("orbex-%s-%s", job.Name, run.ID.String()[:8])
	containerID, err := h.docker.CreateContainer(ctx, docker.ContainerConfig{
		Image:         job.Image,
		Command:       job.Command,
		Env:           job.Env,
		MemoryMB:      job.MemoryMB,
		CPUMillicores: job.CPUMillicores,
		Name:          containerName,
	})
	if err != nil {
		h.failRun(ctx, run.ID, now, fmt.Sprintf("Failed to create container: %v", err))
		return
	}

	// Store container ID
	fmt.Printf("[executeRun] Container created: %s (name: %s)\n", containerID[:12], containerName)
	_, _ = h.db.Pool.Exec(ctx, `
		UPDATE job_runs SET container_id = $1 WHERE id = $2
	`, containerID, run.ID)

	// IMPORTANT: Set up wait channel BEFORE starting container to avoid race condition
	// If container exits before we call WaitContainer, the wait would block forever
	waitCh := make(chan waitResult, 1)
	go func() {
		exitCode, err := h.docker.WaitContainer(ctx, containerID)
		waitCh <- waitResult{exitCode: exitCode, err: err}
	}()

	// Start container
	fmt.Printf("[executeRun] Starting container %s\n", containerID[:12])
	if err := h.docker.StartContainer(ctx, containerID); err != nil {
		h.failRun(ctx, run.ID, now, fmt.Sprintf("Failed to start container: %v", err))
		_ = h.docker.RemoveContainer(ctx, containerID)
		return
	}
	fmt.Printf("[executeRun] Container %s started, waiting for exit...\n", containerID[:12])

	// Wait for container to finish
	result := <-waitCh
	exitCode := result.exitCode
	err = result.err

	finishedAt := time.Now()
	durationMs := finishedAt.Sub(now).Milliseconds()
	fmt.Printf("[executeRun] Container %s finished: exitCode=%d, duration=%dms, err=%v\n", containerID[:12], exitCode, durationMs, err)

	// Get logs before removing the container
	logs, logsErr := h.docker.GetLogs(ctx, containerID, "1000")
	if logsErr != nil {
		fmt.Printf("[executeRun] WARNING: Failed to get logs: %v\n", logsErr)
	}
	fmt.Printf("[executeRun] Logs captured: %d bytes\n", len(logs))

	// Determine final status
	status := models.RunStatusSucceeded
	var errorMsg *string
	if err != nil {
		status = models.RunStatusFailed
		msg := fmt.Sprintf("Container wait error: %v", err)
		errorMsg = &msg
	} else if exitCode != 0 {
		status = models.RunStatusFailed
		msg := fmt.Sprintf("Process exited with code %d", exitCode)
		errorMsg = &msg
	}

	exitCodeInt := int(exitCode)
	if _, err := h.db.Pool.Exec(ctx, `
		UPDATE job_runs
		SET status = $1::run_status, exit_code = $2, finished_at = $3, duration_ms = $4,
		    logs_tail = $5, error_message = $6
		WHERE id = $7
	`, string(status), exitCodeInt, finishedAt, durationMs, logs, errorMsg, run.ID); err != nil {
		fmt.Printf("[executeRun] ERROR updating final status: %v\n", err)
	}

	// Remove from queue
	_, _ = h.db.Pool.Exec(ctx, `DELETE FROM job_queue WHERE run_id = $1`, run.ID)

	// Cleanup container
	_ = h.docker.RemoveContainer(ctx, containerID)
	fmt.Printf("[executeRun] Run %s completed with status: %s\n", run.ID, status)
}

// failRun marks a run as failed with an error message.
func (h *RunHandler) failRun(ctx context.Context, runID uuid.UUID, startedAt time.Time, errorMsg string) {
	finishedAt := time.Now()
	durationMs := finishedAt.Sub(startedAt).Milliseconds()
	_, _ = h.db.Pool.Exec(ctx, `
		UPDATE job_runs
		SET status = 'failed'::run_status, error_message = $1, finished_at = $2, duration_ms = $3
		WHERE id = $4
	`, errorMsg, finishedAt, durationMs, runID)
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
