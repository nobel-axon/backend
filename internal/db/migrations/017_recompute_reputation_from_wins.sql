-- Recompute reputation from historical match + bounty wins only.
-- Each win counts as one feedback with score 22 (realistic judge panel average for winners).
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
SET reputation_score = 22,
    reputation_feedback_count = w.total_wins
FROM win_counts w
WHERE LOWER(s.agent_addr) = w.addr;
