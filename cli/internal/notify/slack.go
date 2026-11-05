package notify

import (
	"context"
	"net/http"
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
		httpClient: newHTTPClient("notify.slack"),
	}
}

// Name implements Notifier.
func (s *SlackClient) Name() string { return "slack" }

// Mention implements Mentioner: with link_names, Slack notifies @username.
func (s *SlackClient) Mention(username string) string { return "@" + username }

// SlackMessage is the payload of a Slack Incoming Webhook.
type SlackMessage struct {
	Text      string `json:"text"`
	Username  string `json:"username,omitempty"`
	LinkNames bool   `json:"link_names,omitempty"`
}

// Send posts a message to the configured Slack webhook.
func (s *SlackClient) Send(ctx context.Context, text string) error {
	return postJSON(ctx, s.httpClient, "slack", s.webhookURL, SlackMessage{Text: text, Username: s.botName, LinkNames: true})
}
