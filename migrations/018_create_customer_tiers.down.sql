-- Down migration: Drop customer_tiers and customers tables

-- Drop customers first due to foreign key
DROP TABLE IF EXISTS customers;

-- Drop customer_tiers
DROP TABLE IF EXISTS customer_tiers;
