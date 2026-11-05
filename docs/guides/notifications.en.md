> [Lire en francais](notifications.fr.md)

# Notifications — Setup and Configuration Guide

## Three channels, one personal setting

oh tells you things in three ways, which are not the same:

| Channel | Who sees it | Where it is set |
|---------|-------------|-----------------|
| **System notification** (macOS notification centre, `notify-send`…) | you, even when oh is closed: the **daemon** shows it | `hub.toml` `[notifications]`, Settings › Notifications |
| **TUI toast** (and its history, `~/.oh/notifications.jsonl`) | you, while the TUI is open | the same setting |
| **Team webhook** (Mattermost, Slack, Discord, Teams) | the whole team | `config.toml` of the team-state, `[notification]` |

The first two are **personal** (your `hub.toml`); the third is **shared** (the team-state repository). This guide covers all three.

---

## Who is told what, and how (defaults)

● on by default · ○ available, off by default.

| Event | For | System | Toast | Webhook | Mention | Dedup / reminder |
|---|---|:-:|:-:|:-:|:-:|---|
| Checkpoint / question / permission | me | ● (not for an attached session) | ● | ○ | ○ | « asleep with a decision waiting » notification; the decision is taken up at wake-up; one reminder when the daemon restarts |
| Budget exhausted | me | ● | ● | ○ | — | once per session and allowance, not at every step |
| Session failed or crashed | me | ● | ● | ○ | — | one per session per 10 minutes |
| Credential refused, token expired (LLM, tracker, git), unknown SSH server | me | ● with the cause | ● | — | — | one per key |
| Step finished (session not attached) | me | ○ | ● | — | — | grouped |
| Session or workflow finished | me | ● | ● | ○ `session.complete` | — | — |
| Remote session ready or failed | me | ● | ● | — | — | — |
| MR or review ready | the team and the reviewer | — | ○ | ● | ● reviewer | — |
| Review verdict | the team and the author | — | ○ | ● | ● author | — |
| Claim handed to me | the recipient | ○ | ● | ○ | ● | — |
| Claim taken, released, in conflict | the team | — | — | ○ | — | — |
| Claim expired or about to | the owner | ● | ● | ○ | ● | one per claim |
| Tracker sync | me | — | summary | — | — | a repeated failure is said once |
| Wiki proposed, accepted, rejected | the team and the author | — | ○ | ● | ○ | same path in the TUI and the CLI |
| Team config changed (`config_changed`) | the team | — | ○ | ○ | — | — |

> The « Toast » column of the team events (review, wiki, claims, configuration) says what **you** may choose to see on your screen; the team hears of them through the webhook.

---

## Your settings: `hub.toml` `[notifications]`

Shown in **Settings › Notifications**; **applied at once**: the daemon and the TUI read `hub.toml` again when it changes, no restart.

```toml
[notifications]
system = true              # system notifications of the daemon (false: only the toasts stay)
sound = true               # the system sound with the notification
quiet_hours = "22:00-08:00" # no system notification or sound (may cross midnight); the toasts stay
when_attached = false      # also notify a session a terminal is attached to
when_tui_focused = true    # also notify while a TUI is open (oh cannot tell which window is in front)

[notifications.events]     # system | toast | off, per event type
step_done = "system"
claim = "toast"
```

Values of `events.<type>`: `system` (system notification **and** toast), `toast` (toast only), `off` (nothing). Types: `checkpoint`, `question`, `permission`, `sleep`, `budget`, `error`, `credential`, `step_done`, `session_complete`, `remote`, `memory`, `review`, `verdict`, `claim_transferred`, `claim`, `claim_expired`, `tracker_sync`, `wiki`, `config_changed`. Only the choices that differ from the default are written.

The former `[session] notify = "off"` still turns the system notifications off as long as `[notifications] system` is not written.

One system notification per family (decisions, budget, errors, alerts, end of step…): one family does not replace another in the notification centre. Alerts say their **cause** in the title (« Bedrock key refused by AWS », « Team-state access refused », « Unknown SSH server »…), one notification per key.

### A decision waiting, TUI closed

The sleep of a server stays at `idle_sleep_minutes` (5 min). If a decision (permission, question, checkpoint) waits at that moment, oh **keeps** it in « To handle » (marked asleep), tells you (« asleep with a decision waiting ») and, when the session wakes up:

- if the tool asks the request again, it is a normal decision again;
- otherwise the request died with the server: oh closes it and **asks the agent to ask again** (a queued instruction).

While the session sleeps, answering the decision is refused with a message that says to resume the session (`oh session resume`). The decisions still open for more than a minute when the daemon starts (and less than 24 h old) are recalled **once**.

---

## The team webhook

### Events sent

By default each destination receives the **review** (`review.*`), the **wiki** (`wiki.*`) and the messages of agents (`custom.notification`). The other events are sent when the destination asks for them with `events`: a type (`claim.transferred`), a family (`claim.*`) or `*`.

| Event | Type | Default |
|-------|------|:---:|
| Review ready / approved / rejected | `review.ready`, `review.approved`, `review.rejected` | Yes |
| Wiki proposed / accepted / rejected | `wiki.proposal`, `wiki.accepted`, `wiki.rejected` | Yes |
| Message of an agent | `custom.notification` | Yes |
| Claim taken / released / transferred / in conflict | `claim.taken`, `claim.released`, `claim.transferred`, `claim.conflict` | No |
| Claim about to expire / expired | `claim.expiring`, `claim.expired` | No |
| Session finished | `session.complete` | No |
| Audit finding | `audit.finding` | No |
| Workflow published, restored, archived | `workflow.published`, `workflow.restored`, `workflow.archived` | No |
| Team config changed | `config_changed` | No |

