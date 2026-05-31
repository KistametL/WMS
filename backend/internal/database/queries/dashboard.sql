-- ══════════════════════════════════════════════════════════════════
-- DASHBOARD
-- ══════════════════════════════════════════════════════════════════
-- ดึงข้อมูล aggregate จาก tables ที่มีอยู่แล้ว — ไม่ต้องมี migration ใหม่
-- ทุก query ใช้ NOW() AT TIME ZONE 'Asia/Bangkok' เพื่อให้ตรงกับเวลาไทย
-- ══════════════════════════════════════════════════════════════════

-- name: GetOrdersOverview :one
-- KPI: จำนวนออเดอร์ + revenue วันนี้ / สัปดาห์นี้ / เดือนนี้
-- กรอง status != cancelled เพราะ cancelled ไม่นับเป็น revenue
SELECT
    -- วันนี้
    COUNT(*) FILTER (
        WHERE created_at >= DATE_TRUNC('day', NOW() AT TIME ZONE 'Asia/Bangkok') AT TIME ZONE 'Asia/Bangkok'
          AND status != 'cancelled'
    )::bigint AS orders_today,

    -- สัปดาห์นี้ (จันทร์ - วันนี้)
    COUNT(*) FILTER (
        WHERE created_at >= DATE_TRUNC('week', NOW() AT TIME ZONE 'Asia/Bangkok') AT TIME ZONE 'Asia/Bangkok'
          AND status != 'cancelled'
    )::bigint AS orders_this_week,

    -- เดือนนี้
    COUNT(*) FILTER (
        WHERE created_at >= DATE_TRUNC('month', NOW() AT TIME ZONE 'Asia/Bangkok') AT TIME ZONE 'Asia/Bangkok'
          AND status != 'cancelled'
    )::bigint AS orders_this_month,

    -- revenue วันนี้
    COALESCE(SUM(total) FILTER (
        WHERE created_at >= DATE_TRUNC('day', NOW() AT TIME ZONE 'Asia/Bangkok') AT TIME ZONE 'Asia/Bangkok'
          AND status NOT IN ('cancelled', 'pending')
    ), 0)::numeric AS revenue_today,

    -- revenue สัปดาห์นี้
    COALESCE(SUM(total) FILTER (
        WHERE created_at >= DATE_TRUNC('week', NOW() AT TIME ZONE 'Asia/Bangkok') AT TIME ZONE 'Asia/Bangkok'
          AND status NOT IN ('cancelled', 'pending')
    ), 0)::numeric AS revenue_this_week,

    -- revenue เดือนนี้
    COALESCE(SUM(total) FILTER (
        WHERE created_at >= DATE_TRUNC('month', NOW() AT TIME ZONE 'Asia/Bangkok') AT TIME ZONE 'Asia/Bangkok'
          AND status NOT IN ('cancelled', 'pending')
    ), 0)::numeric AS revenue_this_month

FROM "order".orders;

-- name: GetCODPending :one
-- ยอด COD ที่ยังไม่ได้เก็บเงิน (is_cod=true, ยังไม่ shipped/delivered/completed/cancelled)
SELECT
    COUNT(*)::bigint       AS cod_order_count,
    COALESCE(SUM(cod_amount), 0)::numeric AS cod_pending_amount
FROM "order".orders
WHERE is_cod = TRUE
  AND status NOT IN ('shipped', 'delivered', 'completed', 'cancelled');

-- name: CountActiveSKUs :one
-- จำนวน SKU ที่ active อยู่ในระบบ
SELECT COUNT(*)::bigint AS total_active_skus
FROM product.skus
WHERE is_active = TRUE AND deleted_at IS NULL;

-- name: GetOrderStatusBreakdown :many
-- นับออเดอร์แยกตาม status ทั้งหมด (ทุก status ไม่ใช่แค่ active)
SELECT
    status,
    COUNT(*)::bigint AS count
FROM "order".orders
GROUP BY status
ORDER BY
    CASE status
        WHEN 'pending'       THEN 1
        WHEN 'confirmed'     THEN 2
        WHEN 'picking'       THEN 3
        WHEN 'packing'       THEN 4
        WHEN 'ready_to_ship' THEN 5
        WHEN 'shipped'       THEN 6
        WHEN 'delivered'     THEN 7
        WHEN 'completed'     THEN 8
        WHEN 'cancelled'     THEN 9
        ELSE 10
    END;

-- name: GetFulfillmentQueueSizes :one
-- จำนวนออเดอร์ในแต่ละขั้นตอน fulfillment ปัจจุบัน
SELECT
    COUNT(*) FILTER (WHERE status = 'confirmed')::bigint     AS awaiting_pick,
    COUNT(*) FILTER (WHERE status = 'picking')::bigint       AS in_picking,
    COUNT(*) FILTER (WHERE status = 'packing')::bigint       AS in_packing,
    COUNT(*) FILTER (WHERE status = 'ready_to_ship')::bigint AS ready_to_ship
FROM "order".orders
WHERE status IN ('confirmed', 'picking', 'packing', 'ready_to_ship');

-- name: GetStockAlerts :many
-- SKU ที่ qty_available <= low_stock_threshold หรือ = 0
-- JOIN product เพื่อแสดงชื่อสินค้า
SELECT
    sl.sku_id,
    sk.sku_code,
    sk.name                                         AS product_name,
    sl.qty_available,
    sl.low_stock_threshold,
    CASE
        WHEN sl.qty_available <= 0 THEN 'out_of_stock'
        ELSE 'low_stock'
    END                                             AS alert_type
FROM inventory.stock_levels sl
JOIN product.skus sk ON sk.id = sl.sku_id
WHERE sl.qty_available <= sl.low_stock_threshold
  AND sk.is_active = TRUE
  AND sk.deleted_at IS NULL
ORDER BY sl.qty_available ASC, sk.sku_code ASC;

-- name: GetRecentOrders :many
-- ออเดอร์ล่าสุด 10 รายการ (ทุก status)
SELECT
    id,
    order_number,
    channel,
    status,
    customer_name,
    total,
    created_at
FROM "order".orders
ORDER BY created_at DESC
LIMIT 10;
