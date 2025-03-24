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
