CREATE SCHEMA IF NOT EXISTS inventory;

CREATE TABLE inventory.stock_levels (
    id                  BIGSERIAL PRIMARY KEY,
    sku_id              UUID NOT NULL UNIQUE REFERENCES product.skus(id),
    qty_on_hand         INTEGER NOT NULL DEFAULT 0,
    qty_reserved        INTEGER NOT NULL DEFAULT 0,
    qty_available       INTEGER GENERATED ALWAYS AS (qty_on_hand - qty_reserved) STORED,
    low_stock_threshold INTEGER NOT NULL DEFAULT 10,
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT chk_qty_on_hand_positive     CHECK (qty_on_hand >= 0),
    CONSTRAINT chk_qty_reserved_positive    CHECK (qty_reserved >= 0),
    CONSTRAINT chk_qty_reserved_lte_on_hand CHECK (qty_reserved <= qty_on_hand)
);

CREATE TABLE inventory.stock_movements (
    id             BIGSERIAL PRIMARY KEY,
    sku_id         UUID NOT NULL REFERENCES product.skus(id),
    movement_type  VARCHAR(20) NOT NULL,
    qty_change     INTEGER NOT NULL,
    qty_before     INTEGER NOT NULL,
    qty_after      INTEGER NOT NULL,
    reference_type VARCHAR(50),
    reference_id   VARCHAR(100),
    note           TEXT,
    created_by     UUID REFERENCES auth.users(id),
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Indexes
CREATE INDEX idx_stock_levels_sku        ON inventory.stock_levels(sku_id);
CREATE INDEX idx_stock_levels_low        ON inventory.stock_levels(qty_available) WHERE qty_available <= low_stock_threshold;
CREATE INDEX idx_movements_sku           ON inventory.stock_movements(sku_id);
CREATE INDEX idx_movements_type          ON inventory.stock_movements(movement_type);
CREATE INDEX idx_movements_reference     ON inventory.stock_movements(reference_type, reference_id);
CREATE INDEX idx_movements_created_at    ON inventory.stock_movements(created_at);
CREATE INDEX idx_movements_created_by    ON inventory.stock_movements(created_by);
