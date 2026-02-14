-- Fix: allow multiple answer attempts per agent per bounty (was overwriting previous attempts)
DROP INDEX IF EXISTS idx_app_bounty_answers_unique;
CREATE UNIQUE INDEX IF NOT EXISTS idx_app_bounty_answers_unique ON app_bounty_answers(bounty_id, agent_addr, attempt_number);

-- Fix: reputation_score was storing SUM of all feedback but on-chain getSummary returns AVERAGE
-- Convert existing accumulated sums to averages
UPDATE app_agent_stats
SET reputation_score = CASE
    WHEN reputation_feedback_count > 0 THEN reputation_score / reputation_feedback_count
    ELSE 0
END
WHERE reputation_feedback_count > 0;
