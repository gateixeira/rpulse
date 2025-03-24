package database

import (
	"time"

	"github.com/gateixeira/rpulse/models"
)

// DatabaseInterface defines the contract for database operations
type DatabaseInterface interface {
	AddOrUpdateJob(ID int64, status models.JobStatus, runnerType models.RunnerType, createdAt, startedAt, completedAt time.Time) error
	CountQueuedJobs() (int, error)
	GetRunningJobs(runnerType models.RunnerType) ([]string, error)
	AddHistoricalEntry(entry models.HistoricalEntry) error
	GetAverageQueueTime() (time.Duration, error)
	AddQueueTimeDuration(ID int64, createdAt time.Time, duration time.Duration) error
	GetHistoricalDataByPeriod(period string) ([]models.HistoricalEntry, error)
	CalculatePeakDemand(period string) (int, string, error)
}

// DBWrapper wraps the actual DB instance and implements DatabaseInterface
type DBWrapper struct{}

// NewDBWrapper creates a new DBWrapper instance
func NewDBWrapper() DatabaseInterface {
	return &DBWrapper{}
}
