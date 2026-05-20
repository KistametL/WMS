-- name: GetUserByEmail :one
SELECT id, email, password_hash, first_name, last_name, is_active, created_at, updated_at, deleted_at
FROM auth.users
WHERE email = $1 AND deleted_at IS NULL;

-- name: GetUserByID :one
SELECT id, email, password_hash, first_name, last_name, is_active, created_at, updated_at, deleted_at
FROM auth.users
WHERE id = $1 AND deleted_at IS NULL;

-- name: CreateUser :one
INSERT INTO auth.users (email, password_hash, first_name, last_name)
VALUES ($1, $2, $3, $4)
RETURNING id, email, first_name, last_name, is_active, created_at;

-- name: GetUserRoles :many
SELECT r.id, r.name, r.description
FROM auth.roles r
JOIN auth.user_roles ur ON ur.role_id = r.id
WHERE ur.user_id = $1;

-- name: GetUserPermissions :many
SELECT DISTINCT p.name
FROM auth.permissions p
JOIN auth.role_permissions rp ON rp.permission_id = p.id
JOIN auth.user_roles ur ON ur.role_id = rp.role_id
WHERE ur.user_id = $1;

-- name: CreateRefreshToken :one
INSERT INTO auth.refresh_tokens (user_id, token_hash, expires_at)
VALUES ($1, $2, $3)
RETURNING id, user_id, expires_at, created_at;

-- name: GetRefreshToken :one
SELECT id, user_id, token_hash, expires_at, revoked_at
FROM auth.refresh_tokens
WHERE token_hash = $1 AND revoked_at IS NULL AND expires_at > NOW();

-- name: RevokeRefreshToken :exec
UPDATE auth.refresh_tokens
SET revoked_at = NOW()
WHERE token_hash = $1;

-- name: RevokeAllUserTokens :exec
UPDATE auth.refresh_tokens
SET revoked_at = NOW()
WHERE user_id = $1 AND revoked_at IS NULL;

-- name: CreateAuditLog :exec
INSERT INTO auth.audit_logs (user_id, action, entity_type, entity_id, old_value, new_value, ip_address)
VALUES ($1, $2, $3, $4, $5, $6, $7);
