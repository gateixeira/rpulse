package handlers

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gateixeira/rpulse/internal/config"
	"github.com/gateixeira/rpulse/models"
	"github.com/gateixeira/rpulse/pkg/logger"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap/zaptest"
)

func setupAPITest(t *testing.T) (*gin.Engine, *MockDB) {
	gin.SetMode(gin.TestMode)
	logger.Logger = zaptest.NewLogger(t)

	mockDB := new(MockDB)
	router := gin.New()
	apiHandler := NewAPIHandler(mockDB)
	webhookHandler := NewWebhookHandler(mockDB)
	mockConfig := &config.Config{
		Vars: config.Vars{
			WebhookSecret: "test-secret",
		},
	}

	router.GET("/running-count", apiHandler.GetRunningCount())
	router.POST("/webhook", ValidateGitHubWebhook(mockConfig), webhookHandler.Handle())

	return router, mockDB
}

func TestAPIHandler_GetRunningCount_Success(t *testing.T) {
	router, mockDB := setupAPITest(t)

	historicalData := []models.HistoricalEntry{
		{
			Timestamp:         time.Now().Format(time.RFC3339),
			CountGitHubHosted: 1,
			CountSelfHosted:   2,
			CountQueued:       0,
		},
	}

	// Setup mock expectations
	mockDB.On("GetHistoricalDataByPeriod", "all").Return(historicalData, nil)
	mockDB.On("GetAverageQueueTime").Return(time.Duration(5*time.Minute), nil)
	mockDB.On("CalculatePeakDemand", "all").Return(10, "2025-03-24T12:00:00Z", nil)
	mockDB.On("GetRunningJobs", models.RunnerTypeGitHubHosted).Return([]string{"job1"}, nil)
	mockDB.On("GetRunningJobs", models.RunnerTypeSelfHosted).Return([]string{"job2", "job3"}, nil)
	mockDB.On("CountQueuedJobs").Return(1, nil)

	req, _ := http.NewRequest("GET", "/running-count", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "current_count_github_hosted")
	assert.Contains(t, w.Body.String(), "current_count_self_hosted")
	assert.Contains(t, w.Body.String(), "historical_data")
	assert.Contains(t, w.Body.String(), "avg_queue_time_ms")
	assert.Contains(t, w.Body.String(), "peak_demand")

	mockDB.AssertExpectations(t)
}

