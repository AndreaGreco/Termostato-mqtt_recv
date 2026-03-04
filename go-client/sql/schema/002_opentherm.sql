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
    "time" TIMESTAMP NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_opentherm_time ON opentherm("time");
