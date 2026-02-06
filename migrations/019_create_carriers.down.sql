-- Migration: Drop carriers table
-- This migration removes the carrier configuration tables

DROP INDEX IF EXISTS idx_carriers_team;
DROP INDEX IF EXISTS idx_carriers_priority;
DROP INDEX IF EXISTS idx_carriers_type;
DROP INDEX IF EXISTS idx_carriers_asn;

DROP TABLE IF EXISTS carriers;
