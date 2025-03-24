package server

import (
	"os"

	"github.com/gateixeira/rpulse/handlers"
	"github.com/gateixeira/rpulse/internal/config"
	"github.com/gateixeira/rpulse/internal/database"
	"github.com/gateixeira/rpulse/pkg/logger"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// SetupAndRun configures the router and starts the server
func SetupAndRun() {
	// Initialize the application state
	config := config.NewConfig()

	logger.InitLogger(config.Vars.LogLevel)
	defer logger.SyncLogger()

	err := database.InitDB(config.GetDSN())
	if err != nil {
		logger.Logger.Error("Failed to initialize database", zap.Error(err))
		os.Exit(1)
	}

	defer func() {
		if err := database.CloseDB(); err != nil {
			logger.Logger.Error("Failed to close database connection", zap.Error(err))
		}
	}()

	// Initialize database wrapper
	db := database.NewDBWrapper()

	// Initialize handlers with dependencies
	webhookHandler := handlers.NewWebhookHandler(db)
	apiHandler := handlers.NewAPIHandler(db)
	dashboardHandler := handlers.NewDashboardHandler()
	rootHandler := handlers.NewRootHandler()

	r := gin.Default()

	r.Static("/static", "./static")
	r.LoadHTMLGlob("templates/*")

	r.GET("/", rootHandler.Root())
	r.POST("/webhook", handlers.ValidateGitHubWebhook(config), webhookHandler.Handle())
	r.GET("/running-count", handlers.ValidateDashboardOrigin(), apiHandler.GetRunningCount())
	r.GET("/dashboard", dashboardHandler.Dashboard())

	logger.Logger.Info("Starting server on :" + config.Vars.Port + "...")
	if err := r.Run(":" + config.Vars.Port); err != nil {
		logger.Logger.Error("Failed to start server", zap.Error(err))
		os.Exit(1)
	}
}
