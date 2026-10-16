-- Schema per la tabella measure
CREATE TABLE IF NOT EXISTS measure (
    id SERIAL PRIMARY KEY,
    "AmbTemp" DOUBLE PRECISION NOT NULL,
    "TempSensA" DOUBLE PRECISION NOT NULL,
    "TempSensB" DOUBLE PRECISION NOT NULL,
    "TempHot" DOUBLE PRECISION NOT NULL,
    "TempOffsetA" DOUBLE PRECISION NOT NULL,
    "TempOffsetB" DOUBLE PRECISION NOT NULL,
    "DiagA" INTEGER NOT NULL,
    "DiagB" INTEGER NOT NULL,
    "AmbientSetpoint" DOUBLE PRECISION NOT NULL,
    "heap_free" INTEGER NOT NULL DEFAULT 0,
    "heap_largest_block" INTEGER NOT NULL DEFAULT 0,
    "boot_count" INTEGER NOT NULL DEFAULT 0,
    "device" TEXT NOT NULL,
    "time" TIMESTAMP NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_measure_time ON measure("time");
CREATE INDEX IF NOT EXISTS idx_measure_device_time ON measure("device", "time");
