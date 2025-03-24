package handlers

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gateixeira/rpulse/internal/config"
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

// ValidateGitHubWebhook middleware validates the GitHub webhook signature
func ValidateGitHubWebhook(config *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		webhookSecret := config.Vars.WebhookSecret
		if webhookSecret == "" {
			logger.Logger.Warn("Warning: GITHUB_WEBHOOK_SECRET not set, webhook signature validation disabled")
			c.Next()
			return
		}

		signature := c.GetHeader("X-Hub-Signature-256")
		if signature == "" {
			logger.Logger.Error("Webhook validation failed: Missing X-Hub-Signature-256 header")
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Missing signature header"})
			c.Abort()
			return
		}

		signatureHash := signature
		if len(signature) > 7 && signature[0:7] == "sha256=" {
			signatureHash = signature[7:]
		}

		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			logger.Logger.Error("Error reading request body", zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read request body"})
			c.Abort()
			return
		}

		c.Request.Body = io.NopCloser(bytes.NewReader(body))

		mac := hmac.New(sha256.New, []byte(webhookSecret))
		mac.Write(body)
		expectedSignature := hex.EncodeToString(mac.Sum(nil))

		expectedBytes, err := hex.DecodeString(expectedSignature)
		if err != nil {
			logger.Logger.Error("Error decoding expected signature", zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to validate signature"})
			c.Abort()
			return
		}

		receivedBytes, err := hex.DecodeString(signatureHash)
		if err != nil {
			logger.Logger.Error("Error decoding received signature", zap.Error(err))
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid signature format"})
			c.Abort()
			return
		}

		if !hmac.Equal(expectedBytes, receivedBytes) {
			logger.Logger.Error("Webhook validation failed: Invalid signature")
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid signature"})
			c.Abort()
			return
		}

		c.Next()
	}
}

// Handle processes incoming webhook events
func (h *WebhookHandler) Handle() gin.HandlerFunc {
	return func(c *gin.Context) {
		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			logger.Logger.Error("Failed to read request body", zap.Error(err))
			c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to read request body"})
			return
		}

		decodedBody, err := url.QueryUnescape(string(body))
		if err != nil {
			logger.Logger.Error("Failed to decode URL-encoded payload", zap.Error(err))
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid URL-encoded payload"})
			return
		}

		const prefix = "payload="
		if !strings.HasPrefix(decodedBody, prefix) {
			logger.Logger.Error("Missing payload parameter")
			c.JSON(http.StatusBadRequest, gin.H{"error": "Missing payload parameter"})
			return
		}
		jsonData := decodedBody[len(prefix):]

		var event models.WebhookEvent
		if err := json.Unmarshal([]byte(jsonData), &event); err != nil {
			logger.Logger.Error("Failed to parse JSON payload",
				zap.Error(err),
				zap.String("jsonData", jsonData),
			)
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

		if err := h.db.AddOrUpdateJob(job.ID, job.Status,
			job.RunnerType, job.CreatedAt, job.StartedAt, job.CompletedAt); err != nil {
			logger.Logger.Error("Error saving job to database", zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save job"})
			return
		}

		if job.Status == models.JobStatusInProgress {
			h.handleInProgressJob(job)
		}

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
