-- +migrate Up
-- Add bounty approval/rejection columns for question screening

ALTER TABLE app_bounties ADD COLUMN approved_at TIMESTAMP;
ALTER TABLE app_bounties ADD COLUMN rejected_at TIMESTAMP;
ALTER TABLE app_bounties ADD COLUMN rejection_reason TEXT;

-- +migrate Down

ALTER TABLE app_bounties DROP COLUMN IF EXISTS approved_at;
ALTER TABLE app_bounties DROP COLUMN IF EXISTS rejected_at;
ALTER TABLE app_bounties DROP COLUMN IF EXISTS rejection_reason;
