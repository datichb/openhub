> [Lire en francais](cli-sessions.fr.md)

# CLI Reference — Sessions

> **v5 — deprecated aliases.** These commands launch their workflow through [`oh run`](cli-workflows.en.md#oh-run) and print a warning: `oh start` → `oh run feature` (`--prompt` = first text input), `--agent <id>` → `oh run libre --agent <id>`, `--dev [-t <id>]` → `oh run ticket --tickets <id>` (an epic chosen in the picker: one session for the whole epic or one per ticket, as chosen), `--onboard` → `oh run onboarding`, `--parallel --tickets` → `oh run ticket --tickets`, `--sweep` → `oh run sweep`, `--worktree <branch>` → `--location new`, `--resume <id>` → `oh session attach <id> --how here`, `oh audit|review|debug` → `oh run audit|review|debug` (flags become inputs when the workflow declares them), `oh review feedback` → `oh run review-feedback` (MR feedback as the text input). They require opencode V2 and the target workflow; the former launch no longer exists (see the [v5 migration guide](../guides/migration-v5.en.md)).

## Sessions

### oh start

Deprecated alias of `oh run` (see above).

| Flag | Short | Type | Description |
|------|-------|------|-------------|
| `--agent` | `-a` | string | Entry agent (`oh run libre --agent`) |
| `--prompt` | `-m` | string | Initial prompt |
| `--provider` | `-P` | string | LLM provider (bedrock, anthropic, openai) |
| `--project` | `-p` | string | Project ID (auto-detected otherwise) |
| `--resume` | `-r` | string | Open an existing session in this terminal (`oh session attach --how here`) |
| `--worktree` | `-w` | string | Branch to launch in a git worktree |
| `--dev` | | bool | Dev mode: epic/ticket picker + orchestrator-dev |
| `--ticket` | `-t` | string | Ticket ID to work on directly (skips picker, requires --dev) |
| `--label` | `-l` | string | Filter tickets by label (requires --dev) |
| `--assignee` | `-A` | string | Filter tickets by assignee (requires --dev) |
| `--onboard` | | bool | Onboarding mode: creates/enriches project wiki |
| `--refresh` | | bool | Force wiki re-discovery (requires --onboard) |
| `--recap` | | bool | Show summary and ask for confirmation before launching |
| `--parallel` | | bool | One session per ticket (`oh run ticket --tickets`), requires `--tickets` |
| `--tickets` | | []string | List of tickets to process in parallel (comma-separated) |
| `--sweep` | | string | High-level sweep objective (activates sweep mode) |
| `--sweep-strategy` | | string | Decomposition strategy: `manual`, `by-file`, `by-package`, `llm` |
| `--sweep-tasks` | | []string | Manual task list (requires `--sweep-strategy=manual`) |
| `--sweep-include` | | []string | Glob patterns to include |
| `--sweep-exclude` | | []string | Glob patterns to exclude |
| `--sweep-verify` | | string | Post-sweep verification: `none`, `tests`, `lint`, `build`, `all`, `custom` |
| `--sweep-verify-cmd` | | string | Custom verification command (requires `--sweep-verify=custom`) |
| `--sweep-dry-run` | | bool | Display decomposed plan without executing |

```bash
oh start -p my-app -m "Fix the login bug"
oh start --resume abc123-session-id
oh start -w feature/auth -a architect
oh start --dev -l "priority:high" -A me
oh start --dev -t TICKET-123
oh start --onboard --refresh
oh start -m "Refactor the auth module" --recap
oh start --parallel --tickets bd-42,bd-43,bd-44
oh start --sweep "Migrate deprecated API calls" --sweep-strategy llm --sweep-verify tests
oh start --sweep "Fix lint warnings" --sweep-strategy by-package --sweep-dry-run
```

> **See also:** [Parallel Mode Guide](../guides/parallel-mode.en.md) | [Sweep Mode Guide](../guides/sweep-mode.en.md)

---

### oh audit

Run an automated audit on a project.

| Flag | Short | Type | Description |
|------|-------|------|-------------|
| `--project` | `-p` | string | Project ID |
| `--type` | `-t` | string | Audit type (security, performance, architecture, accessibility, ecodesign, observability, privacy). Default: security |

```bash
oh audit -p my-app
oh audit -p my-app -t performance
oh audit --type accessibility
```

---

### oh review

Launch an automated code review session with mode selection.

| Flag | Short | Type | Description |
|------|-------|------|-------------|
| `--project` | `-p` | string | Project ID |
| `--mode` | `-m` | string | Review mode (see below) |
| `--branch` | `-b` | string | Branch to review (diff vs main). Default: current branch if feature branch |
| `--publish` | | bool | Create a MR on GitLab and optionally assign a reviewer (requires write_enabled) |
| `--reviewer` | | string | Member ID of the reviewer to assign on the MR (used with --publish) |

**Available modes:**

| Mode | Description |
|------|-------------|
| `standard` | Classical 6-category checklist review |
| `adversarial` | Critical review — maximum skepticism, min. 10 findings, dangerous assumptions |
| `edge-case` | Exhaustive unhandled execution path hunting |
| `standard+adversarial` | Both modes in parallel (independent sessions) + unified report |
| `all` | Standard + Adversarial + Edge-case — maximum coverage |

Without `--mode`, an interactive prompt lets you choose the review mode at session start.

```bash
oh review -p my-app
oh review -m adversarial
oh review -m standard+adversarial -p backend
oh review -m all
oh review --publish --reviewer alice
oh review --publish -b feat/auth
```

> **See also:** [Review & Feedback Guide](../guides/review-feedback.en.md)

---

### oh review feedback

Launch a feedback correction session from MR review discussions. Fetches unresolved GitLab MR discussions and opens an AI session to address each comment.

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

Start a debugging session with AI assistance.

| Flag | Short | Type | Description |
|------|-------|------|-------------|
| `--project` | `-p` | string | Project ID |
| `--issue` | `-i` | string | Issue description |

```bash
oh debug -p my-app -i "Users get 500 on /api/auth/callback"
oh debug --issue "Memory leak in worker process"
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

Proxy to `bd` (Beads CLI). All arguments are passed through directly. Requires `bd` installed.

```bash
oh beads list
oh beads run my-bead
oh beads --help
```

---

---

> **See also:** [Parallel Mode Guide](../guides/parallel-mode.en.md) | [Sweep Mode Guide](../guides/sweep-mode.en.md) | [Review & Feedback Guide](../guides/review-feedback.en.md)
