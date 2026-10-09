-- Add coredumps table (core dump ricevuti via MQTT)
-- Migration 010

CREATE TABLE coredumps (
    "device"       TEXT NOT NULL,
    "id"           TEXT NOT NULL,              -- cid, CRC-32 hex
    "received"     TIMESTAMP NOT NULL DEFAULT now(),
    "size"         INTEGER NOT NULL,
    "elf_sha256"   TEXT,
    "task"         TEXT,
    "pc"           TEXT,
    "panic_reason" TEXT,
    "backtrace"    TEXT[],
    "path"         TEXT NOT NULL,              -- percorso del .elf
    PRIMARY KEY ("device", "id")
);

---- create above / drop below ----

DROP TABLE IF EXISTS coredumps;
