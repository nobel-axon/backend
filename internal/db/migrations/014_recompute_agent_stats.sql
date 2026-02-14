-- Migration 014: Recompute agent stats from source-of-truth tables.
-- Fixes: 3x inflated counters from indexer replays, cross-case duplicate addresses.

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

-- 3. Recompute from source-of-truth tables using CTEs.
WITH
-- All unique agents (lowercased)
agents AS (
    SELECT DISTINCT LOWER(agent_addr) AS addr FROM app_match_players
),
-- Matches played/won + timestamps per agent
match_stats AS (
    SELECT
        LOWER(p.agent_addr) AS addr,
        COUNT(DISTINCT CASE WHEN m.phase = 'settled' THEN m.match_id END) AS matches_played,
        COUNT(DISTINCT CASE WHEN m.phase = 'settled' AND LOWER(m.winner_address) = LOWER(p.agent_addr) THEN m.match_id END) AS matches_won,
        MAX(m.created_at) AS last_active,
        MIN(m.created_at) AS first_seen
    FROM app_match_players p
    JOIN app_matches m ON m.match_id = p.match_id
    GROUP BY LOWER(p.agent_addr)
),
-- MON earned (pool_total of won matches)
earnings AS (
    SELECT
        LOWER(m.winner_address) AS addr,
        SUM(m.pool_total::numeric)::text AS total_earned_mon
    FROM app_matches m
    WHERE m.phase = 'settled' AND m.winner_address IS NOT NULL
    GROUP BY LOWER(m.winner_address)
),
-- NEURON burned from answers
burns AS (
    SELECT
        LOWER(a.agent_addr) AS addr,
        SUM(a.neuron_burned::numeric)::text AS total_burned_neuron
    FROM app_answers a
    WHERE a.neuron_burned IS NOT NULL AND a.neuron_burned != '0'
    GROUP BY LOWER(a.agent_addr)
),
-- Correct/wrong answers
answer_stats AS (
    SELECT
        LOWER(a.agent_addr) AS addr,
        COUNT(*) FILTER (WHERE a.is_correct = TRUE) AS correct_answers,
        COUNT(*) FILTER (WHERE a.is_correct = FALSE) AS wrong_answers
    FROM app_answers a
    WHERE a.is_correct IS NOT NULL
    GROUP BY LOWER(a.agent_addr)
),
-- Bounty stats
bounty_stats AS (
    SELECT
        LOWER(bp.agent_addr) AS addr,
        COUNT(DISTINCT bp.bounty_id) AS bounties_played
    FROM app_bounty_players bp
    GROUP BY LOWER(bp.agent_addr)
),
bounty_wins AS (
    SELECT
        LOWER(b.winner_address) AS addr,
        COUNT(DISTINCT b.bounty_id) AS bounties_won
    FROM app_bounties b
    WHERE b.phase = 'settled' AND b.winner_address IS NOT NULL
    GROUP BY LOWER(b.winner_address)
)
INSERT INTO app_agent_stats (
    agent_addr, matches_played, matches_won,
    total_earned_mon, total_earned_neuron, total_burned_neuron,
    correct_answers, wrong_answers,
    last_active, first_seen,
    bounties_played, bounties_won
)
SELECT
    a.addr,
    COALESCE(ms.matches_played, 0),
    COALESCE(ms.matches_won, 0),
    COALESCE(e.total_earned_mon, '0'),
    '0',
    COALESCE(bu.total_burned_neuron, '0'),
    COALESCE(ans.correct_answers, 0),
    COALESCE(ans.wrong_answers, 0),
    ms.last_active,
    ms.first_seen,
    COALESCE(bs.bounties_played, 0),
    COALESCE(bw.bounties_won, 0)
FROM agents a
LEFT JOIN match_stats ms ON ms.addr = a.addr
LEFT JOIN earnings e ON e.addr = a.addr
LEFT JOIN burns bu ON bu.addr = a.addr
LEFT JOIN answer_stats ans ON ans.addr = a.addr
LEFT JOIN bounty_stats bs ON bs.addr = a.addr
LEFT JOIN bounty_wins bw ON bw.addr = a.addr;

-- 4. Restore reputation data.
UPDATE app_agent_stats s SET
    reputation_score = COALESCE(r.reputation_score, 0),
    reputation_feedback_count = COALESCE(r.reputation_feedback_count, 0),
    erc8004_registered = COALESCE(r.erc8004_registered, FALSE),
    erc8004_agent_id = r.erc8004_agent_id
FROM _rep_backup r
WHERE s.agent_addr = r.agent_addr;

DROP TABLE _rep_backup;
