-- Temperatura esterna dal meteo
-- Migration 013
--
-- Il firmware pubblica weather_temp: l'esterna da Open-Meteo, interpolata
-- sulla serie oraria, la stessa che usa il modello termico. NULL quando il
-- termostato non ha una previsione (rete o servizio meteo assenti, o
-- firmware precedente). Le righe esistenti restano NULL.

ALTER TABLE "opentherm"
ADD COLUMN "weatherTemp" DOUBLE PRECISION;

---- create above / drop below ----

ALTER TABLE "opentherm"
DROP COLUMN IF EXISTS "weatherTemp";
