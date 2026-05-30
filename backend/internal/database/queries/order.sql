-- ══════════════════════════════════════════════════════════════════
-- ORDERS
-- ══════════════════════════════════════════════════════════════════

-- name: CreateOrder :one
INSERT INTO "order".orders (
    order_number, channel, channel_order_id,
    customer_name, customer_phone, shipping_address,
    shipping_method, shipping_cost,
    is_cod, cod_amount,
    subtotal, discount_total, total,
    note, created_by
) VALUES (
    $1, $2, sqlc.narg('channel_order_id'),
    sqlc.narg('customer_name'), sqlc.narg('customer_phone'), sqlc.narg('shipping_address'),
    sqlc.narg('shipping_method'), $3,
    $4, $5,
    $6, $7, $8,
    sqlc.narg('note'), sqlc.narg('created_by')
)
RETURNING id, order_number, channel, channel_order_id, status,
          customer_name, customer_phone, shipping_address,
          shipping_method, shipping_cost,
          is_cod, cod_amount,
          subtotal, discount_total, total,
          note, created_by, created_at, updated_at;

-- name: GetOrderByID :one
SELECT id, order_number, channel, channel_order_id, status,
       customer_name, customer_phone, shipping_address,
       shipping_method, shipping_cost,
       is_cod, cod_amount, cod_collected_at,
       subtotal, discount_total, total,
       note, confirmed_at, packed_at, shipped_at, delivered_at,
       cancelled_at, cancel_reason,
       created_by, created_at, updated_at
FROM "order".orders
WHERE id = $1;

-- name: GetOrderByIDForUpdate :one
-- ล็อก orders row ด้วย SELECT FOR UPDATE
-- ใช้ภายใน transaction ที่ต้องการ serialize การเปลี่ยน status
-- (เช่น confirm, cancel) เพื่อป้องกัน TOCTOU race condition
SELECT id, status FROM "order".orders WHERE id = $1 FOR UPDATE;

-- name: ListOrders :many
SELECT id, order_number, channel, channel_order_id, status,
       customer_name, customer_phone,
       subtotal, discount_total, shipping_cost, total,
       is_cod, cod_amount,
       note, created_by, created_at, updated_at
FROM "order".orders
WHERE (sqlc.narg('status')::text  IS NULL OR status  = sqlc.narg('status')::text)
  AND (sqlc.narg('channel')::text IS NULL OR channel = sqlc.narg('channel')::text)
  AND (sqlc.narg('is_cod')::bool  IS NULL OR is_cod  = sqlc.narg('is_cod')::bool)
  AND (sqlc.narg('search')::text  IS NULL
    OR order_number  ILIKE '%' || sqlc.narg('search') || '%'
    OR customer_name ILIKE '%' || sqlc.narg('search') || '%')
ORDER BY created_at DESC
LIMIT  sqlc.narg('limit_')::int
OFFSET sqlc.narg('offset_')::int;

-- name: CountOrders :one
SELECT COUNT(*)
FROM "order".orders
WHERE (sqlc.narg('status')::text  IS NULL OR status  = sqlc.narg('status')::text)
  AND (sqlc.narg('channel')::text IS NULL OR channel = sqlc.narg('channel')::text)
  AND (sqlc.narg('is_cod')::bool  IS NULL OR is_cod  = sqlc.narg('is_cod')::bool)
  AND (sqlc.narg('search')::text  IS NULL
    OR order_number  ILIKE '%' || sqlc.narg('search') || '%'
    OR customer_name ILIKE '%' || sqlc.narg('search') || '%');

-- name: UpdateOrder :one
-- ใช้สำหรับแก้ข้อมูล order (customer info, shipping, note)
-- อนุญาตเฉพาะ status pending หรือ confirmed เท่านั้น (validate ใน service)
-- total ถูก recalculate ทุกครั้งที่ shipping_cost หรือ discount_total เปลี่ยน
UPDATE "order".orders
SET customer_name    = COALESCE(sqlc.narg('customer_name'),    customer_name),
    customer_phone   = COALESCE(sqlc.narg('customer_phone'),   customer_phone),
    shipping_address = COALESCE(sqlc.narg('shipping_address'), shipping_address),
    shipping_method  = COALESCE(sqlc.narg('shipping_method'),  shipping_method),
    shipping_cost    = COALESCE(sqlc.narg('shipping_cost'),    shipping_cost),
    discount_total   = COALESCE(sqlc.narg('discount_total'),   discount_total),
    total            = subtotal
                     + COALESCE(sqlc.narg('shipping_cost'),    shipping_cost)
                     - COALESCE(sqlc.narg('discount_total'),   discount_total),
    note             = COALESCE(sqlc.narg('note'),             note)
WHERE id = $1
RETURNING id, order_number, channel, status,
          customer_name, customer_phone, shipping_address,
          shipping_method, shipping_cost,
          is_cod, cod_amount,
          subtotal, discount_total, total,
          note, created_at, updated_at;

-- name: UpdateOrderStatus :one
-- อัพเดต status + timestamp ที่เกี่ยวข้องอัตโนมัติ
UPDATE "order".orders
SET status        = $2,
    confirmed_at  = CASE WHEN $2 = 'confirmed'  AND confirmed_at IS NULL
                         THEN NOW() ELSE confirmed_at END,
    packed_at     = CASE WHEN $2 IN ('packing','ready_to_ship') AND packed_at IS NULL
                         THEN NOW() ELSE packed_at END,
    shipped_at    = CASE WHEN $2 = 'shipped'    THEN NOW() ELSE shipped_at    END,
    delivered_at  = CASE WHEN $2 = 'delivered'  THEN NOW() ELSE delivered_at  END,
    cancelled_at  = CASE WHEN $2 = 'cancelled'  THEN NOW() ELSE cancelled_at  END,
    cancel_reason = COALESCE(sqlc.narg('cancel_reason'), cancel_reason)
WHERE id = $1
RETURNING id, order_number, status,
          confirmed_at, packed_at, shipped_at, delivered_at, cancelled_at, cancel_reason,
          updated_at;

-- ══════════════════════════════════════════════════════════════════
-- ORDER ITEMS
-- ══════════════════════════════════════════════════════════════════

-- name: CreateOrderItem :one
INSERT INTO "order".items (
    order_id, sku_id, sku_code, name,
    quantity, unit_price, total_price, discount_amount
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING id, order_id, sku_id, sku_code, name,
          quantity, unit_price, total_price, discount_amount, created_at;

-- name: ListOrderItems :many
SELECT id, order_id, sku_id, sku_code, name,
       quantity, unit_price, total_price, discount_amount, created_at
FROM "order".items
WHERE order_id = $1
ORDER BY id ASC;

-- ══════════════════════════════════════════════════════════════════
-- STATUS HISTORY
-- ══════════════════════════════════════════════════════════════════

-- name: CreateStatusHistory :exec
INSERT INTO "order".status_history (order_id, from_status, to_status, note, changed_by)
VALUES ($1, sqlc.narg('from_status'), $2, sqlc.narg('note'), sqlc.narg('changed_by'));

-- name: ListStatusHistory :many
SELECT id, order_id, from_status, to_status, note, changed_by, changed_at
FROM "order".status_history
WHERE order_id = $1
ORDER BY changed_at ASC;
