-- Create initial schema with TimescaleDB extension and tables
-- Migration 001

CREATE EXTENSION IF NOT EXISTS timescaledb CASCADE;

CREATE TABLE "measure" (
	time TIMESTAMPTZ NOT NULL,
	"AmbTemp" DOUBLE PRECISION NULL,
	"TempSensA" INTEGER NULL,
	"TempSensB" INTEGER NULL,
	"TempHot" INTEGER NULL,
	"TempOffsetA"  int NULL,
	"TempOffsetB"  int NULL,
	"DiagA"  int NULL,
	"DiagB"  int NULL,
	"AmbientSetpoint"  int NULL
);
SELECT create_hypertable('measure', 'time');

CREATE TABLE "opentherm" (
	time TIMESTAMPTZ NOT NULL,
	"heatingWaterTempRead" DOUBLE PRECISION NULL,
	"heatingSetpointRead" DOUBLE PRECISION NULL,
	"modulationLvl" DOUBLE PRECISION NULL,
	"dhwTemperature" DOUBLE PRECISION NULL,
	"dhwSetpoint" DOUBLE PRECISION NULL,
	"flameOn" BOOLEAN NULL,
	"heatingOn" BOOLEAN NULL,
	"dhwOn" BOOLEAN NULL
);
SELECT create_hypertable('opentherm', 'time');

---- create above / drop below ----

DROP TABLE IF EXISTS "opentherm";
DROP TABLE IF EXISTS "measure";
DROP EXTENSION IF EXISTS timescaledb CASCADE;
