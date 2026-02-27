package worker

import (
	"context"
	"log"
	"time"

	"github.com/robfig/cron/v3"
)

const schedulerInterval = 60 * time.Second

// RunScheduler checks for jobs with cron schedules and enqueues runs when due.
// Blocks until ctx is cancelled.
func (w *Worker) RunScheduler(ctx context.Context) {
	log.Printf("[scheduler] Started (interval=%s)", schedulerInterval)

	// Run immediately on startup, then every interval
	w.checkScheduledJobs(ctx)

	ticker := time.NewTicker(schedulerInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("[scheduler] Stopped")
			return
		case <-ticker.C:
			w.checkScheduledJobs(ctx)
		}
	}
}

// scheduledJob holds a job with a cron schedule.
type scheduledJob struct {
	ID       [16]byte // uuid
	UserID   [16]byte
	Schedule string
}

// checkScheduledJobs finds all active jobs with schedules and enqueues runs if they're due.
func (w *Worker) checkScheduledJobs(ctx context.Context) {
	rows, err := w.db.Pool.Query(ctx, `
		SELECT j.id, j.user_id, j.schedule
		FROM jobs j
		WHERE j.schedule IS NOT NULL 
		  AND j.is_active = true
	`)
	if err != nil {
		log.Printf("[scheduler] ERROR querying scheduled jobs: %v", err)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var jobID, userID [16]byte
		var schedule string
		if err := rows.Scan(&jobID, &userID, &schedule); err != nil {
			log.Printf("[scheduler] ERROR scanning job: %v", err)
			continue
		}

		if w.shouldEnqueue(ctx, jobID, schedule) {
			w.enqueueScheduledRun(ctx, jobID, userID)
		}
	}
}

// shouldEnqueue checks if a scheduled job is due for a new run.
func (w *Worker) shouldEnqueue(ctx context.Context, jobID [16]byte, schedule string) bool {
	// Parse cron expression
	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	sched, err := parser.Parse(schedule)
	if err != nil {
		log.Printf("[scheduler] Invalid cron expression for job %x: %v", jobID[:4], err)
		return false
	}

	// Check if there's already a recent pending/running run
	var activeCount int
	err = w.db.Pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM job_runs
		WHERE job_id = $1 AND status IN ('pending'::run_status, 'running'::run_status, 'paused'::run_status)
	`, jobID).Scan(&activeCount)
	if err != nil {
		return false
	}
	if activeCount > 0 {
		return false // Don't stack runs — wait for current one to finish
	}

	// Find the most recent completed run
	var lastRunAt *time.Time
	_ = w.db.Pool.QueryRow(ctx, `
		SELECT created_at FROM job_runs
		WHERE job_id = $1
		ORDER BY created_at DESC
		LIMIT 1
	`, jobID).Scan(&lastRunAt)

	// Determine if it's time for a new run
	now := time.Now()
	if lastRunAt == nil {
		// Never ran before — enqueue now
		return true
	}

	// Get the next scheduled time after the last run
	nextRun := sched.Next(*lastRunAt)
	return now.After(nextRun) || now.Equal(nextRun)
}

// enqueueScheduledRun creates a new job_run and enqueues it.
func (w *Worker) enqueueScheduledRun(ctx context.Context, jobID, userID [16]byte) {
	// Create run record
	var runID [16]byte
	err := w.db.Pool.QueryRow(ctx, `
		INSERT INTO job_runs (job_id, user_id, status)
		VALUES ($1, $2, 'pending'::run_status)
		RETURNING id
	`, jobID, userID).Scan(&runID)
	if err != nil {
		log.Printf("[scheduler] ERROR creating run for job %x: %v", jobID[:4], err)
		return
	}

	// Enqueue
	_, err = w.db.Pool.Exec(ctx, `
		INSERT INTO job_queue (job_id, run_id)
		VALUES ($1, $2)
	`, jobID, runID)
	if err != nil {
		log.Printf("[scheduler] ERROR enqueuing run for job %x: %v", jobID[:4], err)
		return
	}

	log.Printf("[scheduler] Enqueued run %x for scheduled job %x", runID[:4], jobID[:4])
}
