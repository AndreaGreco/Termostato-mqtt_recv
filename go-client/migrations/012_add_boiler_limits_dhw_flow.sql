-- Limiti dichiarati dalla caldaia e portata ACS
-- Migration 012
--
-- NULL = la caldaia non ha (ancora) risposto, o non ha il dato: la portata
-- ACS e' Data-Invalid a rubinetti chiusi. Le righe esistenti restano NULL.

ALTER TABLE "opentherm"
ADD COLUMN "chSetpointMin" DOUBLE PRECISION,
ADD COLUMN "chSetpointMax" DOUBLE PRECISION,
ADD COLUMN "maxChSetpoint" DOUBLE PRECISION,
ADD COLUMN "dhwSetpointMin" DOUBLE PRECISION,
ADD COLUMN "dhwSetpointMax" DOUBLE PRECISION,
ADD COLUMN "dhwFlowRate" DOUBLE PRECISION;

---- create above / drop below ----

ALTER TABLE "opentherm"
DROP COLUMN IF EXISTS "dhwFlowRate",
DROP COLUMN IF EXISTS "dhwSetpointMax",
DROP COLUMN IF EXISTS "dhwSetpointMin",
DROP COLUMN IF EXISTS "maxChSetpoint",
DROP COLUMN IF EXISTS "chSetpointMax",
DROP COLUMN IF EXISTS "chSetpointMin";
