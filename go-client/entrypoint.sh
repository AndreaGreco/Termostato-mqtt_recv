#!/bin/sh
set -e

echo "Waiting for database to be ready..."
until pg_isready -h "$PG_HOST" -U "$PG_USER"; do
  echo "Database is unavailable - sleeping"
  sleep 2
done

echo "Database is ready - checking migration status"
cd /app

# Esegui le migrazioni (se necessario)
# Tern salterà automaticamente le migrazioni già applicate
echo "Running migrations (if needed)..."
tern migrate -c /app/tern.conf -m /app/migrations

echo "Migration check completed"
echo "Starting application..."
exec /app/mqtt_client
