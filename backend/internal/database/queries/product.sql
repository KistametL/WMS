-- ══════════════════════════════════════════════════════════════════
-- CATEGORIES
-- ══════════════════════════════════════════════════════════════════

-- name: ListCategories :many
SELECT id, name, description, parent_id, is_active, updated_by,
       created_at, updated_at
FROM product.categories
WHERE deleted_at IS NULL
ORDER BY name ASC;

-- name: GetCategoryByID :one
SELECT id, name, description, parent_id, is_active, updated_by,
       created_at, updated_at
FROM product.categories
WHERE id = $1 AND deleted_at IS NULL;

-- name: CreateCategory :one
INSERT INTO product.categories (name, description, parent_id)
VALUES (sqlc.arg('name'), sqlc.narg('description'), sqlc.narg('parent_id'))
RETURNING id, name, description, parent_id, is_active, updated_by,
          created_at, updated_at;

-- name: UpdateCategory :one
UPDATE product.categories
SET name        = COALESCE(sqlc.narg('name'),        name),
    description = COALESCE(sqlc.narg('description'), description),
    parent_id   = COALESCE(sqlc.narg('parent_id'),   parent_id),
    is_active   = COALESCE(sqlc.narg('is_active'),   is_active),
    updated_by  = sqlc.narg('updated_by')
WHERE id = sqlc.arg('id') AND deleted_at IS NULL
RETURNING id, name, description, parent_id, is_active, updated_by,
          created_at, updated_at;

-- name: SoftDeleteCategory :exec
UPDATE product.categories SET deleted_at = NOW()
WHERE id = $1 AND deleted_at IS NULL;

-- ══════════════════════════════════════════════════════════════════
-- PRODUCTS
-- ══════════════════════════════════════════════════════════════════

-- name: ListProducts :many
SELECT id, category_id, name, description, is_active, updated_by,
       created_at, updated_at
FROM product.products
WHERE deleted_at IS NULL
  AND (sqlc.narg('search')::text     IS NULL OR name ILIKE '%' || sqlc.narg('search') || '%')
  AND (sqlc.narg('category_id')::int IS NULL OR category_id = sqlc.narg('category_id'))
  AND (sqlc.narg('is_active')::bool  IS NULL OR is_active   = sqlc.narg('is_active'))
ORDER BY created_at DESC
LIMIT  sqlc.arg('limit')
OFFSET sqlc.arg('offset');

-- name: CountProducts :one
SELECT COUNT(*)
FROM product.products
WHERE deleted_at IS NULL
  AND (sqlc.narg('search')::text     IS NULL OR name ILIKE '%' || sqlc.narg('search') || '%')
  AND (sqlc.narg('category_id')::int IS NULL OR category_id = sqlc.narg('category_id'))
  AND (sqlc.narg('is_active')::bool  IS NULL OR is_active   = sqlc.narg('is_active'));

-- name: GetProductByID :one
SELECT id, category_id, name, description, is_active, updated_by,
       created_at, updated_at
FROM product.products
WHERE id = $1 AND deleted_at IS NULL;

-- name: CreateProduct :one
INSERT INTO product.products (category_id, name, description)
VALUES (sqlc.narg('category_id'), sqlc.arg('name'), sqlc.narg('description'))
RETURNING id, category_id, name, description, is_active, updated_by,
          created_at, updated_at;

-- name: UpdateProduct :one
UPDATE product.products
SET category_id = COALESCE(sqlc.narg('category_id'), category_id),
    name        = COALESCE(sqlc.narg('name'),        name),
    description = COALESCE(sqlc.narg('description'), description),
    is_active   = COALESCE(sqlc.narg('is_active'),   is_active),
    updated_by  = sqlc.narg('updated_by')
WHERE id = sqlc.arg('id') AND deleted_at IS NULL
RETURNING id, category_id, name, description, is_active, updated_by,
          created_at, updated_at;

-- name: SoftDeleteProduct :exec
UPDATE product.products SET deleted_at = NOW()
WHERE id = $1 AND deleted_at IS NULL;

-- ══════════════════════════════════════════════════════════════════
-- SKUs
-- ══════════════════════════════════════════════════════════════════

-- name: ListSKUsByProduct :many
SELECT id, product_id, sku_code, name, cost_price, selling_price,
       compare_at_price, weight_grams, attributes, is_active,
       updated_by, created_at, updated_at
FROM product.skus
WHERE product_id = $1 AND deleted_at IS NULL
ORDER BY created_at ASC;

-- name: GetSKUByID :one
SELECT id, product_id, sku_code, name, cost_price, selling_price,
       compare_at_price, weight_grams, attributes, is_active,
       updated_by, created_at, updated_at
FROM product.skus
WHERE id = $1 AND deleted_at IS NULL;

-- name: CreateSKU :one
INSERT INTO product.skus (
    product_id, sku_code, name,
    cost_price, selling_price, compare_at_price,
    weight_grams, attributes
) VALUES (
    sqlc.arg('product_id'),
    sqlc.arg('sku_code'),
    sqlc.arg('name'),
    sqlc.arg('cost_price'),
    sqlc.arg('selling_price'),
    sqlc.narg('compare_at_price'),
    sqlc.narg('weight_grams'),
    sqlc.narg('attributes')
)
RETURNING id, product_id, sku_code, name, cost_price, selling_price,
          compare_at_price, weight_grams, attributes, is_active,
          updated_by, created_at, updated_at;

-- name: UpdateSKU :one
UPDATE product.skus
SET sku_code         = COALESCE(sqlc.narg('sku_code'),         sku_code),
    name             = COALESCE(sqlc.narg('name'),             name),
    cost_price       = COALESCE(sqlc.narg('cost_price'),       cost_price),
    selling_price    = COALESCE(sqlc.narg('selling_price'),    selling_price),
    compare_at_price = COALESCE(sqlc.narg('compare_at_price'), compare_at_price),
    weight_grams     = COALESCE(sqlc.narg('weight_grams'),     weight_grams),
    attributes       = COALESCE(sqlc.narg('attributes'),       attributes),
    is_active        = COALESCE(sqlc.narg('is_active'),        is_active),
    updated_by       = sqlc.narg('updated_by')
WHERE id = sqlc.arg('id') AND deleted_at IS NULL
RETURNING id, product_id, sku_code, name, cost_price, selling_price,
          compare_at_price, weight_grams, attributes, is_active,
          updated_by, created_at, updated_at;

-- name: SoftDeleteSKU :exec
UPDATE product.skus SET deleted_at = NOW()
WHERE id = $1 AND deleted_at IS NULL;
