package worker

import (
	"context"
	"log"
	"time"

	"github.com/google/uuid"
)

const (
	heartbeatInterval = 10 * time.Second
	reaperInterval    = 30 * time.Second
	staleThreshold    = 60 * time.Second
)

// emitHeartbeat updates heartbeat_at for a running job until ctx is cancelled.
func (w *Worker) emitHeartbeat(ctx context.Context, runID uuid.UUID) {
	ticker := time.NewTicker(heartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_, err := w.db.Pool.Exec(ctx, `
				UPDATE job_runs SET heartbeat_at = now()
				WHERE id = $1 AND status IN ('running'::run_status, 'paused'::run_status)
			`, runID)
			if err != nil {
				log.Printf("[heartbeat] Warning: failed to update heartbeat for %s: %v", runID, err)
			}
		}
	}
}

// RunReaper starts the stale run reaper loop. Blocks until ctx is cancelled.
func (w *Worker) RunReaper(ctx context.Context) {
	log.Printf("[reaper] Started (interval=%s, staleThreshold=%s)", reaperInterval, staleThreshold)

	ticker := time.NewTicker(reaperInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("[reaper] Stopped")
			return
		case <-ticker.C:
			w.reapStaleRuns(ctx)
		}
	}
}

// staleRun holds info about a run that has missed its heartbeat.
type staleRun struct {
	ID          uuid.UUID
	ContainerID *string
}

// reapStaleRuns finds runs with expired heartbeats and marks them as failed.
func (w *Worker) reapStaleRuns(ctx context.Context) {
	rows, err := w.db.Pool.Query(ctx, `
		SELECT id, container_id FROM job_runs
		WHERE status IN ('running'::run_status, 'paused'::run_status)
		  AND heartbeat_at IS NOT NULL
		  AND heartbeat_at < now() - $1::interval
	`, staleThreshold.String())
	if err != nil {
		return
	}
	defer rows.Close()

	var stale []staleRun
	for rows.Next() {
		var sr staleRun
		if err := rows.Scan(&sr.ID, &sr.ContainerID); err != nil {
			continue
		}
		stale = append(stale, sr)
	}

	for _, sr := range stale {
		log.Printf("[reaper] Reaping stale run %s (heartbeat expired)", sr.ID)

		// Force kill the container if it still exists
		if sr.ContainerID != nil && *sr.ContainerID != "" {
			if err := w.docker.StopContainer(ctx, *sr.ContainerID, 10); err != nil {
				log.Printf("[reaper] Warning: failed to stop container for %s: %v", sr.ID, err)
			}
			_ = w.docker.RemoveContainer(ctx, *sr.ContainerID)
		}

		// Mark as failed
		_, err := w.db.Pool.Exec(ctx, `
			UPDATE job_runs SET 
				status = 'failed'::run_status, 
				error_message = 'heartbeat timeout: worker may have crashed',
				finished_at = now(),
				heartbeat_at = NULL
			WHERE id = $1
		`, sr.ID)
		if err != nil {
			log.Printf("[reaper] ERROR marking stale run %s as failed: %v", sr.ID, err)
		}

		// Cleanup queue
		_, _ = w.db.Pool.Exec(ctx, `DELETE FROM job_queue WHERE run_id = $1`, sr.ID)

		log.Printf("[reaper] Reaped stale run %s", sr.ID)
	}
}
