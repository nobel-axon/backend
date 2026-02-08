package models

import "encoding/json"

// FeedItem is a single match card in the history feed.
type FeedItem struct {
	Match         MatchResponse        `json:"match"`
	Personalities json.RawMessage      `json:"personalities,omitempty"`
	JudgePanel    json.RawMessage      `json:"judgePanel,omitempty"`
	Players       []MatchPlayer        `json:"players"`
	Answers       []AnswerResponse     `json:"answers"`
	Commentary    []CommentaryResponse `json:"commentary"`
}

// FeedResponse is the paginated feed response.
type FeedResponse struct {
	Items      []FeedItem `json:"items"`
	NextCursor string     `json:"nextCursor,omitempty"`
	HasMore    bool       `json:"hasMore"`
}
