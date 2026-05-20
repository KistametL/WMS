CREATE SCHEMA IF NOT EXISTS product;

CREATE TABLE product.categories (
    id        SERIAL PRIMARY KEY,
    name      VARCHAR(100) NOT NULL,
    parent_id INTEGER REFERENCES product.categories(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE product.products (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    category_id INTEGER REFERENCES product.categories(id),
    name        VARCHAR(255) NOT NULL,
    description TEXT,
    is_active   BOOLEAN NOT NULL DEFAULT TRUE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at  TIMESTAMPTZ
);

CREATE TABLE product.skus (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    product_id    UUID NOT NULL REFERENCES product.products(id) ON DELETE CASCADE,
    sku_code      VARCHAR(100) NOT NULL UNIQUE,
    name          VARCHAR(255) NOT NULL,
    cost_price    NUMERIC(15,2) NOT NULL DEFAULT 0,
    selling_price NUMERIC(15,2) NOT NULL DEFAULT 0,
    weight_grams  INTEGER,
    attributes    JSONB,
    is_active     BOOLEAN NOT NULL DEFAULT TRUE,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at    TIMESTAMPTZ
);

CREATE TABLE product.barcodes (
    id         SERIAL PRIMARY KEY,
    sku_id     UUID NOT NULL REFERENCES product.skus(id) ON DELETE CASCADE,
    barcode    VARCHAR(100) NOT NULL UNIQUE,
    type       VARCHAR(20) NOT NULL DEFAULT 'CODE128',
    is_primary BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Indexes
CREATE INDEX idx_products_category ON product.products(category_id);
CREATE INDEX idx_products_deleted  ON product.products(deleted_at) WHERE deleted_at IS NULL;
CREATE INDEX idx_skus_product      ON product.skus(product_id);
CREATE INDEX idx_skus_code         ON product.skus(sku_code);
CREATE INDEX idx_skus_deleted      ON product.skus(deleted_at) WHERE deleted_at IS NULL;
CREATE INDEX idx_barcodes_sku      ON product.barcodes(sku_id);
CREATE INDEX idx_barcodes_barcode  ON product.barcodes(barcode);

-- Triggers
CREATE TRIGGER trg_categories_updated_at
    BEFORE UPDATE ON product.categories
    FOR EACH ROW EXECUTE FUNCTION update_updated_at();

CREATE TRIGGER trg_products_updated_at
    BEFORE UPDATE ON product.products
    FOR EACH ROW EXECUTE FUNCTION update_updated_at();

CREATE TRIGGER trg_skus_updated_at
    BEFORE UPDATE ON product.skus
    FOR EACH ROW EXECUTE FUNCTION update_updated_at();
