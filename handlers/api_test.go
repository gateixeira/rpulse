package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

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
	router.GET("/running-count", apiHandler.GetRunningCount())

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
