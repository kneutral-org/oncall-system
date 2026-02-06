-- Migration: Drop teams and team_members tables

DROP INDEX IF EXISTS idx_team_members_user;
DROP TABLE IF EXISTS team_members;
DROP TABLE IF EXISTS teams;
