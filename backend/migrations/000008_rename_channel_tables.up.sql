-- ชื่อเดิมซ้ำซ้อน schema ชื่อ channel อยู่แล้ว ไม่ต้องมี channel_ นำหน้าอีก
ALTER TABLE channel.channel_configs  RENAME TO configs;
ALTER TABLE channel.channel_products RENAME TO products;
