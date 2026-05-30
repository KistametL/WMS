-- ══════════════════════════════════════════════════════════════════
-- Migration 000012: Order module fixes from code review
-- ══════════════════════════════════════════════════════════════════

-- ─────────────────────────────────────────────────────────
-- 1. confirmed_at — บันทึกเวลาที่ order ถูก confirm
--    ใช้วิเคราะห์ SLA "pending → confirmed ใช้เวลานานแค่ไหน"
-- ─────────────────────────────────────────────────────────
ALTER TABLE "order".orders
    ADD COLUMN confirmed_at TIMESTAMPTZ;

-- ─────────────────────────────────────────────────────────
-- 2. Unique constraint สำหรับ channel orders
--    ป้องกัน Shopee/Lazada webhook ส่งซ้ำแล้ว order duplicate
--    Partial index: เฉพาะ row ที่มี channel_order_id เท่านั้น
--    (manual orders ไม่มี channel_order_id → ไม่ถูก constraint)
-- ─────────────────────────────────────────────────────────
CREATE UNIQUE INDEX IF NOT EXISTS idx_orders_channel_order_unique
    ON "order".orders(channel, channel_order_id)
    WHERE channel_order_id IS NOT NULL;

-- ─────────────────────────────────────────────────────────
-- 3. pg_trgm indexes สำหรับ ILIKE search
--    ILIKE '%text%' โดยไม่มี trgm index = full table scan
--    GIN trgm index ทำให้ '%text%' ใช้ index ได้
-- ─────────────────────────────────────────────────────────
CREATE EXTENSION IF NOT EXISTS pg_trgm;

CREATE INDEX IF NOT EXISTS idx_orders_order_number_trgm
    ON "order".orders USING gin (order_number gin_trgm_ops);

CREATE INDEX IF NOT EXISTS idx_orders_customer_name_trgm
    ON "order".orders USING gin (customer_name gin_trgm_ops)
    WHERE customer_name IS NOT NULL;
