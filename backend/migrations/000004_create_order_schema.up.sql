CREATE SCHEMA IF NOT EXISTS "order";

CREATE TABLE "order".orders (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_number     VARCHAR(50)  NOT NULL UNIQUE,
    channel          VARCHAR(20)  NOT NULL,
    channel_order_id VARCHAR(100),
    status           VARCHAR(20)  NOT NULL DEFAULT 'pending',

    customer_name    VARCHAR(255),
    customer_phone   VARCHAR(20),
    shipping_address JSONB,
    shipping_method  VARCHAR(100),
    shipping_cost    NUMERIC(15,2) NOT NULL DEFAULT 0,

    is_cod           BOOLEAN NOT NULL DEFAULT FALSE,
    cod_amount       NUMERIC(15,2) NOT NULL DEFAULT 0,
    cod_collected_at TIMESTAMPTZ,

    subtotal         NUMERIC(15,2) NOT NULL DEFAULT 0,
    total            NUMERIC(15,2) NOT NULL DEFAULT 0,

    note             TEXT,
    packed_at        TIMESTAMPTZ,
    shipped_at       TIMESTAMPTZ,
    delivered_at     TIMESTAMPTZ,
    cancelled_at     TIMESTAMPTZ,
    cancel_reason    TEXT,

    created_by       UUID REFERENCES auth.users(id),
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT chk_order_channel CHECK (channel IN ('shopee','lazada','tiktok','pos','manual')),
    CONSTRAINT chk_order_status  CHECK (status  IN ('pending','confirmed','picking','packing','ready_to_ship','shipped','delivered','completed','cancelled'))
);

-- snapshot ราคา ณ วันที่สั่ง เพราะราคาสินค้าอาจเปลี่ยนทีหลัง
CREATE TABLE "order".order_items (
    id            BIGSERIAL PRIMARY KEY,
    order_id      UUID          NOT NULL REFERENCES "order".orders(id) ON DELETE CASCADE,
    sku_id        UUID          NOT NULL REFERENCES product.skus(id),
    sku_code      VARCHAR(100)  NOT NULL,
    name          VARCHAR(255)  NOT NULL,
    quantity      INTEGER       NOT NULL,
    unit_price    NUMERIC(15,2) NOT NULL,
    total_price   NUMERIC(15,2) NOT NULL,
    created_at    TIMESTAMPTZ   NOT NULL DEFAULT NOW(),

    CONSTRAINT chk_quantity_positive CHECK (quantity > 0)
);

-- บันทึกทุกครั้งที่ status เปลี่ยน
CREATE TABLE "order".order_status_history (
    id          BIGSERIAL PRIMARY KEY,
    order_id    UUID        NOT NULL REFERENCES "order".orders(id) ON DELETE CASCADE,
    from_status VARCHAR(20),
    to_status   VARCHAR(20) NOT NULL,
    note        TEXT,
    changed_by  UUID REFERENCES auth.users(id),
    changed_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Indexes
CREATE INDEX idx_orders_channel          ON "order".orders(channel);
CREATE INDEX idx_orders_channel_order_id ON "order".orders(channel_order_id);
CREATE INDEX idx_orders_status           ON "order".orders(status);
CREATE INDEX idx_orders_created_at       ON "order".orders(created_at);
CREATE INDEX idx_orders_is_cod           ON "order".orders(is_cod) WHERE is_cod = TRUE;
CREATE INDEX idx_order_items_order       ON "order".order_items(order_id);
CREATE INDEX idx_order_items_sku         ON "order".order_items(sku_id);
CREATE INDEX idx_status_history_order    ON "order".order_status_history(order_id);

-- Trigger
CREATE TRIGGER trg_orders_updated_at
    BEFORE UPDATE ON "order".orders
    FOR EACH ROW EXECUTE FUNCTION update_updated_at();
