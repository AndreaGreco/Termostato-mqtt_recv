-- Schema per la tabella devices (anagrafica termostati)
CREATE TABLE IF NOT EXISTS devices (
    "id"         TEXT PRIMARY KEY,
    "name"       TEXT,
    "hostname"   TEXT,
    "ip"         TEXT,
    "fw_version" TEXT,
    "partition"  TEXT,
    "online"     BOOLEAN NOT NULL DEFAULT false,
    "last_seen"  TIMESTAMP NOT NULL DEFAULT now()
);
