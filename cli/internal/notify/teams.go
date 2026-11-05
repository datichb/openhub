package notify

import (
	"context"
	"net/http"
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
		httpClient: newHTTPClient("notify.teams"),
	}
}

// Name implements Notifier.
func (t *TeamsClient) Name() string { return "teams" }

// Send posts a message to the Teams webhook using the MessageCard format.
// Compatible with both legacy Incoming Webhooks and Power Automate connectors.
func (t *TeamsClient) Send(ctx context.Context, text string) error {
	payload := struct {
		Type    string `json:"@type"`
		Context string `json:"@context"`
		Text    string `json:"text"`
	}{
		Type:    "MessageCard",
		Context: "http://schema.org/extensions",
		Text:    text,
	}
	return postJSON(ctx, t.httpClient, "teams", t.webhookURL, payload)
}
