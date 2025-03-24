SELECT remove_retention_policy('historical_entries');
SELECT remove_retention_policy('workflow_jobs');
SELECT remove_retention_policy('queue_time_durations');

DROP VIEW IF EXISTS monthly_runner_stats;
DROP VIEW IF EXISTS weekly_runner_stats;
DROP VIEW IF EXISTS daily_runner_stats;

DROP TABLE IF EXISTS queue_time_durations;
DROP TABLE IF EXISTS workflow_jobs;
DROP TABLE IF EXISTS historical_entries;

DROP EXTENSION IF EXISTS timescaledb;