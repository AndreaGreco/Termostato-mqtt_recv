-- Schema per la tabella opentherm
CREATE TABLE IF NOT EXISTS opentherm (
    id SERIAL PRIMARY KEY,
    "heatingWaterTempRead" DOUBLE PRECISION NOT NULL,
    "heatingSetpointRead" DOUBLE PRECISION NOT NULL,
    "modulationLvl" DOUBLE PRECISION NOT NULL,
    "dhwTemperature" DOUBLE PRECISION NOT NULL,
    "dhwSetpoint" DOUBLE PRECISION NOT NULL,
    "flameOn" BOOLEAN NOT NULL,
    "heatingOn" BOOLEAN NOT NULL,
    "dhwOn" BOOLEAN NOT NULL,
    -- Sensor & Informational
    "returnWaterTemp" DOUBLE PRECISION NOT NULL DEFAULT 0,
    "chWaterPressure" DOUBLE PRECISION NOT NULL DEFAULT 0,
    "outsideTemperature" DOUBLE PRECISION NOT NULL DEFAULT 0,
    "exhaustTemperature" DOUBLE PRECISION NOT NULL DEFAULT 0,
    "heatExchangerTemperature" DOUBLE PRECISION NOT NULL DEFAULT 0,
    -- Fault & Diagnostics
    "faultFlags" INTEGER NOT NULL DEFAULT 0,
    "oemFaultCode" INTEGER NOT NULL DEFAULT 0,
    "oemDiagnostic" INTEGER NOT NULL DEFAULT 0,
    -- Boiler capacity
    "maxBoilerCapacityKw" INTEGER NOT NULL DEFAULT 0,
    "minModulationLevel" INTEGER NOT NULL DEFAULT 0,
    -- Operating hours & starts
    "burnerStarts" INTEGER NOT NULL DEFAULT 0,
    "burnerOperationHours" INTEGER NOT NULL DEFAULT 0,
    "chPumpStarts" INTEGER NOT NULL DEFAULT 0,
    "chPumpOperationHours" INTEGER NOT NULL DEFAULT 0,
    "dhwBurnerStarts" INTEGER NOT NULL DEFAULT 0,
    "dhwBurnerOperationHours" INTEGER NOT NULL DEFAULT 0,
    -- Limiti dichiarati dalla caldaia e portata ACS (NULL = non disponibile)
    "chSetpointMin" DOUBLE PRECISION,
    "chSetpointMax" DOUBLE PRECISION,
    "maxChSetpoint" DOUBLE PRECISION,
    "dhwSetpointMin" DOUBLE PRECISION,
    "dhwSetpointMax" DOUBLE PRECISION,
    "dhwFlowRate" DOUBLE PRECISION,
    "device" TEXT NOT NULL,
    "time" TIMESTAMP NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_opentherm_time ON opentherm("time");
CREATE INDEX IF NOT EXISTS idx_opentherm_device_time ON opentherm("device", "time");
