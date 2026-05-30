-- Reverse migration 000007

ALTER TABLE "order".order_items  DROP COLUMN IF EXISTS discount_amount;
ALTER TABLE "order".orders        DROP COLUMN IF EXISTS discount_total;

ALTER TABLE inventory.stock_movements DROP CONSTRAINT IF EXISTS chk_movement_type;

DROP TABLE IF EXISTS product.images;

ALTER TABLE product.barcodes DROP CONSTRAINT IF EXISTS chk_barcode_type;

ALTER TABLE product.skus DROP COLUMN IF EXISTS compare_at_price;

DROP INDEX IF EXISTS product.idx_categories_active;
DROP INDEX IF EXISTS product.idx_categories_deleted;

ALTER TABLE product.categories
    DROP COLUMN IF EXISTS deleted_at,
    DROP COLUMN IF EXISTS is_active,
    DROP COLUMN IF EXISTS description;
