package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// SlackClient sends messages via a Slack Incoming Webhook.
type SlackClient struct {
	webhookURL string
	botName    string
	httpClient *http.Client
}

// NewSlack creates a new Slack notification client.
func NewSlack(webhookURL, botName string) *SlackClient {
	return &SlackClient{
		webhookURL: webhookURL,
		botName:    botName,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

// Name implements Notifier.
func (s *SlackClient) Name() string { return "slack" }

// Send posts a message to the configured Slack webhook.
// Uses Slack's Incoming Webhooks format.
func (s *SlackClient) Send(ctx context.Context, text string) error {
	payload := struct {
		Text     string `json:"text"`
		Username string `json:"username,omitempty"`
	}{
		Text:     text,
		Username: s.botName,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshaling slack message: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.webhookURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("creating slack request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("sending slack notification: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("slack webhook returned %d", resp.StatusCode)
	}
	return nil
}
