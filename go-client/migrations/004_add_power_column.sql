-- Add Power column to measure table
-- Migration 004

ALTER TABLE measure ADD COLUMN IF NOT EXISTS "Power" DOUBLE PRECISION NOT NULL DEFAULT 0.0;

---- create above / drop below ----

ALTER TABLE measure DROP COLUMN IF EXISTS "Power";
