#!/bin/sh
set -e

: "${WEBHOOK_SECRET:?WEBHOOK_SECRET must be set}"
: "${DB_PASSWORD:?DB_PASSWORD must be set}"

until PGPASSWORD=$DB_PASSWORD psql -h "$DB_HOST" -U "$DB_USER" -d "$DB_NAME" -c '\q'; do
  >&2 echo "Postgres is unavailable - sleeping"
  sleep 1
done

>&2 echo "Postgres is up - executing command"

exec "$@"