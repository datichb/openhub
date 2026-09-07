package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/datichb/openhub/cli/internal/httplog"
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
		httpClient: httplog.Wrap(&http.Client{Timeout: 10 * time.Second}, "notify.discord", httplog.WithMaskURL()),
	}
}

// Name implements Notifier.
func (d *DiscordClient) Name() string { return "discord" }

// Send posts a message to the Discord webhook.
// Uses Discord's webhook execute format.
func (d *DiscordClient) Send(ctx context.Context, text string) error {
	payload := struct {
		Content  string `json:"content"`
		Username string `json:"username,omitempty"`
	}{
		Content:  text,
		Username: d.botName,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshaling discord message: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, d.webhookURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("creating discord request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := d.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("sending discord notification: %w", err)
	}
	defer resp.Body.Close()

	// Discord returns 204 No Content on success
	if resp.StatusCode != http.StatusNoContent && (resp.StatusCode < 200 || resp.StatusCode >= 300) {
		return fmt.Errorf("discord webhook returned %d", resp.StatusCode)
	}
	return nil
}
