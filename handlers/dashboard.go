package handlers

import (
	"net/http"
	"net/url"
	"time"

	"github.com/gateixeira/rpulse/internal/utils"
	"github.com/gin-gonic/gin"
)

type DashboardHandler struct{}

func NewDashboardHandler() *DashboardHandler {
	return &DashboardHandler{}
}

// ValidateDashboardOrigin middleware ensures requests come from the dashboard UI
func ValidateDashboardOrigin() gin.HandlerFunc {
	return func(c *gin.Context) {
		referer := c.Request.Header.Get("Referer")
		if referer == "" {
			c.JSON(http.StatusForbidden, gin.H{
				"error": "Access denied. Missing referer header.",
			})
			c.Abort()
			return
		}

		// Parse the referer URL
		refererURL, err := url.Parse(referer)
		if err != nil {
			c.JSON(http.StatusForbidden, gin.H{
				"error": "Access denied. Invalid referer.",
			})
			c.Abort()
			return
		}

		// Get the request host
		requestHost := c.Request.Host

		// Compare hosts and path
		if refererURL.Host != requestHost || refererURL.Path != "/dashboard" {
			c.JSON(http.StatusForbidden, gin.H{
				"error": "Access denied. This endpoint can only be accessed from the local dashboard.",
			})
			c.Abort()
			return
		}

		// Validate CSRF token
		csrfCookie, err := c.Cookie(utils.CookieName)
		if err != nil {
			c.JSON(http.StatusForbidden, gin.H{
				"error": "Invalid CSRF cookie",
			})
			c.Abort()
			return
		}

		csrfHeader := c.GetHeader(utils.HeaderName)
		if csrfHeader == "" || csrfHeader != csrfCookie {
			c.JSON(http.StatusForbidden, gin.H{
				"error": "Invalid CSRF token",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// Dashboard serves the dashboard HTML page
func (h *DashboardHandler) Dashboard() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Generate CSRF token
		csrfToken, err := utils.GenerateCSRFToken()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate security token"})
			return
		}

		// Set secure cookie with CSRF token
		c.SetSameSite(http.SameSiteStrictMode)
		c.SetCookie(
			utils.CookieName,
			csrfToken,
			int(12*time.Hour.Seconds()), // 12 hour expiry
			"/",                         // Path
			"",                          // Domain (empty = current domain only)
			true,                        // Secure (HTTPS only)
			true,                        // HTTP only
		)

		// Pass CSRF token to template
		c.HTML(http.StatusOK, "dashboard.html", gin.H{
			"csrfToken": csrfToken,
		})
	}
}
