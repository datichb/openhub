> [Lire en francais](cli-tools.fr.md)

# CLI Reference — Tools

## Skills

`oh skill` checks the skill catalogue. The skills of a session are those of the hub and of the team catalogue (`catalog/skills/` of the team-state), delivered by its session bundle (see [`oh bundle show`](cli-workflows.en.md#oh-bundle-build--show), which also gives the budget with `--budget`).

> **Disconnected in v5** ([ADR-051](../architecture/adr/051-community-skills-disconnection.en.md)): the community skills registry (`oh skill add`, `list`, `remove`, `search`) and `oh skill budget` are removed, and `~/.oh/skills` is no longer read. `oh doctor` reports the packages left there.

### oh skill check

Checks the hub skills (`skills/**/*.md`, the `skills/templates/` annexes excepted) and the skills named by the hub agents. Exits with code 1 on errors.

With an active team (solo space included), it also applies the `unknown_key` rule to the skills of the team catalogue (`catalog/skills/**/*.md` of the team-state, `templates/` excepted): their problems carry the `team:` prefix (`team:<path>`), and the `extends` key (replacement of a hub skill, `extends: hub:<ref>`) is allowed there. An unreadable frontmatter there is an `invalid_frontmatter`. The description rules apply there as warnings: `description_not_trigger`, and `long_description` past 200 characters (an error past 1024, the limit of the standard).

| Kind | Level | Problem |
|------|-------|---------|
| `duplicate_id` | error | Two skills with the same identifier (last path component) |
| `missing_require` | error | `requires:` names a skill not found |
| `require_cycle` | error | `requires:` cycle |
| `invalid_frontmatter` | error | Frontmatter that is not valid YAML |
| `name_mismatch` | error | `name:` different from the file name |
| `invalid_name` | error | `name:` over 64 characters or not kebab-case (lowercase letters, digits, single hyphens) |
| `long_description` | error | `description:` over 200 characters (hub catalogue limit; a warning for a skill of the team catalogue, an error over 1024, the Agent Skills standard limit) |
| `unknown_key` | error | Frontmatter key that is neither an Agent Skills key (`name`, `description`, `license`, `compatibility`, `metadata`, `allowed-tools`) nor a hub key (`requires`, `annexes`, `plugin`, `mcp`, `when`, `statuses`, `versions`, `reviewed`) |
| `invalid_when` | error | Invalid `when:` condition or `<!-- oh:when … -->` markers: unknown key or role, marker not alone on its line, unbalanced blocks |
| `missing_annex` | error | `annexes:` names a file not found |
| `missing_freshness` | error | Stack skill (`developer/stacks/`) without `versions:` or without `reviewed:` (see [Stack skill freshness](../guides/skill-refresh.en.md)) |
| `invalid_freshness` | error | Empty `versions:`, key not in lowercase (`react`, `spring-boot`) or empty version; `reviewed:` that is not a `YYYY-MM-DD` date or is in the future |
| `no_description` | warning | Missing `description:` |
| `legacy_bucket` | warning | Obsolete `bucket:` field |
| `missing_agent_skill` | warning | Skill named in an agent's `skills:` or `native_skills:` but not found |
| `undeclared_annex` | warning | Template referenced in the text but missing from `annexes:` |
| `orphan_annex` | warning | File of `skills/templates/` used by no skill |
| `description_not_trigger` | warning | `description:` that does not say when to load the skill: it must open with « Utiliser … » (« Utiliser quand… », « Utiliser pour… »; « Use … » in English); skills generated from the workflow are left out |

Only the Agent Skills keys reach the delivered `SKILL.md`; the hub keys are read by oh (`plugin:` and `mcp:` ship the skill only when the workflow loads that plugin or the session has that MCP server). See [Skill format](../architecture/skills.en.md#skill-format).

```
oh skill check [--json] [--hub <dir>]
oh skill check --stale [--max-age 90d] [--strict] [--json] [--hub <dir>]
```

| Flag | Type | Description |
|------|------|-------------|
| `--json` | bool | Problems as JSON (`kind`, `error`, `ref`, `detail`); with `--stale`, the freshness report |
| `--hub` | path | Hub directory to check, for instance a checkout of the repository (`--hub .`); default: the installed hub (`~/.oh/hub`) |
| `--stale` | bool | Lists the stack skills to review instead of checking the catalogue |
| `--max-age` | duration | With `--stale`: maximum age of a review, in days (`90`, `90d`) or weeks (`12w`); default `90d` |
| `--strict` | bool | With `--stale`: exit code 1 when a skill is to review (scheduled task, CI) |

#### oh skill check --stale

Lists the stack skills (`developer/stacks/`, and any other skill that carries `versions:` or `reviewed:`) whose last review (`reviewed:`) is older than `--max-age` days, and those whose metadata is missing or unreadable: the latter first, then the oldest review first, with the versions covered. Without `--strict`, the exit code stays 0. No network call: the command compares dates, not the published versions.

```
$ oh skill check --stale --hub .
Stack skills to review (41 checked, review every 90 days at most, today 2026-10-13):

  ! developer/stacks/dev-standards-cypress: reviewed on 2026-04-30 (166 days ago), covers cypress 12+
  ! developer/stacks/dev-standards-kubernetes: reviewed on 2026-04-30 (166 days ago), covers kubernetes 1.21+

  2 skill(s) to review: oh run skill-refresh (docs/guides/skill-refresh.en.md)
```

With `--json`: `today`, `max_age_days`, `checked` (skills checked) and `skills` (the skills to review: `ref`, `status` `stale` or `missing`, `reviewed`, `age_days`, `versions`, `detail`). The 90-day default is one review per quarter; the update is done with the [`skill-refresh`](../guides/skill-refresh.en.md) workflow.

---

## Evaluations

### oh eval

Measures the behaviour of the shipped workflows with a real model, before and after a content change. Each case of `evals/<workflow>/<case>/` describes a session (workflow, mode, inputs, Beads tickets), a fixture (the starting project), optional hidden tests and the assertions checked on the session trace ([`oh session trace`](cli-sessions.en.md#oh-session-trace)). The case format is described in [`evals/README.en.md`](../../evals/README.en.md).

```
oh eval [--case S1,S2] [--reps N] [--models m1,m2] [--budget USD] [--parallel N] [--budget-total USD] [--out <dir>] [--baseline <file>] [--record-baseline <file>] [--variant <name>[=<definition>]]… [--rescore <dir>] [--list] [--strict]
```

Each pass runs isolated: a temporary `OH_HOME` and `HOME` (your sessions, database, keychain and daemon are never touched), a throwaway clone of the fixture, a dedicated oh daemon. The session runs without interface; its checkpoints, permissions and questions get the answers written in the case, through the oh decision API (`answers.wiki`: `accept`, `refuse` or `pending` first decides the wiki proposals on the card of a commit checkpoint). Once the session is done (idle with no decision waiting), oh reads its trace, copies the hidden tests over the clone and checks the assertions.

| Assertion | Checks |
|-----------|--------|
| `ran` | A shell command ran (before an event with `before`, or never with `absent`); refused calls do not count |
| `checkpoint_order` | The checkpoints are reached in this order |
| `no_question_in_subagent` | No subagent (or those of `agents`) calls the `question` tool; `include_entry` also checks the entry agent |
| `skill_loaded_before` | A skill is loaded (before an event with `before`); a failed load (skill missing from the bundle) does not count |
| `changed_within` | The clone changed only within `paths` (`max_files`, `min_files`; each pattern of `include` matches at least one changed file; nothing at all without parameter) |
| `called` | A tool was called (pattern of `tool`, for instance `*wiki/propose`; before an event with `before`, or never with `absent`, restricted to `agent`); refused calls do not count |
| `hidden_tests` | The hidden tests pass on the clone |
| `no_denied_loop` | A refused call (rule, rejected permission, unknown skill) is not repeated more than `max_repeats` times |
| `text_matches` | An agent wrote (or never, with `absent`) a text; `questions` also looks at the questions asked |

The model is Haiku by default; `OH_EVAL_MODELS` (comma separated) adds others, for example `claude-sonnet-4-6`. Each pass is capped by the session budget the credential proxy enforces (`--budget` or `OH_EVAL_BUDGET_USD`; $1 by default, $2.50 as soon as a Sonnet model is among the models): beyond it, the session is stopped and the pass counts as `budget`. `--budget-total` (or `OH_EVAL_BUDGET_TOTAL`) caps the whole evaluation: once that much is spent, no new pass starts, the running ones finish, and the report lists the passes not started. The Bedrock key comes from `OH_EVAL_BEDROCK_TOKEN`, else from `opencode auth`.

`--parallel N` (or `OH_EVAL_PARALLEL`) runs up to N passes at the same time (1 by default). Each pass is a separate `oh` process, with its own `OH_HOME`, `HOME` and `TMPDIR`, its clone, its daemon (socket in its `OH_HOME`), its credential proxy and its tool server on ports picked by the system; the passes share nothing but the Go caches. The reports are the same as in a sequential run: passes sorted by case, model and repetition. On the console, `→` marks a pass started and `[k/N]` each pass done.

A pass stopped by a provider rate limit (Bedrock throttling: `ThrottlingException`, "Too many requests/tokens", an HTTP 429 from the provider, not from the proxy cap) runs again on a fresh clone, up to three times, after 30 s, 1 min then 2 min. The files of the attempts given up stay in `passes/<case>/<model>/<n>/attempt-<k>/`; the report lists them (« Relances ») and counts their cost in the total cost. Any other failure does not run again.

| Flag | Type | Description |
|------|------|-------------|
| `--cases` | string | Cases directory (default: `evals/` or `../evals`) |
| `--case` | strings | Cases to run (default: every active case) |
| `--models` | strings | Models (default: `OH_EVAL_MODELS`, else Haiku) |
| `--reps` | int | Repetitions per case and model (default: `OH_EVAL_REPS`, else 1) |
| `--budget` | float | Cost cap of a pass, in dollars |
| `--parallel` | int | Number of passes run at the same time (default: `OH_EVAL_PARALLEL`, else 1) |
| `--budget-total` | float | Cost cap of the whole evaluation, in dollars (default: `OH_EVAL_BUDGET_TOTAL`, else none) |
| `--out` | string | Directory of the traces (`passes/<case>/<model>/<n>/trace.jsonl`) and of the `report.json` and `report.md` reports |
| `--baseline` | string | Reference measure to compare with (« Référence » column of the report) |
| `--record-baseline` | string | Record this run as the reference measure |
| `--note` | string | Note recorded with the report |
| `--keep` | bool | Keep the work directories of the passes |
| `--list` | bool | List the cases without running anything |
| `--strict` | bool | Exit with code 1 when a pass fails |
| `--rescore` | string | Check the assertions again on the traces of a previous run (its `--out` directory), without calling a model; `changed_within` and `hidden_tests` keep their result; the variants and their comparison are computed again |
| `--variant` | string, repeatable | Hub variant: `name` (embedded hub), `name=-skill:<id>` (without this skill), `name=<hub directory>[,-skill:<id>…]`; the first one is the reference of the verdict. Not with `--record-baseline` |

**Hub variants** (`--variant`, two or more): every case runs with every variant, to measure the effect of a content change, a skill for instance. `-skill:<id>` (name or category/name) removes the file of the skill and its mentions in `skills` and `native_skills` of the agents' frontmatter; the body of the agents and skills is not rewritten. A `<hub directory>` holds `agents/`, `skills/`, `workflows/` (and `permissions/`), for example the repository root (`--variant repo=..`, without `make sync-content`). Before any pass, the hub of each variant is checked: skill catalogue (errors of `oh skill check`), validation of the workflows of the cases and composition of their bundle; a variant that breaks the hub stops the evaluation with the list of errors. Each pass installs the hub of its variant in its temporary `OH_HOME`. The repetitions alternate the variants (case, model, repetition, variant), `--parallel` and `--budget-total` cover the whole run, and the traces go to `passes/<case>/<variant>/<model>/<n>/`. The report adds a « Variante » column and a « Comparaison avec la référence » section: per case, pass rate, mean cost and duration of each variant, gap with the reference and verdict (`gain`, `no_gain`, `ambiguous`, `incomplete` in `report.json`); the same comparison without the `skill_loaded_before` assertions (« Résultat hors chargement des skills », `outcome` in `report.json`), then a verdict per variant over every case and the pass rate of each assertion in both variants. The rule is described in [`evals/README.en.md`](../../evals/README.en.md#comparing-with-and-without-a-skill).

From `cli/`: `make test-eval` (one repetition), `make test-eval OH_EVAL_REPS=3`, `make test-eval OH_EVAL_REPS=3 OH_EVAL_PARALLEL=4 OH_EVAL_BUDGET_TOTAL=5` (four passes at a time, $5 at most), `make test-eval OH_EVAL_REPS=3 OH_EVAL_CASES=S6 OH_EVAL_VARIANTS="base without-23=-skill:debugger-phase-2-3"` (with and without a skill), or `make test-eval OH_EVAL_REPS=3 OH_EVAL_RECORD=1` to record the reference measure again (`cli/internal/evalrun/testdata/evals/baseline.json`). The tests of `go test ./...` never call a model.

---

## Git Worktree

`oh worktree` manages the git worktrees of the project. **Alias:** `oh wt`. `oh run --location new` also creates one worktree per session (see [`oh run`](cli-workflows.en.md#oh-run)).

### oh worktree list

Lists the worktrees of the project.

**Alias:** `oh worktree ls`

```
oh worktree list [options]
```

| Flag | Type | Description |
|------|------|-------------|
| `--json` | bool | Output in JSON format |

**Example:**

```bash
oh worktree list
oh worktree ls --json
```

---

### oh worktree add

Creates a worktree for a branch. Interactive when the branch is omitted.

```
oh worktree add [branch]
```

**Example:**

```bash
oh worktree add feat/new-feature
oh worktree add fix/bug-123
```

---

### oh worktree remove

Removes a worktree.

**Alias:** `oh worktree rm`

```
oh worktree remove [path]
```

| Flag | Short | Type | Description |
|------|-------|------|-------------|
| `--force` | `-f` | bool | Force the removal |

**Example:**

```bash
oh worktree remove ../project-feat-auth
oh worktree rm -f ../project-fix-old
```

---

### oh worktree cleanup

Removes the worktrees whose branch is fully merged into the base branch (detected with `git branch --merged`).

```
oh worktree cleanup [options]
```

| Flag | Short | Type | Description |
|------|-------|------|-------------|
| `--base` | `-b` | string | Base branch (default: auto-detected) |
| `--force` | `-f` | bool | Remove without confirmation |

**Example:**

```bash
oh worktree cleanup
oh worktree cleanup -b develop --force
```

---

## Metrics & Dashboard

### oh metrics

Usage metrics: sessions, tokens, costs and savings.

```
oh metrics [options]
```

| Flag | Short | Type | Description |
|------|-------|------|-------------|
| `--period` | `-d` | string | Analysis period: `7d`, `30d` or `all` (default: `all`) |

**Example:**

```bash
oh metrics
oh metrics -d 30d
oh metrics -d 7d
```

---

### oh dashboard

Interactive dashboard (TUI): projects, sessions, tokens. No flags.

```
oh dashboard
```

---

### oh board

Full-screen ticket kanban board.

```
oh board [options]
```

| Flag | Type | Description |
|------|------|-------------|
| `--watch` | bool | Automatic refresh every 5 s |

**Example:**

```bash
oh board
oh board --watch
```

`oh optimize` (usage analysis and suggestions) and `oh yield` (sessions ↔ commits report) still exist but are hidden from the help, without flags.

---

## Secrets Management

### oh secrets

Secrets stored in the system keychain (or, as a fallback, in an encrypted file `~/.oh/secrets.enc`, passphrase prompted or `OH_PASSPHRASE`). They are referenced by name in `hub.toml` (`token_key`) and resolved at runtime, in project or global scope. Resolution order: project secret (inside a project folder), then global secret, then the matching environment variable.

| Command | Flags | Description |
|---------|-------|-------------|
| `oh secrets set <key>` | `--global`, `--project <id>` | Stores a secret; the value is typed in the terminal (hidden), or read from the standard input without a terminal (`printf %s "$TOKEN" \| oh secrets set <key>`). Scope: project when the current folder is a registered project, otherwise global; `--global` forces the global scope, `--project` targets a project |
| `oh secrets get <key>` | `--global`, `--project <id>`, `--reveal` | Shows a secret, masked unless `--reveal`; `--global`: global scope only |
| `oh secrets list` | | Lists the known secrets |
| `oh secrets delete <key>` | `--global`, `--project <id>` | Deletes a secret from the keychain |
| `oh secrets cleanup` | `--dry-run` | Removes orphaned keychain entries (absent from the secrets index); `--dry-run`: show them without deleting |

```bash
oh secrets set openhub.mcp.gitlab.token
oh secrets set openhub.mcp.gitlab.token --project t-sru-b267fbf1
oh secrets get openhub.mcp.gitlab.token --reveal
oh secrets list
oh secrets delete openhub.mcp.gitlab.token --global
oh secrets cleanup --dry-run
```

---

## Utilities

### oh version

Prints the version of the `oh` binary.

```
oh version
```

---

### oh completion

Generates the completion script for the given shell.

```
oh completion bash|zsh|fish|powershell
```

**Example:**

```bash
source <(oh completion zsh)
oh completion zsh > "${fpath[1]}/_oh"
oh completion bash > /etc/bash_completion.d/oh
oh completion fish > ~/.config/fish/completions/oh.fish
oh completion powershell | Out-String | Invoke-Expression
```
