-- Recompute reputation as cumulative sum (22 per win) instead of flat average.
-- This corrects 017 which set reputation_score = 22 for all agents regardless of wins.
WITH win_counts AS (
    SELECT addr, SUM(wins) AS total_wins FROM (
        SELECT LOWER(m.winner_address) AS addr, COUNT(*) AS wins
        FROM app_matches m
        WHERE m.phase = 'settled' AND m.winner_address IS NOT NULL
        GROUP BY LOWER(m.winner_address)

        UNION ALL

        SELECT LOWER(b.winner_address) AS addr, COUNT(*) AS wins
        FROM app_bounties b
        WHERE b.phase = 'settled' AND b.winner_address IS NOT NULL
        GROUP BY LOWER(b.winner_address)
    ) combined
    GROUP BY addr
)
UPDATE app_agent_stats s
SET reputation_score = 22 * w.total_wins
FROM win_counts w
WHERE LOWER(s.agent_addr) = w.addr;
