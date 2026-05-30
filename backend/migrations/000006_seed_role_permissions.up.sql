-- Seed role_permissions
-- เชื่อม roles กับ permissions ตาม responsibility ของแต่ละ role
--
-- super_admin      → ทุก permission
-- admin            → ทุก permission ยกเว้น user:write (ป้องกันสร้าง admin เอง)
-- warehouse_manager→ inventory, order (read), fulfillment, product (read), report, pos
-- staff            → inventory (read), order, fulfillment, product (read), pos
-- viewer           → read-only ทุก module

INSERT INTO auth.role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM auth.roles r, auth.permissions p
WHERE r.name = 'super_admin'
  AND p.name IN (
    'product:read',
    'product:write',
    'inventory:read',
    'inventory:write',
    'order:read',
    'order:write',
    'fulfillment:read',
    'fulfillment:write',
    'pos:use',
    'user:read',
    'user:write',
    'report:read'
  );

INSERT INTO auth.role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM auth.roles r, auth.permissions p
WHERE r.name = 'admin'
  AND p.name IN (
    'product:read',
    'product:write',
    'inventory:read',
    'inventory:write',
    'order:read',
    'order:write',
    'fulfillment:read',
    'fulfillment:write',
    'pos:use',
    'user:read',
    'report:read'
    -- ไม่มี user:write เพื่อป้องกัน admin สร้าง admin คนอื่นได้เอง
  );

INSERT INTO auth.role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM auth.roles r, auth.permissions p
WHERE r.name = 'warehouse_manager'
  AND p.name IN (
    'product:read',
    'inventory:read',
    'inventory:write',
    'order:read',
    'fulfillment:read',
    'fulfillment:write',
    'report:read',
    'pos:use'
  );

INSERT INTO auth.role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM auth.roles r, auth.permissions p
WHERE r.name = 'staff'
  AND p.name IN (
    'product:read',
    'inventory:read',
    'order:read',
    'order:write',
    'fulfillment:read',
    'fulfillment:write',
    'pos:use'
  );

INSERT INTO auth.role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM auth.roles r, auth.permissions p
WHERE r.name = 'viewer'
  AND p.name IN (
    'product:read',
    'inventory:read',
    'order:read',
    'fulfillment:read',
    'report:read'
  );
