-- Add device column to data tables and devices registry (multi-device)
-- Migration 009

-- Le righe storiche non hanno un device: il DEFAULT le assegna a 'legacy'
-- senza UPDATE (niente riscrittura dei chunk). L'utente puo' poi riassegnarle
-- con un UPDATE all'id della scheda di produzione, se sa che erano sue.
-- Il DEFAULT viene poi tolto: il client deve sempre indicare il device.
ALTER TABLE measure   ADD COLUMN "device" TEXT NOT NULL DEFAULT 'legacy';
ALTER TABLE opentherm ADD COLUMN "device" TEXT NOT NULL DEFAULT 'legacy';
ALTER TABLE "logs"    ADD COLUMN "device" TEXT NOT NULL DEFAULT 'legacy';

ALTER TABLE measure   ALTER COLUMN "device" DROP DEFAULT;
ALTER TABLE opentherm ALTER COLUMN "device" DROP DEFAULT;
ALTER TABLE "logs"    ALTER COLUMN "device" DROP DEFAULT;

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

DROP TABLE IF EXISTS devices;

DROP INDEX IF EXISTS idx_logs_device_time;
DROP INDEX IF EXISTS idx_opentherm_device_time;
DROP INDEX IF EXISTS idx_measure_device_time;

ALTER TABLE "logs"    DROP COLUMN IF EXISTS "device";
ALTER TABLE opentherm DROP COLUMN IF EXISTS "device";
ALTER TABLE measure   DROP COLUMN IF EXISTS "device";
