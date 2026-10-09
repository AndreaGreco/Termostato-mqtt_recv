-- name: InsertCoredump :exec
-- DO NOTHING: due reinvii ricomposti in parallelo non devono fallire
INSERT INTO coredumps (
    "device",
    "id",
    "size",
    "elf_sha256",
    "task",
    "pc",
    "panic_reason",
    "backtrace",
    "path"
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
ON CONFLICT ("device", "id") DO NOTHING;

-- name: ListCoredumpKeys :many
SELECT "device", "id" FROM coredumps;
