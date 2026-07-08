# MCP Server: team

## Overview

The `team` MCP server exposes team collaboration data to AI agents via the MCP protocol (stdio JSON-RPC). It reads from the local `~/.oh/team-state/` clone and provides read-only access to all agents, with write capabilities limited to specific agents.

## Activation

The team MCP server is deployed automatically when `[team].enabled = true` in `hub.toml`. No token is required.

```json
// Injected into opencode.json by oh deploy
{
  "mcpServers": {
    "team": {
      "command": "oh",
      "args": ["mcp", "serve", "team"]
    }
  }
}
```

## Tools

### `team_members`

List all team members.

**Input:** `{}` (no parameters)

**Output:** JSON array of members

```json
[
  {
    "ID": "benjamin",
    "DisplayName": "Benjamin",
    "GitLabUsername": "bdatiche",
    "MattermostUsername": "benjamin.datiche",
    "Role": "lead",
    "DefaultMode": "semi-auto"
  }
]
```

**Access:** All agents

---

### `team_claims`

List active ticket claims.

**Input:**
```json
{
  "project": "T-SRU"  // optional, empty = all projects
}
```

**Output:** JSON array of claims

```json
[
  {
    "TicketID": "SRU-142",
    "Project": "T-SRU",
    "ClaimedBy": "benjamin",
    "ClaimedAt": "2026-07-07T14:30:00Z",
    "Worktree": "feat/SRU-142-user-auth",
    "Status": "in_progress"
  }
]
```

**Access:** All agents

---

### `team_wiki_list`

List available wiki pages.

**Input:** `{}` (no parameters)

**Output:** JSON array of page names (without `.md` extension)

```json
["decisions", "patterns", "onboarding"]
```

**Access:** All agents

---

### `team_wiki_read`

Read a wiki page.

**Input:**
```json
{
  "page": "decisions"  // required
}
```

**Output:** Markdown content of the page (plain text)

**Access:** All agents

---

### `team_wiki_write`

Propose a new entry to the wiki (creates a pending proposal).

**Input:**
```json
{
  "page": "decisions",           // required
  "content": "## New Decision\n\nContent...",  // required, max 200 lines
  "confidence": "CONFIRMED",     // required: CONFIRMED | INFERRED | UNCERTAIN
  "project": "T-SRU"            // required: originating project
}
```

**Output:** Confirmation message

**Access:** `documentarian` only

**Behavior:**
1. Validates format and size constraints
2. Creates a proposal file in `wiki/.pending/`
3. Emits a `wiki.proposal` event
4. Sends a Mattermost notification
5. Returns confirmation — does NOT directly modify wiki pages

---

### `team_events`

List recent team activity events.

**Input:**
```json
{
  "project": "T-SRU",  // optional
  "limit": 20          // optional, default: 20
}
```

**Output:** JSON array of events (newest first)

```json
[
  {
    "ts": "2026-07-07T15:45:00Z",
    "actor": "benjamin",
    "event": "session.complete",
    "project": "T-SRU",
    "ticket": "SRU-142",
    "data": {"duration_min": 75}
  }
]
```

**Access:** All agents

---

### `team_notify`

Send a custom notification to the team Mattermost channel.

**Input:**
```json
{
  "message": "Implementation of SRU-142 is ready for review"  // required
}
```

**Output:** Confirmation

**Access:** `orchestrator-dev`, `reviewer`, `auditor`

## Agent Permission Configuration

In `opencode.json`, permissions are set per-agent:

```json
{
  "agent": {
    "documentarian": {
      "permission": {
        "team_members": "allow",
        "team_claims": "allow",
        "team_wiki_list": "allow",
        "team_wiki_read": "allow",
        "team_wiki_write": "allow",
        "team_events": "allow",
        "team_notify": "deny"
      }
    },
    "orchestrator-dev": {
      "permission": {
        "team_members": "allow",
        "team_claims": "allow",
        "team_wiki_list": "allow",
        "team_wiki_read": "allow",
        "team_wiki_write": "deny",
        "team_events": "allow",
        "team_notify": "allow"
      }
    }
  }
}
```

## Event Types

| Type | Trigger | Auto-emitted |
|------|---------|-------------|
| `session.complete` | Session ends | Yes (CLI) |
| `review.ready` | Reviewer finishes | Yes (CLI) |
| `audit.finding` | Auditor finds issues | Yes (CLI) |
| `claim.taken` | `oh claim` | Yes (CLI) |
| `claim.conflict` | Claim on taken ticket | Yes (CLI) |
| `claim.transferred` | `oh claim transfer` | Yes (CLI) |
| `claim.released` | `oh release` | Yes (CLI) |
| `wiki.proposal` | `team_wiki_write` | Yes (MCP) |
| `wiki.accepted` | `oh team wiki review` | Yes (CLI) |
| `wiki.rejected` | `oh team wiki review` | Yes (CLI) |
