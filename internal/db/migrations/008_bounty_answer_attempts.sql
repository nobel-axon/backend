-- 008_bounty_answer_attempts.sql
-- Add attempt tracking columns to bounty answers and answer_count to bounties

-- Track which attempt number this answer was and how much NEURON was burned
ALTER TABLE app_bounty_answers ADD COLUMN IF NOT EXISTS attempt_number INTEGER NOT NULL DEFAULT 1;
ALTER TABLE app_bounty_answers ADD COLUMN IF NOT EXISTS neuron_burned VARCHAR(78) NOT NULL DEFAULT '0';

-- Track answer count on bounties for efficient list queries
ALTER TABLE app_bounties ADD COLUMN IF NOT EXISTS answer_count INTEGER NOT NULL DEFAULT 0;
