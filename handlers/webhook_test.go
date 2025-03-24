package handlers

import (
	"bytes"
	"encoding/json"
	"errors"
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

// MockDB is a mock implementation of DatabaseInterface
type MockDB struct {
	mock.Mock
}

func (m *MockDB) AddOrUpdateJob(jobID int64, status models.JobStatus, runnerType models.RunnerType,
	createdAt, startedAt, completedAt time.Time) error {
	args := m.Called(jobID, status, runnerType, createdAt, startedAt, completedAt)
	return args.Error(0)
}

func (m *MockDB) GetRunningJobs(runnerType models.RunnerType) ([]string, error) {
	args := m.Called(runnerType)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]string), args.Error(1)
}

func (m *MockDB) CountQueuedJobs() (int, error) {
	args := m.Called()
	return args.Int(0), args.Error(1)
}

func (m *MockDB) AddHistoricalEntry(entry models.HistoricalEntry) error {
	args := m.Called(entry)
	return args.Error(0)
}

func (m *MockDB) GetHistoricalDataByPeriod(period string) ([]models.HistoricalEntry, error) {
	args := m.Called(period)
	return args.Get(0).([]models.HistoricalEntry), args.Error(1)
}

func (m *MockDB) CalculatePeakDemand(period string) (int, string, error) {
	args := m.Called(period)
	return args.Int(0), args.String(1), args.Error(2)
}

func (m *MockDB) AddQueueTimeDuration(jobID int64, createdAt time.Time, duration time.Duration) error {
	args := m.Called(jobID, createdAt, duration)
	return args.Error(0)
}

func (m *MockDB) GetAverageQueueTime() (time.Duration, error) {
	args := m.Called()
	return args.Get(0).(time.Duration), args.Error(1)
}

func setupTest(t *testing.T) (*gin.Engine, *MockDB) {
	// Set gin to test mode
	gin.SetMode(gin.TestMode)

	// Setup test logger
	logger.Logger = zaptest.NewLogger(t)

	// Create mock DB
	mockDB := new(MockDB)

	// Create router with webhook handler
	router := gin.New()
	handler := NewWebhookHandler(mockDB)
	router.POST("/webhook", handler.Handle())

	return router, mockDB
}

func timeMatch(expected time.Time) interface{} {
	return mock.MatchedBy(func(actual time.Time) bool {
		return expected.Equal(actual)
	})
}

func TestWebhookHandler_Handle_Success(t *testing.T) {
	router, mockDB := setupTest(t)

	// Create test webhook event
	now := time.Now()
	startTime := now.Add(5 * time.Minute)
	event := models.WebhookEvent{
		Action: "in_progress",
		WorkflowJob: models.WebhookWorkflowJob{
			ID:          123,
			Labels:      []string{"self-hosted"},
			CreatedAt:   now,
			StartedAt:   startTime,
			CompletedAt: time.Time{},
		},
	}

	// Setup mock expectations
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

	// Create request
	body, _ := json.Marshal(event)
	req, _ := http.NewRequest("POST", "/webhook", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	// Perform request
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Assert response
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "success")

	// Verify mock expectations
	mockDB.AssertExpectations(t)
}

