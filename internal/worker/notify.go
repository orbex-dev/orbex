package worker

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/google/uuid"
)

// notificationPayload is the JSON sent to webhook URLs on run completion.
type notificationPayload struct {
	Event     string    `json:"event"`
	RunID     string    `json:"run_id"`
	JobID     string    `json:"job_id"`
	JobName   string    `json:"job_name"`
	Status    string    `json:"status"`
	ExitCode  int64     `json:"exit_code"`
	Duration  int64     `json:"duration_ms"`
	Error     string    `json:"error,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

// sendNotification checks if the job has a notify_webhook URL and POSTs the result.
func (w *Worker) sendNotification(ctx context.Context, jobID, runID uuid.UUID, status string, exitCode int64, durationMs int64, errorMsg string) {
	var notifyWebhook *string
	var jobName string

	err := w.db.Pool.QueryRow(ctx, `
		SELECT name, notify_webhook FROM jobs WHERE id = $1
	`, jobID).Scan(&jobName, &notifyWebhook)
	if err != nil || notifyWebhook == nil || *notifyWebhook == "" {
		return // No notification configured
	}

	payload := notificationPayload{
		Event:     "run.completed",
		RunID:     runID.String(),
		JobID:     jobID.String(),
		JobName:   jobName,
		Status:    status,
		ExitCode:  exitCode,
		Duration:  durationMs,
		Error:     errorMsg,
		Timestamp: time.Now(),
	}

	data, _ := json.Marshal(payload)
	go func() {
		client := &http.Client{Timeout: 10 * time.Second}
		resp, err := client.Post(*notifyWebhook, "application/json", bytes.NewReader(data))
		if err != nil {
			log.Printf("[notify] Webhook delivery failed for run %s: %v", runID, err)
			return
		}
		defer resp.Body.Close()
		log.Printf("[notify] Webhook delivered for run %s → %s (status %d)", runID, *notifyWebhook, resp.StatusCode)
	}()
}
