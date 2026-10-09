-- Schema per la tabella coredumps (core dump ricevuti via MQTT)
CREATE TABLE IF NOT EXISTS coredumps (
    "device"       TEXT NOT NULL,
    "id"           TEXT NOT NULL,
    "received"     TIMESTAMP NOT NULL DEFAULT now(),
    "size"         INTEGER NOT NULL,
    "elf_sha256"   TEXT,
    "task"         TEXT,
    "pc"           TEXT,
    "panic_reason" TEXT,
    "backtrace"    TEXT[],
    "path"         TEXT NOT NULL,
    PRIMARY KEY ("device", "id")
);
