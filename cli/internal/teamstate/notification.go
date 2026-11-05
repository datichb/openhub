package teamstate

import (
	"net/url"
	"strings"
)

// InferNotifierType is the kind of notifier a webhook URL belongs to, by its
// server: Slack, Discord and Microsoft Teams have a known one, anything else
// is a Mattermost (the default of the team notifications).
func InferNotifierType(webhook string) string {
	u, err := url.Parse(strings.TrimSpace(webhook))
	if err != nil {
		return "mattermost"
	}
	host := strings.ToLower(u.Hostname())
	switch {
	case host == "hooks.slack.com":
		return "slack"
	case host == "discord.com" || host == "discordapp.com" || strings.HasSuffix(host, ".discord.com"):
		return "discord"
	case strings.HasSuffix(host, ".webhook.office.com") || host == "outlook.office.com" || strings.HasSuffix(host, ".logic.azure.com"):
		return "teams"
	}
	return "mattermost"
}

// EffectiveURL is the webhook of the team: webhook_url, else the former
// mattermost_webhook.
func (n NotificationConfig) EffectiveURL() string {
	if n.WebhookURL != "" {
		return n.WebhookURL
	}
	return n.MattermostWebhook
}

// EffectiveType is the kind of notifier of the team: type, else the one the
// webhook belongs to (a team that wrote a webhook and no type, as the former
// assistant did, gets its notifications). Empty when there is no webhook.
func (n NotificationConfig) EffectiveType() string {
	if n.Type != "" {
		return n.Type
	}
	if u := n.EffectiveURL(); u != "" {
		return InferNotifierType(u)
	}
	return ""
}

// UsesFormerWebhookKey reports whether the webhook is still under the former
// mattermost_webhook key (oh doctor --fix moves it to webhook_url and type).
func (n NotificationConfig) UsesFormerWebhookKey() bool {
	return n.MattermostWebhook != "" && n.WebhookURL == ""
}

// MaskWebhook is a webhook URL as it may be shown: the server and the last
// characters, never the token that makes the URL a secret. Empty stays empty.
func MaskWebhook(webhook string) string {
	webhook = strings.TrimSpace(webhook)
	if webhook == "" {
		return ""
	}
	u, err := url.Parse(webhook)
	if err != nil || u.Host == "" {
		return "••••"
	}
	tail := ""
	if p := strings.Trim(u.Path, "/"); len(p) > 4 {
		tail = p[len(p)-4:]
	}
	return u.Scheme + "://" + u.Host + "/…" + tail
}

// MentionOn reports whether the people an event is for are mentioned (the
// default).
func (n NotificationConfig) MentionOn() bool { return n.Mention == nil || *n.Mention }

// MentionOn is NotificationConfig.MentionOn for a destination.
func (d NotificationDestination) MentionOn() bool { return d.Mention == nil || *d.Mention }
