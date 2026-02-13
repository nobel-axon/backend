-- 011_match_settlement_details.sql
-- Adds settlement detail columns to matches and creates refund/burn tracking tables.

-- Match settlement details (treasury fee, burn allocation from MatchSettled event)
ALTER TABLE app_matches ADD COLUMN IF NOT EXISTS treasury_fee VARCHAR(78);
ALTER TABLE app_matches ADD COLUMN IF NOT EXISTS burn_allocation VARCHAR(78);

-- Individual refund credits per player (from RefundCredited event)
CREATE TABLE IF NOT EXISTS app_match_refunds (
    id BIGSERIAL PRIMARY KEY,
    match_id BIGINT NOT NULL,
    player_addr VARCHAR(42) NOT NULL,
    amount VARCHAR(78) NOT NULL DEFAULT '0',
    refund_withdrawn BOOLEAN NOT NULL DEFAULT FALSE,
    withdrawn_at TIMESTAMP WITH TIME ZONE,
    credited_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_app_match_refunds_match ON app_match_refunds(match_id);
CREATE INDEX IF NOT EXISTS idx_app_match_refunds_player ON app_match_refunds(player_addr);

-- Burn allocation claims (from BurnAllocationClaimed event)
CREATE TABLE IF NOT EXISTS app_match_burn_claims (
    id BIGSERIAL PRIMARY KEY,
    operator VARCHAR(42) NOT NULL,
    winner VARCHAR(42) NOT NULL,
    amount VARCHAR(78) NOT NULL DEFAULT '0',
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_app_match_burn_claims_winner ON app_match_burn_claims(winner);
