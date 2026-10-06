> [Lire en francais](cli-sessions.fr.md)

# CLI Reference — Sessions

> **v5 — deprecated aliases.** These commands launch their workflow through [`oh run`](cli-workflows.en.md#oh-run) and print a warning: `oh start` → `oh run feature` (`--prompt` = first text input), `--agent <id>` → `oh run libre --agent <id>`, `--dev [-t <id>]` → `oh run ticket --tickets <id>` (an epic chosen in the picker: one session for the whole epic or one per ticket, as chosen), `--onboard` → `oh run onboarding`, `--parallel --tickets a,b` → `oh run ticket --tickets a,b` (without `--tickets`: refused), `--sweep <goal>` → `oh run sweep -i goal=<goal>`, `--worktree <branch>` → `--location new`, `--resume <id>` → `oh session attach <id> --how here`, `oh audit|review|debug` → `oh run audit|review|debug` (flags become inputs when the workflow declares them), `oh review feedback` → `oh run review-feedback` (MR feedback as the text input). They require opencode V2 and the target workflow; the former launch no longer exists (see the [v5 migration guide](../guides/migration-v5.en.md)). Sessions are then followed with `oh session …` or the TUI **Sessions** view (see [Sessions v5](../guides/sessions-v5.en.md)).

## Sessions

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
| `--provider` | `-P` | LLM provider (bedrock, anthropic, openai) | `-P` |
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

`--max-sessions`, `--priority` and `--sweep-branch-prefix` are still accepted but have no effect (hidden from the help). The former sweep (one worktree per subtask) and the former parallel mode (monitor, merge view) no longer exist.

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

### oh budget

Session restrictions (off by default): max working sessions, budget per session and per day, memory cap, allowed models. See [Sessions v5 › Restrictions](../guides/sessions-v5.en.md#restrictions).

```bash
oh budget show [-p <project>] [--json]          # effective values, origin, spent today
oh budget set session_budget_usd 5              # hub.toml [limits]
oh budget set max_active_sessions 2 -p my-app   # project level
oh budget unset daily_budget_usd
oh budget raise <session> [amount]              # answer a $ decision
```

---

### oh beads

Proxy to `bd` (Beads CLI). All arguments are passed through directly to `bd`.

```
oh beads [arguments...]
```

Requires `bd` installed and available in the PATH.

**Example:**

```bash
oh beads list
oh beads run my-bead
oh beads status
```

---

> **See also:** [Shipped workflows](workflows.en.md) | [Sessions v5](../guides/sessions-v5.en.md) | [Review & Feedback Guide](../guides/review-feedback.en.md)
