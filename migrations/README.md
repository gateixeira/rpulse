# Database Migrations

This directory contains database migrations for the RPulse application.

## Prerequisites

The application requires PostgreSQL with TimescaleDB extension installed. For TimescaleDB installation instructions, please refer to the [official documentation](https://docs.timescale.com/self-hosted/latest/install/installation-docker/).

## Migration Files

Migrations follow the format: `{version}_{description}.{up|down}.sql`

Example:

- 000001_create_initial_schema.up.sql
- 000001_create_initial_schema.down.sql

## Creating New Migrations

Use the following format for new migration files:

1. Increment the version number
2. Add descriptive name
3. Create both up and down migrations
