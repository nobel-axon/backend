-- Migration 014: Recompute agent stats from source-of-truth tables.
-- Fixes: 3x inflated counters from indexer replays, cross-case duplicate addresses.

BEGIN;

-- 1. Stash reputation data (not derivable from match data).
CREATE TEMP TABLE _rep_backup AS
SELECT
    LOWER(agent_addr) AS agent_addr,
    SUM(reputation_score) AS reputation_score,
    SUM(reputation_feedback_count) AS reputation_feedback_count,
    BOOL_OR(erc8004_registered) AS erc8004_registered,
    MAX(erc8004_agent_id) AS erc8004_agent_id
FROM app_agent_stats
GROUP BY LOWER(agent_addr);

-- 2. Wipe existing (inflated) stats.
DELETE FROM app_agent_stats;

-- 3. Recompute from source-of-truth tables.
INSERT INTO app_agent_stats (
    agent_addr, matches_played, matches_won,
    total_earned_mon, total_earned_neuron, total_burned_neuron,
    correct_answers, wrong_answers,
    last_active, first_seen,
    bounties_played, bounties_won
)
SELECT
    LOWER(p.agent_addr) AS agent_addr,

    -- matches_played: count of settled matches this agent was in
    COUNT(DISTINCT CASE WHEN m.phase = 'settled' THEN m.match_id END) AS matches_played,

    -- matches_won: count of settled matches where this agent was the winner
    COUNT(DISTINCT CASE WHEN m.phase = 'settled' AND LOWER(m.winner_address) = LOWER(p.agent_addr) THEN m.match_id END) AS matches_won,

    -- total_earned_mon: sum of pool_total for matches this agent won
    COALESCE(
        (SELECT SUM(m2.pool_total::numeric)::text
         FROM app_matches m2
         WHERE m2.phase = 'settled' AND LOWER(m2.winner_address) = LOWER(p.agent_addr)),
        '0'
    ) AS total_earned_mon,

    -- total_earned_neuron: keep at 0 (neuron earnings tracked via burns, not prizes)
    '0' AS total_earned_neuron,

    -- total_burned_neuron: sum from answers table
    COALESCE(
        (SELECT SUM(a.neuron_burned::numeric)::text
         FROM app_answers a
         WHERE LOWER(a.agent_addr) = LOWER(p.agent_addr) AND a.neuron_burned IS NOT NULL AND a.neuron_burned != '0'),
        '0'
    ) AS total_burned_neuron,

    -- correct_answers / wrong_answers from verified answers
    COALESCE((SELECT COUNT(*) FROM app_answers a WHERE LOWER(a.agent_addr) = LOWER(p.agent_addr) AND a.is_correct = TRUE), 0) AS correct_answers,
    COALESCE((SELECT COUNT(*) FROM app_answers a WHERE LOWER(a.agent_addr) = LOWER(p.agent_addr) AND a.is_correct = FALSE), 0) AS wrong_answers,

    -- last_active: most recent match created_at
    MAX(m.created_at) AS last_active,

    -- first_seen: earliest match created_at
    MIN(m.created_at) AS first_seen,

    -- bounties: recomputed from bounty tables
    COALESCE((SELECT COUNT(DISTINCT bounty_id) FROM app_bounty_players bp WHERE LOWER(bp.agent_addr) = LOWER(p.agent_addr)), 0) AS bounties_played,
    COALESCE((SELECT COUNT(DISTINCT b.bounty_id) FROM app_bounties b WHERE LOWER(b.winner_address) = LOWER(p.agent_addr) AND b.phase = 'settled'), 0) AS bounties_won

FROM app_match_players p
JOIN app_matches m ON m.match_id = p.match_id
GROUP BY LOWER(p.agent_addr);

-- 4. Restore reputation data.
UPDATE app_agent_stats s SET
    reputation_score = COALESCE(r.reputation_score, 0),
    reputation_feedback_count = COALESCE(r.reputation_feedback_count, 0),
    erc8004_registered = COALESCE(r.erc8004_registered, FALSE),
    erc8004_agent_id = r.erc8004_agent_id
FROM _rep_backup r
WHERE s.agent_addr = r.agent_addr;

DROP TABLE _rep_backup;

COMMIT;
