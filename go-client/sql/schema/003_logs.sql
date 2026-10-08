-- Schema per la tabella logs (log ESP32)
CREATE TABLE IF NOT EXISTS "logs" (
    id        BIGSERIAL PRIMARY KEY,
    "time"    TIMESTAMP NOT NULL,
    "task"    TEXT      NOT NULL,
    "msg"     TEXT      NOT NULL,
    "device"  TEXT      NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_logs_time ON "logs"("time");
CREATE INDEX IF NOT EXISTS idx_logs_device_time ON "logs"("device", "time");
