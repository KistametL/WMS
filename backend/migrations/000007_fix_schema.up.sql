-- ══════════════════════════════════════════════════════════════════════
-- Migration 000007: Fix schema gaps across all modules
-- ══════════════════════════════════════════════════════════════════════

-- ─────────────────────────────────────────────────────────
-- PRODUCT: categories — เพิ่ม soft delete และ description
-- ─────────────────────────────────────────────────────────

ALTER TABLE product.categories
    ADD COLUMN description TEXT,
    ADD COLUMN is_active   BOOLEAN     NOT NULL DEFAULT TRUE,
    ADD COLUMN deleted_at  TIMESTAMPTZ;

CREATE INDEX idx_categories_deleted ON product.categories(deleted_at) WHERE deleted_at IS NULL;
CREATE INDEX idx_categories_active  ON product.categories(is_active)  WHERE is_active = TRUE;

-- ─────────────────────────────────────────────────────────
-- PRODUCT: skus — เพิ่ม compare_at_price สำหรับแสดง
-- ราคาเดิม (ขีดฆ่า) ก่อนลด เช่น ~~฿200~~ → ฿150
-- NULL = ไม่มีราคาเปรียบเทียบ
-- ─────────────────────────────────────────────────────────

ALTER TABLE product.skus
    ADD COLUMN compare_at_price NUMERIC(15,2);

-- ─────────────────────────────────────────────────────────
-- PRODUCT: barcodes — lock ประเภทที่รองรับ
-- ─────────────────────────────────────────────────────────

ALTER TABLE product.barcodes
    ADD CONSTRAINT chk_barcode_type
        CHECK (type IN ('CODE128', 'EAN13', 'EAN8', 'QR', 'CODE39', 'UPC_A'));

-- ─────────────────────────────────────────────────────────
-- PRODUCT: images — รูปภาพสินค้า
-- product_id required เสมอ
-- sku_id optional — สำหรับรูปเฉพาะ variant เช่น เสื้อสีแดง
-- ─────────────────────────────────────────────────────────

CREATE TABLE product.images (
    id         UUID    PRIMARY KEY DEFAULT gen_random_uuid(),
    product_id UUID    NOT NULL REFERENCES product.products(id) ON DELETE CASCADE,
    sku_id     UUID    REFERENCES product.skus(id) ON DELETE SET NULL,
    url        TEXT    NOT NULL,
    alt_text   VARCHAR(255),
    sort_order SMALLINT     NOT NULL DEFAULT 0,
    is_primary BOOLEAN      NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_images_product ON product.images(product_id);
CREATE INDEX idx_images_sku     ON product.images(sku_id) WHERE sku_id IS NOT NULL;
CREATE INDEX idx_images_primary ON product.images(product_id) WHERE is_primary = TRUE;

-- ─────────────────────────────────────────────────────────
-- INVENTORY: stock_movements — lock ประเภทที่รองรับ
-- receive   = รับสินค้าเข้า (จากซัพพลายเออร์)
-- adjust    = ปรับยอดด้วยมือ (นับ stock จริง)
-- reserve   = จอง stock สำหรับ order
-- unreserve = ยกเลิก reservation
-- fulfill   = จัดส่งออกไปแล้ว (ตัด stock จริง)
-- return    = รับคืนจากลูกค้า
-- damage    = สินค้าเสียหาย/สูญหาย
-- transfer  = โอนย้ายระหว่าง location (รองรับอนาคต)
-- ─────────────────────────────────────────────────────────

ALTER TABLE inventory.stock_movements
    ADD CONSTRAINT chk_movement_type
        CHECK (movement_type IN (
            'receive',
            'adjust',
            'reserve',
            'unreserve',
            'fulfill',
            'return',
            'damage',
            'transfer'
        ));

-- ─────────────────────────────────────────────────────────
-- ORDER: เพิ่ม discount_total
-- สมการที่ถูกต้อง:
--   subtotal - discount_total + shipping_cost = total
-- ถ้าไม่มี discount_total → ตัวเลขการเงินไม่สมดุล
-- ─────────────────────────────────────────────────────────

ALTER TABLE "order".orders
    ADD COLUMN discount_total NUMERIC(15,2) NOT NULL DEFAULT 0;

-- ORDER: order_items — discount ต่อชิ้น
-- สำหรับส่วนลด bundle หรือ voucher เฉพาะสินค้า

ALTER TABLE "order".order_items
    ADD COLUMN discount_amount NUMERIC(15,2) NOT NULL DEFAULT 0;
