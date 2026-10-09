# Specifica: più termostati sullo stesso broker

Stato: **da implementare** nel `go-client`. Il firmware è già pronto (Termostato
Firmware, master dopo `14249f3`).

## Il problema

Fino ad oggi ogni termostato pubblicava sugli stessi tre topic:

```
termostato/sample      termostato/opentherm      termostato/logs
```

e nessun payload dice da quale scheda arriva. La scheda di test e quella in
produzione finiscono nelle stesse righe di `measure`, `opentherm` e `logs`, e
le dashboard Grafana mostrano i due flussi mescolati.

## Cosa fa adesso il firmware

L'identità del device è `<id>` = le ultime 3 cifre del MAC WiFi STA in
esadecimale minuscolo, 6 caratteri, es. `a1b2c3`. Non si configura e non cambia.
È anche esposta come `device-id` in `GET /api/v1/system/info` del termostato,
per sapere quale id è la scheda di produzione.

| topic                         | payload                         | QoS | retained |
|-------------------------------|---------------------------------|-----|----------|
| `termostato/<id>/sample`      | JSON metriche, **invariato**    | 0   | no       |
| `termostato/<id>/opentherm`   | JSON caldaia, **invariato**     | 1   | no       |
| `termostato/<id>/logs`        | JSON log, **invariato**         | 0   | no       |
| `termostato/<id>/status`      | stringa `online` / `offline`    | 1   | **sì**   |
| `termostato/<id>/info`        | JSON identità (sotto)           | 1   | **sì**   |

- `status` = `online` lo pubblica il device a ogni connessione; `offline` è il
  *last will*, lo pubblica il broker quando la connessione cade senza disconnect.
- `info` è pubblicato a ogni connessione:

  ```json
  {"id":"a1b2c3","hostname":"termostato","ip":"192.168.178.103",
   "fw_version":"14249f3-dirty","partition":"ota_0"}
  ```

- Il client id MQTT del device è `termostato-<id>`.
- `termostato/` è `CONFIG_MQTT_TOPIC` nel firmware: trattarlo come prefisso
  configurabile, non come costante.
- Riservato per un lavoro successivo, **da non implementare ora**:
  `termostato/<id>/coredump/#`. La sottoscrizione `termostato/+/...` scelta qui
  non deve catturarlo per sbaglio (vedi sotto: niente `termostato/#`).

I payload di `sample`, `opentherm` e `logs` non cambiano: il device si ricava
**solo dal topic**.

## Cosa deve fare il go-client

### 1. Configurazione

`config.yaml` passa da tre topic fissi a un prefisso:

```yaml
topicPrefix: termostato/        # obbligatorio, con lo slash finale
legacyTopics: true              # vedi punto 4; default true
```

Le chiavi `measureTopic`, `OpenThermTopic`, `logTopic` spariscono. `LoadConfig`
valida `topicPrefix` (non vuoto, aggiunge `/` se manca).

### 2. Sottoscrizioni

Una sottoscrizione per tipo, con il wildcard a un livello:

```
<prefix>+/sample       -> InsertMeasure
<prefix>+/opentherm    -> InsertOpenTherm
<prefix>+/logs         -> InsertLog
<prefix>+/status       -> UpsertDeviceStatus
<prefix>+/info         -> UpsertDeviceInfo
```

Il device è il segmento fra il prefisso e la foglia: una funzione pura
`deviceFromTopic(prefix, topic string) (id, leaf string, ok bool)` con test.
Rifiuta (log Warn, messaggio scartato) id vuoti, con `/`, o più lunghi di 32
caratteri. Non controllare che sia esattamente 6 caratteri esadecimali: un
domani l'id potrebbe diventare un nome.

**Non** usare `termostato/#` con uno switch sulla foglia: catturerebbe anche il
futuro flusso `coredump` (binario, a pezzi) e lo passerebbe a un
`json.Unmarshal`.

`info` e `status` sono retained: alla (ri)connessione il broker li consegna
subito per ogni device mai visto, ed è così che la tabella `devices` si popola
anche dopo un riavvio del client.

Le tre `subscribeXxxTopic` attuali sono quasi identiche: è l'occasione per una
sola funzione `subscribe(filter string, qos byte, handler func(device string, payload []byte))`.

Con `opts.SetAutoReconnect(true)` paho **non** rifà le sottoscrizioni da solo
se la sessione è pulita: spostarle in `opts.SetOnConnectHandler`, così
sopravvivono a un riavvio del broker. Oggi è un bug esistente, non introdotto
da questa modifica, ma con cinque sottoscrizioni conviene sistemarlo.

### 3. Database — migrazione `009_add_device.sql` (Tern)

