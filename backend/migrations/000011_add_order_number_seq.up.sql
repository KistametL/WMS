-- Migration 000011: Add sequence for human-readable order numbers
--
-- ORD-YYYYMMDD-NNNNNN เช่น ORD-20260531-000001
-- ใช้ global sequence (ไม่ reset ต่อ day) เพื่อความ simple
-- และ guaranteed uniqueness โดยไม่ต้องใช้ lock ระดับ table
CREATE SEQUENCE IF NOT EXISTS "order".order_number_seq START 1;
