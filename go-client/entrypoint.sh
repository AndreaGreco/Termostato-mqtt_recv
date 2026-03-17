#!/bin/sh
set -e

echo "Waiting for database to be ready..."
until pg_isready -h "$PG_HOST" -U "$PG_USER"; do
  echo "Database is unavailable - sleeping"
  sleep 2
done

echo "Database is ready"
echo "Starting application..."
exec /app/mqtt_client
