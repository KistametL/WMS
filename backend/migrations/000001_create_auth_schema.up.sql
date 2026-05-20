-- Auto-update updated_at trigger function (ใช้ร่วมกันทุก table)
CREATE OR REPLACE FUNCTION update_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE SCHEMA IF NOT EXISTS auth;

CREATE TABLE auth.roles (
    id   SERIAL PRIMARY KEY,
    name VARCHAR(50) NOT NULL UNIQUE,
    description TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE auth.users (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email         VARCHAR(255) NOT NULL UNIQUE,
    password_hash VARCHAR(255) NOT NULL,
    first_name    VARCHAR(100) NOT NULL,
    last_name     VARCHAR(100) NOT NULL,
    is_active     BOOLEAN NOT NULL DEFAULT TRUE,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at    TIMESTAMPTZ
);

CREATE TABLE auth.permissions (
    id          SERIAL PRIMARY KEY,
    name        VARCHAR(100) NOT NULL UNIQUE,
    description TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- users ↔ roles (many-to-many)
CREATE TABLE auth.user_roles (
    user_id     UUID    NOT NULL REFERENCES auth.users(id) ON DELETE CASCADE,
    role_id     INTEGER NOT NULL REFERENCES auth.roles(id) ON DELETE CASCADE,
    assigned_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    assigned_by UUID REFERENCES auth.users(id),
    PRIMARY KEY (user_id, role_id)
);

-- roles ↔ permissions (many-to-many)
CREATE TABLE auth.role_permissions (
    role_id       INTEGER NOT NULL REFERENCES auth.roles(id) ON DELETE CASCADE,
    permission_id INTEGER NOT NULL REFERENCES auth.permissions(id) ON DELETE CASCADE,
    PRIMARY KEY (role_id, permission_id)
);

CREATE TABLE auth.refresh_tokens (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    UUID NOT NULL REFERENCES auth.users(id) ON DELETE CASCADE,
    token_hash VARCHAR(255) NOT NULL UNIQUE,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    revoked_at TIMESTAMPTZ
);

-- Audit log — บันทึกทุก action สำคัญในระบบ
CREATE TABLE auth.audit_logs (
    id          BIGSERIAL PRIMARY KEY,
    user_id     UUID REFERENCES auth.users(id),
    action      VARCHAR(100) NOT NULL,  -- e.g. "stock.adjust", "order.create"
    entity_type VARCHAR(50),            -- e.g. "product", "order"
    entity_id   VARCHAR(100),
    old_value   JSONB,
    new_value   JSONB,
    ip_address  INET,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Indexes
CREATE INDEX idx_users_email      ON auth.users(email);
CREATE INDEX idx_users_active     ON auth.users(deleted_at) WHERE deleted_at IS NULL;
CREATE INDEX idx_refresh_user     ON auth.refresh_tokens(user_id);
CREATE INDEX idx_refresh_expires  ON auth.refresh_tokens(expires_at);
CREATE INDEX idx_audit_user       ON auth.audit_logs(user_id);
CREATE INDEX idx_audit_entity     ON auth.audit_logs(entity_type, entity_id);
CREATE INDEX idx_audit_created    ON auth.audit_logs(created_at);

-- Triggers
CREATE TRIGGER trg_roles_updated_at
    BEFORE UPDATE ON auth.roles
    FOR EACH ROW EXECUTE FUNCTION update_updated_at();

CREATE TRIGGER trg_users_updated_at
    BEFORE UPDATE ON auth.users
    FOR EACH ROW EXECUTE FUNCTION update_updated_at();

-- Seed: default roles
INSERT INTO auth.roles (name, description) VALUES
    ('super_admin',       'Full system access'),
    ('admin',             'Manage products, orders, and users'),
    ('warehouse_manager', 'Manage inventory and fulfillment'),
    ('staff',             'Scan, pack, and process orders'),
    ('viewer',            'Read-only dashboard access');

-- Seed: default permissions
INSERT INTO auth.permissions (name, description) VALUES
    ('product:read',      'View products'),
    ('product:write',     'Create and edit products'),
    ('inventory:read',    'View stock levels'),
    ('inventory:write',   'Adjust stock'),
    ('order:read',        'View orders'),
    ('order:write',       'Manage orders'),
    ('fulfillment:read',  'View fulfillment tasks'),
    ('fulfillment:write', 'Process fulfillment'),
    ('pos:use',           'Use POS system'),
    ('user:read',         'View users'),
    ('user:write',        'Manage users'),
    ('report:read',       'View reports and dashboard');
