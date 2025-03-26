package handlers

import (
	"html/template"
	"net/http"
	"net/url"
	"time"

	"github.com/gateixeira/rpulse/internal/utils"
	"github.com/gin-gonic/gin"
)

type DashboardHandler struct {
	template *template.Template
}

func NewDashboardHandler() *DashboardHandler {
	// Parse templates at initialization
	tmpl, err := template.ParseFiles("templates/dashboard.html")
	if err != nil {
		// Log error but don't panic - will attempt to reload template on each request if not loaded
		return &DashboardHandler{}
	}
	return &DashboardHandler{
		template: tmpl,
	}
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

		// Create template data
		templateData := gin.H{
			"csrfToken": csrfToken,
			"timestamp": time.Now().Unix(), // Add timestamp to prevent caching
		}

		// If template wasn't loaded at initialization, try loading it now
		if h.template == nil {
			tmpl, err := template.ParseFiles("templates/dashboard.html")
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load template"})
				return
			}
			h.template = tmpl
		}

		// Render template with data
		c.Header("Cache-Control", "no-store, must-revalidate")
		c.Header("Pragma", "no-cache")
		c.Header("Expires", "0")
		c.HTML(http.StatusOK, "dashboard.html", templateData)
	}
}
