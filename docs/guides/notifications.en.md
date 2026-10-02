> [Lire en francais](notifications.fr.md)

# Notifications — Setup and Configuration Guide

## Overview

openhub can dispatch notifications to team messaging platforms when key events occur during AI sessions. Notifications are sent to Slack, Discord, Mattermost, or Microsoft Teams via incoming webhooks.

### Supported Platforms

| Platform | Payload Format | Bot Name | Channel Override |
|----------|---------------|----------|-----------------|
| Slack | `{"text", "username"}` | Yes | No |
| Discord | `{"content", "username"}` | Yes | No |
| Mattermost | `{"text", "username", "channel"}` | Yes | Yes |
| Microsoft Teams | MessageCard (`@type: MessageCard`) | No | No |

---

## Events

Not all team events trigger notifications. The table below shows which events are automatically dispatched:

| Event | Type | Auto-notified | Description |
|-------|------|:---:|-------------|
| Review ready | `review.ready` | Yes | AI review completed, MR ready for human review |
| Review approved | `review.approved` | Yes | Human reviewer approved the MR |
| Review rejected | `review.rejected` | Yes | Human reviewer rejected the MR |
| Wiki proposal | `wiki.proposal` | Yes | Agent proposed a wiki page update |
| Custom notification | `custom.notification` | Yes | Agent sent a custom message via `team_notify` |
| Session complete | `session.complete` | No | AI session finished (logged only) |
| Claim taken | `claim.taken` | No | Agent claimed a ticket (logged only) |
| Claim conflict | `claim.conflict` | No | Two agents tried to claim the same ticket (logged only) |
| Claim transferred | `claim.transferred` | No | Ticket transferred between agents (logged only) |
| Claim released | `claim.released` | No | Agent released a ticket (logged only) |
| Wiki accepted | `wiki.accepted` | No | Wiki proposal accepted (logged only) |
| Wiki rejected | `wiki.rejected` | No | Wiki proposal rejected (logged only) |
| Audit finding | `audit.finding` | No | Audit reported findings (logged only) |

> **Note:** "Logged only" events are recorded in the team-state event log and visible in the TUI notification history, but are not dispatched to external platforms.

---

## Configuration

Notifications are configured in the **team-state** repository's `config.toml` (NOT in `hub.toml`).

### Single destination

```toml
[notification]
enabled = true
type = "slack"                    # slack | discord | mattermost | teams
webhook_url = "https://hooks.slack.com/services/T.../B.../..."
bot_name = "OpenHub"
```

### Multi-destination

```toml
[notification]
enabled = true
bot_name = "OpenHub"              # Default for all destinations

[[notification.destinations]]
type = "slack"
webhook_url = "https://hooks.slack.com/services/T.../B.../..."

[[notification.destinations]]
type = "discord"
webhook_url = "https://discord.com/api/webhooks/.../..."

[[notification.destinations]]
type = "mattermost"
webhook_url = "https://mattermost.example.com/hooks/..."
channel = "#dev-ai"               # Mattermost only
bot_name = "MattermostBot"        # Per-destination override
```

### Platform-specific setup

**Slack:** Create an [Incoming Webhook](https://api.slack.com/messaging/webhooks) in your Slack workspace settings.

**Discord:** In channel settings > Integrations > Webhooks > New Webhook. Copy the webhook URL.

**Mattermost:** In System Console > Integrations > Incoming Webhooks. Optionally specify a `channel` override.

**Microsoft Teams:** Create a [Power Automate workflow](https://learn.microsoft.com/en-us/microsoftteams/platform/webhooks-and-connectors/how-to/add-incoming-webhook) or use a legacy incoming webhook connector.

---

## Setup Flow

1. **During `oh team init`** — The interactive wizard offers notification setup as an optional step
2. **Manual** — Edit `~/.oh/team-state/config.toml` directly
3. **Commit** — Push the config to share with the team:
   ```bash
   cd ~/.oh/team-state && git add config.toml && git commit -m "feat: add notifications" && git push
   ```

---

## Testing

```bash
oh team notify test                      # Send a test notification
oh team notify test --message "Hello!"   # Custom test message
```

---

## TUI Integration

Notifications also appear in the TUI:

- **Toast notifications** — Pop up in real-time for all events
- **Notification history** — Type `notifications` (or `notif`, `logs`, `messages`, `toasts`) in the omnibar
- **Persistence** — Last 50 notifications stored in `~/.oh/notifications.jsonl`
- **Errors** — Error-level notifications are written to stderr

---

## Troubleshooting

### Notifications not sent

1. Verify `enabled = true` in `[notification]`
2. Verify the webhook URL is correct and accessible
3. Test with `oh team notify test`
4. Check logs: webhook URLs are masked in logs for security (`httplog.WithMaskURL`)

### Wrong platform format

Each platform expects a specific payload format. If you see `400 Bad Request`, verify the `type` field matches the actual webhook platform.

### Mattermost channel not found

The `channel` field must include the `#` prefix (e.g., `#dev-ai`). If omitted, the webhook's default channel is used.

---

## Known Limitations

- **Notification messages are in French only** — All event format strings are currently hardcoded in French. Internationalization of notification messages is planned but not yet implemented.
- **No event filtering** — All auto-notified events are dispatched to all destinations. Per-event or per-destination filtering is not supported.
- **No retry on failure** — If a webhook call fails, the error is logged but the notification is not retried.
- **10-second timeout** — Webhook HTTP calls timeout after 10 seconds.

---

## Resources

- [Team Setup Guide](team-setup.en.md)
- [MCP Team Server Reference](../reference/mcp-team.en.md) — `team_notify` tool
- [Configuration Reference](../reference/config.en.md)

---

## Support

```bash
oh team notify test      # Test notification dispatch
oh team status           # Check team configuration
```

For issues: [GitHub Issues](https://github.com/datichb/openhub/issues)
