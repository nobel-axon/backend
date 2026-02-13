-- 009_bounty_base_answer_fee.sql
-- Add base_answer_fee column to bounties table

ALTER TABLE app_bounties ADD COLUMN IF NOT EXISTS base_answer_fee VARCHAR(78) NOT NULL DEFAULT '0';
