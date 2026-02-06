-- Migration: Drop routing_audit_logs table

DROP INDEX IF EXISTS idx_audit_timestamp;
DROP INDEX IF EXISTS idx_audit_fingerprint;
DROP INDEX IF EXISTS idx_audit_alert;
DROP TABLE IF EXISTS routing_audit_logs;
