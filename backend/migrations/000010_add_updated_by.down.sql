DROP INDEX IF EXISTS channel.idx_configs_updated_by;
DROP INDEX IF EXISTS product.idx_skus_updated_by;
DROP INDEX IF EXISTS product.idx_products_updated_by;
DROP INDEX IF EXISTS product.idx_categories_updated_by;
DROP INDEX IF EXISTS auth.idx_users_updated_by;

ALTER TABLE channel.configs     DROP COLUMN IF EXISTS updated_by;
ALTER TABLE product.skus        DROP COLUMN IF EXISTS updated_by;
ALTER TABLE product.products    DROP COLUMN IF EXISTS updated_by;
ALTER TABLE product.categories  DROP COLUMN IF EXISTS updated_by;
ALTER TABLE auth.users          DROP COLUMN IF EXISTS updated_by;
