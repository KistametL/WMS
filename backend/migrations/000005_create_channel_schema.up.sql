CREATE SCHEMA IF NOT EXISTS channel;

-- credentials และ settings ของแต่ละ channel
CREATE TABLE channel.channel_configs (
    id           SERIAL PRIMARY KEY,
    channel      VARCHAR(20)  NOT NULL,
    name         VARCHAR(100) NOT NULL,
    credentials  JSONB        NOT NULL DEFAULT '{}',
    settings     JSONB        NOT NULL DEFAULT '{}',
    is_active    BOOLEAN      NOT NULL DEFAULT TRUE,
    last_synced_at TIMESTAMPTZ,
    created_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW(),

    CONSTRAINT chk_channel_name CHECK (channel IN ('shopee','lazada','tiktok'))
);

-- mapping ระหว่าง SKU ของเรากับ ID ของแต่ละ channel
CREATE TABLE channel.channel_products (
    id                 BIGSERIAL PRIMARY KEY,
    channel_config_id  INTEGER NOT NULL REFERENCES channel.channel_configs(id) ON DELETE CASCADE,
    sku_id             UUID    NOT NULL REFERENCES product.skus(id),
    channel_product_id VARCHAR(100) NOT NULL,
    channel_sku_id     VARCHAR(100) NOT NULL,
    channel_sku_name   VARCHAR(255),
    is_active          BOOLEAN     NOT NULL DEFAULT TRUE,
    last_synced_at     TIMESTAMPTZ,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    UNIQUE (channel_config_id, channel_sku_id)
);

-- log ทุกครั้งที่ sync กับ channel
CREATE TABLE channel.sync_logs (
    id                 BIGSERIAL PRIMARY KEY,
    channel_config_id  INTEGER     NOT NULL REFERENCES channel.channel_configs(id),
    sync_type          VARCHAR(20) NOT NULL,
    status             VARCHAR(10) NOT NULL,
    records_processed  INTEGER     NOT NULL DEFAULT 0,
    records_failed     INTEGER     NOT NULL DEFAULT 0,
    error_message      TEXT,
    started_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at       TIMESTAMPTZ,

    CONSTRAINT chk_sync_type   CHECK (sync_type IN ('orders','stock','products')),
    CONSTRAINT chk_sync_status CHECK (status    IN ('running','success','failed','partial'))
);

-- Indexes
CREATE INDEX idx_channel_configs_channel    ON channel.channel_configs(channel);
CREATE INDEX idx_channel_configs_active     ON channel.channel_configs(is_active) WHERE is_active = TRUE;
CREATE INDEX idx_channel_products_config    ON channel.channel_products(channel_config_id);
CREATE INDEX idx_channel_products_sku       ON channel.channel_products(sku_id);
CREATE INDEX idx_channel_products_ch_sku    ON channel.channel_products(channel_sku_id);
CREATE INDEX idx_sync_logs_config           ON channel.sync_logs(channel_config_id);
CREATE INDEX idx_sync_logs_status           ON channel.sync_logs(status);
CREATE INDEX idx_sync_logs_started_at       ON channel.sync_logs(started_at);

-- Triggers
CREATE TRIGGER trg_channel_configs_updated_at
    BEFORE UPDATE ON channel.channel_configs
    FOR EACH ROW EXECUTE FUNCTION update_updated_at();

CREATE TRIGGER trg_channel_products_updated_at
    BEFORE UPDATE ON channel.channel_products
    FOR EACH ROW EXECUTE FUNCTION update_updated_at();
