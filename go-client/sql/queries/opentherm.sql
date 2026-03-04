-- name: InsertOpenTherm :exec
INSERT INTO opentherm (
    "heatingWaterTempRead",
    "heatingSetpointRead",
    "modulationLvl",
    "dhwTemperature",
    "dhwSetpoint",
    "flameOn",
    "heatingOn",
    "dhwOn",
    "time"
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9);
