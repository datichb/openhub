package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"time"

	"github.com/datichb/openhub/cli/internal/httplog"
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
		httpClient: httplog.Wrap(&http.Client{Timeout: 10 * time.Second}, "notify.mattermost", httplog.WithMaskURL()),
	}
}

// Name implements Notifier.
func (m *MattermostClient) Name() string { return "mattermost" }

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

// FormatEvent converts a team event into a human-readable notification message.
func FormatEvent(e teamstate.Event) string {
	prefix := fmt.Sprintf("[%s]", e.Project)

	switch e.Type {
	case teamstate.EventSessionComplete:
		duration := ""
		if d, ok := e.Data["duration_s"].(float64); ok && d > 0 {
			duration = fmt.Sprintf(" (%.0f min)", math.Round(d/60))
		}
		summary := ""
		if s, ok := e.Data["summary"].(string); ok && s != "" {
			summary = fmt.Sprintf(" — %s", s)
		}
		return fmt.Sprintf("%s %s a terminé %s%s%s", prefix, e.Actor, e.Ticket, duration, summary)
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
		to := ""
		if t, ok := e.Data["to"]; ok {
			to = fmt.Sprintf("%v", t)
		}
		return fmt.Sprintf("%s %s transféré de %s à %s", prefix, e.Ticket, e.Actor, to)
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
	case teamstate.EventWikiRejected:
		page := ""
		if p, ok := e.Data["page"]; ok {
			page = fmt.Sprintf(" %s", p)
		}
		return fmt.Sprintf("[Équipe] Proposition wiki%s rejetée par %s", page, e.Actor)
	case teamstate.EventReviewApproved:
		return fmt.Sprintf("%s Review approuvée pour %s par %s", prefix, e.Ticket, e.Actor)
	case teamstate.EventReviewRejected:
		reason := ""
		if r, ok := e.Data["reason"].(string); ok && r != "" {
			reason = fmt.Sprintf(" (%s)", r)
		}
		return fmt.Sprintf("%s Review rejetée pour %s par %s%s", prefix, e.Ticket, e.Actor, reason)
	case "custom.notification":
		if msg, ok := e.Data["message"]; ok {
			return fmt.Sprintf("%v", msg)
		}
		return ""
	default:
		return ""
	}
}
