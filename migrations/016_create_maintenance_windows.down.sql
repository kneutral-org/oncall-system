-- Migration: Drop maintenance_windows table

DROP INDEX IF EXISTS idx_maint_scope;
DROP INDEX IF EXISTS idx_maint_time;
DROP INDEX IF EXISTS idx_maint_status;
DROP TABLE IF EXISTS maintenance_windows;
