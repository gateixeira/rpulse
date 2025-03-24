package handlers

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
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

func setupWebhookTest(t *testing.T) (*gin.Engine, *MockDB, *config.Config) {
	gin.SetMode(gin.TestMode)
	logger.Logger = zaptest.NewLogger(t)

	mockDB := new(MockDB)
	router := gin.New()
	handler := NewWebhookHandler(mockDB)

	cfg := &config.Config{
		Vars: config.Vars{
			WebhookSecret: "test-secret",
		},
	}

	router.POST("/webhook", ValidateGitHubWebhook(cfg), handler.Handle())
	return router, mockDB, cfg
}

func generateWebhookSignature(payload []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

func TestWebhookHandler_Handle_Success(t *testing.T) {
	router, mockDB, cfg := setupWebhookTest(t)

	// Create raw JSON with properly formatted RFC3339 timestamps
	rawJSON := `{
		"action": "in_progress",
		"workflow_job": {
			"id": 123,
			"labels": ["self-hosted"],
			"created_at": "2025-03-24T17:25:36Z",
			"started_at": "2025-03-24T17:30:36Z",
			"completed_at": "0001-01-01T00:00:00Z"
		}
	}`

	// Parse the timestamps for mock expectations
	var event models.WebhookEvent
	if err := json.Unmarshal([]byte(rawJSON), &event); err != nil {
		t.Fatal("Failed to parse test JSON:", err)
	}

	// Setup mock expectations
	mockDB.On("AddOrUpdateJob",
		event.WorkflowJob.ID,
		models.JobStatus(event.Action),
		models.RunnerTypeSelfHosted,
		event.WorkflowJob.CreatedAt,
		event.WorkflowJob.StartedAt,
		event.WorkflowJob.CompletedAt).
		Return(nil)

	mockDB.On("AddQueueTimeDuration",
		event.WorkflowJob.ID,
		event.WorkflowJob.CreatedAt,
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

	// Create request with signature and payload parameter
	payloadBody := []byte("payload=" + rawJSON)
	req, _ := http.NewRequest("POST", "/webhook", bytes.NewBuffer(payloadBody))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("X-Hub-Signature-256", generateWebhookSignature(payloadBody, cfg.Vars.WebhookSecret))

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
	router, _, cfg := setupWebhookTest(t)

	// Create invalid JSON request with payload parameter
	body := []byte("payload=invalid json")
	req, _ := http.NewRequest("POST", "/webhook", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("X-Hub-Signature-256", generateWebhookSignature(body, cfg.Vars.WebhookSecret))

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
			router, mockDB, cfg := setupWebhookTest(t)

			// Create raw JSON with properly formatted RFC3339 timestamps
			rawJSON := `{
					"action": "in_progress",
					"workflow_job": {
						"id": 123,
						"labels": ["self-hosted"],
						"created_at": "2025-03-24T17:25:36Z",
						"started_at": "2025-03-24T17:30:36Z",
						"completed_at": "0001-01-01T00:00:00Z"
					}
				}`

			// Parse the timestamps for mock setup
			var event models.WebhookEvent
			if err := json.Unmarshal([]byte(rawJSON), &event); err != nil {
				t.Fatal("Failed to parse test JSON:", err)
			}

			// Setup mocks
			tc.setupMocks(mockDB)

			// Create request with payload parameter
			payloadBody := []byte("payload=" + rawJSON)
			req, _ := http.NewRequest("POST", "/webhook", bytes.NewBuffer(payloadBody))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			req.Header.Set("X-Hub-Signature-256", generateWebhookSignature(payloadBody, cfg.Vars.WebhookSecret))

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

func TestValidateGitHubWebhook(t *testing.T) {
	testCases := []struct {
		name           string
		webhookSecret  string
		setupRequest   func(*http.Request, string)
		expectedStatus int
		expectedBody   string
	}{
		{
			name:          "Valid signature",
			webhookSecret: "test-secret",
			setupRequest: func(req *http.Request, secret string) {
				payload := []byte("test-payload")
				signature := generateWebhookSignature(payload, secret)
				req.Header.Set("X-Hub-Signature-256", signature)
				req.Body = io.NopCloser(bytes.NewBuffer(payload))
			},
			expectedStatus: http.StatusOK,
		},
		{
			name:          "Missing signature header",
			webhookSecret: "test-secret",
			setupRequest: func(req *http.Request, secret string) {
				req.Body = io.NopCloser(bytes.NewBufferString("test-payload"))
			},
			expectedStatus: http.StatusUnauthorized,
			expectedBody:   "Missing signature header",
		},
		{
			name:          "Invalid signature",
			webhookSecret: "test-secret",
			setupRequest: func(req *http.Request, secret string) {
				req.Header.Set("X-Hub-Signature-256", "sha256=invalid")
				req.Body = io.NopCloser(bytes.NewBufferString("test-payload"))
			},
			expectedStatus: http.StatusUnauthorized,
			expectedBody:   "Invalid signature",
		},
		{
			name:          "Empty webhook secret",
			webhookSecret: "",
			setupRequest: func(req *http.Request, secret string) {
				req.Body = io.NopCloser(bytes.NewBufferString("test-payload"))
			},
			expectedStatus: http.StatusOK,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			router := gin.New()
			cfg := &config.Config{
				Vars: config.Vars{
					WebhookSecret: tc.webhookSecret,
				},
			}

			// Setup a simple handler that always returns 200 OK
			router.POST("/webhook", ValidateGitHubWebhook(cfg), func(c *gin.Context) {
				c.Status(http.StatusOK)
			})

			req, _ := http.NewRequest("POST", "/webhook", nil)
			tc.setupRequest(req, tc.webhookSecret)

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tc.expectedStatus, w.Code)
			if tc.expectedBody != "" {
				assert.Contains(t, w.Body.String(), tc.expectedBody)
			}
		})
	}
}