func TestAPIHandler_GetRunningCount_DatabaseErrors(t *testing.T) {
	testCases := []struct {
		name       string
		setupMocks func(*MockDB)
	}{
		{
			name: "GetHistoricalDataByPeriod error",
			setupMocks: func(mockDB *MockDB) {
				mockDB.On("GetHistoricalDataByPeriod", mock.Anything).
					Return([]models.HistoricalEntry{}, assert.AnError)
				mockDB.On("GetAverageQueueTime").Return(time.Duration(5*time.Minute), nil)
				mockDB.On("CalculatePeakDemand", mock.Anything).Return(10, "2025-03-24T12:00:00Z", nil)
				mockDB.On("GetRunningJobs", models.RunnerTypeGitHubHosted).Return([]string{"job1"}, nil)
				mockDB.On("GetRunningJobs", models.RunnerTypeSelfHosted).Return([]string{"job2"}, nil)
				mockDB.On("CountQueuedJobs").Return(0, nil)
			},
		},
		{
			name: "GetAverageQueueTime error",
			setupMocks: func(mockDB *MockDB) {
				mockDB.On("GetHistoricalDataByPeriod", mock.Anything).
					Return([]models.HistoricalEntry{}, nil)
				mockDB.On("GetAverageQueueTime").
					Return(time.Duration(0), assert.AnError)
				mockDB.On("CalculatePeakDemand", mock.Anything).Return(10, "2025-03-24T12:00:00Z", nil)
				mockDB.On("GetRunningJobs", models.RunnerTypeGitHubHosted).Return([]string{"job1"}, nil)
				mockDB.On("GetRunningJobs", models.RunnerTypeSelfHosted).Return([]string{"job2"}, nil)
				mockDB.On("CountQueuedJobs").Return(0, nil)
			},
		},
		{
			name: "CalculatePeakDemand error",
			setupMocks: func(mockDB *MockDB) {
				mockDB.On("GetHistoricalDataByPeriod", mock.Anything).
					Return([]models.HistoricalEntry{}, nil)
				mockDB.On("GetAverageQueueTime").
					Return(time.Duration(5*time.Minute), nil)
				mockDB.On("CalculatePeakDemand", mock.Anything).
					Return(0, "", assert.AnError)
				mockDB.On("GetRunningJobs", models.RunnerTypeGitHubHosted).Return([]string{"job1"}, nil)
				mockDB.On("GetRunningJobs", models.RunnerTypeSelfHosted).Return([]string{"job2"}, nil)
				mockDB.On("CountQueuedJobs").Return(0, nil)
			},
		},
		{
			name: "GetRunningJobs GitHub-hosted error",
			setupMocks: func(mockDB *MockDB) {
				mockDB.On("GetHistoricalDataByPeriod", mock.Anything).
					Return([]models.HistoricalEntry{}, nil)
				mockDB.On("GetAverageQueueTime").
					Return(time.Duration(5*time.Minute), nil)
				mockDB.On("CalculatePeakDemand", mock.Anything).
					Return(10, "2025-03-24T12:00:00Z", nil)
				mockDB.On("GetRunningJobs", models.RunnerTypeGitHubHosted).
					Return(nil, assert.AnError)
				mockDB.On("GetRunningJobs", models.RunnerTypeSelfHosted).
					Return([]string{"job2"}, nil)
				mockDB.On("CountQueuedJobs").Return(0, nil)
			},
		},
		{
			name: "GetRunningJobs Self-hosted error",
			setupMocks: func(mockDB *MockDB) {
				mockDB.On("GetHistoricalDataByPeriod", mock.Anything).
					Return([]models.HistoricalEntry{}, nil)
				mockDB.On("GetAverageQueueTime").
					Return(time.Duration(5*time.Minute), nil)
				mockDB.On("CalculatePeakDemand", mock.Anything).
					Return(10, "2025-03-24T12:00:00Z", nil)
				mockDB.On("GetRunningJobs", models.RunnerTypeGitHubHosted).
					Return([]string{"job1"}, nil)
				mockDB.On("GetRunningJobs", models.RunnerTypeSelfHosted).
					Return(nil, assert.AnError)
				mockDB.On("CountQueuedJobs").Return(0, nil)
			},
		},
		{
			name: "CountQueuedJobs error",
			setupMocks: func(mockDB *MockDB) {
				mockDB.On("GetHistoricalDataByPeriod", mock.Anything).
					Return([]models.HistoricalEntry{}, nil)
				mockDB.On("GetAverageQueueTime").
					Return(time.Duration(5*time.Minute), nil)
				mockDB.On("CalculatePeakDemand", mock.Anything).
					Return(10, "2025-03-24T12:00:00Z", nil)
				mockDB.On("GetRunningJobs", models.RunnerTypeGitHubHosted).Return([]string{"job1"}, nil)
				mockDB.On("GetRunningJobs", models.RunnerTypeSelfHosted).Return([]string{"job2"}, nil)
				mockDB.On("CountQueuedJobs").Return(0, assert.AnError)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			router, mockDB := setupAPITest(t)

			tc.setupMocks(mockDB)

			req, _ := http.NewRequest("GET", "/running-count", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusInternalServerError, w.Code)
			assert.Contains(t, w.Body.String(), "Failed to retrieve data")

			mockDB.AssertExpectations(t)
		})
	}
}

