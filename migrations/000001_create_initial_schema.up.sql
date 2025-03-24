CREATE EXTENSION IF NOT EXISTS timescaledb;

CREATE TABLE IF NOT EXISTS historical_entries (
    id SERIAL,
    timestamp TIMESTAMPTZ NOT NULL,
    count_self_hosted INTEGER NOT NULL,
    count_github_hosted INTEGER NOT NULL,
    count_queued INTEGER NOT NULL,
    CONSTRAINT historical_entries_pkey PRIMARY KEY (id, timestamp)
);

SELECT create_hypertable('historical_entries', 'timestamp', if_not_exists => TRUE);

CREATE TABLE IF NOT EXISTS workflow_jobs (
    id BIGINT,
    status TEXT NOT NULL,
    runner_type TEXT,
    created_at TIMESTAMPTZ NOT NULL,
    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    CONSTRAINT workflow_jobs_pkey PRIMARY KEY (id, created_at)
);

SELECT create_hypertable('workflow_jobs', 'created_at', if_not_exists => TRUE);

CREATE TABLE IF NOT EXISTS queue_time_durations (
    id SERIAL,
    job_id BIGINT NOT NULL,
    job_created_at TIMESTAMPTZ NOT NULL,
    duration_ms BIGINT NOT NULL,
    recorded_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT queue_time_durations_pkey PRIMARY KEY (id, recorded_at)
);

SELECT create_hypertable('queue_time_durations', 'recorded_at', if_not_exists => TRUE);
    
CREATE OR REPLACE VIEW daily_runner_stats AS
SELECT
    time_bucket('3 minutes', timestamp) AS bucket,
    AVG(count_self_hosted) AS avg_self_hosted,
    AVG(count_github_hosted) AS avg_github_hosted,
    AVG(count_queued) AS avg_queued,
    MAX(count_self_hosted + count_github_hosted + count_queued) AS peak_total
FROM historical_entries
WHERE timestamp >= NOW() - INTERVAL '1 day'
GROUP BY bucket
ORDER BY bucket;

CREATE OR REPLACE VIEW weekly_runner_stats AS
SELECT
    time_bucket('30 minutes', timestamp) AS bucket,
    AVG(count_self_hosted) AS avg_self_hosted,
    AVG(count_github_hosted) AS avg_github_hosted,
    AVG(count_queued) AS avg_queued,
    MAX(count_self_hosted + count_github_hosted + count_queued) AS peak_total
FROM historical_entries
WHERE timestamp >= NOW() - INTERVAL '1 week'
GROUP BY bucket
ORDER BY bucket;

CREATE OR REPLACE VIEW monthly_runner_stats AS
SELECT
    time_bucket('2 hours', timestamp) AS bucket,
    AVG(count_self_hosted) AS avg_self_hosted,
    AVG(count_github_hosted) AS avg_github_hosted,
    AVG(count_queued) AS avg_queued,
    MAX(count_self_hosted + count_github_hosted + count_queued) AS peak_total
FROM historical_entries
WHERE timestamp >= NOW() - INTERVAL '1 month'
GROUP BY bucket
ORDER BY bucket;

SELECT add_retention_policy('historical_entries', INTERVAL '30 days');
SELECT add_retention_policy('workflow_jobs', INTERVAL '30 days');
SELECT add_retention_policy('queue_time_durations', INTERVAL '30 days');