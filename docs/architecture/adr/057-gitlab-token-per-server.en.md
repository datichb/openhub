> [Lire en français](057-gitlab-token-per-server.fr.md)

# ADR-057 — Strict GitLab token per server

## Status

Accepted — evolves [ADR-028](028-tracker-sync-bidirectional.en.md) (the tracker reuses the GitLab tokens of the MCP), complements [ADR-029](029-multi-team-support.en.md) (`[[teams]]`) and [ADR-030](030-enforced-recommended-config.en.md) (recommended team settings); extends the "secrets stay on the machine" principle of [ADR-046](046-beads-gateways.en.md)

## Date

2026-11-02

## Context

oh talks to GitLab in three ways: the team repository (team-state, through git), the tracker (issue sync, board, `oh mr create`) and the `gitlab` MCP server of the agents. Tokens come from several places: the `GITLAB_TOKEN` variable, the tracker key (`tracker_token_key`), the project token, the team token, the hub MCP token, and the former tracker key. A cascade picked the first one found, **without looking at the server**.

When the team repository and the tracker are on two different GitLab servers (for instance an internal GitLab and gitlab.com), the team repository token was sent to the tracker: a 401, then "token invalid or expired" without saying where the token came from, which key or which server. Beyond the 401, **it is a secret leak**: a token issued by one server is presented to another. [ADR-028](028-tracker-sync-bidirectional.en.md) had the tracker reuse the tokens already configured for the MCP, hence the cascade. `tracker_token_key` could also only be set with `oh team config`, and the "Tracker token" field of team-detail showed a default value, wrote the hub MCP key and had the wrong scope.

## Decision

**Each GitLab token source is tied to a server, and a source only answers for the server being called.** A single cascade (`tracker.ResolveGitLabToken`, called through `services/team.GitLabToken`) serves every command: sync, board, `oh mr create`, identity check, `oh team config status`, MCP.

### 1. The server of a source

| Source | Server |
|---|---|
| `GITLAB_TOKEN` | **to declare**: `OH_GITLAB_URL`, otherwise `GITLAB_URL` |
| `tracker_token_key` and the former tracker key | `tracker_token_url` if declared (next to the key, at the same level); otherwise **implicit**: the tracker (`tracker_url` of the level of the key, else the effective one) |
| project token, hub MCP token | the URL of the GitLab MCP (`[mcp.gitlab] url`, project, or the URL the team recommends); with no URL, **implicit**: `https://gitlab.com`, where the MCP server goes |
| team token | `[[teams]] gitlab_url` if declared (repository reached through an SSH alias, whose host is not a server); otherwise the host of `state_repo` |

A source whose server is neither declared nor implicit is **never** used: only a lone `GITLAB_TOKEN` stays strict. A declared server wins over the implicit one.

### 2. Comparing servers

`tracker.ServerOf` reduces a URL, a git remote or a bare host to its identity: lower-case host without user, trailing dot or path, port only when it is not the web default (80, 443), scheme ignored. The port of an `ssh://` or `git://` remote is git's, not the API's: it is dropped; `user@host:path` reads as the host. `tracker.SameServer` compares two identities; two empty or unreadable values are not the same server. Two GitLab servers that share a host and only differ by a path prefix are therefore the same server (accepted limit).

### 3. Declaring and fixing

- `tracker_token_url` is set along with `tracker_token_key` (`oh init`, `oh team rejoin`, `oh team config`, team-detail, `oh config set`), in `hub.toml` or in the shared team config. A `hub.toml` key does not inherit the server declared next to the team's.
- `[[teams]] gitlab_url` (`--gitlab-url`) for an SSH alias. The wizards no longer fill the key by themselves: empty = keep, `-` = clear.
- With no token for the server called, the command stops with the server, the sources read (key, empty, for another server, server not declared, "implicit" mention), how to declare the key and the command to run. A 401 or 403 refusal (`TokenRejectedError`) names the source, the key and the server.

## Consequences

### Positive

- A token is never sent to a server other than its own: the 401 and the leak go away together.
- A single cascade, a single place to read: `oh team config status`, the sync and `oh mr create` give the same answer.
- The messages say what to fix.

### Negative / trade-offs

- **5.1 breaking change**: `GITLAB_TOKEN` requires `OH_GITLAB_URL` (or `GITLAB_URL`). Tracker keys and MCP tokens keep working for their implicit server; declaring `tracker_token_url` or `mcp.gitlab.url` is only needed when that server is another one. A configuration that worked "by luck" with a token sent to the right server must now declare it.
- One more setting (`tracker_token_url`, `gitlab_url`), machine-specific or shared by the team.
- Implicit servers are a convenience, not a guarantee: a key set next to a `tracker_url` does not answer for another server.
- For Jira, `tracker_token_url` makes no sense: the field is hidden in the TUI.

## Alternatives considered

| Alternative | Rejected because |
|-------------|------------------|
| Keep the cascade with no server and only improve the messages | The token would still be presented to the wrong server |
| Try each token until one succeeds | Presents a secret to a server that did not issue it; noise in the server's audit logs |
| Infer the server from the token (`glpat-` prefix) | The prefix does not tell the instance |
| Require an explicit declaration everywhere | Breaks simple configurations (a single GitLab) for nothing; hence the implicit servers |
| Merge the team token and the tracker token | Two servers, possibly two accounts: the very case that motivated the decision |
