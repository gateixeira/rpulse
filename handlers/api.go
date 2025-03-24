package handlers

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"time"

	"github.com/gateixeira/rpulse/internal/config"
	"github.com/gateixeira/rpulse/internal/database"
	"github.com/gateixeira/rpulse/internal/utils"
	"github.com/gateixeira/rpulse/models"
	"github.com/gateixeira/rpulse/pkg/logger"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type dataResult struct {
	value interface{}
	err   error
}

type APIHandler struct {
	db database.DatabaseInterface
}

func NewAPIHandler(db database.DatabaseInterface) *APIHandler {
	return &APIHandler{db: db}
}

// ValidateGitHubWebhook middleware validates the GitHub webhook signature
func ValidateGitHubWebhook(config *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get the webhook secret from the app state
		webhookSecret := config.Vars.WebhookSecret
		if webhookSecret == "" {
			logger.Logger.Warn("Warning: GITHUB_WEBHOOK_SECRET not set, webhook signature validation disabled")
			c.Next()
			return
		}

		// Get the signature from headers
		signature := c.GetHeader("X-Hub-Signature-256")
		if signature == "" {
			logger.Logger.Error("Webhook validation failed: Missing X-Hub-Signature-256 header")
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Missing signature header"})
			c.Abort()
			return
		}

		// Read the request body
		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			logger.Logger.Error("Error reading request body: %v", zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read request body"})
			c.Abort()
			return
		}

		// Restore the request body
		c.Request.Body = io.NopCloser(bytes.NewReader(body))

		// Compute the expected signature
		mac := hmac.New(sha256.New, []byte(webhookSecret))
		mac.Write(body)
		expectedSignature := "sha256=" + hex.EncodeToString(mac.Sum(nil))

		// Validate the signature
		if !hmac.Equal([]byte(signature), []byte(expectedSignature)) {
			logger.Logger.Error("Webhook validation failed: Invalid signature")
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid signature"})
			c.Abort()
			return
		}

		// Signature is valid, continue processing
		c.Next()
	}
}

// GetRunningCount returns the current count of running workflows and historical data
func (h *APIHandler) GetRunningCount() gin.HandlerFunc {
	return func(c *gin.Context) {
		period := c.DefaultQuery("period", "all")

		historicalChan := make(chan dataResult)
		queueTimeChan := make(chan dataResult)
		peakDemandChan := make(chan dataResult)
		githubHostedChan := make(chan dataResult)
		selfHostedChan := make(chan dataResult)
		queuedChan := make(chan dataResult)

		go func() {
			data, err := h.db.GetHistoricalDataByPeriod(period)
			historicalChan <- dataResult{value: data, err: err}
		}()

		go func() {
			avgTime, err := h.db.GetAverageQueueTime()
			queueTimeChan <- dataResult{value: avgTime, err: err}
		}()

		go func() {
			peak, timestamp, err := h.db.CalculatePeakDemand(period)
			peakDemandChan <- dataResult{value: map[string]interface{}{
				"count":     peak,
				"timestamp": timestamp,
			}, err: err}
		}()

		go func() {
			workflows, err := h.db.GetRunningJobs(utils.GetRunnerType([]string{}))
			githubHostedChan <- dataResult{value: len(workflows), err: err}
		}()

		go func() {
			workflows, err := h.db.GetRunningJobs(utils.GetRunnerType([]string{"self-hosted"}))
			selfHostedChan <- dataResult{value: len(workflows), err: err}
		}()

		go func() {
			count, err := h.db.CountQueuedJobs()
			if err == nil && count > 0 {
				logger.Logger.Debug("Queued jobs", zap.Int("count", count))
			}
			queuedChan <- dataResult{value: count, err: err}
		}()

		historical := <-historicalChan
		queueTime := <-queueTimeChan
		peakDemand := <-peakDemandChan
		githubHosted := <-githubHostedChan
		selfHosted := <-selfHostedChan
		queued := <-queuedChan

		for _, result := range []dataResult{historical, queueTime, peakDemand, githubHosted, selfHosted, queued} {
			if result.err != nil {
				logger.Logger.Error("Error retrieving data", zap.Error(result.err))
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve data"})
				return
			}
		}

		c.JSON(http.StatusOK, gin.H{
			"current_count_github_hosted": githubHosted.value.(int),
			"current_count_self_hosted":   selfHosted.value.(int),
			"current_queued_count":        queued.value.(int),
			"historical_data":             historical.value.([]models.HistoricalEntry),
			"avg_queue_time_ms":           queueTime.value.(time.Duration).Milliseconds(),
			"peak_demand":                 peakDemand.value.(map[string]interface{})["count"],
			"peak_demand_timestamp":       peakDemand.value.(map[string]interface{})["timestamp"],
			"period":                      period,
		})
	}
}
