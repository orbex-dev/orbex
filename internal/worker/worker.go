package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/orbex-dev/orbex/internal/database"
	"github.com/orbex-dev/orbex/internal/docker"
	"github.com/orbex-dev/orbex/internal/models"
)

// Config holds worker configuration.
type Config struct {
	MaxConcurrent int           // Max parallel container runs
	PollInterval  time.Duration // How often to check for work
}

// DefaultConfig returns sensible defaults.
func DefaultConfig() Config {
	return Config{
		MaxConcurrent: 5,
		PollInterval:  time.Second,
	}
}

// Worker polls the job_queue and executes runs.
type Worker struct {
	db     *database.DB
	docker *docker.Client
	cfg    Config

	activeRuns atomic.Int32
	wg         sync.WaitGroup
	stopCh     chan struct{}
}

// New creates a new Worker.
func New(db *database.DB, dockerClient *docker.Client, cfg Config) *Worker {
	if cfg.MaxConcurrent <= 0 {
		cfg.MaxConcurrent = 5
	}
	if cfg.PollInterval <= 0 {
		cfg.PollInterval = time.Second
	}

	return &Worker{
		db:     db,
		docker: dockerClient,
		cfg:    cfg,
		stopCh: make(chan struct{}),
	}
}

// Run starts the worker poll loop. Blocks until ctx is cancelled.
func (w *Worker) Run(ctx context.Context) {
	log.Printf("[worker] Started (maxConcurrent=%d, pollInterval=%s)", w.cfg.MaxConcurrent, w.cfg.PollInterval)

	ticker := time.NewTicker(w.cfg.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("[worker] Context cancelled, stopping...")
			return
		case <-w.stopCh:
			log.Println("[worker] Stop signal received")
			return
		case <-ticker.C:
			if int(w.activeRuns.Load()) < w.cfg.MaxConcurrent {
				w.pollAndExecute(ctx)
			}
		}
	}
}

// Shutdown gracefully waits for all in-flight runs to complete.
func (w *Worker) Shutdown(timeout time.Duration) {
	close(w.stopCh)

	done := make(chan struct{})
	go func() {
		w.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		log.Println("[worker] All runs completed")
	case <-time.After(timeout):
		log.Println("[worker] Shutdown timed out, some runs may be orphaned")
	}
}

// ActiveRuns returns the number of currently executing runs.
func (w *Worker) ActiveRuns() int {
	return int(w.activeRuns.Load())
}

// queuedJob holds the joined data from job_queue + jobs.
type queuedJob struct {
	QueueID        uuid.UUID
	RunID          uuid.UUID
	JobID          uuid.UUID
	JobName        string
	Image          string
	Command        []string
	EnvJSON        []byte
	MemoryMB       int
	CPUMillicores  int
	TimeoutSeconds int
}

// pollAndExecute claims one job from the queue using SKIP LOCKED and executes it.
func (w *Worker) pollAndExecute(ctx context.Context) {
	tx, err := w.db.Pool.Begin(ctx)
	if err != nil {
		return
	}

	var qj queuedJob
	err = tx.QueryRow(ctx, `
		SELECT q.id, q.run_id, q.job_id,
		       j.name, j.image, j.command, j.env,
		       j.memory_mb, j.cpu_millicores, j.timeout_seconds
		FROM job_queue q
		JOIN jobs j ON j.id = q.job_id
		WHERE q.picked_at IS NULL
		  AND q.scheduled_at <= now()
		ORDER BY q.priority DESC, q.scheduled_at ASC
		LIMIT 1
		FOR UPDATE OF q SKIP LOCKED
	`).Scan(
		&qj.QueueID, &qj.RunID, &qj.JobID,
		&qj.JobName, &qj.Image, &qj.Command, &qj.EnvJSON,
		&qj.MemoryMB, &qj.CPUMillicores, &qj.TimeoutSeconds,
	)
	if err != nil {
		tx.Rollback(ctx)
		return // No work available (pgx.ErrNoRows) or error
	}

	// Mark as picked
	_, err = tx.Exec(ctx, `UPDATE job_queue SET picked_at = now() WHERE id = $1`, qj.QueueID)
	if err != nil {
		tx.Rollback(ctx)
		return
	}

	if err := tx.Commit(ctx); err != nil {
		return
	}

	// Parse env
	var env map[string]string
	_ = json.Unmarshal(qj.EnvJSON, &env)

	job := models.Job{
		ID:             qj.JobID,
		Name:           qj.JobName,
		Image:          qj.Image,
		Command:        qj.Command,
		Env:            env,
		MemoryMB:       qj.MemoryMB,
		CPUMillicores:  qj.CPUMillicores,
		TimeoutSeconds: qj.TimeoutSeconds,
	}

	// Execute in background
	w.activeRuns.Add(1)
	w.wg.Add(1)
	go func() {
		defer w.wg.Done()
		defer w.activeRuns.Add(-1)
		w.executeRun(job, qj.RunID, qj.QueueID)
	}()
}

