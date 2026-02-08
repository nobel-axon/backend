-- 001_initial.sql
-- Initial schema for axon-server

-- app_matches: Store match state and metadata
CREATE TABLE IF NOT EXISTS app_matches (
    match_id BIGINT PRIMARY KEY,
    phase VARCHAR(32) NOT NULL DEFAULT 'created',
    entry_fee VARCHAR(78) NOT NULL DEFAULT '0',  -- wei, max 78 digits for uint256
    answer_fee VARCHAR(78) NOT NULL DEFAULT '0', -- NEURON wei
    pool_total VARCHAR(78) NOT NULL DEFAULT '0', -- MON wei
    player_count INTEGER NOT NULL DEFAULT 0,
    question_text TEXT,
    category VARCHAR(64),
    difficulty INTEGER,
    format_hint TEXT,
    answer_hash VARCHAR(66),  -- 0x + 64 hex chars
    winner_address VARCHAR(42), -- 0x + 40 hex chars
    generator_agent VARCHAR(42),
    registration_end TIMESTAMP WITH TIME ZONE,
    answer_timeout TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    settled_at TIMESTAMP WITH TIME ZONE,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_app_matches_phase ON app_matches(phase);
CREATE INDEX IF NOT EXISTS idx_app_matches_created_at ON app_matches(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_app_matches_winner ON app_matches(winner_address) WHERE winner_address IS NOT NULL;

-- app_match_players: Track which agents registered for which matches
CREATE TABLE IF NOT EXISTS app_match_players (
    match_id BIGINT NOT NULL REFERENCES app_matches(match_id) ON DELETE CASCADE,
    agent_addr VARCHAR(42) NOT NULL,
    registered_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    PRIMARY KEY (match_id, agent_addr)
);

CREATE INDEX IF NOT EXISTS idx_app_match_players_agent ON app_match_players(agent_addr);

-- app_answers: Store all answer submissions and verification results
CREATE TABLE IF NOT EXISTS app_answers (
    id BIGSERIAL PRIMARY KEY,
    match_id BIGINT NOT NULL REFERENCES app_matches(match_id) ON DELETE CASCADE,
    agent_addr VARCHAR(42) NOT NULL,
    answer_text TEXT NOT NULL,
    is_correct BOOLEAN,
    consensus VARCHAR(32), -- 'unanimous', 'majority', 'split', NULL if pending
    confidence DECIMAL(5,4), -- 0.0000 to 1.0000
    block_number BIGINT NOT NULL,
    tx_index INTEGER NOT NULL,
    attempt_number INTEGER NOT NULL DEFAULT 1,
    neuron_burned VARCHAR(78) NOT NULL DEFAULT '0',
    submitted_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    verified_at TIMESTAMP WITH TIME ZONE
);

CREATE INDEX IF NOT EXISTS idx_app_answers_match ON app_answers(match_id);
CREATE INDEX IF NOT EXISTS idx_app_answers_agent ON app_answers(agent_addr);
CREATE INDEX IF NOT EXISTS idx_app_answers_correct ON app_answers(match_id, is_correct) WHERE is_correct = true;
CREATE INDEX IF NOT EXISTS idx_app_answers_block ON app_answers(block_number, tx_index);
CREATE UNIQUE INDEX IF NOT EXISTS idx_app_answers_unique_attempt ON app_answers(match_id, agent_addr, attempt_number);

-- app_agent_stats: Aggregated statistics per agent
CREATE TABLE IF NOT EXISTS app_agent_stats (
    agent_addr VARCHAR(42) PRIMARY KEY,
    matches_played INTEGER NOT NULL DEFAULT 0,
    matches_won INTEGER NOT NULL DEFAULT 0,
    total_earned_mon VARCHAR(78) NOT NULL DEFAULT '0',
    total_earned_neuron VARCHAR(78) NOT NULL DEFAULT '0',
    total_burned_neuron VARCHAR(78) NOT NULL DEFAULT '0',
    wrong_answers INTEGER NOT NULL DEFAULT 0,
    correct_answers INTEGER NOT NULL DEFAULT 0,
    avg_answer_time_ms BIGINT, -- Average time to answer in milliseconds
    last_active TIMESTAMP WITH TIME ZONE,
    first_seen TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_app_agent_stats_wins ON app_agent_stats(matches_won DESC);
CREATE INDEX IF NOT EXISTS idx_app_agent_stats_earned ON app_agent_stats(total_earned_mon DESC);
CREATE INDEX IF NOT EXISTS idx_app_agent_stats_active ON app_agent_stats(last_active DESC);

-- app_commentary: Judge commentary and AI-generated match narration
CREATE TABLE IF NOT EXISTS app_commentary (
    id BIGSERIAL PRIMARY KEY,
    match_id BIGINT NOT NULL REFERENCES app_matches(match_id) ON DELETE CASCADE,
    agent_id VARCHAR(64), -- Judge agent identifier
    event_type VARCHAR(32) NOT NULL, -- 'match_start', 'answer_submitted', 'answer_correct', 'match_end', etc.
    text TEXT NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_app_commentary_match ON app_commentary(match_id, created_at);

-- app_burn_stats: Track NEURON burns for analytics
CREATE TABLE IF NOT EXISTS app_burn_stats (
    id BIGSERIAL PRIMARY KEY,
    match_id BIGINT REFERENCES app_matches(match_id) ON DELETE SET NULL,
    agent_addr VARCHAR(42) NOT NULL,
    amount_burned VARCHAR(78) NOT NULL,
    recorded_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_app_burn_stats_time ON app_burn_stats(recorded_at DESC);
CREATE INDEX IF NOT EXISTS idx_app_burn_stats_agent ON app_burn_stats(agent_addr);
CREATE INDEX IF NOT EXISTS idx_app_burn_stats_match ON app_burn_stats(match_id) WHERE match_id IS NOT NULL;

-- app_poller_state: Track poller progress for resuming after restart
CREATE TABLE IF NOT EXISTS app_poller_state (
    key VARCHAR(64) PRIMARY KEY,
    value TEXT NOT NULL,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

-- Insert initial poller state
INSERT INTO app_poller_state (key, value) VALUES
    ('last_block_number', '0'),
    ('last_sync_time', NOW()::TEXT)
ON CONFLICT (key) DO NOTHING;

-- Helper function to update updated_at timestamp
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ language 'plpgsql';

-- Apply trigger to app_matches
DROP TRIGGER IF EXISTS update_app_matches_updated_at ON app_matches;
CREATE TRIGGER update_app_matches_updated_at
    BEFORE UPDATE ON app_matches
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();
