-- 006_settle_tx_hash.sql
-- Add settle transaction hash to matches for on-chain verification links

ALTER TABLE app_matches ADD COLUMN IF NOT EXISTS settle_tx_hash VARCHAR(66);
