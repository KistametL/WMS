-- ══════════════════════════════════════════════════════════════════
-- USERS
-- ══════════════════════════════════════════════════════════════════

-- name: GetUserProfile :one
-- ใช้สำหรับ display — ไม่รวม password_hash
SELECT id, email, first_name, last_name, is_active, created_at, updated_at
FROM auth.users
WHERE id = $1 AND deleted_at IS NULL;

-- name: ListUsers :many
SELECT id, email, first_name, last_name, is_active, created_at, updated_at
FROM auth.users
WHERE deleted_at IS NULL
  AND (sqlc.narg('search')::text IS NULL
    OR email      ILIKE '%' || sqlc.narg('search') || '%'
    OR first_name ILIKE '%' || sqlc.narg('search') || '%'
    OR last_name  ILIKE '%' || sqlc.narg('search') || '%')
  AND (sqlc.narg('is_active')::bool IS NULL OR is_active = sqlc.narg('is_active')::bool)
ORDER BY created_at DESC
LIMIT  sqlc.narg('limit_')::int
OFFSET sqlc.narg('offset_')::int;

-- name: CountUsers :one
SELECT COUNT(*)
FROM auth.users
WHERE deleted_at IS NULL
  AND (sqlc.narg('search')::text IS NULL
    OR email      ILIKE '%' || sqlc.narg('search') || '%'
    OR first_name ILIKE '%' || sqlc.narg('search') || '%'
    OR last_name  ILIKE '%' || sqlc.narg('search') || '%')
  AND (sqlc.narg('is_active')::bool IS NULL OR is_active = sqlc.narg('is_active')::bool);

-- name: UpdateUser :one
UPDATE auth.users
SET first_name = COALESCE(sqlc.narg('first_name'), first_name),
    last_name  = COALESCE(sqlc.narg('last_name'),  last_name),
    email      = COALESCE(sqlc.narg('email'),      email),
    is_active  = COALESCE(sqlc.narg('is_active'),  is_active)
WHERE id = $1 AND deleted_at IS NULL
RETURNING id, email, first_name, last_name, is_active, created_at, updated_at;

-- name: SoftDeleteUser :exec
UPDATE auth.users SET deleted_at = NOW()
WHERE id = $1 AND deleted_at IS NULL;

-- name: UpdatePassword :exec
UPDATE auth.users SET password_hash = $2
WHERE id = $1 AND deleted_at IS NULL;

-- ══════════════════════════════════════════════════════════════════
-- ROLES
-- ══════════════════════════════════════════════════════════════════

-- name: ListRoles :many
SELECT id, name, description FROM auth.roles ORDER BY id;

-- name: GetRoleByID :one
SELECT id, name, description FROM auth.roles WHERE id = $1;

-- name: AssignRole :exec
INSERT INTO auth.user_roles (user_id, role_id, assigned_by)
VALUES ($1, $2, sqlc.narg('assigned_by'))
ON CONFLICT (user_id, role_id) DO NOTHING;

-- name: RemoveRole :exec
DELETE FROM auth.user_roles WHERE user_id = $1 AND role_id = $2;
