-- 010_bounty_pipeline_fixes.sql
-- Adds missing columns to app_bounty_players and creates app_bounty_claims table.

-- Add agent_id and snapshot_reputation to bounty players
ALTER TABLE app_bounty_players ADD COLUMN IF NOT EXISTS agent_id BIGINT;
ALTER TABLE app_bounty_players ADD COLUMN IF NOT EXISTS snapshot_reputation TEXT;

-- Bounty claims history
CREATE TABLE IF NOT EXISTS app_bounty_claims (
    id BIGSERIAL PRIMARY KEY,
    bounty_id BIGINT NOT NULL REFERENCES app_bounties(bounty_id) ON DELETE CASCADE,
    claim_type VARCHAR(50) NOT NULL,
    address VARCHAR(42) NOT NULL,
    amount TEXT NOT NULL DEFAULT '0',
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_app_bounty_claims_bounty ON app_bounty_claims(bounty_id);
CREATE INDEX IF NOT EXISTS idx_app_bounty_claims_address ON app_bounty_claims(address);
