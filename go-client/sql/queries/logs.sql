-- name: InsertLog :exec
INSERT INTO "logs" (
    "time",
    "task",
    "msg"
) VALUES ($1, $2, $3);
