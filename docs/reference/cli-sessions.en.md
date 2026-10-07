> [Lire en francais](cli-sessions.fr.md)

# CLI Reference — Sessions

This page covers **v5 session management** (`oh session …`), **restrictions** (`oh budget`), **history** (`oh history`), the `oh beads` proxy and the **deprecated launch aliases** (`oh start`, `oh audit`, `oh review`, `oh debug`). Launching itself is done with [`oh run`](cli-workflows.en.md#oh-run). Guide: [Sessions v5](../guides/sessions-v5.en.md); decisions: [ADR-042](../architecture/adr/042-checkpoints-headless-decisions.en.md), [ADR-047](../architecture/adr/047-session-interaction-daemon.en.md).

## Session management (v5)

"oh = control tower, opencode = cockpit": a session runs on an `opencode serve` server (one per **group**: bundle version, project, runtime); closing the opencode interface does not stop the session. A session idle for 5 min with no pending decision goes **to sleep** (server stopped); it resumes with the same bundle (`attach`, `open` or `resume`).

- **Referring to a session**: its full id or a **unique prefix** (with or without `ses_`); `oh session list` shows them.
- **Referring to a decision** (`approve`, `answer`, `dismiss`, `oh budget raise`): the decision id (`oh session inbox`) or the session id; when the session has several pending decisions of the expected kind, oh refuses and lists them.
- **Decision kinds**: ⏸ checkpoint, ? question, ! permission, $ budget, ✗ error or circuit breaker. The **first answer wins** (oh, opencode interface, browser, CLI); a late answer tells who already decided.
- Live follow-up, notifications and decisions go through the `ohd` daemon ([`oh daemon`](cli-infra.en.md#oh-daemon)).

| Command | Purpose |
|---------|---------|
| [`oh session list`](#oh-session-list) | Running, waiting, sleeping sessions (`--all`: finished ones too) |
| [`oh session inbox`](#oh-session-inbox) | Pending decisions of every session |
| [`oh session attach`](#oh-session-attach) | Open the interface of a session (resumes a sleeping session) |
| [`oh session follow`](#oh-session-follow) | Follow a session live, read-only |
| [`oh session approve`](#oh-session-approve) | Answer a permission or a checkpoint |
| [`oh session answer`](#oh-session-answer) | Answer a question of the agent |
| [`oh session dismiss`](#oh-session-dismiss) | Dismiss an alert (error, circuit breaker, budget) |
| [`oh session send`](#oh-session-send) | Send a short instruction |
| [`oh session interrupt`](#oh-session-interrupt) | Interrupt the current step |
| [`oh session compact`](#oh-session-compact) | Compact the history |
| [`oh session model`](#oh-session-model) | Change the model of the next steps |
| [`oh session fork`](#oh-session-fork) | Create a variant (copy of the history) |
| [`oh session results`](#oh-session-results) | Changed files, branch, cost, MR description, diff |
| [`oh session resume`](#oh-session-resume) | Resume a sleeping session without interface |
| [`oh session stop`](#oh-session-stop) | Stop a session |
| [`oh session open`](#oh-session-open) | Open a session in the browser |
| [`oh session fetch`](#oh-session-fetch) | Fetch a finished remote session |
| [`oh session resolve`](#oh-session-resolve) | Replay the Beads journal of a remote session |

### oh session list

```
oh session list [--all] [--json]
```

Lists the v5 sessions: id, project, entry agent, state, pending decisions (badges), cost, start.

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--all` | bool | `false` | Include finished sessions |
| `--json` | bool | `false` | JSON output (sessions and their pending decisions) |

### oh session inbox

```
oh session inbox [--json]
```

Pending decisions of every session: kind (⏸ ? ! $ ✗), decision id, agent, summary, age.

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--json` | bool | `false` | JSON output |

### oh session attach

```
oh session attach <session> [--how <opening>] [--iterm-style tab|split|window]
```

Opens the opencode interface on the session (new tab or window, tmux, browser or current terminal). A sleeping session is resumed first (its server restarts). When no terminal can be opened, the interface opens in the current terminal.

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--how` | string | `auto` | `auto` (current terminal first: iTerm2 or Terminal.app, then the other one, then tmux when oh runs inside tmux), `iterm`, `terminal`, `tmux`, `browser`, `suspend` (in the current terminal; `here` is accepted as a synonym) |
| `--iterm-style` | string | `tab` | With iTerm2: `tab`, `split` or `window` |
| `--exec` | bool | `false` | Internal, hidden from the help: runs the interface in the current process (command of the windows opened by oh) |

```bash
oh session attach 7f3a
oh session attach 7f3a --how here      # replaces oh start --resume <id>
oh session attach 7f3a --how tmux
```

### oh session follow

```
oh session follow <session>
```

Follows a session live, read-only: current agent, tools, messages; the cost is printed at the end. `Ctrl+C` to quit (the session goes on). Requires the oh daemon (otherwise: "live follow-up unavailable").

### oh session approve

```
oh session approve <session|decision> [--decision <choice>] [-m "<message>"]
```

Answers a pending **permission** (!) or **checkpoint** (⏸), without opening the session.

| Flag | Short | Type | Default | Description |
|------|-------|------|---------|-------------|
| `--decision` | | string | `once` | Permission: `once` (allow this time), `always` (refused when the session is in strict isolation), `reject`. Checkpoint: `once` (validate), `fix` (fix first), `other` (other instruction), `reject` |
| `--message` | `-m` | string | | Message passed to the agent; **required** with `fix` and `other` |

```bash
oh session approve 7f3a                                  # permission or checkpoint: once
oh session approve dec_91 --decision reject
oh session approve 7f3a --decision fix -m "Add the tests first"
```

### oh session answer

```
oh session answer <session|decision> --field key=value…
```

Answers a **question** (?) of the agent (form). Without `--field`, oh prints the expected fields (type, required, possible choices) and exits with an error.

| Flag | Type | Description |
|------|------|-------------|
| `--field` | string (repeatable) | Answer `key=value`; for a list: `key=a,b` |

```bash
oh session answer 7f3a --field scope=api --field targets=auth,users
```

### oh session dismiss

```
oh session dismiss <session|decision>
```

Dismisses an alert: ✗ error, ✗ circuit breaker (resets the delegation counter), $ budget (one more step is allowed, then the decision comes back; to raise the budget: [`oh budget raise`](#oh-budget)).

### oh session send

```
oh session send <session> "instruction…" [--queue] [--synthetic]
```

Sends a short instruction, taken into account at the next step. The words after the id are joined.

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--queue` | bool | `false` | After the current step (instead of the next step) |
| `--synthetic` | bool | `false` | oh message (not from the user) |

### oh session interrupt

```
oh session interrupt <session>
```

Interrupts the current step; the session stays open.

### oh session compact

```
oh session compact <session>
```

Compacts the session history.

### oh session model

```
oh session model <session> <provider/model>
```

Changes the model of the next steps (e.g. `amazon-bedrock/eu.anthropic.claude-sonnet-4-5`). Model restrictions (`limits.models`) apply.

### oh session fork

```
oh session fork <session>
```

Creates a variant of the session: copy of its history, on the same server. Prints the id of the new session.

### oh session results

```
oh session results <session> [--mr | --patch | --json]
```

Results: recap, branch, changed files (`+additions −deletions`), cost. When the server no longer runs, oh shows the last snapshot.

| Flag | Type | Description |
|------|------|-------------|
| `--mr` | bool | MR description (Markdown) |
| `--patch` | bool | Full diff |
| `--json` | bool | JSON output (`results`, `live`) |

### oh session resume

```
oh session resume <session>
```

Resumes a sleeping session (restarts its server, same bundle) without opening an interface. No effect when it already runs.

### oh session stop

```
oh session stop <session>
```

Stops the session, and its server when no other session of the group uses it.

### oh session open

```
oh session open <session> [--browser] [--print]
```

Opens the session in the browser with a one-time pairing code (valid 5 min); the URL is printed too. A sleeping session is resumed first.

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--browser` | bool | `true` | Open in the browser (only available mode) |
| `--print` | bool | `false` | Print the URL without opening the browser |

### oh session fetch

```
oh session fetch <session> [--no-import]
```

Finished remote session ([remote execution](../guides/remote-runners.en.md)): downloads the artifacts of the `oh-runner` pipeline (Beads journal, summary, session export) and imports the session into a local server (worktree of the branch pushed by the job), to resume it with `oh session attach`. The Beads journal is not replayed here: see `oh session resolve`.

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--no-import` | bool | `false` | Download the artifacts without importing the session |

### oh session resolve

```
oh session resolve <session> [--dry-run] [--yes] [--all keep-local|apply-remote|merge-notes]
                             [--keep-local a,b] [--apply-remote a,b] [--merge-notes a,b]
```

Replays on the machine the Beads journal of a fetched remote session: each write is checked again (workflow `beads.allow`, refused options, tickets of the session) then applied after confirmation. A ticket changed on the machine since sending is a **conflict**: keep the local version, apply the remote one, or merge the notes only. Can be run again: only the remaining writes are applied.

| Flag | Short | Type | Description |
|------|-------|------|-------------|
| `--dry-run` | | bool | Show the replay without applying anything |
| `--yes` | `-y` | bool | Apply without confirmation |
| `--all` | | string | Resolution of every conflict: `keep-local`, `apply-remote` or `merge-notes` |
| `--keep-local` | | list | Conflicting tickets whose local version is kept |
| `--apply-remote` | | list | Conflicting tickets whose remote writes are applied |
| `--merge-notes` | | list | Conflicting tickets whose notes only are merged |

---

## Restrictions

### oh budget

Session restrictions, **off by default**: maximum active sessions, per-session and daily budget (USD), memory cap, allowed models. Cascade hub → team (recommended or enforced) → project → workflow (`limits:`). See [Sessions v5 › Restrictions](../guides/sessions-v5.en.md#restrictions) and [ADR-044](../architecture/adr/044-credential-proxy-session-limits.en.md).

| Subcommand | Usage | Description |
|------------|-------|-------------|
| `show` | `oh budget show [-p <project>] [--json]` | Effective restrictions with their origin, and today's spending. `-p`: project (default: the one of the current folder) |
| `set` | `oh budget set <restriction> <value> [-p <project>]` | Sets a restriction of the hub (`hub.toml [limits]`) or, with `-p`, of a project |
| `unset` | `oh budget unset <restriction> [-p <project>]` | Removes a restriction of the hub or of a project |
| `raise` | `oh budget raise <session\|decision> [amount]` | Answers a $ decision: adds the given amount in USD to the reached budget, of the session or of the day (default: the configured budget once more); the session goes on as soon as the budget covers its spending |

| Restriction | Value | Note |
|-------------|-------|------|
| `max_active_sessions` | integer | Sessions active at the same time |
| `session_budget_usd` | USD amount | Per-session budget |
| `daily_budget_usd` | USD amount | Daily budget |
| `memory_mb` | integer (MB) | Memory cap; **hub only** (refused with `-p`) |
| `models` | comma-separated list | Allowed models |

`0`, `off` or an empty value turn a restriction off.

```bash
oh budget show --json
oh budget set session_budget_usd 5
oh budget set max_active_sessions 2 -p my-app
oh budget set models amazon-bedrock/eu.anthropic.claude-sonnet-4-5,amazon-bedrock/eu.anthropic.claude-haiku-4-5
oh budget unset daily_budget_usd
oh budget raise 7f3a 3
```

---

## History

### oh history

History of the sessions recorded by oh (`--team`: finished sessions of your member, read from the team-state).

```
oh history [--limit 20] [--team]
oh history export [--output <file>]
oh history import <file> [--member-id <id>]
```

| Command / flag | Type | Default | Description |
|----------------|------|---------|-------------|
| `--limit` | int | `20` | Maximum number of entries |
| `--team` | bool | `false` | Team history (team-state) |
| `export --output` | string | `oh-history-<date>.json` | Export file (JSON) |
| `import --member-id` | string | | Replaces the `member_id` of the imported sessions |

---

## Beads

### oh beads

Proxy to `bd` (Beads CLI): every argument is passed as is to `bd`. For `bd init`, oh adds the "zero-impact" options (no hooks, no agent files, `.gitignore` unchanged).

```
oh beads [arguments...]
```

Requires `bd` installed and on the PATH. When `bd` fails, oh exits with code 1.

```bash
oh beads list
oh beads show bd-42
oh beads ready
```

---

## Deprecated aliases

> **v5 — deprecated aliases.** These commands launch their workflow through [`oh run`](cli-workflows.en.md#oh-run) and print a warning: `oh start` → `oh run feature` (`--prompt` = first text input), `--agent <id>` → `oh run libre --agent <id>`, `--dev [-t <id>]` → `oh run ticket --tickets <id>` (an epic chosen in the picker: one session for the whole epic or one per ticket, as chosen), `--onboard` → `oh run onboarding`, `--parallel --tickets a,b` → `oh run ticket --tickets a,b` (without `--tickets`: refused), `--sweep <goal>` → `oh run sweep -i goal=<goal>`, `--worktree <branch>` → `--location new`, `--resume <id>` → `oh session attach <id> --how here`, `oh audit|review|debug` → `oh run audit|review|debug` (flags become inputs when the workflow declares them), `oh review feedback` → `oh run review-feedback -i mr=<url>` (preview of the discussions, then the workflow reads them itself). They require opencode V2 and the target workflow; the former launch no longer exists (see the [v5 migration guide](../guides/migration-v5.en.md)). Sessions are then followed with `oh session …` or the TUI **Sessions** view (see [Sessions v5](../guides/sessions-v5.en.md)).

### oh start

Deprecated alias of `oh run` (see above).

```
oh start [options]
```

| Flag | Short | Description | v5 equivalent |
|------|-------|-------------|---------------|
| `--agent` | `-a` | Entry agent | `oh run libre --agent <id>` (the prompt becomes the `request` input). Refused with `--dev`, `--onboard`, `--parallel` or `--sweep` |
| `--assignee` | `-A` | Filter the picker tickets by assignee (requires `--dev`, exclusive with `--label`) | No flag: pass the tickets with `oh run ticket --tickets` |
| `--dev` | | Epic/ticket picker, then `ticket` | `oh run ticket --tickets <id>` (epic in one session: `--one-session`) |
| `--label` | `-l` | Filter the picker tickets by label (requires `--dev`, exclusive with `--assignee`) | No flag: pass the tickets with `oh run ticket --tickets` |
| `--onboard` | | Create or enrich the project wiki | `oh run onboarding` |
| `--parallel` | | One session per ticket, requires `--tickets` (refused otherwise) | `oh run ticket --tickets a,b` |
| `--project` | `-p` | Project ID (auto-detected otherwise) | `-p` |
| `--prompt` | `-m` | Initial prompt | First text input (`-i request=…` for `feature`) |
| `--provider` | `-P` | LLM provider (bedrock, anthropic, openrouter, github-copilot) | `-P` |
| `--recap` | | Show summary and ask for confirmation | `--recap` |
| `--refresh` | | Re-discover the wiki (requires `--onboard`) | `oh run onboarding -i refresh=true` |
| `--resume` | `-r` | Open an existing session in this terminal | `oh session attach <id> --how here` |
| `--sweep` | | High-level sweep goal | `oh run sweep -i goal=<goal>` |
| `--sweep-dry-run` | | Show the decomposition without executing | `-i dry_run=true` |
| `--sweep-exclude` | | Glob patterns to exclude | `-i exclude=<patterns>` |
| `--sweep-include` | | Glob patterns to include | `-i include=<patterns>` |
| `--sweep-strategy` | | Decomposition: `manual`, `by-file`, `by-package`, `llm` | `-i strategy=<strategy>` (workflow default: `llm`) |
| `--sweep-tasks` | | Manual task list (`--sweep-strategy=manual`) | `-i tasks=<tasks>` (one per line) |
| `--sweep-verify` | | Final verification: `none`, `tests`, `lint`, `build`, `all`, `custom` | `-i verify=<value>` |
| `--sweep-verify-cmd` | | Verification command (`--sweep-verify=custom`) | `-i verify_cmd=<command>` |
| `--ticket` | `-t` | Ticket to work on directly (skips the picker, requires `--dev`) | `oh run ticket --tickets <id>` |
| `--tickets` | | Comma-separated tickets (with `--parallel`) | `--tickets` |
| `--worktree` | `-w` | Branch to launch in a git worktree | `--location new` |

`--max-sessions`, `--priority` and `--sweep-branch-prefix` are still accepted but have no effect (hidden from the help), like `-y, --yes` (direct launch is the default; `--recap` to confirm). The former sweep (one worktree per subtask) and the former parallel mode (monitor, merge view) no longer exist.

**Example:**

```bash
oh run feature -p my-app -i request="Fix the login bug"
oh run libre --agent debugger -i request="Test X has been failing since yesterday"
oh session attach abc123-session-id --how here
oh run ticket --tickets TICKET-123 --location new
oh run onboarding -i refresh=true
oh run feature -i request="Refactor the auth module" --recap
oh run ticket --tickets bd-42,bd-43,bd-44
oh run sweep -i goal="Migrate deprecated API calls" -i strategy=llm -i verify=tests
oh run sweep -i goal="Fix lint warnings" -i strategy=by-package -i dry_run=true

# Equivalent deprecated aliases
oh start -p my-app -m "Fix the login bug"
oh start -a debugger -m "Test X has been failing since yesterday"
oh start --resume abc123-session-id
oh start --dev -t TICKET-123 -w feat/ticket-123
oh start --onboard --refresh
oh start -m "Refactor the auth module" --recap
oh start --parallel --tickets bd-42,bd-43,bd-44
oh start --sweep "Migrate deprecated API calls" --sweep-strategy llm --sweep-verify tests
oh start --sweep "Fix lint warnings" --sweep-strategy by-package --sweep-dry-run
```

> **See also:** [Shipped workflows](workflows.en.md) | [Sessions v5](../guides/sessions-v5.en.md) | [Parallel mode (replaced)](../guides/parallel-mode.en.md) | [Sweep mode (replaced)](../guides/sweep-mode.en.md)

---

### oh audit

Deprecated alias of `oh run audit` (read-only code audit).

```
oh audit [options]
```

| Flag | Short | Description | v5 equivalent |
|------|-------|-------------|---------------|
| `--project` | `-p` | Project ID | `-p` |
| `--type` | `-t` | Audit type (default: security) | `-i type=<type>` |

Available types: `security`, `performance`, `architecture`, `accessibility`, `ecodesign`, `observability`, `privacy`.

**Example:**

```bash
oh run audit -p my-app -i type=security
oh run audit -p my-app -i type=performance
oh run audit -i type=accessibility

# Equivalent deprecated alias
oh audit -p my-app -t performance
```

---

### oh review

Deprecated alias of `oh run review` (read-only code review). `--publish` is not a workflow: it remains an oh command.

```
oh review [options]
```

| Flag | Short | Description | v5 equivalent |
|------|-------|-------------|---------------|
| `--branch` | `-b` | Branch to review (diff vs main). Default: current branch if feature branch | `-i branch=<branch>` |
| `--mode` | `-m` | Review mode (see below) | `-i review_mode=<mode>` |
| `--project` | `-p` | Project ID | `-p` |
| `--publish` | | Create a MR on GitLab and optionally assign a reviewer (requires write_enabled) | Unchanged: `oh review --publish` |
| `--reviewer` | | Member ID of the reviewer to assign on the MR (with `--publish`) | Unchanged |

**Available modes:**

| Mode | Description |
|------|-------------|
| `standard` | Classical 6-category checklist review |
| `adversarial` | Critical review — maximum skepticism, min. 10 findings, dangerous assumptions |
| `edge-case` | Exhaustive unhandled execution path hunting |
| `standard+adversarial` | Both modes in parallel (independent sessions) + unified report |
| `all` | Standard + Adversarial + Edge-case — maximum coverage |

Without a mode, the reviewer offers the choice at session start.

**Example:**

```bash
oh run review -p my-app
oh run review -i review_mode=adversarial
oh run review -i review_mode=standard+adversarial -p backend
oh run review -i review_mode=all -i branch=feat/auth
oh review --publish --reviewer alice
oh review --publish -b feat/auth

# Equivalent deprecated alias
oh review -m adversarial
```

> **See also:** [Review & Feedback Guide](../guides/review-feedback.en.md)

---

### oh review feedback

Fetches the unresolved GitLab discussions of a MR, shows a preview, asks for confirmation, then launches the `review-feedback` workflow (`mr`, `branch` and `feedback` inputs filled by oh). Warns that the alias is deprecated.

```
oh review feedback <ticket-or-branch>
```

| Flag | Short | Type | Description |
|------|-------|------|-------------|
| `--project` | `-p` | string | Project ID |
| `--yes` | | bool | Skip confirmation prompt |

```bash
oh review feedback TICKET-123
oh review feedback feat/my-branch -p backend
oh review feedback TICKET-123 --yes
```

> **Limits:** Max 30 discussions per session, 2000 chars per note body.

---

### oh debug

Deprecated alias of `oh run debug` (bug diagnosis).

```
oh debug [options]
```

| Flag | Short | Description | v5 equivalent |
|------|-------|-------------|---------------|
| `--issue` | `-i` | Issue description | `-i issue=<description>` |
| `--project` | `-p` | Project ID | `-p` |

**Example:**

```bash
oh run debug -p my-app -i issue="Users get 500 on /api/auth/callback"
oh run debug -i issue="Memory leak in worker process"

# Equivalent deprecated alias
oh debug -p my-app -i "Users get 500 on /api/auth/callback"
```

---

> **See also:** [Shipped workflows](workflows.en.md) | [Sessions v5](../guides/sessions-v5.en.md) | [Review & Feedback guide](../guides/review-feedback.en.md)
