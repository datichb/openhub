package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// TeamsClient sends messages via a Microsoft Teams Incoming Webhook.
type TeamsClient struct {
	webhookURL string
	httpClient *http.Client
}

// NewTeams creates a new Microsoft Teams notification client.
func NewTeams(webhookURL string) *TeamsClient {
	return &TeamsClient{
		webhookURL: webhookURL,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

// Name implements Notifier.
func (t *TeamsClient) Name() string { return "teams" }

// Send posts a message to the Teams webhook using the MessageCard format.
// Compatible with both legacy Incoming Webhooks and Power Automate connectors.
func (t *TeamsClient) Send(ctx context.Context, text string) error {
	// Teams uses the MessageCard format for incoming webhooks
	payload := struct {
		Type    string `json:"@type"`
		Context string `json:"@context"`
		Text    string `json:"text"`
	}{
		Type:    "MessageCard",
		Context: "http://schema.org/extensions",
		Text:    text,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshaling teams message: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, t.webhookURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("creating teams request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("sending teams notification: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("teams webhook returned %d", resp.StatusCode)
	}
	return nil
}