func TestWebhookHandler_Handle_InvalidJSON(t *testing.T) {
	router, _ := setupTest(t)

	// Create invalid JSON request
	req, _ := http.NewRequest("POST", "/webhook", bytes.NewBufferString("invalid json"))
	req.Header.Set("Content-Type", "application/json")

	// Perform request
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Assert response
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestWebhookHandler_Handle_DatabaseErrors(t *testing.T) {
	testCases := []struct {
		name          string
		setupMocks    func(*MockDB)
		expectedCode  int
		expectedError string
	}{
		{
			name: "AddOrUpdateJob error",
			setupMocks: func(mockDB *MockDB) {
				mockDB.On("AddOrUpdateJob",
					mock.AnythingOfType("int64"),
					mock.AnythingOfType("models.JobStatus"),
					mock.AnythingOfType("models.RunnerType"),
					mock.Anything,
					mock.Anything,
					mock.Anything).
					Return(errors.New("database error"))
			},
			expectedCode:  http.StatusInternalServerError,
			expectedError: "Failed to save job",
		},
		{
			name: "GetRunningJobs self-hosted error",
			setupMocks: func(mockDB *MockDB) {
				mockDB.On("AddOrUpdateJob",
					mock.Anything, mock.Anything, mock.Anything,
					mock.Anything, mock.Anything, mock.Anything).
					Return(nil)
				mockDB.On("AddQueueTimeDuration",
					mock.Anything, mock.Anything, mock.Anything).
					Return(nil)
				mockDB.On("GetRunningJobs", models.RunnerTypeSelfHosted).
					Return(nil, errors.New("database error"))
			},
			expectedCode:  http.StatusInternalServerError,
			expectedError: "Failed to get counts",
		},
		{
			name: "GetRunningJobs github-hosted error",
			setupMocks: func(mockDB *MockDB) {
				mockDB.On("AddOrUpdateJob",
					mock.Anything, mock.Anything, mock.Anything,
					mock.Anything, mock.Anything, mock.Anything).
					Return(nil)
				mockDB.On("AddQueueTimeDuration",
					mock.Anything, mock.Anything, mock.Anything).
					Return(nil)
				mockDB.On("GetRunningJobs", models.RunnerTypeSelfHosted).
					Return([]string{}, nil)
				mockDB.On("GetRunningJobs", models.RunnerTypeGitHubHosted).
					Return(nil, errors.New("database error"))
			},
			expectedCode:  http.StatusInternalServerError,
			expectedError: "Failed to get counts",
		},
		{
			name: "CountQueuedJobs error",
			setupMocks: func(mockDB *MockDB) {
				mockDB.On("AddOrUpdateJob",
					mock.Anything, mock.Anything, mock.Anything,
					mock.Anything, mock.Anything, mock.Anything).
					Return(nil)
				mockDB.On("AddQueueTimeDuration",
					mock.Anything, mock.Anything, mock.Anything).
					Return(nil)
				mockDB.On("GetRunningJobs", models.RunnerTypeSelfHosted).
					Return([]string{}, nil)
				mockDB.On("GetRunningJobs", models.RunnerTypeGitHubHosted).
					Return([]string{}, nil)
				mockDB.On("CountQueuedJobs").
					Return(0, errors.New("database error"))
			},
			expectedCode:  http.StatusInternalServerError,
			expectedError: "Failed to get counts",
		},
		{
			name: "AddHistoricalEntry error",
			setupMocks: func(mockDB *MockDB) {
				mockDB.On("AddOrUpdateJob",
					mock.Anything, mock.Anything, mock.Anything,
					mock.Anything, mock.Anything, mock.Anything).
					Return(nil)
				mockDB.On("AddQueueTimeDuration",
					mock.Anything, mock.Anything, mock.Anything).
					Return(nil)
				mockDB.On("GetRunningJobs", models.RunnerTypeSelfHosted).
					Return([]string{}, nil)
				mockDB.On("GetRunningJobs", models.RunnerTypeGitHubHosted).
					Return([]string{}, nil)
				mockDB.On("CountQueuedJobs").Return(0, nil)
				mockDB.On("AddHistoricalEntry", mock.Anything).
					Return(errors.New("database error"))
			},
			expectedCode:  http.StatusInternalServerError,
			expectedError: "Failed to add historical entry",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			router, mockDB := setupTest(t)

			// Setup test webhook event
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

			// Setup mocks
			tc.setupMocks(mockDB)

			// Create request
			body, _ := json.Marshal(event)
			req, _ := http.NewRequest("POST", "/webhook", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")

			// Perform request
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			// Assert response
			assert.Equal(t, tc.expectedCode, w.Code)
			assert.Contains(t, w.Body.String(), tc.expectedError)

			// Verify mock expectations
			mockDB.AssertExpectations(t)
		})
	}
}
