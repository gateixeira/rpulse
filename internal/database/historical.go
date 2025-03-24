package database

import (
	"database/sql"
	"fmt"

	"github.com/gateixeira/rpulse/models"
)

var (
	hourlyQuery = `SELECT 
        timestamp::text,
        count_self_hosted,
        count_github_hosted,
        count_queued
    FROM historical_entries
    WHERE timestamp >= NOW() - INTERVAL '1 hour'
    ORDER BY timestamp`

	aggregatedQuery = `SELECT 
        bucket as timestamp,
        ROUND(avg_self_hosted) as count_self_hosted,
        ROUND(avg_github_hosted) as count_github_hosted,
        ROUND(avg_queued) as count_queued
    FROM %s
    WHERE bucket >= NOW() - INTERVAL '1 %s'
    ORDER BY bucket`

	peakHourlyQuery = `SELECT 
        (count_self_hosted + count_github_hosted + count_queued) as peak,
        timestamp::text
    FROM historical_entries
    WHERE timestamp >= NOW() - INTERVAL '1 hour'
    ORDER BY peak DESC
    LIMIT 1`

	peakAggregatedQuery = `SELECT 
        peak_total, 
        bucket::text
    FROM %s
    WHERE bucket >= NOW() - INTERVAL '1 %s'
    ORDER BY peak_total DESC
    LIMIT 1`

	validPeriods = map[string]string{
		"hour":  "historical_entries",
		"day":   "daily_runner_stats",
		"week":  "weekly_runner_stats",
		"month": "monthly_runner_stats",
	}
)

// AddHistoricalEntry adds a new historical data entry to the database
func (db *DBWrapper) AddHistoricalEntry(entry models.HistoricalEntry) error {
	_, err := DB.Exec(
		"INSERT INTO historical_entries (timestamp, count_self_hosted, count_github_hosted, count_queued) VALUES ($1, $2, $3, $4)",
		entry.Timestamp, entry.CountSelfHosted, entry.CountGitHubHosted, entry.CountQueued,
	)
	return err
}

// GetHistoricalDataByPeriod retrieves historical data entries filtered by time period
func (db *DBWrapper) GetHistoricalDataByPeriod(period string) ([]models.HistoricalEntry, error) {
	tableName := validPeriods[period]

	var query string
	if period == "hour" {
		query = hourlyQuery
	} else {
		query = fmt.Sprintf(aggregatedQuery, tableName, period)
	}

	rows, err := DB.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query historical data: %w", err)
	}
	defer rows.Close()

	var entries []models.HistoricalEntry
	for rows.Next() {
		var entry models.HistoricalEntry
		if err := rows.Scan(&entry.Timestamp, &entry.CountSelfHosted, &entry.CountGitHubHosted, &entry.CountQueued); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}
		entries = append(entries, entry)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return entries, nil
}

// CalculatePeakDemand returns the peak number of concurrent workflows and its timestamp for the given period
func (db *DBWrapper) CalculatePeakDemand(period string) (int, string, error) {
	tableName := validPeriods[period]

	var query string
	if period == "hour" {
		query = peakHourlyQuery
	} else {
		query = fmt.Sprintf(peakAggregatedQuery, tableName, period)
	}

	var peak sql.NullInt64
	var timestamp sql.NullString
	err := DB.QueryRow(query).Scan(&peak, &timestamp)
	if err != nil {
		return 0, "", err
	}

	if !peak.Valid || !timestamp.Valid {
		return 0, "", nil
	}

	return int(peak.Int64), timestamp.String, nil
}
