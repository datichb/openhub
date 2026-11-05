package notify

import (
	"context"
	"net/http"
)

// MattermostClient sends messages via an incoming webhook.
type MattermostClient struct {
	webhookURL string
	channel    string
	botName    string
	httpClient *http.Client
}

// MattermostMessage represents a payload sent to the Mattermost webhook.
// Exported for test assertions.
type MattermostMessage struct {
	Channel  string `json:"channel,omitempty"`
	Username string `json:"username,omitempty"`
	Text     string `json:"text"`
}

// NewMattermost creates a new Mattermost notification client.
func NewMattermost(webhookURL, channel, botName string) *MattermostClient {
	return &MattermostClient{
		webhookURL: webhookURL,
		channel:    channel,
		botName:    botName,
		httpClient: newHTTPClient("notify.mattermost"),
	}
}

// Name implements Notifier.
func (m *MattermostClient) Name() string { return "mattermost" }

// Mention implements Mentioner: Mattermost notifies the user named @username.
func (m *MattermostClient) Mention(username string) string { return "@" + username }

// Send posts a message to the configured Mattermost channel.
func (m *MattermostClient) Send(ctx context.Context, text string) error {
	return postJSON(ctx, m.httpClient, "mattermost", m.webhookURL, MattermostMessage{
		Channel:  m.channel,
		Username: m.botName,
		Text:     text,
	})
}
