DROP INDEX IF EXISTS "order".idx_orders_customer_name_trgm;
DROP INDEX IF EXISTS "order".idx_orders_order_number_trgm;
DROP UNIQUE INDEX IF EXISTS "order".idx_orders_channel_order_unique;
ALTER TABLE "order".orders DROP COLUMN IF EXISTS confirmed_at;
