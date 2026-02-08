-- 003_match_history.sql
-- Persist match lifecycle data (personalities, evaluations, revealed answers) for history page

-- New table: store judge personalities and panel per match
CREATE TABLE IF NOT EXISTS app_match_personalities (
    match_id BIGINT NOT NULL REFERENCES app_matches(match_id) ON DELETE CASCADE,
    personalities JSONB NOT NULL,
    judge_panel JSONB NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    PRIMARY KEY (match_id)
);

-- Add evaluation columns to answers
ALTER TABLE app_answers ADD COLUMN IF NOT EXISTS total_score INTEGER;
ALTER TABLE app_answers ADD COLUMN IF NOT EXISTS agreement VARCHAR(32);
ALTER TABLE app_answers ADD COLUMN IF NOT EXISTS reasoning TEXT;
ALTER TABLE app_answers ADD COLUMN IF NOT EXISTS evaluations JSONB;

-- Add revealed answer columns to matches
ALTER TABLE app_matches ADD COLUMN IF NOT EXISTS revealed_answer TEXT;
ALTER TABLE app_matches ADD COLUMN IF NOT EXISTS revealed_salt VARCHAR(66);
