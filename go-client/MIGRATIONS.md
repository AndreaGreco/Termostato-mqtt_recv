# Migrazioni con Tern

Il progetto ora utilizza [Tern](https://github.com/jackc/tern) per la gestione delle migrazioni del database.

## Struttura

- `migrations/` - Directory contenente tutti i file di migrazione
- `tern.conf` - File di configurazione per Tern
- `init_existing_db.sql` - Script per inizializzare Tern su database esistenti

## Setup per Database Esistente

Se hai già un database con lo schema aggiornato, **esegui prima questo comando**:

```bash
docker compose exec timescaledb psql -U termostato -d termostato -f /app/init_existing_db.sql
```

Oppure, se il database non è ancora in un container:

```bash
psql -h localhost -p 5435 -U termostato -d termostato < go-client/init_existing_db.sql
```

Questo inizializza la tabella `schema_version` di Tern marcando tutte le migrazioni come già applicate.

## Migrazioni Disponibili

1. **001_initial_schema.sql** - Crea lo schema iniziale (timescaledb, tabelle measure e opentherm)
2. **002_convert_temp_types.sql** - Converte i tipi delle colonne temperature a DOUBLE PRECISION
3. **003_add_opentherm_columns_to_measure.sql** - Aggiunge colonne OpenTherm alla tabella measure
4. **004_add_power_column.sql** - Aggiunge la colonna Power alla tabella measure
5. **005_add_thermo_sensor_table.sql** - Crea la tabella thermo_sensor (sensore ThermoPro)
6. **006_add_esp_metrics_columns.sql** - Aggiunge heap_free, heap_largest_block, boot_count a measure
7. **007_add_opentherm_columns.sql** - Aggiunge i campi diagnostici/contatori OpenTherm
8. **008_add_logs_table.sql** - Crea la tabella logs (log dei task ESP32)
9. **009_add_device.sql** - Multi-device: colonna `device` su measure/opentherm/logs (righe storiche = `'legacy'`) e tabella `devices`
10. **010_add_coredumps.sql** - Tabella `coredumps` (core dump ricevuti via MQTT, file in `coredumpDir`)
11. **011_drop_power_column.sql** - Rimuove la colonna Power da measure (il firmware non pubblica più `power_mw`)
12. **012_add_boiler_limits_dhw_flow.sql** - Limiti dichiarati dalla caldaia (mandata, ACS) e portata ACS su opentherm, nullable
13. **013_add_weather_temp.sql** - Temperatura esterna dal meteo (`weather_temp`) su opentherm, nullable

## Esecuzione Automatica

Le migrazioni vengono eseguite automaticamente all'avvio del container `mqttclient`:
- Il container attende che il database sia pronto
- Esegue tutte le migrazioni pendenti usando Tern
- Avvia l'applicazione

## Comandi Utili

### Eseguire le migrazioni manualmente
```bash
docker compose exec mqttclient tern migrate -c /app/tern.conf -m /app/migrations
```

### Verificare lo stato delle migrazioni
```bash
docker compose exec mqttclient tern status -c /app/tern.conf -m /app/migrations
```

### Creare una nuova migrazione localmente
```bash
cd go-client
tern new -m migrations nome_migrazione
```

### Rollback dell'ultima migrazione (usare con cautela)
```bash
docker compose exec mqttclient tern migrate -c /app/tern.conf -m /app/migrations -d -1
```

## Formato delle Migrazioni

Ogni file di migrazione ha due sezioni separate da `---- create above / drop below ----`:

- **Sezione superiore**: Comandi SQL eseguiti durante la migrazione (migrate up)
- **Sezione inferiore**: Comandi SQL eseguiti durante il rollback (migrate down)

Esempio:
```sql
-- Descrizione della migrazione
-- Migration XXX

ALTER TABLE measure ADD COLUMN new_column TEXT;

---- create above / drop below ----

ALTER TABLE measure DROP COLUMN new_column;
```

## Note Importanti

- Le migrazioni vengono applicate in ordine numerico
- Tern tiene traccia delle migrazioni eseguite nella tabella `schema_version`
- Non modificare mai una migrazione già applicata in produzione
- Per nuove modifiche, crea sempre una nuova migrazione
