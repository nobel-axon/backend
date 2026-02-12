-- 007_bounties_and_reputation.sql
-- V2: Bounty tables and reputation columns for ERC-8004 integration

-- app_bounties: Store bounty state and metadata
CREATE TABLE IF NOT EXISTS app_bounties (
    bounty_id BIGINT PRIMARY KEY,
    creator_address VARCHAR(42) NOT NULL,
    question_text TEXT NOT NULL,
    category VARCHAR(64),
    difficulty INTEGER NOT NULL DEFAULT 1,
    entry_fee VARCHAR(78) NOT NULL DEFAULT '0',
    pool_total VARCHAR(78) NOT NULL DEFAULT '0',
    min_rating VARCHAR(78) NOT NULL DEFAULT '0',
    max_participants INTEGER NOT NULL DEFAULT 10,
    player_count INTEGER NOT NULL DEFAULT 0,
    phase VARCHAR(32) NOT NULL DEFAULT 'open',
    deadline TIMESTAMP WITH TIME ZONE,
    winner_address VARCHAR(42),
    settle_tx_hash VARCHAR(66),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    settled_at TIMESTAMP WITH TIME ZONE,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_app_bounties_phase ON app_bounties(phase);
CREATE INDEX IF NOT EXISTS idx_app_bounties_creator ON app_bounties(creator_address);
CREATE INDEX IF NOT EXISTS idx_app_bounties_category ON app_bounties(category);
CREATE INDEX IF NOT EXISTS idx_app_bounties_created_at ON app_bounties(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_app_bounties_winner ON app_bounties(winner_address) WHERE winner_address IS NOT NULL;

-- app_bounty_players: Track which agents joined which bounties
CREATE TABLE IF NOT EXISTS app_bounty_players (
    bounty_id BIGINT NOT NULL REFERENCES app_bounties(bounty_id) ON DELETE CASCADE,
    agent_addr VARCHAR(42) NOT NULL,
    registered_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    PRIMARY KEY (bounty_id, agent_addr)
);

CREATE INDEX IF NOT EXISTS idx_app_bounty_players_agent ON app_bounty_players(agent_addr);

-- app_bounty_answers: Store bounty answer submissions and evaluation results
CREATE TABLE IF NOT EXISTS app_bounty_answers (
    id BIGSERIAL PRIMARY KEY,
    bounty_id BIGINT NOT NULL REFERENCES app_bounties(bounty_id) ON DELETE CASCADE,
    agent_addr VARCHAR(42) NOT NULL,
    answer_text TEXT NOT NULL,
    reasoning TEXT,
    total_score INTEGER,
    agreement VARCHAR(32),
    evaluations JSONB,
    tx_hash VARCHAR(66),
    submitted_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    evaluated_at TIMESTAMP WITH TIME ZONE
);

CREATE INDEX IF NOT EXISTS idx_app_bounty_answers_bounty ON app_bounty_answers(bounty_id);
CREATE INDEX IF NOT EXISTS idx_app_bounty_answers_agent ON app_bounty_answers(agent_addr);
CREATE UNIQUE INDEX IF NOT EXISTS idx_app_bounty_answers_unique ON app_bounty_answers(bounty_id, agent_addr);

-- Add reputation columns to app_agent_stats
ALTER TABLE app_agent_stats ADD COLUMN IF NOT EXISTS reputation_score INTEGER NOT NULL DEFAULT 0;
ALTER TABLE app_agent_stats ADD COLUMN IF NOT EXISTS reputation_feedback_count INTEGER NOT NULL DEFAULT 0;
ALTER TABLE app_agent_stats ADD COLUMN IF NOT EXISTS erc8004_registered BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE app_agent_stats ADD COLUMN IF NOT EXISTS bounties_played INTEGER NOT NULL DEFAULT 0;
ALTER TABLE app_agent_stats ADD COLUMN IF NOT EXISTS bounties_won INTEGER NOT NULL DEFAULT 0;

-- Index for reputation-based leaderboard
CREATE INDEX IF NOT EXISTS idx_app_agent_stats_reputation ON app_agent_stats(reputation_score DESC);

-- Apply updated_at trigger to app_bounties
DROP TRIGGER IF EXISTS update_app_bounties_updated_at ON app_bounties;
CREATE TRIGGER update_app_bounties_updated_at
    BEFORE UPDATE ON app_bounties
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();
