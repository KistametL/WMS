-- ══════════════════════════════════════════════════════════════════
-- STOCK LEVELS
-- ══════════════════════════════════════════════════════════════════

-- name: GetStockBySKU :one
-- ใช้สำหรับ read-only (GET /inventory/stock/:sku_id)
SELECT sl.id, sl.sku_id, sl.qty_on_hand, sl.qty_reserved, sl.qty_available,
       sl.low_stock_threshold, sl.updated_at,
       s.sku_code, s.name AS sku_name
FROM inventory.stock_levels sl
JOIN product.skus s ON s.id = sl.sku_id
WHERE sl.sku_id = $1;

-- name: GetStockBySKUForUpdate :one
-- ใช้ภายใน transaction เพื่อ lock row ก่อน update (SELECT FOR UPDATE)
-- ไม่ JOIN เพื่อหลีกเลี่ยงความซับซ้อนกับ lock
SELECT id, sku_id, qty_on_hand, qty_reserved, qty_available,
       low_stock_threshold, updated_at
FROM inventory.stock_levels
WHERE sku_id = $1
FOR UPDATE;

-- name: ListStockLevels :many
SELECT sl.id, sl.sku_id, sl.qty_on_hand, sl.qty_reserved, sl.qty_available,
       sl.low_stock_threshold, sl.updated_at,
       s.sku_code, s.name AS sku_name
FROM inventory.stock_levels sl
JOIN product.skus s ON s.id = sl.sku_id
WHERE s.deleted_at IS NULL
  AND (sqlc.narg('low_stock_only')::bool IS NOT TRUE
    OR sl.qty_available <= sl.low_stock_threshold)
ORDER BY sl.updated_at DESC
LIMIT  sqlc.narg('limit_')::int
OFFSET sqlc.narg('offset_')::int;

-- name: CountStockLevels :one
SELECT COUNT(*)
FROM inventory.stock_levels sl
JOIN product.skus s ON s.id = sl.sku_id
WHERE s.deleted_at IS NULL
  AND (sqlc.narg('low_stock_only')::bool IS NOT TRUE
    OR sl.qty_available <= sl.low_stock_threshold);

-- name: InitStockLevel :one
-- สร้าง stock level ด้วย qty=0 ถ้ายังไม่มี; ถ้ามีแล้วไม่เปลี่ยนค่า
-- ON CONFLICT DO UPDATE ต้องเปลี่ยนอะไรบางอย่าง (PostgreSQL requirement)
-- → touch updated_at เพื่อให้ RETURNING ส่งค่าปัจจุบันกลับมาเสมอ
INSERT INTO inventory.stock_levels (sku_id, qty_on_hand, qty_reserved, low_stock_threshold)
VALUES ($1, 0, 0, 10)
ON CONFLICT (sku_id) DO UPDATE
    SET updated_at = inventory.stock_levels.updated_at
RETURNING id, sku_id, qty_on_hand, qty_reserved, qty_available, low_stock_threshold, updated_at;

-- name: UpdateStockLevel :one
-- ใช้ภายใน transaction หลัง GetStockBySKUForUpdate
-- caller รับผิดชอบตรวจสอบ qty_on_hand >= qty_reserved ก่อนเรียก
UPDATE inventory.stock_levels
SET qty_on_hand  = $2,
    qty_reserved = $3,
    updated_at   = NOW()
WHERE sku_id = $1
RETURNING id, sku_id, qty_on_hand, qty_reserved, qty_available, low_stock_threshold, updated_at;

-- name: UpdateLowStockThreshold :one
UPDATE inventory.stock_levels
SET low_stock_threshold = $2,
    updated_at          = NOW()
WHERE sku_id = $1
RETURNING id, sku_id, qty_on_hand, qty_reserved, qty_available, low_stock_threshold, updated_at;

-- ══════════════════════════════════════════════════════════════════
-- STOCK MOVEMENTS
-- ══════════════════════════════════════════════════════════════════

-- name: CreateStockMovement :one
INSERT INTO inventory.stock_movements (
    sku_id, movement_type, qty_change, qty_before, qty_after,
    reference_type, reference_id, note, created_by
) VALUES (
    $1, $2, $3, $4, $5,
    sqlc.narg('reference_type'), sqlc.narg('reference_id'),
    sqlc.narg('note'), sqlc.narg('created_by')
)
RETURNING id, sku_id, movement_type, qty_change, qty_before, qty_after,
          reference_type, reference_id, note, created_by, created_at;

-- name: GetStockMovement :one
SELECT id, sku_id, movement_type, qty_change, qty_before, qty_after,
       reference_type, reference_id, note, created_by, created_at
FROM inventory.stock_movements
WHERE id = $1;

-- name: ListStockMovements :many
SELECT id, sku_id, movement_type, qty_change, qty_before, qty_after,
       reference_type, reference_id, note, created_by, created_at
FROM inventory.stock_movements
WHERE (sqlc.narg('sku_id')::uuid   IS NULL OR sku_id        = sqlc.narg('sku_id')::uuid)
  AND (sqlc.narg('movement_type')::text IS NULL OR movement_type = sqlc.narg('movement_type')::text)
ORDER BY created_at DESC
LIMIT  sqlc.narg('limit_')::int
OFFSET sqlc.narg('offset_')::int;

-- name: CountStockMovements :one
SELECT COUNT(*)
FROM inventory.stock_movements
WHERE (sqlc.narg('sku_id')::uuid        IS NULL OR sku_id        = sqlc.narg('sku_id')::uuid)
  AND (sqlc.narg('movement_type')::text IS NULL OR movement_type = sqlc.narg('movement_type')::text);
