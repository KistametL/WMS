-- ══════════════════════════════════════════════════════════════════════
-- Migration 000010: Add updated_by to master data tables
--
-- updated_by ≠ audit_logs
--   audit_logs  → ประวัติทุก action (old/new value) — ต้อง JOIN ถึงดูได้
--   updated_by  → "ใครแก้ล่าสุด" อ่านได้ทันทีจาก record นั้นเลย
--
-- ใส่เฉพาะตารางที่:
--   1. มนุษย์ edit โดยตรง (ไม่ใช่ระบบ update อัตโนมัติ)
--   2. มีผลต่อ financial, security, หรือ master data
--   3. ไม่ใช่ append-only log (stock_movements, status_history ฯลฯ)
-- ══════════════════════════════════════════════════════════════════════

-- ─────────────────────────────────────────────────────────
-- auth.users — ใคร deactivate user, ใคร assign role
-- security compliance: ต้องรู้ว่าใครจัดการ user ล่าสุด
-- ─────────────────────────────────────────────────────────
ALTER TABLE auth.users
    ADD COLUMN updated_by UUID REFERENCES auth.users(id) ON DELETE SET NULL;

-- ─────────────────────────────────────────────────────────
-- product.categories — ใคร rename/deactivate category
-- ถ้า category หาย ส่งผลต่อ product ทุกตัวในนั้น
-- ─────────────────────────────────────────────────────────
ALTER TABLE product.categories
    ADD COLUMN updated_by UUID REFERENCES auth.users(id) ON DELETE SET NULL;

-- ─────────────────────────────────────────────────────────
-- product.products — ใคร deactivate/เปลี่ยนชื่อ product
-- master data accountability
-- ─────────────────────────────────────────────────────────
ALTER TABLE product.products
    ADD COLUMN updated_by UUID REFERENCES auth.users(id) ON DELETE SET NULL;

-- ─────────────────────────────────────────────────────────
-- product.skus — ใคร เปลี่ยน cost_price / selling_price
-- financial accountability: ราคาผิด → ต้องหาคนรับผิดชอบได้
-- ─────────────────────────────────────────────────────────
ALTER TABLE product.skus
    ADD COLUMN updated_by UUID REFERENCES auth.users(id) ON DELETE SET NULL;

-- ─────────────────────────────────────────────────────────
-- channel.configs — ใคร เปลี่ยน API credentials ของ Shopee/Lazada
-- security: ถ้า credentials รั่ว ต้องหา root cause ได้ทันที
-- ─────────────────────────────────────────────────────────
ALTER TABLE channel.configs
    ADD COLUMN updated_by UUID REFERENCES auth.users(id) ON DELETE SET NULL;

-- Indexes — ใช้ filter "งาน user X แก้ไขล่าสุด" หรือ audit report
CREATE INDEX idx_users_updated_by      ON auth.users(updated_by)      WHERE updated_by IS NOT NULL;
CREATE INDEX idx_categories_updated_by ON product.categories(updated_by) WHERE updated_by IS NOT NULL;
CREATE INDEX idx_products_updated_by   ON product.products(updated_by)   WHERE updated_by IS NOT NULL;
CREATE INDEX idx_skus_updated_by       ON product.skus(updated_by)       WHERE updated_by IS NOT NULL;
CREATE INDEX idx_configs_updated_by    ON channel.configs(updated_by)    WHERE updated_by IS NOT NULL;