```sql
ALTER TABLE measure   ADD COLUMN "device" TEXT;
ALTER TABLE opentherm ADD COLUMN "device" TEXT;
ALTER TABLE "logs"    ADD COLUMN "device" TEXT;

-- Le righe storiche non hanno un device. 'legacy' le tiene visibili e
-- riconoscibili; l'utente puo' poi riassegnarle con un UPDATE all'id della
-- scheda di produzione, se sa che erano sue.
UPDATE measure   SET "device" = 'legacy' WHERE "device" IS NULL;
UPDATE opentherm SET "device" = 'legacy' WHERE "device" IS NULL;
UPDATE "logs"    SET "device" = 'legacy' WHERE "device" IS NULL;

ALTER TABLE measure   ALTER COLUMN "device" SET NOT NULL;
ALTER TABLE opentherm ALTER COLUMN "device" SET NOT NULL;
ALTER TABLE "logs"    ALTER COLUMN "device" SET NOT NULL;

CREATE INDEX idx_measure_device_time   ON measure("device", "time");
CREATE INDEX idx_opentherm_device_time ON opentherm("device", "time");
CREATE INDEX idx_logs_device_time      ON "logs"("device", "time");

CREATE TABLE devices (
    "id"         TEXT PRIMARY KEY,
    "name"       TEXT,                 -- etichetta umana, la mette l'utente: 'prod', 'test'
    "hostname"   TEXT,
    "ip"         TEXT,
    "fw_version" TEXT,
    "partition"  TEXT,
    "online"     BOOLEAN NOT NULL DEFAULT false,
    "last_seen"  TIMESTAMP NOT NULL DEFAULT now()
);

---- create above / drop below ----
-- (inverso: drop degli indici, delle colonne device, della tabella devices)
```

- Se `measure`/`opentherm` sono hypertable TimescaleDB (migrazione 001),
  verificare che `ALTER ... SET NOT NULL` e l'`UPDATE` passino su tabelle con
  chunk compressi; se la compressione è attiva, decomprimere prima o lasciare la
  colonna nullable e documentarlo.
- Aggiornare **anche** `sql/schema/*.sql` (lo legge sqlc) e
  `init_existing_db.sql`, poi `make sqlc`. Aggiornare l'elenco in
  `MIGRATIONS.md`.
- `name` non viene mai scritto dal client: l'upsert di `info` tocca solo
  hostname, ip, fw_version, partition, last_seen.

### 4. Query (sqlc)

- `InsertMeasure`, `InsertOpenTherm`, `InsertLog`: un parametro `device` in più.
- `UpsertDeviceInfo(id, hostname, ip, fw_version, partition)`:
  `INSERT ... ON CONFLICT (id) DO UPDATE`, `last_seen = now()`.
- `UpsertDeviceStatus(id, online bool)`: stesso schema; `online` = payload
  `== "online"`. Payload diverso da `online`/`offline` → Warn e scarto.
- Ogni messaggio di dati (`sample`/`opentherm`/`logs`) aggiorna anche
  `last_seen` del device (upsert senza toccare gli altri campi), così un device
  che non ha mai mandato `info` compare comunque in `devices`.

### 5. Transizione: topic vecchi

La scheda in produzione resta sul firmware vecchio finché non le si fa l'OTA, e
continua a pubblicare su `termostato/sample` ecc. Con `legacyTopics: true` il
client si sottoscrive **anche** a:

```
<prefix>sample  <prefix>opentherm  <prefix>logs
```

e salva quelle righe con `device = 'legacy'`. (Nota MQTT: `termostato/+/sample`
non riceve `termostato/sample`, i livelli sono diversi: servono entrambe le
sottoscrizioni.) Quando tutte le schede sono aggiornate si mette
`legacyTopics: false`.

### 6. Grafana (manuale, fuori dal codice Go)

Le dashboard stanno nel volume `term_grafana`, non nel repo. Da fare a mano su
ogni dashboard:

- variabile `device`, tipo Query, datasource TimescaleDB:
  ```sql
  SELECT id AS __value, COALESCE(name, hostname, id) AS __text FROM devices
  UNION SELECT 'legacy', 'legacy (prima del multi-device)'
  ORDER BY 1
  ```
- in ogni pannello `AND "device" = '$device'` nella WHERE.
- dare un nome alle schede una volta:
  `UPDATE devices SET name = 'prod' WHERE id = '<id dalla system/info>';`

Valutare, se comodo, di esportare le dashboard in JSON nel repo
(`infra/grafana/`) con il provisioning di Grafana: così la prossima modifica è
versionata.

## Test minimi

- `deviceFromTopic`: `termostato/a1b2c3/sample` → (`a1b2c3`, `sample`, true);
  `termostato/sample` → false; `termostato//sample` → false;
  `termostato/a/b/sample` → false; prefisso senza slash finale in config.
- Parsing di `info` con campi mancanti: non deve fallire, solo lasciare NULL.
- Prova manuale con `mosquitto_pub`:
  ```sh
  mosquitto_pub -p 1885 -t termostato/test01/info -r -m '{"id":"test01","hostname":"banco"}'
  mosquitto_pub -p 1885 -t termostato/test01/status -r -m online
  mosquitto_pub -p 1885 -t termostato/test01/sample -m '{"amb_temp":20.5,"timestamp":1760000000}'
  ```
  → una riga in `devices`, una in `measure` con `device = 'test01'`.
  Pulizia dei retained: `mosquitto_pub -t termostato/test01/info -r -n`.

## Fuori da questa specifica

- Il flusso `termostato/<id>/coredump/#` (core dump a pezzi con conferma): avrà
  una specifica sua.
- Il campo `t` / `timestamp` dei payload e le colonne non ancora mappate (es.
  `heap_min_free`, `alloc_fail`, campi thermal di `opentherm`): invariati qui.