Every event written to the activity of the team is announced **once, when its transaction is committed**, whoever wrote it (a command, the daemon, an agent, the TUI). A merge request created by an agent (`gitlab_create_mr`) announces `review.ready` like `oh mr create`.

### Mentions

The reviewer asked (`oh mr create --reviewer`), the author of a review or of a wiki proposal, the member a ticket is handed to, the owner of a claim that expires are **mentioned** with the `mattermost_username` of their card (`oh team profile`), on Mattermost and Slack. On Discord and Teams the text gives their name, without a mention. `mention = false` turns them off. A person without a `mattermost_username` is not mentioned.

### Configuration

In the `config.toml` of the team-state (not in `hub.toml`):

```toml
[notification]
enabled = true                    # without this switch nothing is sent
type = "slack"                    # slack | discord | mattermost | teams; inferred from the webhook when missing
webhook_url = "https://hooks.slack.com/services/T.../B.../..."
bot_name = "OpenHub"
events = ["review.*", "wiki.*", "claim.transferred"]   # optional
mention = true                    # optional (true by default)
```

Several destinations, each with its `events` and `mention`:

```toml
[notification]
enabled = true
bot_name = "OpenHub"

[[notification.destinations]]
type = "slack"
webhook_url = "https://hooks.slack.com/services/T.../B.../..."

[[notification.destinations]]
type = "mattermost"
webhook_url = "https://mattermost.example.com/hooks/..."
channel = "dev-ai"
events = ["*"]
```

Without a terminal: `oh team config set notification.events "review.*, claim.*"`, `oh team config set notification.destinations.0.mention false`. In the TUI: team detail › Notifications (published with Ctrl+S).

**The webhook is a secret**: it is written in the team-state repository, so it is masked in every screen, summary, event and error message; `oh doctor` warns when that repository can be read by anybody.

### Per platform

| Platform | Payload | Bot name | Channel | Mention |
|----------|---------|----------|---------|---------|
| Slack | `{"text", "username", "link_names"}` | Yes | No | `@name` |
| Discord | `{"content", "username"}` (no mention is parsed) | Yes | No | No |
| Mattermost | `{"text", "username", "channel"}` | Yes | Yes | `@name` |
| Microsoft Teams | MessageCard | No | No | No |

**Slack:** create an [Incoming Webhook](https://api.slack.com/messaging/webhooks). **Discord:** channel › Integrations › Webhooks. **Mattermost:** System Console › Integrations › Incoming Webhooks. **Teams:** a [Power Automate workflow](https://learn.microsoft.com/en-us/microsoftteams/platform/webhooks-and-connectors/how-to/add-incoming-webhook) or an incoming connector.

### Text and reliability

- The text is written **in the language of the sender**, with the names of the members and projects (not their IDs) and the symbols of the activity (`✔ ✗ ●`), which work on every platform.
- The destinations are reached **in parallel** (8 s at most each).
- A message that does not reach its webhook (server down, network cut, 5xx, 429) is **kept** in `~/.oh/notify/outbox/`; the daemon sends it again after 1 min, 5 min, 15 min, 1 h, 3 h, then 6 h, then gives up and says so in the notification history. A revoked webhook (404, 403…) is not retried: the error is said at once.
- Errors never contain the address of the webhook.

---

## The `notify` MCP tool of agents

An agent can write to the team with `team_notify`. The message is **signed** (member and project), limited to 1000 characters and 5 messages per 10 minutes, and cannot call `@channel`, `@all`, `@here`. When notifications are off, without a destination, or failing, the tool **answers with an error**: the agent no longer believes it told the team.

---

## Test and diagnosis

```bash
oh team notify test                      # send a test message to the destinations
oh team notify test --message "Hello!"   # custom message
oh doctor                                # « Notifications » check
```

`oh doctor` checks the system notification tool (oh cannot tell whether the system allows it: if nothing shows, allow the notifications of your terminal), reports the messages waiting for another attempt, and that the server of each webhook answers (without sending it a message).

The history (`notifications` in the omnibar) keeps the last 500 lines of `~/.oh/notifications.jsonl`: toasts, tracker syncs done by the daemon, claims about to expire, webhooks given up. The same message is not shown or recorded twice within 5 seconds.

---

## Troubleshooting

1. Nothing reaches the webhook: `enabled = true`? a `type` and a `webhook_url`? is the event in `events` (review and wiki only by default)? `oh team notify test`, then `oh doctor`.
2. No system notification: `[notifications] system`, `quiet_hours`, the event type (`events.<type>` set to `system`), an attached session (`when_attached`), an open TUI (`when_tui_focused`), and the permission of the system.
3. `400 Bad Request`: the `type` does not match the platform of the webhook.
4. Mattermost channel not found: `channel` is the name of the channel.

---

## Resources

- [Team setup guide](team-setup.en.md)
- [Team MCP server reference](../reference/mcp-team.en.md) — `team_notify` tool
- [Configuration reference](../reference/config.en.md)

To report a problem: [GitHub Issues](https://github.com/datichb/openhub/issues)
