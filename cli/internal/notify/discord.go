package notify

import (
	"context"
	"net/http"
)

// DiscordClient sends messages via a Discord webhook.
type DiscordClient struct {
	webhookURL string
	botName    string
	httpClient *http.Client
}

// NewDiscord creates a new Discord notification client.
func NewDiscord(webhookURL, botName string) *DiscordClient {
	return &DiscordClient{
		webhookURL: webhookURL,
		botName:    botName,
		httpClient: newHTTPClient("notify.discord"),
	}
}

// Name implements Notifier.
func (d *DiscordClient) Name() string { return "discord" }

// Send posts a message to the Discord webhook (Discord answers 204 No Content).
// No mention is parsed: a message never pings @everyone.
func (d *DiscordClient) Send(ctx context.Context, text string) error {
	payload := struct {
		Content         string         `json:"content"`
		Username        string         `json:"username,omitempty"`
		AllowedMentions map[string]any `json:"allowed_mentions"`
	}{
		Content:         text,
		Username:        d.botName,
		AllowedMentions: map[string]any{"parse": []string{}},
	}
	return postJSON(ctx, d.httpClient, "discord", d.webhookURL, payload)
}
