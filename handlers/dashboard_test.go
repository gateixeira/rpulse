package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gateixeira/rpulse/internal/utils"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func setupDashboardTest() *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.Default()
	router.LoadHTMLGlob("../templates/*")
	handler := NewDashboardHandler()
	router.GET("/dashboard", handler.Dashboard())
	return router
}

func TestNewDashboardHandler(t *testing.T) {
	handler := NewDashboardHandler()
	assert.NotNil(t, handler, "NewDashboardHandler should return a non-nil handler")
}

func TestDashboard(t *testing.T) {
	router := setupDashboardTest()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/dashboard", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code, "Response status code should be 200")
	assert.Contains(t, w.Header().Get("Set-Cookie"), utils.CookieName, "Response should set CSRF cookie")
	assert.Contains(t, w.Body.String(), "csrfToken", "Response should include CSRF token in HTML")
}

func TestValidateDashboardOrigin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(ValidateDashboardOrigin())
	router.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	tests := []struct {
		name           string
		referer        string
		host           string
		csrfCookie     string
		csrfHeader     string
		expectedStatus int
	}{
		{
			name:           "valid request",
			referer:        "http://localhost:8080/dashboard",
			host:           "localhost:8080",
			csrfCookie:     "validtoken",
			csrfHeader:     "validtoken",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "missing referer",
			referer:        "",
			host:           "localhost:8080",
			csrfCookie:     "validtoken",
			csrfHeader:     "validtoken",
			expectedStatus: http.StatusForbidden,
		},
		{
			name:           "invalid referer host",
			referer:        "http://malicious.com/dashboard",
			host:           "localhost:8080",
			csrfCookie:     "validtoken",
			csrfHeader:     "validtoken",
			expectedStatus: http.StatusForbidden,
		},
		{
			name:           "invalid referer path",
			referer:        "http://localhost:8080/wrong-path",
			host:           "localhost:8080",
			csrfCookie:     "validtoken",
			csrfHeader:     "validtoken",
			expectedStatus: http.StatusForbidden,
		},
		{
			name:           "missing CSRF cookie",
			referer:        "http://localhost:8080/dashboard",
			host:           "localhost:8080",
			csrfCookie:     "",
			csrfHeader:     "validtoken",
			expectedStatus: http.StatusForbidden,
		},
		{
			name:           "missing CSRF header",
			referer:        "http://localhost:8080/dashboard",
			host:           "localhost:8080",
			csrfCookie:     "validtoken",
			csrfHeader:     "",
			expectedStatus: http.StatusForbidden,
		},
		{
			name:           "mismatched CSRF token",
			referer:        "http://localhost:8080/dashboard",
			host:           "localhost:8080",
			csrfCookie:     "validtoken",
			csrfHeader:     "invalidtoken",
			expectedStatus: http.StatusForbidden,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", "/test", nil)
			req.Host = tt.host
			if tt.referer != "" {
				req.Header.Set("Referer", tt.referer)
			}
			if tt.csrfHeader != "" {
				req.Header.Set(utils.HeaderName, tt.csrfHeader)
			}
			if tt.csrfCookie != "" {
				req.AddCookie(&http.Cookie{
					Name:  utils.CookieName,
					Value: tt.csrfCookie,
				})
			}

			router.ServeHTTP(w, req)
			assert.Equal(t, tt.expectedStatus, w.Code, "Test case: %s", tt.name)
		})
	}
}
