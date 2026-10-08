-- name: InsertLog :exec
INSERT INTO "logs" (
    "time",
    "task",
    "msg",
    "device"
) VALUES ($1, $2, $3, $4);
