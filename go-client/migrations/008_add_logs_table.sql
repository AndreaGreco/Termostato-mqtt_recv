-- Add logs table for ESP32 task logs
-- Migration 008

CREATE TABLE "logs" (
    id        BIGSERIAL PRIMARY KEY,
    "time"    TIMESTAMP NOT NULL,
    "task"    TEXT      NOT NULL,
    "msg"     TEXT      NOT NULL
);

CREATE INDEX idx_logs_time ON "logs"("time");

---- create above / drop below ----

DROP INDEX IF EXISTS idx_logs_time;
DROP TABLE IF EXISTS "logs";
