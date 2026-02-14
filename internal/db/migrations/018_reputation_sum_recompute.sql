-- Recompute reputation from on-chain feedback (source of truth), with fallback
-- to 22*wins for agents that have no indexed feedback yet.

-- Step 1: Seed from on-chain feedback events (ERC-8004 Reputation Registry)
WITH onchain_rep AS (
    SELECT LOWER(r.owner) AS addr,
           COUNT(*)::int AS feedback_count,
           COALESCE(SUM(f.value), 0)::int AS total_score
    FROM chain_agent_registered r
    JOIN chain_feedback_given f ON f."agentId" = r."agentId"
    GROUP BY LOWER(r.owner)
)
UPDATE app_agent_stats s
SET reputation_score = o.total_score,
    reputation_feedback_count = o.feedback_count
FROM onchain_rep o
WHERE LOWER(s.agent_addr) = o.addr;

-- Step 2: Fallback for agents with wins but no on-chain feedback
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
SET reputation_score = 22 * w.total_wins,
    reputation_feedback_count = w.total_wins
FROM win_counts w
WHERE LOWER(s.agent_addr) = w.addr
  AND s.reputation_score = 0;
