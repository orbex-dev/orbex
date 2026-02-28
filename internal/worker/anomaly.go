package worker

import (
	"context"
	"fmt"
	"log"
	"math"

	"github.com/google/uuid"
)

const (
	// MinRunsForBaseline is the minimum number of completed runs before anomaly detection kicks in.
	MinRunsForBaseline = 10
	// AnomalyThreshold is the number of standard deviations beyond mean to flag as anomalous.
	AnomalyThreshold = 3.0
)

// jobStats represents tracked statistics for a job's run durations.
type jobStats struct {
	RunCount      int
	AvgDurationMs int64
	StddevMs      int64
	MinDurationMs int64
	MaxDurationMs int64
}

// updateJobStats updates the running statistics for a job after a successful run completes.
// Uses Welford's online algorithm for computing mean and standard deviation incrementally.
func (w *Worker) updateJobStats(ctx context.Context, jobID uuid.UUID, durationMs int64) {
	// Upsert: get current stats or create new entry
	var stats jobStats
	err := w.db.Pool.QueryRow(ctx, `
		SELECT run_count, avg_duration_ms, stddev_duration_ms, min_duration_ms, max_duration_ms
		FROM job_stats WHERE job_id = $1
	`, jobID).Scan(&stats.RunCount, &stats.AvgDurationMs, &stats.StddevMs, &stats.MinDurationMs, &stats.MaxDurationMs)
	if err != nil {
		// First run — initialize stats
		_, err = w.db.Pool.Exec(ctx, `
			INSERT INTO job_stats (job_id, run_count, avg_duration_ms, stddev_duration_ms, min_duration_ms, max_duration_ms, updated_at)
			VALUES ($1, 1, $2, 0, $2, $2, now())
			ON CONFLICT (job_id) DO NOTHING
		`, jobID, durationMs)
		if err != nil {
			log.Printf("[anomaly] Failed to initialize stats for job %s: %v", jobID, err)
		}
		return
	}

	// Welford's algorithm: incrementally update mean and variance
	n := int64(stats.RunCount + 1)
	oldMean := float64(stats.AvgDurationMs)
	newMean := oldMean + (float64(durationMs)-oldMean)/float64(n)

	// For stddev: we track (n-1) * variance, then stddev = sqrt(variance)
	// On the Nth sample: M2_new = M2_old + (x - oldMean) * (x - newMean)
	oldVariance := float64(stats.StddevMs * stats.StddevMs)
	oldM2 := oldVariance * float64(n-1)
	newM2 := oldM2 + (float64(durationMs)-oldMean)*(float64(durationMs)-newMean)
	newStddev := int64(0)
	if n > 1 {
		newStddev = int64(math.Sqrt(newM2 / float64(n-1)))
	}

	// Update min/max
	minMs := stats.MinDurationMs
	if durationMs < minMs || minMs == 0 {
		minMs = durationMs
	}
	maxMs := stats.MaxDurationMs
	if durationMs > maxMs {
		maxMs = durationMs
	}

	_, err = w.db.Pool.Exec(ctx, `
		UPDATE job_stats SET
			run_count = $1, avg_duration_ms = $2, stddev_duration_ms = $3,
			min_duration_ms = $4, max_duration_ms = $5, updated_at = now()
		WHERE job_id = $6
	`, n, int64(newMean), newStddev, minMs, maxMs, jobID)
	if err != nil {
		log.Printf("[anomaly] Failed to update stats for job %s: %v", jobID, err)
	}
}

// checkAnomaly checks if the current run duration is anomalous based on historical stats.
// Returns true if the duration exceeds mean + AnomalyThreshold * stddev.
func (w *Worker) checkAnomaly(ctx context.Context, jobID uuid.UUID, currentDurationMs int64) (isAnomaly bool, details string) {
	var stats jobStats
	err := w.db.Pool.QueryRow(ctx, `
		SELECT run_count, avg_duration_ms, stddev_duration_ms
		FROM job_stats WHERE job_id = $1
	`, jobID).Scan(&stats.RunCount, &stats.AvgDurationMs, &stats.StddevMs)
	if err != nil || stats.RunCount < MinRunsForBaseline {
		return false, "" // Not enough data yet
	}

	threshold := float64(stats.AvgDurationMs) + AnomalyThreshold*float64(stats.StddevMs)
	if float64(currentDurationMs) > threshold {
		return true, fmt.Sprintf(
			"Duration %dms exceeds baseline (avg=%dms, stddev=%dms, threshold=%.0fms, %dσ deviation)",
			currentDurationMs, stats.AvgDurationMs, stats.StddevMs, threshold,
			int64((float64(currentDurationMs)-float64(stats.AvgDurationMs))/float64(stats.StddevMs)),
		)
	}
	return false, ""
}
