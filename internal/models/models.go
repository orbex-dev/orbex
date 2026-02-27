package models

import (
	"time"

	"github.com/google/uuid"
)

// RunStatus represents the state of a job run.
type RunStatus string

const (
	RunStatusPending   RunStatus = "pending"
	RunStatusRunning   RunStatus = "running"
	RunStatusSucceeded RunStatus = "succeeded"
	RunStatusFailed    RunStatus = "failed"
	RunStatusPaused    RunStatus = "paused"
	RunStatusCancelled RunStatus = "cancelled"
)

// User represents a registered user.
type User struct {
	ID        uuid.UUID `json:"id"`
	Email     string    `json:"email"`
	Password  string    `json:"-"` // Never serialize password
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// APIKey represents an API key for authentication.
type APIKey struct {
	ID        uuid.UUID  `json:"id"`
	UserID    uuid.UUID  `json:"user_id"`
	Name      string     `json:"name"`
	KeyHash   string     `json:"-"` // Never serialize hash
	Prefix    string     `json:"prefix"`
	LastUsed  *time.Time `json:"last_used,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
}

// Job represents a job definition.
type Job struct {
	ID             uuid.UUID         `json:"id"`
	UserID         uuid.UUID         `json:"user_id"`
	Name           string            `json:"name"`
	Image          string            `json:"image"`
	Command        []string          `json:"command,omitempty"`
	Env            map[string]string `json:"env,omitempty"`
	MemoryMB       int               `json:"memory_mb"`
	CPUMillicores  int               `json:"cpu_millicores"`
	TimeoutSeconds int               `json:"timeout_seconds"`
	Schedule       *string           `json:"schedule,omitempty"`
	WebhookToken   *string           `json:"webhook_token,omitempty"`
	IsActive       bool              `json:"is_active"`
	CreatedAt      time.Time         `json:"created_at"`
	UpdatedAt      time.Time         `json:"updated_at"`
}

// JobRun represents a single execution of a job.
type JobRun struct {
	ID           uuid.UUID  `json:"id"`
	JobID        uuid.UUID  `json:"job_id"`
	UserID       uuid.UUID  `json:"user_id"`
	Status       RunStatus  `json:"status"`
	ContainerID  *string    `json:"container_id,omitempty"`
	ExitCode     *int       `json:"exit_code,omitempty"`
	ErrorMessage *string    `json:"error_message,omitempty"`
	StartedAt    *time.Time `json:"started_at,omitempty"`
	FinishedAt   *time.Time `json:"finished_at,omitempty"`
	PausedAt     *time.Time `json:"paused_at,omitempty"`
	HeartbeatAt  *time.Time `json:"heartbeat_at,omitempty"`
	DurationMs   *int64     `json:"duration_ms,omitempty"`
	LogsTail     *string    `json:"logs_tail,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
}

// QueueItem represents a job waiting to be executed.
type QueueItem struct {
	ID          uuid.UUID  `json:"id"`
	JobID       uuid.UUID  `json:"job_id"`
	RunID       uuid.UUID  `json:"run_id"`
	Priority    int        `json:"priority"`
	ScheduledAt time.Time  `json:"scheduled_at"`
	PickedAt    *time.Time `json:"picked_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
}

// --- Request/Response types ---

// CreateJobRequest is the payload for creating a new job.
type CreateJobRequest struct {
	Name           string            `json:"name"`
	Image          string            `json:"image"`
	Command        []string          `json:"command,omitempty"`
	Env            map[string]string `json:"env,omitempty"`
	MemoryMB       int               `json:"memory_mb,omitempty"`
	CPUMillicores  int               `json:"cpu_millicores,omitempty"`
	TimeoutSeconds int               `json:"timeout_seconds,omitempty"`
	Schedule       *string           `json:"schedule,omitempty"`
}

// RegisterRequest is the payload for user registration.
type RegisterRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// APIKeyResponse includes the full key (only returned once at creation).
type APIKeyResponse struct {
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	Key       string    `json:"key"` // Full key — only shown once
	Prefix    string    `json:"prefix"`
	CreatedAt time.Time `json:"created_at"`
}

// ErrorResponse is the standard error format.
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message,omitempty"`
}
