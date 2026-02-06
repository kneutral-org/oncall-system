-- Migration: Drop routing_rules, routing_conditions, and routing_actions tables

DROP INDEX IF EXISTS idx_actions_rule;
DROP TABLE IF EXISTS routing_actions;
DROP INDEX IF EXISTS idx_conditions_rule;
DROP TABLE IF EXISTS routing_conditions;
DROP TABLE IF EXISTS routing_rules;
