package database

import (
	"database/sql"
	"time"

	"github.com/gateixeira/rpulse/models"
)

// AddOrUpdateJob adds or updates a job to the database with retries
func (db *DBWrapper) AddOrUpdateJob(ID int64, status models.JobStatus,
	runnerType models.RunnerType, createdAt time.Time, startedAt time.Time, completedAt time.Time) error {
	var err error
	maxRetries := 3

	for i := 0; i < maxRetries; i++ {
		_, err = DB.Exec(
			`INSERT INTO workflow_jobs (id, status, runner_type, created_at, started_at, completed_at) 
			VALUES ($1, $2, $3, $4, $5, $6)
			ON CONFLICT (id, created_at) DO UPDATE SET
				status = EXCLUDED.status,
				runner_type = EXCLUDED.runner_type,
				started_at = EXCLUDED.started_at,
				completed_at = EXCLUDED.completed_at`,
			ID, string(status), string(runnerType), createdAt, startedAt, completedAt,
		)
		if err == nil {
			return nil
		}
		// Wait a bit before retrying
		time.Sleep(time.Millisecond * 100)
	}
	return err
}

// CountQueuedJobs returns the count of queued jobs
func (db *DBWrapper) CountQueuedJobs() (int, error) {
	var count int
	err := DB.QueryRow("SELECT COUNT(*) FROM workflow_jobs WHERE status = $1", string(models.JobStatusQueued)).Scan(&count)
	return count, err
}

// GetRunningJobs returns all running workflow jobs of a specific type
func (db *DBWrapper) GetRunningJobs(runnerType models.RunnerType) ([]string, error) {
	rows, err := DB.Query(
		"SELECT id FROM workflow_jobs WHERE runner_type = $1 AND status = $2",
		string(runnerType), string(models.JobStatusInProgress),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var IDs []string
	for rows.Next() {
		var workflowID string
		if err := rows.Scan(&workflowID); err != nil {
			return nil, err
		}
		IDs = append(IDs, workflowID)
	}

	return IDs, nil
}

// AddQueueTimeDuration adds a record of queue duration to the database
func (db *DBWrapper) AddQueueTimeDuration(ID int64, createdAt time.Time, duration time.Duration) error {
	_, err := DB.Exec(
		"INSERT INTO queue_time_durations (job_id, job_created_at, duration_ms, recorded_at) VALUES ($1, $2, $3, $4)",
		ID, createdAt, duration.Milliseconds(), time.Now(),
	)
	return err
}

// GetAverageQueueTime calculates and returns the average queue time
func (db *DBWrapper) GetAverageQueueTime() (time.Duration, error) {
	var avgMilliseconds sql.NullFloat64
	err := DB.QueryRow("SELECT AVG(duration_ms) FROM queue_time_durations").Scan(&avgMilliseconds)
	if err != nil {
		return 0, err
	}

	if !avgMilliseconds.Valid {
		return 0, nil
	}

	return time.Duration(int64(avgMilliseconds.Float64)) * time.Millisecond, nil
}
