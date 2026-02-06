-- Migration: Drop sites table

DROP INDEX IF EXISTS idx_sites_labels;
DROP INDEX IF EXISTS idx_sites_region;
DROP INDEX IF EXISTS idx_sites_type;
DROP INDEX IF EXISTS idx_sites_code;
DROP TABLE IF EXISTS sites;
