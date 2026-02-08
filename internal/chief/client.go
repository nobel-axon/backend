// Package chief provides an HTTP client for communicating with axon-chief.
package chief

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/axon-arena/axon-server/internal/config"
)

// Client is an HTTP client for the Chief service.
type Client struct {
	baseURL    string
	httpClient *http.Client
	retryCount int
	retryDelay time.Duration
}

// NewClient creates a new Chief client.
func NewClient(cfg *config.ChiefConfig) *Client {
	return &Client{
		baseURL: cfg.URL,
		httpClient: &http.Client{
			Timeout: cfg.GetTimeout(),
		},
		retryCount: cfg.RetryCount,
		retryDelay: cfg.GetRetryDelay(),
	}
}

// EventRequest is the request body for reporting events to Chief.
type EventRequest struct {
	Type    string                 `json:"type"`
	MatchID int64                  `json:"matchId"`
	Data    map[string]interface{} `json:"data,omitempty"`
}

// EventResponse is the response from Chief.
type EventResponse struct {
	Acknowledged bool   `json:"acknowledged"`
	Message      string `json:"message,omitempty"`
}

// ReportEvent sends an event to the Chief service.
func (c *Client) ReportEvent(ctx context.Context, eventType string, matchID int64, data map[string]interface{}) (*EventResponse, error) {
	req := EventRequest{
		Type:    eventType,
		MatchID: matchID,
		Data:    data,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal event: %w", err)
	}

	var lastErr error
	for i := 0; i <= c.retryCount; i++ {
		if i > 0 {
			log.Printf("Chief client: retrying POST /event type=%s matchId=%d (attempt %d/%d)",
				eventType, matchID, i+1, c.retryCount+1)
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(c.retryDelay):
			}
		}

		resp, err := c.doRequest(ctx, "POST", "/event", body)
		if err != nil {
			lastErr = err
			log.Printf("Chief client: POST /event type=%s matchId=%d failed: %v", eventType, matchID, err)
			continue
		}

		var eventResp EventResponse
		if err := json.Unmarshal(resp, &eventResp); err != nil {
			lastErr = fmt.Errorf("failed to unmarshal response: %w", err)
			continue
		}

		log.Printf("Chief client: POST /event type=%s matchId=%d -> acknowledged=%v", eventType, matchID, eventResp.Acknowledged)
		return &eventResp, nil
	}

	return nil, fmt.Errorf("failed after %d retries: %w", c.retryCount, lastErr)
}

// Health checks if Chief is healthy.
func (c *Client) Health(ctx context.Context) error {
	_, err := c.doRequest(ctx, "GET", "/health", nil)
	return err
}

func (c *Client) doRequest(ctx context.Context, method, path string, body []byte) ([]byte, error) {
	var bodyReader io.Reader
	if body != nil {
		bodyReader = bytes.NewReader(body)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	return respBody, nil
}

// Event types to report to Chief
const (
	EventTypeRegistrationClosed = "registration_closed"
	EventTypeAgentJoinedQueue   = "agent_joined_queue"
	EventTypeAnswerSubmitted    = "answer_submitted"
	EventTypeAnswerTimeout      = "answer_timeout"
)

// MatchDetailResponse is the response from Chief's GET /matches/:matchId endpoint.
type MatchDetailResponse struct {
	MatchID        string                 `json:"matchId"`
	Phase          string                 `json:"phase"`
	Pool           string                 `json:"pool"`
	PlayerCount    int                    `json:"playerCount"`
	Question       *QuestionResponse      `json:"question,omitempty"`
	Personalities  []PersonalityResponse  `json:"personalities,omitempty"`
	Evaluations    []EvaluationResponse   `json:"evaluations,omitempty"`
	QueueDeadline  int64                  `json:"queueDeadline,omitempty"`
	AnswerDeadline int64                  `json:"answerDeadline,omitempty"`
	Winner         string                 `json:"winner,omitempty"`
}

// QuestionResponse is the question data in the match detail response.
type QuestionResponse struct {
	Text       string `json:"text"`
	Category   string `json:"category"`
	Difficulty uint8  `json:"difficulty"`
	FormatHint string `json:"formatHint"`
}

// PersonalityResponse is a judge personality in the match detail response.
type PersonalityResponse struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Perspective string   `json:"perspective"`
	Values      []string `json:"values"`
	Style       string   `json:"style"`
}

// EvaluationResponse is a participant's evaluation in the match detail response.
type EvaluationResponse struct {
	Participant string                          `json:"participant"`
	Answer      string                          `json:"answer"`
	Reasoning   string                          `json:"reasoning,omitempty"`
	Evaluations []PersonalityEvaluationResponse `json:"evaluations"`
	TotalScore  int                             `json:"totalScore"`
	Agreement   string                          `json:"agreement"`
}

// PersonalityEvaluationResponse is a single personality's evaluation.
type PersonalityEvaluationResponse struct {
	PersonalityID   string `json:"personalityId"`
	PersonalityName string `json:"personalityName"`
	Score           int    `json:"score"`
	Convinced       string `json:"convinced"`
	Concerns        string `json:"concerns"`
	Verdict         string `json:"verdict"`
	AgreeLevel      string `json:"agreeLevel"`
}

// GetMatchState fetches detailed match state from Chief including question and personalities.
func (c *Client) GetMatchState(ctx context.Context, matchID int64) (*MatchDetailResponse, error) {
	path := fmt.Sprintf("/matches/%d", matchID)

	var lastErr error
	for i := 0; i <= c.retryCount; i++ {
		if i > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(c.retryDelay):
			}
		}

		resp, err := c.doRequest(ctx, "GET", path, nil)
		if err != nil {
			lastErr = err
			continue
		}

		var matchResp MatchDetailResponse
		if err := json.Unmarshal(resp, &matchResp); err != nil {
			lastErr = fmt.Errorf("failed to unmarshal response: %w", err)
			continue
		}

		return &matchResp, nil
	}

	return nil, fmt.Errorf("failed after %d retries: %w", c.retryCount, lastErr)
}
