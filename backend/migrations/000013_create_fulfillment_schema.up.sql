-- ══════════════════════════════════════════════════════════════════
-- Migration 000013: Fulfillment schema
-- ══════════════════════════════════════════════════════════════════

CREATE SCHEMA IF NOT EXISTS fulfillment;

-- shipments — บันทึกข้อมูล courier + tracking ทุก order ที่ถูก ship
-- UNIQUE(order_id): ป้องกัน duplicate shipment สำหรับ order เดียวกัน
CREATE TABLE fulfillment.shipments (
    id               UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id         UUID         NOT NULL UNIQUE REFERENCES "order".orders(id),
    courier          VARCHAR(50)  NOT NULL,
    tracking_number  VARCHAR(100) NOT NULL,
    label_url        TEXT,

    -- Audit trail: ใครหยิบ / แพ็ก / ส่ง
    -- picked_at / packed_at: populate ได้จาก status_history แต่เก็บไว้เพื่อ convenience
    picked_by        UUID         REFERENCES auth.users(id),
    packed_by        UUID         REFERENCES auth.users(id),
    shipped_by       UUID         REFERENCES auth.users(id),
    picked_at        TIMESTAMPTZ,
    packed_at        TIMESTAMPTZ,
    shipped_at       TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    created_at       TIMESTAMPTZ  NOT NULL DEFAULT NOW(),

    CONSTRAINT chk_shipment_courier
        CHECK (courier IN ('kerry','flash','jnt','spx','thaipost','manual'))
);
