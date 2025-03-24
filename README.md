# RPulse - GitHub Actions Runner Monitoring

A Go application for tracking GitHub Actions workflow runners demand

![Screenshot 2025-03-24 at 13 26 52](https://github.com/user-attachments/assets/e3b256ab-0bd0-4a67-988c-63de46516c96)

## Overview

This application tracks running GitHub Actions workflows and provides a visual dashboard of demand over time. It includes:

- A webhook endpoint to receive workflow status events
- A dashboard to visualize runners demand over time

## Requirements

- Go 1.23 or higher
- PostgreSQL database
- Dependencies (automatically installed with go mod):
  - github.com/gin-gonic/gin
  - github.com/golang-migrate/migrate/v4
  - github.com/lib/pq
  - go.uber.org/zap

## Installation

1. Clone this repository
2. Install dependencies:

```bash
make deps
```

## Running the Application

### Local Development

There are several make commands available:

```bash
make build    # Build the rpulse binary
make run      # Run the application
make test     # Run tests
make clean    # Clean build files
make lint     # Run linter
make deps     # Install dependencies
```

The server will start on port 8080.

### Docker Deployment

The application can be run using Docker:

```bash
make docker-build  # Build the Docker image
make docker-run    # Run with docker-compose
```

This will start:

- The main application
- A TimescaleDB instance (PostgreSQL with time-series extensions)

The docker-compose setup includes:

- Automatic database initialization
- Health checks for database connectivity
- Volume persistence for TimescaleDB data
- Configurable environment variables

### Environment Variables

The application uses the following environment variables:

- `PORT`: Server port (default: 8080)
- `DB_HOST`: PostgreSQL host (default: localhost)
- `DB_PORT`: PostgreSQL port (default: 5432)
- `DB_USER`: PostgreSQL user (default: postgres)
- `DB_PASSWORD`: PostgreSQL password (required)
- `DB_NAME`: PostgreSQL database name (default: rpulse)
- `WEBHOOK_SECRET`: Secret used to validate incoming GitHub webhook requests
- `LOG_LEVEL`: Logging level (default: info)

If `WEBHOOK_SECRET` is not set, webhook signature validation will be disabled (not recommended for production).

## API Endpoints

- `GET /` - Simple health check endpoint
- `POST /webhook` - Webhook endpoint for workflow events (requires valid signature)
- `GET /running-count` - Get current count of running workflows and historical data
- `GET /dashboard` - Dashboard UI to visualize running workflows

## Webhook Security

The webhook endpoint is secured using GitHub's webhook signature validation. When configuring your GitHub webhook:

1. Generate a secure random string to use as your webhook secret
2. Set this secret in GitHub when creating the webhook
3. Set the same secret as the `WEBHOOK_SECRET` environment variable when running this application

GitHub will include a signature header (`X-Hub-Signature-256`) with each webhook request, which this application validates before processing the webhook data.

## Setting up GitHub Webhook

To configure a webhook in your GitHub repository:

1. Go to your repository settings
2. Click on "Webhooks" in the left sidebar
3. Click "Add webhook"
4. Configure the webhook:
   - Payload URL: Your application's `/webhook` endpoint (e.g., `https://your-domain.com/webhook`)
   - Content type: `application/json`
   - Secret: Generate a secure random string and use it here
   - Events: Select "Workflow jobs" under "Individual events"
   - Active: Check this box to enable the webhook

### Local Testing with ngrok

For local development, you'll need to make your local webhook endpoint publicly accessible. You can use [ngrok](https://ngrok.com/) for this:

1. Install ngrok from https://ngrok.com/download
2. Start your application locally first
3. In a new terminal, start ngrok:
   ```bash
   ngrok http 8080
   ```
4. ngrok will provide a public URL (e.g., `https://a1b2c3d4.ngrok.io`)
5. Update your GitHub webhook's payload URL to: `https://a1b2c3d4.ngrok.io/webhook`
6. Copy your webhook secret and update it in your `.env` file:
   ```
   WEBHOOK_SECRET=your_generated_secret_here
   ```
7. Restart your application to apply the new secret

Note: The ngrok URL changes every time you restart ngrok (unless you have a paid plan). Remember to update the webhook URL in GitHub when this happens.

## Webhook Format

The webhook endpoint expects POST requests with the following JSON format:

```json
{
  "action": "queued"|"in_progress"|"completed",
  "labels": ["self-hosted"],
  "workflow_job": {
    "id": "workflow-id",
    "labels": ["self-hosted"],
    "created_at": "2025-03-20T22:10:14Z",
    "started_at": "2025-03-20T22:10:18Z",
    "completed_at": "2025-03-20T22:10:24Z",
  }
}
```

## Testing

You can manually test the application by visiting:

- Dashboard: `http://localhost:8080/dashboard`

### Load Test

Load tests can be run with [k6](https://github.com/grafana/k6).

1. Run the application with `docker compose`
2. Run k6: `k6 run load-tests/webhook-load.js`
