-- Drop equipment_types table and related objects
DROP TRIGGER IF EXISTS trigger_equipment_types_updated_at ON equipment_types;
DROP FUNCTION IF EXISTS update_equipment_types_updated_at();
DROP INDEX IF EXISTS idx_equipment_types_name_lower;
DROP INDEX IF EXISTS idx_equipment_types_criticality;
DROP INDEX IF EXISTS idx_equipment_types_vendor;
DROP INDEX IF EXISTS idx_equipment_types_category;
DROP TABLE IF EXISTS equipment_types;
