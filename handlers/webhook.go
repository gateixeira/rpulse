package handlers

import (
	"net/http"
	"time"

	"github.com/gateixeira/rpulse/internal/database"
	"github.com/gateixeira/rpulse/internal/utils"
	"github.com/gateixeira/rpulse/models"
	"github.com/gateixeira/rpulse/pkg/logger"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type WebhookHandler struct {
	db database.DatabaseInterface
}

func NewWebhookHandler(db database.DatabaseInterface) *WebhookHandler {
	return &WebhookHandler{db: db}
}

func (h *WebhookHandler) handleInProgressJob(job models.WorkflowJob) {
	logger.Logger.Debug("Job is running", zap.Int64("ID", job.ID))

	queueTime := job.StartedAt.Sub(job.CreatedAt)

	if err := h.db.AddQueueTimeDuration(job.ID, job.CreatedAt, queueTime); err != nil {
		logger.Logger.Error("Error adding queue time duration", zap.Error(err))
		// Continue execution even if we fail to add queue time
	}

	logger.Logger.Debug("Job was in queue for", zap.Int64("ID", job.ID), zap.Duration("queueTime", queueTime))
}

// Handle processes incoming webhook events
func (h *WebhookHandler) Handle() gin.HandlerFunc {
	return func(c *gin.Context) {
		var event models.WebhookEvent
		if err := c.BindJSON(&event); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		job := models.WorkflowJob{
			ID:          event.WorkflowJob.ID,
			Status:      models.JobStatus(event.Action),
			RunnerType:  utils.GetRunnerType(event.WorkflowJob.Labels),
			CreatedAt:   event.WorkflowJob.CreatedAt,
			StartedAt:   event.WorkflowJob.StartedAt,
			CompletedAt: event.WorkflowJob.CompletedAt,
		}

		// First save the job
		if err := h.db.AddOrUpdateJob(job.ID, job.Status,
			job.RunnerType, job.CreatedAt, job.StartedAt, job.CompletedAt); err != nil {
			logger.Logger.Error("Error saving job to database", zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save job"})
			return
		}

		// Only proceed with additional processing if job was saved
		if job.Status == models.JobStatusInProgress {
			h.handleInProgressJob(job)
		}

		// Get counts only after successful job save
		selfHostedCount, err := h.db.GetRunningJobs(models.RunnerTypeSelfHosted)
		if err != nil {
			logger.Logger.Error("Error getting self-hosted count", zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get counts"})
			return
		}

		githubHostedCount, err := h.db.GetRunningJobs(models.RunnerTypeGitHubHosted)
		if err != nil {
			logger.Logger.Error("Error getting github-hosted count", zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get counts"})
			return
		}

		queuedCount, err := h.db.CountQueuedJobs()
		if err != nil {
			logger.Logger.Error("Error getting queued count", zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get counts"})
			return
		}

		// Only create historical entry if we got all counts successfully
		historicalEntry := models.HistoricalEntry{
			Timestamp:         time.Now().Format(time.RFC3339),
			CountSelfHosted:   len(selfHostedCount),
			CountGitHubHosted: len(githubHostedCount),
			CountQueued:       queuedCount,
		}

		if err := h.db.AddHistoricalEntry(historicalEntry); err != nil {
			logger.Logger.Error("Error adding historical entry", zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to add historical entry"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"status": "success"})
	}
}
