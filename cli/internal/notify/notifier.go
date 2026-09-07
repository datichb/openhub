// Package notify provides notification dispatch to external channels.
// Supported: Mattermost, Slack, Discord, Microsoft Teams.
package notify

import (
	"context"
	"log/slog"
	"time"

	"github.com/datichb/openhub/cli/internal/teamstate"
)

// Notifier is the interface for all notification backends.
type Notifier interface {
	// Send dispatches a text message to the configured channel.
	Send(ctx context.Context, text string) error
	// Name returns the notifier type name (e.g. "slack", "mattermost").
	Name() string
}

// NewDispatcher creates a Dispatcher from team-state notification config.
// Returns a no-op dispatcher if notifications are disabled.
func NewDispatcher(cfg *teamstate.TeamConfig) *Dispatcher {
	if !cfg.Notification.Enabled {
		return &Dispatcher{enabled: false}
	}

	var notifiers []Notifier

	// Multi-destination mode
	if len(cfg.Notification.Destinations) > 0 {
		for _, dest := range cfg.Notification.Destinations {
			if n := newNotifierFromDest(dest, cfg.Notification.BotName); n != nil {
				notifiers = append(notifiers, n)
			}
		}
	} else {
		// Single destination (backward compat + type-based routing)
		n := newNotifierFromConfig(&cfg.Notification)
		if n != nil {
			notifiers = append(notifiers, n)
		}
	}

	if len(notifiers) == 0 {
		return &Dispatcher{enabled: false}
	}

	return &Dispatcher{enabled: true, notifiers: notifiers}
}

// newNotifierFromConfig creates a single Notifier from the main NotificationConfig.
func newNotifierFromConfig(cfg *teamstate.NotificationConfig) Notifier {
	notifyType := cfg.Type
	if notifyType == "" {
		// Backward compat: if MattermostWebhook is set, assume mattermost
		if cfg.MattermostWebhook != "" {
			notifyType = "mattermost"
		} else if cfg.WebhookURL != "" {
			return nil // type required when using generic webhook_url
		}
	}

	botName := cfg.BotName
	if botName == "" {
		botName = "OpenHub"
	}

	switch notifyType {
	case "mattermost":
		url := cfg.WebhookURL
		if url == "" {
			url = cfg.MattermostWebhook
		}
		if url == "" {
			return nil
		}
		return NewMattermost(url, cfg.Channel, botName)
	case "slack":
		if cfg.WebhookURL == "" {
			return nil
		}
		return NewSlack(cfg.WebhookURL, botName)
	case "discord":
		if cfg.WebhookURL == "" {
			return nil
		}
		return NewDiscord(cfg.WebhookURL, botName)
	case "teams":
		if cfg.WebhookURL == "" {
			return nil
		}
		return NewTeams(cfg.WebhookURL)
	}
	return nil
}

// newNotifierFromDest creates a Notifier from a NotificationDestination entry.
func newNotifierFromDest(dest teamstate.NotificationDestination, defaultBotName string) Notifier {
	botName := dest.BotName
	if botName == "" {
		botName = defaultBotName
	}
	if botName == "" {
		botName = "OpenHub"
	}
	if dest.WebhookURL == "" {
		return nil
	}
	switch dest.Type {
	case "mattermost":
		return NewMattermost(dest.WebhookURL, dest.Channel, botName)
	case "slack":
		return NewSlack(dest.WebhookURL, botName)
	case "discord":
		return NewDiscord(dest.WebhookURL, botName)
	case "teams":
		return NewTeams(dest.WebhookURL)
	}
	return nil
}

// Dispatcher routes events to one or more notification backends.
type Dispatcher struct {
	notifiers []Notifier
	enabled   bool
}

// Dispatch formats and sends a notification for the given event.
// Returns nil immediately if notifications are disabled.
// Sends to all configured backends; returns the last error if any fail.
func (d *Dispatcher) Dispatch(ctx context.Context, e teamstate.Event) error {
	if !d.enabled {
		return nil
	}

	text := FormatEvent(e)
	if text == "" {
		return nil
	}

	var lastErr error
	for _, n := range d.notifiers {
		slog.Debug("notify.dispatch.start", "type", n.Name(), "event", e.Type)
		start := time.Now()
		if err := n.Send(ctx, text); err != nil {
			elapsed := time.Since(start)
			slog.Warn("notify.dispatch.failed", "type", n.Name(), "error", err, "duration", elapsed)
			lastErr = err
		} else {
			elapsed := time.Since(start)
			slog.Debug("notify.dispatch.done", "type", n.Name(), "duration", elapsed)
		}
	}
	return lastErr
}
