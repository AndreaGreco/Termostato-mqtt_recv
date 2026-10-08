-- "name" non viene mai scritto dal client: lo assegna l'utente

-- name: UpsertDeviceInfo :exec
INSERT INTO devices (
    "id",
    "hostname",
    "ip",
    "fw_version",
    "partition"
) VALUES ($1, $2, $3, $4, $5)
ON CONFLICT ("id") DO UPDATE SET
    "hostname"   = EXCLUDED."hostname",
    "ip"         = EXCLUDED."ip",
    "fw_version" = EXCLUDED."fw_version",
    "partition"  = EXCLUDED."partition",
    "last_seen"  = now();

-- name: UpsertDeviceStatus :exec
INSERT INTO devices (
    "id",
    "online"
) VALUES ($1, $2)
ON CONFLICT ("id") DO UPDATE SET
    "online"    = EXCLUDED."online",
    "last_seen" = now();

-- name: TouchDevice :exec
INSERT INTO devices ("id") VALUES ($1)
ON CONFLICT ("id") DO UPDATE SET
    "last_seen" = now();
