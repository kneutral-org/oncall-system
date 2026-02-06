-- Migration: Drop schedules, rotations, rotation_members, and schedule_overrides tables

DROP INDEX IF EXISTS idx_overrides_schedule_time;
DROP TABLE IF EXISTS schedule_overrides;
DROP TABLE IF EXISTS rotation_members;
DROP INDEX IF EXISTS idx_rotations_schedule;
DROP TABLE IF EXISTS rotations;
DROP INDEX IF EXISTS idx_schedules_team;
DROP TABLE IF EXISTS schedules;
