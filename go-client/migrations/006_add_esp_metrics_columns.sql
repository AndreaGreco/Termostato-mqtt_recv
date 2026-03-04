-- Add ESP32 system metrics columns to measure table
-- Migration 006

ALTER TABLE measure ADD COLUMN IF NOT EXISTS "heap_free" INTEGER NOT NULL DEFAULT 0;
ALTER TABLE measure ADD COLUMN IF NOT EXISTS "heap_largest_block" INTEGER NOT NULL DEFAULT 0;
ALTER TABLE measure ADD COLUMN IF NOT EXISTS "boot_count" INTEGER NOT NULL DEFAULT 0;

---- create above / drop below ----

ALTER TABLE measure DROP COLUMN IF EXISTS "heap_free";
ALTER TABLE measure DROP COLUMN IF EXISTS "heap_largest_block";
ALTER TABLE measure DROP COLUMN IF EXISTS "boot_count";
