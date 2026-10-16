-- Drop Power column from measure table: il firmware non pubblica piu' power_mw
-- Migration 011

ALTER TABLE measure DROP COLUMN IF EXISTS "Power";

---- create above / drop below ----

ALTER TABLE measure ADD COLUMN IF NOT EXISTS "Power" DOUBLE PRECISION NOT NULL DEFAULT 0.0;
