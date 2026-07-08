// Package notify provides notification dispatch to external channels (Mattermost).
package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/datichb/openhub/cli/internal/teamstate"
)

// MattermostClient sends messages via an incoming webhook.
type MattermostClient struct {
	webhookURL string
	channel    string
	botName    string
	httpClient *http.Client
}

// MattermostMessage represents a payload sent to the Mattermost webhook.
type MattermostMessage struct {
	Channel  string `json:"channel,omitempty"`
	Username string `json:"username,omitempty"`
	Text     string `json:"text"`
	IconURL  string `json:"icon_url,omitempty"`
}

// NewMattermost creates a new Mattermost notification client.
func NewMattermost(webhookURL, channel, botName string) *MattermostClient {
	return &MattermostClient{
		webhookURL: webhookURL,
		channel:    channel,
		botName:    botName,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

// Send posts a message to the configured Mattermost channel.
func (m *MattermostClient) Send(ctx context.Context, text string) error {
	msg := MattermostMessage{
		Channel:  m.channel,
		Username: m.botName,
		Text:     text,
	}

	body, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshaling message: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, m.webhookURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := m.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("sending notification: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("mattermost webhook returned %d", resp.StatusCode)
	}
	return nil
}

// Dispatcher routes events to notification channels.
type Dispatcher struct {
	mattermost *MattermostClient
	enabled    bool
}

// NewDispatcher creates a Dispatcher from team-state config.
// Returns a no-op dispatcher if notifications are disabled.
func NewDispatcher(cfg *teamstate.TeamConfig) *Dispatcher {
	if !cfg.Notification.Enabled || cfg.Notification.MattermostWebhook == "" {
		return &Dispatcher{enabled: false}
	}
	return &Dispatcher{
		enabled: true,
		mattermost: NewMattermost(
			cfg.Notification.MattermostWebhook,
			cfg.Notification.Channel,
			cfg.Notification.BotName,
		),
	}
}

// Dispatch formats and sends a notification for the given event.
// Returns nil immediately if notifications are disabled.
func (d *Dispatcher) Dispatch(ctx context.Context, e teamstate.Event) error {
	if !d.enabled {
		return nil
	}

	text := FormatEvent(e)
	if text == "" {
		return nil
	}

	return d.mattermost.Send(ctx, text)
}

// FormatEvent converts a team event into a human-readable notification message.
func FormatEvent(e teamstate.Event) string {
	prefix := fmt.Sprintf("[%s]", e.Project)

	switch e.Type {
	case teamstate.EventSessionComplete:
		duration := ""
		if d, ok := e.Data["duration_min"]; ok {
			duration = fmt.Sprintf(" (%v min)", d)
		}
		branch := ""
		if b, ok := e.Data["branch"]; ok {
			branch = fmt.Sprintf(" (%s)", b)
		}
		return fmt.Sprintf("%s %s a terminé %s%s%s", prefix, e.Actor, e.Ticket, duration, branch)

	case teamstate.EventReviewReady:
		mr := ""
		if url, ok := e.Data["mr_url"]; ok {
			mr = fmt.Sprintf(" — %s", url)
		}
		return fmt.Sprintf("%s Review prête pour %s%s", prefix, e.Ticket, mr)

	case teamstate.EventAuditFinding:
		count := ""
		if n, ok := e.Data["finding_count"]; ok {
			count = fmt.Sprintf(": %v finding(s)", n)
		}
		domain := ""
		if d, ok := e.Data["domain"]; ok {
			domain = fmt.Sprintf(" %s", d)
		}
		return fmt.Sprintf("%s Audit%s%s sur %s", prefix, domain, count, e.Ticket)

	case teamstate.EventClaimTaken:
		return fmt.Sprintf("%s %s a pris %s", prefix, e.Actor, e.Ticket)

	case teamstate.EventClaimConflict:
		owner := ""
		if o, ok := e.Data["current_owner"]; ok {
			owner = fmt.Sprintf(" (déjà pris par %s)", o)
		}
		return fmt.Sprintf("%s :warning: Conflit de claim sur %s%s", prefix, e.Ticket, owner)

	case teamstate.EventClaimTransferred:
		from := e.Actor
		to := ""
		if t, ok := e.Data["to"]; ok {
			to = fmt.Sprintf("%s", t)
		}
		return fmt.Sprintf("%s %s transféré de %s à %s", prefix, e.Ticket, from, to)

	case teamstate.EventClaimReleased:
		return fmt.Sprintf("%s %s a libéré %s", prefix, e.Actor, e.Ticket)

	case teamstate.EventWikiProposal:
		page := ""
		if p, ok := e.Data["page"]; ok {
			page = fmt.Sprintf(" (%s)", p)
		}
		return fmt.Sprintf("[Équipe] Proposition wiki%s par %s (depuis %s)", page, e.Actor, e.Project)

	case teamstate.EventWikiAccepted:
		page := ""
		if p, ok := e.Data["page"]; ok {
			page = fmt.Sprintf(" %s", p)
		}
		return fmt.Sprintf("[Équipe] Wiki%s mis à jour par %s", page, e.Actor)

	default:
		return ""
	}
}
