-- Feed index for cursor-paginated match history.
-- Partial index: only settled/cancelled rows with a settled_at timestamp.
CREATE INDEX IF NOT EXISTS idx_matches_feed
  ON app_matches (settled_at DESC, match_id DESC)
  WHERE phase IN ('settled', 'cancelled') AND settled_at IS NOT NULL;
