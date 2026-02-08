package models

import (
	"encoding/json"
	"time"
)

// MatchPersonalities stores judge personalities and panel for a match.
type MatchPersonalities struct {
	MatchID       int64           `db:"match_id" json:"matchId"`
	Personalities json.RawMessage `db:"personalities" json:"personalities"`
	JudgePanel    json.RawMessage `db:"judge_panel" json:"judgePanel"`
	CreatedAt     time.Time       `db:"created_at" json:"createdAt"`
}
