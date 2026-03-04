-- Add OpenTherm columns to measure table
-- Migration 003

ALTER TABLE "measure"
ADD COLUMN "heatingWaterTempRead" DOUBLE PRECISION NULL,
ADD COLUMN "heatingSetpointRead" DOUBLE PRECISION NULL,
ADD COLUMN "modulationLvl" DOUBLE PRECISION NULL,
ADD COLUMN "dhwTemperature" DOUBLE PRECISION NULL,
ADD COLUMN "dhwSetpoint" DOUBLE PRECISION NULL,
ADD COLUMN "flameOn" BOOLEAN NULL,
ADD COLUMN "heatingOn" BOOLEAN NULL,
ADD COLUMN "dhwOn" BOOLEAN NULL;

---- create above / drop below ----

ALTER TABLE "measure"
DROP COLUMN IF EXISTS "dhwOn",
DROP COLUMN IF EXISTS "heatingOn",
DROP COLUMN IF EXISTS "flameOn",
DROP COLUMN IF EXISTS "dhwSetpoint",
DROP COLUMN IF EXISTS "dhwTemperature",
DROP COLUMN IF EXISTS "modulationLvl",
DROP COLUMN IF EXISTS "heatingSetpointRead",
DROP COLUMN IF EXISTS "heatingWaterTempRead";
