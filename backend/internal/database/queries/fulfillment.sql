-- ══════════════════════════════════════════════════════════════════
-- FULFILLMENT QUEUE
-- sorted ASC: oldest order first = highest priority for warehouse staff
-- ══════════════════════════════════════════════════════════════════

-- name: ListFulfillmentOrders :many
SELECT id, order_number, channel, status,
       customer_name, customer_phone,
       total, is_cod, cod_amount, note,
       created_at, updated_at
FROM "order".orders
WHERE status = $1
ORDER BY created_at ASC
LIMIT  sqlc.narg('limit_')::int
OFFSET sqlc.narg('offset_')::int;

-- name: CountFulfillmentOrders :one
SELECT COUNT(*)
FROM "order".orders
WHERE status = $1;

-- ══════════════════════════════════════════════════════════════════
-- SHIPMENTS
-- ══════════════════════════════════════════════════════════════════

-- name: CreateShipment :one
INSERT INTO fulfillment.shipments (
    order_id, courier, tracking_number, label_url,
    picked_by, packed_by, shipped_by
) VALUES (
    $1, $2, $3,
    sqlc.narg('label_url'),
    sqlc.narg('picked_by'), sqlc.narg('packed_by'), sqlc.narg('shipped_by')
)
RETURNING id, order_id, courier, tracking_number, label_url,
          picked_by, packed_by, shipped_by,
          picked_at, packed_at, shipped_at, created_at;

-- name: GetShipmentByOrderID :one
SELECT id, order_id, courier, tracking_number, label_url,
       picked_by, packed_by, shipped_by,
       picked_at, packed_at, shipped_at, created_at
FROM fulfillment.shipments
WHERE order_id = $1;
