-- name: CountUsers :one
SELECT COUNT(*) FROM users;

-- name: CreateUser :exec
INSERT INTO users (id, username, password_hash, created_at)
VALUES (?, ?, ?, ?);

-- name: GetUserByUsername :one
SELECT id, username, password_hash, created_at
FROM users
WHERE username = ?;

-- name: GetUserByID :one
SELECT id, username, password_hash, created_at
FROM users
WHERE id = ?;

-- name: CreateAPIKey :exec
INSERT INTO api_keys (id, user_id, name, key_prefix, key_hash, created_at)
VALUES (?, ?, ?, ?, ?, ?);

-- name: GetAPIKeyByHash :one
SELECT
  ak.id,
  ak.user_id,
  ak.name,
  ak.key_prefix,
  ak.key_hash,
  ak.created_at,
  u.username
FROM api_keys ak
JOIN users u ON u.id = ak.user_id
WHERE ak.key_hash = ?;

-- name: ListAPIKeysByUser :many
SELECT id, user_id, name, key_prefix, created_at
FROM api_keys
WHERE user_id = ?
ORDER BY created_at DESC, name ASC;

-- name: DeleteAPIKey :exec
DELETE FROM api_keys
WHERE id = ? AND user_id = ?;