// executeRun pulls the image, creates a container, runs it, and captures the result.
func (w *Worker) executeRun(job models.Job, runID, queueID uuid.UUID) {
	ctx := context.Background()
	startedAt := time.Now()

	// Panic recovery
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[worker] PANIC in run %s: %v", runID, r)
			w.failRun(ctx, runID, startedAt, fmt.Sprintf("panic: %v", r))
		}
	}()

	log.Printf("[worker] Executing run %s for job %s (image: %s)", runID, job.Name, job.Image)

	// Mark as running
	if _, err := w.db.Pool.Exec(ctx, `
		UPDATE job_runs SET status = 'running'::run_status, started_at = $1, heartbeat_at = $1
		WHERE id = $2
	`, startedAt, runID); err != nil {
		log.Printf("[worker] ERROR marking run %s as running: %v", runID, err)
		return
	}

	// Pull image
	if err := w.docker.PullImage(ctx, job.Image); err != nil {
		w.failRun(ctx, runID, startedAt, fmt.Sprintf("image pull failed: %v", err))
		w.cleanupQueue(ctx, queueID)
		return
	}

	// Create container
	containerName := fmt.Sprintf("orbex-%s-%s", job.Name, runID.String()[:8])
	containerID, err := w.docker.CreateContainer(ctx, docker.ContainerConfig{
		Name:          containerName,
		Image:         job.Image,
		Command:       job.Command,
		Env:           job.Env,
		MemoryMB:      job.MemoryMB,
		CPUMillicores: job.CPUMillicores,
	})
	if err != nil {
		w.failRun(ctx, runID, startedAt, fmt.Sprintf("container create failed: %v", err))
		w.cleanupQueue(ctx, queueID)
		return
	}

	// Store container ID
	_, _ = w.db.Pool.Exec(ctx, `UPDATE job_runs SET container_id = $1 WHERE id = $2`, containerID, runID)

	// Set up wait channel BEFORE starting (avoid race with fast-exiting containers)
	type waitResult struct {
		exitCode int64
		err      error
	}
	waitCh := make(chan waitResult, 1)
	go func() {
		exitCode, err := w.docker.WaitContainer(ctx, containerID)
		waitCh <- waitResult{exitCode, err}
	}()

	// Start container
	if err := w.docker.StartContainer(ctx, containerID); err != nil {
		w.failRun(ctx, runID, startedAt, fmt.Sprintf("container start failed: %v", err))
		_ = w.docker.RemoveContainer(ctx, containerID)
		w.cleanupQueue(ctx, queueID)
		return
	}

	// Start heartbeat emitter
	heartbeatCtx, heartbeatCancel := context.WithCancel(ctx)
	go w.emitHeartbeat(heartbeatCtx, runID)

	// Wait for container to exit (with timeout enforcement)
	var result struct {
		exitCode int64
		err      error
	}
	timedOut := false

	if job.TimeoutSeconds > 0 {
		timer := time.NewTimer(time.Duration(job.TimeoutSeconds) * time.Second)
		select {
		case wr := <-waitCh:
			timer.Stop()
			result.exitCode = wr.exitCode
			result.err = wr.err
		case <-timer.C:
			timedOut = true
			log.Printf("[worker] Run %s timed out after %ds — killing container", runID, job.TimeoutSeconds)
			_ = w.docker.StopContainer(ctx, containerID, 5)
			wr := <-waitCh // Wait for container to actually stop
			result.exitCode = wr.exitCode
			result.err = wr.err
		}
	} else {
		wr := <-waitCh
		result.exitCode = wr.exitCode
		result.err = wr.err
	}

	heartbeatCancel()
	duration := time.Since(startedAt)

	// Capture logs (GetLogs already demuxes via stdcopy)
	logStr, err := w.docker.GetLogs(ctx, containerID, "all")
	if err != nil {
		log.Printf("[worker] Warning: failed to get logs for %s: %v", runID, err)
	}

	// Determine final status
	var status string
	exitCode := result.exitCode

	if timedOut {
		status = "failed"
		_, updateErr := w.db.Pool.Exec(ctx, `
			UPDATE job_runs SET 
				status = 'failed'::run_status, exit_code = $1, 
				error_message = $2, finished_at = $3, duration_ms = $4, 
				logs_tail = $5, heartbeat_at = NULL
			WHERE id = $6
		`, exitCode, fmt.Sprintf("timeout exceeded (%ds limit)", job.TimeoutSeconds),
			time.Now(), duration.Milliseconds(), logStr, runID)
		if updateErr != nil {
			log.Printf("[worker] ERROR updating timeout status for %s: %v", runID, updateErr)
		}
	} else if result.err != nil {
		status = "failed"
		errMsg := result.err.Error()
		_, updateErr := w.db.Pool.Exec(ctx, `
			UPDATE job_runs SET 
				status = 'failed'::run_status, exit_code = $1, error_message = $2,
				finished_at = $3, duration_ms = $4, logs_tail = $5, heartbeat_at = NULL
			WHERE id = $6
		`, exitCode, errMsg, time.Now(), duration.Milliseconds(), logStr, runID)
		if updateErr != nil {
			log.Printf("[worker] ERROR updating failed status for %s: %v", runID, updateErr)
		}
	} else if exitCode == 0 {
		status = "succeeded"
		_, updateErr := w.db.Pool.Exec(ctx, `
			UPDATE job_runs SET 
				status = 'succeeded'::run_status, exit_code = 0,
				finished_at = $1, duration_ms = $2, logs_tail = $3, heartbeat_at = NULL
			WHERE id = $4
		`, time.Now(), duration.Milliseconds(), logStr, runID)
		if updateErr != nil {
			log.Printf("[worker] ERROR updating succeeded status for %s: %v", runID, updateErr)
		}
	} else {
		status = "failed"
		_, updateErr := w.db.Pool.Exec(ctx, `
			UPDATE job_runs SET 
				status = 'failed'::run_status, exit_code = $1,
				error_message = $2, finished_at = $3, duration_ms = $4, 
				logs_tail = $5, heartbeat_at = NULL
			WHERE id = $6
		`, exitCode, fmt.Sprintf("exit code %d", exitCode), time.Now(), duration.Milliseconds(), logStr, runID)
		if updateErr != nil {
			log.Printf("[worker] ERROR updating failed status for %s: %v", runID, updateErr)
		}
	}

	// Cleanup
	w.cleanupQueue(ctx, queueID)
	_ = w.docker.RemoveContainer(ctx, containerID)

	log.Printf("[worker] Run %s completed: status=%s exitCode=%d duration=%dms logs=%d bytes",
		runID, status, exitCode, duration.Milliseconds(), len(logStr))
}

// failRun marks a run as failed.
func (w *Worker) failRun(ctx context.Context, runID uuid.UUID, startedAt time.Time, errorMsg string) {
	duration := time.Since(startedAt)
	_, err := w.db.Pool.Exec(ctx, `
		UPDATE job_runs SET 
			status = 'failed'::run_status, error_message = $1,
			finished_at = $2, duration_ms = $3, heartbeat_at = NULL
		WHERE id = $4
	`, errorMsg, time.Now(), duration.Milliseconds(), runID)
	if err != nil {
		log.Printf("[worker] ERROR marking run %s as failed: %v", runID, err)
	}
	log.Printf("[worker] Run %s failed: %s", runID, errorMsg)
}

// cleanupQueue removes the queue item for a completed run.
func (w *Worker) cleanupQueue(ctx context.Context, queueID uuid.UUID) {
	_, _ = w.db.Pool.Exec(ctx, `DELETE FROM job_queue WHERE id = $1`, queueID)
}
