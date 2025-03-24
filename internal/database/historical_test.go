package database

import (
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gateixeira/rpulse/models"
)

func TestAddHistoricalEntry(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Error creating mock database: %v", err)
	}
	defer db.Close()
	DB = db
	dbWrapper := &DBWrapper{}

	entry := models.HistoricalEntry{
		Timestamp:         "2025-03-24 10:00:00",
		CountSelfHosted:   5,
		CountGitHubHosted: 10,
		CountQueued:       2,
	}

	mock.ExpectExec("INSERT INTO historical_entries").
		WithArgs(entry.Timestamp, entry.CountSelfHosted, entry.CountGitHubHosted, entry.CountQueued).
		WillReturnResult(sqlmock.NewResult(1, 1))

	err = dbWrapper.AddHistoricalEntry(entry)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("There were unfulfilled expectations: %s", err)
	}
}

func TestGetHistoricalDataByPeriod(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Error creating mock database: %v", err)
	}
	defer db.Close()
	DB = db
	dbWrapper := &DBWrapper{}

	testCases := []struct {
		name     string
		period   string
		mockRows *sqlmock.Rows
		wantLen  int
		wantErr  bool
	}{
		{
			name:   "hourly data",
			period: "hour",
			mockRows: sqlmock.NewRows([]string{"timestamp", "count_self_hosted", "count_github_hosted", "count_queued"}).
				AddRow("2025-03-24 10:00:00", 5, 10, 2).
				AddRow("2025-03-24 10:15:00", 6, 11, 3),
			wantLen: 2,
			wantErr: false,
		},
		{
			name:   "daily data",
			period: "day",
			mockRows: sqlmock.NewRows([]string{"timestamp", "count_self_hosted", "count_github_hosted", "count_queued"}).
				AddRow("2025-03-24", 15, 30, 5),
			wantLen: 1,
			wantErr: false,
		},
		{
			name:    "invalid period",
			period:  "invalid",
			wantLen: 0,
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.mockRows != nil {
				mock.ExpectQuery("SELECT").WillReturnRows(tc.mockRows)
			}

			entries, err := dbWrapper.GetHistoricalDataByPeriod(tc.period)
			if (err != nil) != tc.wantErr {
				t.Errorf("GetHistoricalDataByPeriod() error = %v, wantErr %v", err, tc.wantErr)
				return
			}

			if len(entries) != tc.wantLen {
				t.Errorf("GetHistoricalDataByPeriod() got %v entries, want %v", len(entries), tc.wantLen)
			}
		})
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("There were unfulfilled expectations: %s", err)
	}
}

func TestCalculatePeakDemand(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Error creating mock database: %v", err)
	}
	defer db.Close()
	DB = db
	dbWrapper := &DBWrapper{}

	testCases := []struct {
		name         string
		period       string
		mockRows     *sqlmock.Rows
		expectedPeak int
		expectedTime string
		expectError  bool
	}{
		{
			name:   "hourly peak",
			period: "hour",
			mockRows: sqlmock.NewRows([]string{"peak", "timestamp"}).
				AddRow(25, "2025-03-24 10:00:00"),
			expectedPeak: 25,
			expectedTime: "2025-03-24 10:00:00",
			expectError:  false,
		},
		{
			name:   "daily peak",
			period: "day",
			mockRows: sqlmock.NewRows([]string{"peak_total", "bucket"}).
				AddRow(50, "2025-03-24"),
			expectedPeak: 50,
			expectedTime: "2025-03-24",
			expectError:  false,
		},
		{
			name:   "no data",
			period: "hour",
			mockRows: sqlmock.NewRows([]string{"peak", "timestamp"}).
				AddRow(nil, nil),
			expectedPeak: 0,
			expectedTime: "",
			expectError:  false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mock.ExpectQuery("SELECT").WillReturnRows(tc.mockRows)

			peak, timestamp, err := dbWrapper.CalculatePeakDemand(tc.period)
			if (err != nil) != tc.expectError {
				t.Errorf("CalculatePeakDemand() error = %v, expectError %v", err, tc.expectError)
				return
			}

			if peak != tc.expectedPeak {
				t.Errorf("CalculatePeakDemand() peak = %v, want %v", peak, tc.expectedPeak)
			}

			if timestamp != tc.expectedTime {
				t.Errorf("CalculatePeakDemand() timestamp = %v, want %v", timestamp, tc.expectedTime)
			}
		})
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("There were unfulfilled expectations: %s", err)
	}
}
