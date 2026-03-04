# Utilizzo di sqlc

Questo progetto utilizza [sqlc](https://sqlc.dev/) per generare codice Go type-safe dalle query SQL.

## Struttura

- `sql/schema/` - Contiene gli schema delle tabelle del database
- `sql/queries/` - Contiene le query SQL che vengono trasformate in codice Go
- `sqlc.yaml` - File di configurazione per sqlc
- `internal/db/` - Codice Go generato automaticamente da sqlc (non modificare!)

## Comandi

### Generare il codice

Dopo aver modificato i file SQL in `sql/queries/` o `sql/schema/`, esegui:

```bash
~/go/bin/sqlc generate
```

Questo rigenererà i file nella directory `internal/db/`.

### Aggiungere nuove query

1. Crea o modifica un file `.sql` in `sql/queries/`
2. Scrivi la query seguendo il formato sqlc:

```sql
-- name: NomeQuery :exec|:one|:many
SELECT ...
```

3. Esegui `sqlc generate`
4. Usa il codice generato nel tuo progetto

## Esempio di query

```sql
-- name: InsertMeasure :exec
INSERT INTO measure (
    "AmbTemp",
    "TempSensA",
    ...
) VALUES ($1, $2, ...);
```

Genera un metodo `InsertMeasure(ctx, params)` nel package `db`.

## Vantaggi

- **Type-safe**: Gli errori SQL vengono rilevati a compile-time
- **Leggibile**: Query SQL pulite e separate dal codice Go
- **Manutenibile**: Modifica le query SQL senza toccare il codice Go
- **Performance**: Nessun overhead a runtime, codice generato ottimizzato
