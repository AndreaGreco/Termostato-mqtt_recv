-- name: InsertMeasure :exec
INSERT INTO measure (
    "AmbTemp",
    "TempSensA",
    "TempSensB",
    "TempHot",
    "TempOffsetA",
    "TempOffsetB",
    "DiagA",
    "DiagB",
    "AmbientSetpoint",
    "Power",
    "heap_free",
    "heap_largest_block",
    "boot_count",
    "time"
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14);