func TestAPIHandler_GetRunningCount_WithPeriod(t *testing.T) {
	router, mockDB := setupAPITest(t)

	historicalData := []models.HistoricalEntry{
		{
			Timestamp:         time.Now().Format(time.RFC3339),
			CountGitHubHosted: 1,
			CountSelfHosted:   2,
			CountQueued:       0,
		},
	}

	period := "24h"

	// Setup mock expectations
	mockDB.On("GetHistoricalDataByPeriod", period).Return(historicalData, nil)
	mockDB.On("GetAverageQueueTime").Return(time.Duration(5*time.Minute), nil)
	mockDB.On("CalculatePeakDemand", period).Return(10, "2025-03-24T12:00:00Z", nil)
	mockDB.On("GetRunningJobs", models.RunnerTypeGitHubHosted).Return([]string{"job1"}, nil)
	mockDB.On("GetRunningJobs", models.RunnerTypeSelfHosted).Return([]string{"job2", "job3"}, nil)
	mockDB.On("CountQueuedJobs").Return(1, nil)

	req, _ := http.NewRequest("GET", "/running-count?period=24h", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"period":"24h"`)
	mockDB.AssertExpectations(t)
}

func TestValidateGitHubWebhook_Success(t *testing.T) {
	router, mockDB := setupAPITest(t)

	// Create a sample payload and compute its HMAC
	now := time.Now()
	event := models.WebhookEvent{
		Action: "in_progress",
		WorkflowJob: models.WebhookWorkflowJob{
			ID:          123,
			Labels:      []string{"self-hosted"},
			CreatedAt:   now,
			StartedAt:   now.Add(5 * time.Minute),
			CompletedAt: time.Time{},
		},
	}

	payload, _ := json.Marshal(event)
	mac := computeHMAC(payload, "test-secret")

	// Setup mock expectations for webhook handling
	mockDB.On("AddOrUpdateJob",
		event.WorkflowJob.ID,
		models.JobStatus(event.Action),
		models.RunnerTypeSelfHosted,
		timeMatch(event.WorkflowJob.CreatedAt),
		timeMatch(event.WorkflowJob.StartedAt),
		timeMatch(event.WorkflowJob.CompletedAt)).
		Return(nil)

	mockDB.On("AddQueueTimeDuration",
		event.WorkflowJob.ID,
		timeMatch(event.WorkflowJob.CreatedAt),
		5*time.Minute).
		Return(nil)

	mockDB.On("GetRunningJobs", models.RunnerTypeSelfHosted).
		Return([]string{"123", "456"}, nil)

	mockDB.On("GetRunningJobs", models.RunnerTypeGitHubHosted).
		Return([]string{"789"}, nil)

	mockDB.On("CountQueuedJobs").Return(3, nil)

	mockDB.On("AddHistoricalEntry", mock.MatchedBy(func(entry models.HistoricalEntry) bool {
		return entry.CountSelfHosted == 2 &&
			entry.CountGitHubHosted == 1 &&
			entry.CountQueued == 3
	})).Return(nil)

	req, _ := http.NewRequest("POST", "/webhook", bytes.NewBuffer(payload))
	req.Header.Set("X-Hub-Signature-256", "sha256="+hex.EncodeToString(mac))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "success")
	mockDB.AssertExpectations(t)
}

func TestValidateGitHubWebhook_Failures(t *testing.T) {
	testCases := []struct {
		name           string
		signature      string
		expectedStatus int
		expectedError  string
	}{
		{
			name:           "Missing signature",
			signature:      "",
			expectedStatus: http.StatusUnauthorized,
			expectedError:  "Missing signature header",
		},
		{
			name:           "Invalid signature",
			signature:      "sha256=invalid",
			expectedStatus: http.StatusUnauthorized,
			expectedError:  "Invalid signature",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			router, _ := setupAPITest(t)

			// Create a sample payload
			payload := []byte(`{"test":"data"}`)

			req, _ := http.NewRequest("POST", "/webhook", bytes.NewBuffer(payload))
			if tc.signature != "" {
				req.Header.Set("X-Hub-Signature-256", tc.signature)
			}

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tc.expectedStatus, w.Code)
			assert.Contains(t, w.Body.String(), tc.expectedError)
		})
	}
}

func TestValidateGitHubWebhook_NoSecret(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger.Logger = zaptest.NewLogger(t)

	router := gin.New()

	// Create config with empty webhook secret
	mockConfig := &config.Config{
		Vars: config.Vars{
			WebhookSecret: "",
		},
	}

	router.POST("/webhook", ValidateGitHubWebhook(mockConfig))

	req, _ := http.NewRequest("POST", "/webhook", bytes.NewBufferString("test"))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Should allow the request through when no secret is set
	assert.Equal(t, http.StatusOK, w.Code)
}

func computeHMAC(payload []byte, secret string) []byte {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	return mac.Sum(nil)
}
