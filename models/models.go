package models

import "time"

// HistoricalEntry represents a point in time with the count of running workflows
type HistoricalEntry struct {
	Timestamp         string `json:"timestamp"`
	CountSelfHosted   int    `json:"count_self_hosted"`
	CountGitHubHosted int    `json:"count_github_hosted"`
	CountQueued       int    `json:"count_queued"`
}

// RunnerType represents the type of runner (GitHub-hosted or self-hosted)
type RunnerType string

const (
	RunnerTypeGitHubHosted RunnerType = "github-hosted"
	RunnerTypeSelfHosted   RunnerType = "self-hosted"
)

// JobStatus represents the status of a workflow job
type JobStatus string

const (
	JobStatusQueued     JobStatus = "queued"
	JobStatusInProgress JobStatus = "in_progress"
	JobStatusCompleted  JobStatus = "completed"
)

// QueueTimeEntry tracks a workflow's queue state with its timestamp
type QueueTimeEntry struct {
	WorkflowID      string    `json:"workflow_id"`
	QueuedTimestamp time.Time `json:"queued_timestamp"`
}

// WebhookEvent represents the incoming webhook payload
type WebhookEvent struct {
	Action      string             `json:"action"`
	WorkflowJob WebhookWorkflowJob `json:"workflow_job"`
}

// WebhookWorkflowJob is part of the webhook payload identifying a workflow job
type WebhookWorkflowJob struct {
	ID          int64     `json:"id"`
	Labels      []string  `json:"labels"`
	CreatedAt   time.Time `json:"created_at"`
	StartedAt   time.Time `json:"started_at"`
	CompletedAt time.Time `json:"completed_at"`
}

// WorkflowJob represents a job in the workflow_jobs table
type WorkflowJob struct {
	ID          int64      `json:"id"`
	Status      JobStatus  `json:"status"`
	RunnerType  RunnerType `json:"runner_type"`
	CreatedAt   time.Time  `json:"created_at"`
	StartedAt   time.Time  `json:"started_at"`
	CompletedAt time.Time  `json:"completed_at"`
}
