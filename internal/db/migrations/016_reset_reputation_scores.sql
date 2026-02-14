-- Reset all reputation scores to 0.
-- Going forward, only match/bounty winners earn reputation.
UPDATE app_agent_stats SET reputation_score = 0, reputation_feedback_count = 0;
