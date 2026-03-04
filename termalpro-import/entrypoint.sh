#!/bin/bash
set -e

CSV_FILE="/data/import-temperature.csv"

echo "=== ThermoProSensor Import Service ==="
echo "Attendendo disponibilità database..."

# Attendi che il database sia pronto
until PGPASSWORD=$PG_PASS psql -h "$PG_HOST" -p "$PG_PORT" -U "$PG_USER" -d "$PG_DB" -c '\q' 2>/dev/null; do
  echo "Database non ancora disponibile, attendo..."
  sleep 2
done

echo "✓ Database disponibile"

# Controlla se esiste il file CSV
if [ -f "$CSV_FILE" ]; then
    echo "Trovato file CSV: $CSV_FILE"
    echo "Inizio importazione..."
    
    python /app/import_thermopro.py "$CSV_FILE" \
        --host "$PG_HOST" \
        --port "$PG_PORT" \
        --database "$PG_DB" \
        --user "$PG_USER" \
        --password "$PG_PASS" \
        --sensor-id "${SENSOR_ID:-TP357S}"
    
    if [ $? -eq 0 ]; then
        echo "✓ Importazione completata con successo"
        echo "Eliminazione file CSV..."
        rm -f "$CSV_FILE"
        echo "✓ File CSV eliminato"
    else
        echo "✗ Errore durante l'importazione"
        exit 1
    fi
else
    echo "Nessun file CSV da importare ($CSV_FILE non trovato)"
fi

echo "=== Import Service terminato ==="
