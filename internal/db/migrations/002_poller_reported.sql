-- 002_poller_reported.sql
-- Add columns to track when events have been reported to Chief (idempotency)

ALTER TABLE app_matches ADD COLUMN IF NOT EXISTS registration_reported_at TIMESTAMP WITH TIME ZONE;
ALTER TABLE app_matches ADD COLUMN IF NOT EXISTS timeout_reported_at TIMESTAMP WITH TIME ZONE;

-- Index for efficient queries on unreported matches
CREATE INDEX IF NOT EXISTS idx_app_matches_unreported_registration
    ON app_matches(registration_end)
    WHERE phase = 'registration' AND registration_reported_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_app_matches_unreported_timeout
    ON app_matches(answer_timeout)
    WHERE phase = 'question_live' AND timeout_reported_at IS NULL;
