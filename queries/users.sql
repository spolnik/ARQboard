-- name: GetUserByEmail :one
SELECT id, email, password_hash, display_name, is_admin, created_at, updated_at
FROM users
WHERE lower(email) = lower($1);

-- name: CreateAdminUser :one
INSERT INTO users (email, password_hash, display_name, is_admin)
VALUES ($1, $2, $3, true)
RETURNING id, email, display_name, is_admin, created_at, updated_at;
