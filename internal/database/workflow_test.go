package database

import (
	"database/sql"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gateixeira/rpulse/models"
)

func TestAddOrUpdateJob(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Error creating mock database: %v", err)
	}
	defer db.Close()

	DB = db
	dbWrapper := &DBWrapper{}

	jobID := int64(123)
	status := models.JobStatusQueued
	runnerType := models.RunnerTypeSelfHosted
	createdAt := time.Now()
	startedAt := createdAt.Add(time.Minute)
	completedAt := startedAt.Add(time.Minute)

	// Successful insert case
	mock.ExpectExec("INSERT INTO workflow_jobs").
		WithArgs(jobID, string(status), string(runnerType), createdAt, startedAt, completedAt).
		WillReturnResult(sqlmock.NewResult(1, 1))

	err = dbWrapper.AddOrUpdateJob(jobID, status, runnerType, createdAt, startedAt, completedAt)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// Test retry on error
	mock.ExpectExec("INSERT INTO workflow_jobs").
		WithArgs(jobID, string(status), string(runnerType), createdAt, startedAt, completedAt).
		WillReturnError(sql.ErrConnDone)
	mock.ExpectExec("INSERT INTO workflow_jobs").
		WithArgs(jobID, string(status), string(runnerType), createdAt, startedAt, completedAt).
		WillReturnResult(sqlmock.NewResult(1, 1))

	err = dbWrapper.AddOrUpdateJob(jobID, status, runnerType, createdAt, startedAt, completedAt)
	if err != nil {
		t.Errorf("Expected no error after retry, got %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("There were unfulfilled expectations: %s", err)
	}
}

func TestCountQueuedJobs(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Error creating mock database: %v", err)
	}
	defer db.Close()

	DB = db
	dbWrapper := &DBWrapper{}

	rows := sqlmock.NewRows([]string{"count"}).AddRow(5)
	mock.ExpectQuery("SELECT COUNT.*FROM workflow_jobs").
		WithArgs(string(models.JobStatusQueued)).
		WillReturnRows(rows)

	count, err := dbWrapper.CountQueuedJobs()
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if count != 5 {
		t.Errorf("Expected count of 5, got %d", count)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("There were unfulfilled expectations: %s", err)
	}
}

func TestGetRunningJobs(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Error creating mock database: %v", err)
	}
	defer db.Close()

	DB = db
	dbWrapper := &DBWrapper{}

	expectedIDs := []string{"123", "456", "789"}
	rows := sqlmock.NewRows([]string{"id"})
	for _, id := range expectedIDs {
		rows.AddRow(id)
	}

	mock.ExpectQuery("SELECT id FROM workflow_jobs").
		WithArgs(string(models.RunnerTypeSelfHosted), string(models.JobStatusInProgress)).
		WillReturnRows(rows)

	ids, err := dbWrapper.GetRunningJobs(models.RunnerTypeSelfHosted)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if len(ids) != len(expectedIDs) {
		t.Errorf("Expected %d IDs, got %d", len(expectedIDs), len(ids))
	}

	for i, id := range ids {
		if id != expectedIDs[i] {
			t.Errorf("Expected ID %s at position %d, got %s", expectedIDs[i], i, id)
		}
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("There were unfulfilled expectations: %s", err)
	}
}

func TestAddQueueTimeDuration(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Error creating mock database: %v", err)
	}
	defer db.Close()

	DB = db
	dbWrapper := &DBWrapper{}

	jobID := int64(123)
	createdAt := time.Now()
	duration := time.Duration(5 * time.Minute)

	mock.ExpectExec("INSERT INTO queue_time_durations").
		WithArgs(jobID, createdAt, duration.Milliseconds(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	err = dbWrapper.AddQueueTimeDuration(jobID, createdAt, duration)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("There were unfulfilled expectations: %s", err)
	}
}

func TestGetAverageQueueTime(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Error creating mock database: %v", err)
	}
	defer db.Close()

	DB = db
	dbWrapper := &DBWrapper{}

	t.Run("with valid average", func(t *testing.T) {
		expectedAvg := float64(300000) // 5 minutes in milliseconds
		rows := sqlmock.NewRows([]string{"avg"}).AddRow(expectedAvg)
		mock.ExpectQuery("SELECT AVG.*FROM queue_time_durations").
			WillReturnRows(rows)

		avgDuration, err := dbWrapper.GetAverageQueueTime()
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}

		expectedDuration := time.Duration(expectedAvg) * time.Millisecond
		if avgDuration != expectedDuration {
			t.Errorf("Expected duration %v, got %v", expectedDuration, avgDuration)
		}
	})

	t.Run("with null average", func(t *testing.T) {
		rows := sqlmock.NewRows([]string{"avg"}).AddRow(nil)
		mock.ExpectQuery("SELECT AVG.*FROM queue_time_durations").
			WillReturnRows(rows)

		avgDuration, err := dbWrapper.GetAverageQueueTime()
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}

		if avgDuration != 0 {
			t.Errorf("Expected duration 0, got %v", avgDuration)
		}
	})

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("There were unfulfilled expectations: %s", err)
	}
}
