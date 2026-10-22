> [Lire en francais](contributing.fr.md)

# Contribution Guide

This guide explains how to contribute to the hub: Go CLI development,
adding agents and skills, and the PR process.

---

## CLI Development (code contributions)

### Prerequisites

| Tool | Version | Installation |
|------|---------|-------------|
| Go | 1.26.4+ | [go.dev/dl](https://go.dev/dl/) |
| golangci-lint | 2.1+ | `brew install golangci-lint` or [docs](https://golangci-lint.run/welcome/install/) |
| git | 2.5+ | System-provided |

### Local setup

```bash
git clone https://github.com/datichb/openhub.git
cd openhub/cli

# Download dependencies
make deps

# Sync the hub content into the Go embed directory and build the fake bd of containers
# (MANDATORY step before first build — GoReleaser does this automatically in release)
make embed-sync

# Build the binary
make build          # -> bin/oh
```

> **Important:** the binary embeds a copy of `agents/`, `skills/`, `permissions/` and
> `workflows/` (in `internal/hubcontent/hub/`). After changing that content, run
> `make sync-content`: it copies it again without rebuilding the fake bd (faster than
> `make embed-sync`). While the copy differs from the sources, `TestEmbeddedContentInSync`
> fails and lists the files at fault (only in the development repository: release builds
> copy the sources first).

### Running tests

```bash
make test           # All tests with -race detector (timeout: TEST_TIMEOUT=40m)
make test-unit      # Unit tests only (-short, faster)
make test-tui       # TUI tests only (./internal/tui/...)
make test-cover     # Generates coverage.out + coverage.html
```

Measuring the suite durations: `make test-durations` runs `go test ./... -short -count=1 -json`
(`TEST_DURATIONS_PKGS`, `TEST_DURATIONS_ARGS` and `TEST_DURATIONS_OUT` change the packages, the
arguments — for instance `-race` — and the JSON file, `bin/test-durations.json` by default), then
prints the real duration, the number of passed, failed and skipped tests, the duration of each package
and the slowest top-level tests (`TEST_DURATIONS_TOP`). The same summary can be read from any
`go test -json` output, and two measures are compared package by package:

```bash
go test ./internal/semver/... -json | go run scripts/testdurations/main.go
go run scripts/testdurations/main.go -compare before.json after.json   # from cli/
```

CI enforces a minimum coverage threshold of **60%** with the race detector enabled.

GitHub CI (`.github/workflows/ci.yml`) runs, on Linux, macOS and Windows, `go vet`, the lint,
`govulncheck` and one single `go test -race` run of the suite (40-minute timeout) that also writes the
coverage profile, whose total is checked against the threshold on Linux and macOS; a separate
job runs the contract tests (tag `integration`) against a real `opencode serve` of a pinned
version, without calling any model, on the packages that have such tests only (the list is computed
from the `//go:build integration` test files, so a new package joins it by itself; today
`internal/adapters/opencodev2`, `internal/gateway`, `internal/runsvc` and `internal/services/remote`). The e2e tests (tag `e2e`, real Bedrock model) and the
evaluations (`make test-eval`) never run in CI: they are local and manual, and no model key is
given to the GitHub secrets.

On Windows, CI only tests the packages listed in `.github/windows-test-packages.txt`: the tests of
the 24 others, written for Unix, fail there. They are listed with their cause in
`.github/windows-test-packages-excluded.txt` and followed by the `windows-tests` batch, which will make
them green. A new package that is in neither file is not tested on Windows: CI reports it with a
warning (`.github/scripts/windows-packages.sh check`); add it to one of the two files. The coverage
threshold is not computed on Windows, so that it stays the one of the whole module.

The full suite takes longer than the default `go test` timeout (10 minutes), especially while
other suites run in parallel: outside `make test`, pass the timeout yourself
(`go test -timeout 40m ./...`).

The diagrams of `docs/` are ` ```mermaid ` blocks (sources in `docs/diagrams/*.mermaid`, no SVG
anymore). `internal/docstest` checks that no diagram SVG is referenced, that a block followed by
"Diagram source" is identical to its source and, when `mmdc` is installed
(`npm i -g @mermaid-js/mermaid-cli`), that every diagram renders without error.

#### Automatic test isolation

Tests never touch your real hub nor your real `HOME`, even when your shell exports `OH_HOME` or
`OH_TEST_*` variables (for instance after `source openhub-test/env.sh`). The `TestMain` of the
packages concerned calls `internal/testenv`, which, before `m.Run()`:

- creates a temporary folder and puts `HOME` and `XDG_CONFIG_HOME`, `XDG_DATA_HOME`,
  `XDG_CACHE_HOME`, `XDG_STATE_HOME` in it;
- removes the `OH_*` variables (`OH_HOME`, `OH_TEST_*`, `OH_PASSPHRASE`, session and gateway…),
  except `OH_E2E_*` and `OH_EVAL_*` (tests that call a real model). The hub is then `~/.oh` of the
  temporary `HOME`; a test that wants another hub sets `OH_HOME` with `t.Setenv`;
- keeps the `PATH` (`opencode`, `bd`) and the Go caches;
- replaces your global git configuration by the one of the tests (`gittest.Defaults`: a fake
  identity, `main` as the initial branch, no signing, no automatic maintenance), as on a CI runner
  that has neither: a test must depend neither on your identity nor on your git settings.

A watchdog compares, before and after the tests, the size and modification time of `hub.toml` and
`oh.db` in the former `OH_HOME` and in `~/.oh`: if either changed, the package fails (message
"changed the real hub"). It is read only: it creates nothing. If you used oh on that hub while the
tests ran, run them again.

`env -u OH_HOME go test` is therefore no longer needed. A new package whose tests read `OH_HOME`,
`HOME` or a derived path (`~/.oh`, `~/.config`, keychain…), directly or through the code under
test, adds a `main_test.go`:

```go
func TestMain(m *testing.M) {
	finish := testenv.Isolate()
	os.Exit(finish(m.Run()))
}
```

(with `gittest.Main(m)` instead of `m.Run()` if the tests run git). A test that re-executes the test
binary as a helper process inherits the isolated environment.

#### Test fixtures

Four shared tool packages save every package from writing the same setup again. A test uses them
instead of a local helper, and does not add a new local helper that duplicates one of them: if
something is missing, extend the shared tool.

- **`sqlitetest`** (`internal/storage/sqlite/sqlitetest`): a temporary SQLite database, migrations
  applied and closed at the end of the test, and a way to fill it through the public stores. The
  `sqlite` package itself cannot import it (cycle): its tests open the database themselves.

  ```go
  st := sqlitetest.New(t)
  sqlitetest.Seed(t, st).
      Project("p1", "/p1").
      Session(&domain.Session{ID: "ses_a", ProjectID: "p1", Status: domain.SessionStatusRunning})
  ```

- **`gittest`** (`internal/gitutil/gittest`): `Run`, `Repo`, `Commit` and `BareClone` run git with
  the identity and settings of `gittest.Defaults`, with or without `gittest.Main`: they never read
  your git configuration. `Run` fails the test with the output of git and returns the standard
  output without surrounding whitespace.

  ```go
  bare, clone := gittest.BareClone(t)          // bare repository + clone on main, ready to push
  gittest.Commit(t, clone, "docs/a.md", "a\n", "add a")
  gittest.Run(t, clone, "push", "-q")
  head := gittest.Run(t, bare, "rev-parse", "main")
  ```

- **`configtest`** (`internal/config/configtest`): `Home(t)` puts `HOME` in a temporary folder (the
  hub is `<home>/.oh`), clears `OH_HOME` and resets the `config` cache before the test and when it
  ends.

  ```go
  home := configtest.Home(t)
  require.NoError(t, config.Save(cfg)) // writes <home>/.oh/hub.toml
  ```

- **`golden`** (`internal/testutil/golden`): `golden.Assert(t, "render", output)` compares the output
  (`string` or `[]byte`) with `testdata/render.golden`. On a difference, the failure shows the diff
  and the command that regenerates the file; `-update` rewrites it (then review the diff of the golden
  file like code). The `-update` flag is declared by this package: a package that imports it does
  not declare its own (`flag.Bool("update", …)`), otherwise the test binary panics at start.

  ```go
  golden.Assert(t, "render", out) // testdata/render.golden
  ```

Tests that run the real `bd` (Dolt startup, several seconds) call `bdtest.RequireRealBd(t)`: they
are skipped by `go test -short` and by `make test-unit`, but run in the full suite if `bd` is
installed. `bdtest.InitedRepo(t)` provides an already initialized repository. The scheduling tests of
`evalrun` use `Runner.Prepare` to avoid git.

### Linting and formatting

```bash
make lint           # golangci-lint run ./...
make vet            # go vet ./...
make fmt            # gofmt -s -w .
```

**One golangci-lint cache per worktree.** golangci-lint keeps one cache for every directory
(`~/Library/Caches/golangci-lint` on macOS). With several worktrees of the repository linted in
parallel, that shared cache can return results of another tree or block on its lock. Give each
worktree its own cache:

```bash
# A throwaway cache, for one run
GOLANGCI_LINT_CACHE=$(mktemp -d) golangci-lint run ./...

# Or a cache of the worktree, kept between runs
export GOLANGCI_LINT_CACHE="${TMPDIR:-/tmp}/golangci-lint-$(basename "$(git rev-parse --show-toplevel)")"   # outside the tree, one per worktree
```

Active linters include: `bodyclose`, `gocritic` (diagnostic + style), `misspell`, `nilerr`.

### Code overview

The CLI is a single Go binary (`cli/main.go` -> `cli/cmd/root.go`). Internal architecture:

| Package | Responsibility |
|---------|---------------|
| `cmd/` | Cobra commands (CLI entry point) |
| `internal/config/` | Hub configuration (`hub.toml`, TOML + Viper) |
| `internal/mcp/` | Built-in MCP servers (Figma, GitLab, GitHub, Jira, Linear, GSlides, Team, Workflow) |
| `internal/teamstate/` | Team state management (claims, wiki, policies, patterns, team workflows) |
| `internal/workflow/` | Declarative `oh/v1` workflows (schema, layers, validation) |
| `internal/bundle/` | Session bundle (agents, skills, permissions, compiled outside the project under `~/.oh/bundles/`) |
| `internal/sessionspec/` | Tool-agnostic session model (bundle, location, runtime, provider) |
| `internal/adapters/` | oh ↔ agentic tool contract; opencode V2 adapter (`opencodev2/`) |
| `internal/runsvc/` | v5 session launcher (server groups, proxy, closed world, checkpoints) |
| `internal/daemon/` | `ohd` daemon (session supervision, decisions, notifications) |
| `internal/credproxy/` | LLM credential proxy (per-group tokens) |
| `internal/runtime/`, `internal/remote/` | Execution environments (container, GitLab CI remote) |
| `internal/limits/` | Session restrictions (active sessions, budgets, memory, models) |
| `internal/deploycleanup/` | Cleanup of former deployments (`oh migrate deploy-cleanup`) |
| `internal/tui/` | TUI interface (tview; huh for a few inline prompts outside the TUI; shell, views, widgets) |
| `internal/storage/` | SQLite (projects, sessions), keychain, file encryption |
| `internal/tracker/` | External tracker sync (GitLab, Jira) |
| `internal/i18n/` | Internationalization (FR + EN, JSON) |

The binary embeds agents, skills, and permissions via `go:embed` (`internal/hubcontent/`).
`CGO_ENABLED=0` — no C dependencies, static binary.

### Code conventions

- **Thin commands**: a Cobra command (`cmd/<domain>.go`, `cmd/<domain>_<action>.go`) reads its flags, calls a service or a package of `internal/` and formats the output (`cmd.OutOrStdout()`); it returns its errors, never `os.Exit`. Its startup level is declared when it is registered (`withInitDB`, `withInitLevel(cmd, levelNone)`); stores come from `MustApp()` (or `TryApp()`).
- **Logic shared by the CLI and the TUI** in `internal/services/<domain>/`: the TUI does not call the commands.
- **Contracts in `internal/domain/`** (types, `XxxStore` interfaces, `ErrXxx` sentinel errors), implementations in `internal/storage/sqlite/` (migrations in `migrate.go`), checked at compile time (`var _ domain.XxxStore = (*XxxStore)(nil)`).
- **Independence from the agentic tool**: only `internal/adapters/<tool>/` and the composition root (`cmd/v5_adapters.go`) name the tool; `internal/archtest/` checks it.
- **Displayed texts** through `i18n.T` / `i18n.Tf`, keys in `internal/i18n/locales/fr.json` and `en.json` (a test checks they match). No message is written in the code of `cmd/` or of the TUI views (`TestNoLiteralUserMessages`; the few exceptions are listed with their reason in `cmd/messages_literal_test.go`), and a key of `en.json` that the code does not use anywhere is refused (`TestNoOrphanKeys`; a key built at run time, `i18n.T("prefix." + x)`, is recognised by its prefix). A confirmation typed on the keyboard goes through `confirmed()` (`cmd/confirm.go`: `o`, `oui`, `y`, `yes`).
- **Errors** wrapped with their context (`fmt.Errorf("creating project %s: %w", id, err)`), tested with `errors.Is` / `errors.As`.
- **Tests** in the same package, with `testify` (`require` for preconditions), table-driven cases, `t.TempDir()` and `t.Helper()` helpers. A test that starts processes (git, bd, servers) waits for them to end before its directory is cleaned up; the tests that push to bare repositories turn git's automatic maintenance off (`internal/gitutil/gittest`).

---

## Adding an agent

### 1. Create the agent file

```bash
touch agents/<family>/<id>.md
```

Follow the naming convention:
- `<domain>-<speciality>.md` for sub-agents (e.g. `auditor-subagent.md`)
- `<role>.md` for primary agents (e.g. `orchestrator.md`)

### 2. Minimal frontmatter structure

```markdown
---
id: <unique-identifier>
label: <DisplayName>
description: <Short description in one sentence — visible in agent lists>
permission:
  skill: allow        # allow for agents using native skills; deny for coordinators
  ctx_search: allow   # allow for agents that analyse code or data
skills: [path/to/skill, ...]          # Bucket A — always-on inline skills
native_skills: [path/to/skill, ...]   # Bucket B — on-demand native skills (optional)
---
```

**Rules:**
- `id`: unique slug, lowercase, hyphens allowed, no spaces
- `label`: PascalCase, displayed in the AI tool
- `description`: one sentence, starts with a verb or a role name
- `skills`: Bucket A paths relative to `skills/` — workflow protocols, handoff formats, universal principles (always active)
- `native_skills`: Bucket B paths relative to `skills/` — domain standards, checklists, contextual skills (loaded on-demand via the `skill` tool)
- `permission.skill`: `allow` if the agent uses native skills; `deny` for coordinators/orchestrators
- `permission.ctx_*`: ctx tools must be explicitly allowed per agent — they are **not** inherited. See the table below for the recommended set per agent type.

**ctx permissions by agent type:**

| Agent type | Recommended ctx permissions |
|---|---|
| Orchestrators (planning) | `ctx_search`, `ctx_stats`, `ctx_batch_execute` |
| Developer (developer) | `ctx_search`, `ctx_execute`, `ctx_execute_file`, `ctx_batch_execute`, `ctx_fetch_and_index`, `ctx_index` |
| Quality agents (debugger, reviewer) | `ctx_search`, `ctx_execute`, `ctx_execute_file`, `ctx_batch_execute` |
| Planning agents (planner, pathfinder, onboarder) | `ctx_search`, `ctx_stats`, `ctx_batch_execute` |
| Design agents (designer) | `ctx_search`, `ctx_batch_execute` |
| Documentation (documentarian) | `ctx_search`, `ctx_batch_execute`, `ctx_index` |
| Audit agents (auditor, auditor-subagent) | `ctx_search`, `ctx_batch_execute` |

See [ADR-010](../architecture/adr/010-hybrid-skills-architecture.en.md) for the Bucket A / B rationale.

### 3. Agent body

Recommended structure (see `agents/auditor/auditor.md` as a reference for coordinators,
`agents/developer/developer.md` for implementer agents):

```markdown
# <DisplayName>

<Identity sentence: who you are and what you do in 2-3 lines>

## What you do

- <Action 1>
- <Action 2>

## What you do NOT do

- <Constraint 1>
- <Constraint 2>

## Workflow

<Condensed workflow in 4-6 steps>

## Invocation examples (optional)

| Request | Action |
|---------|--------|
| "..." | ... |
```

### 4. Create or reference skills

If the agent requires a dedicated protocol, create the corresponding skill
(see the "Adding a skill" section below) before referencing it in the frontmatter.

### 5. Build and test

```bash
# Build the session bundle of a workflow that uses the agent
oh bundle build <workflow>
# Verify the agent appears in the bundle
oh bundle show <workflow>
```

---

## Adding a skill

### 1. Choose the right folder

Skills are organised by domain in `skills/`:

| Folder | Usage |
|--------|-------|
| `skills/developer/` | Development standards (shared between developers and reviewer) |
| `skills/auditor/` | Audit protocols |
| `skills/orchestrator/` | Coordination protocols |
| `skills/planning/` | Planning protocols |
| `skills/posture/` | Posture and cross-cutting behavior (`expert-posture`, `core-posture`, `user-facing-posture`, `coordinator-posture`) |
| `skills/qa/` | Quality protocols |
| `skills/debugger/` | Diagnostic protocols |
| `skills/reviewer/` | Review protocols |
| `skills/documentarian/` | Documentation protocols |
| `skills/designer/` | Design protocols (designer) |
| `skills/design/` | Design handoff contracts |
| `skills/quality/` | Quality handoff contracts (debugger and agents not in qa/ or reviewer/) |

For a new domain, create a new sub-folder.

### 2. Minimal frontmatter structure

```markdown
---
name: <skill-name>
description: <Short description — visible in the agent's skill list>
---
```

> The `name` key is documentary. The bundle builder reads `description`.
> The file path is the reference used in agent frontmatter.

### 3. Skill content

A good skill contains:

- **Role**: reminder of the identity of the agent using this skill
- **Absolute rules**: ❌/✅ — non-negotiable constraints
- **Protocol / workflow**: detailed steps
- **Output formats**: exact report structures, with examples
- **Checklists**: systematic verifications
- **What you do NOT do**: explicit anti-patterns

See `skills/reviewer/review-protocol.md` as an example.
For Bucket B native skills that handle mode selection and parallel orchestration, see
`skills/reviewer/reviewer-adversarial.md`, `skills/reviewer/reviewer-edge-case.md`, and
`skills/reviewer/review-merge.md` (report fusion from parallel sessions).

### 4. Reference the skill in an agent

Decide whether the skill is Bucket A or Bucket B (see [ADR-010](../architecture/adr/010-hybrid-skills-architecture.en.md)):

**Bucket A** — add to `skills:` in the agent frontmatter:
```markdown
---
skills: [path/to/my-skill]
---
```

**Bucket B** — add to `native_skills:` in the agent frontmatter, and add `permission: skill: allow`:
```markdown
---
permission:
  skill: allow
native_skills: [path/to/my-skill]
---
```
Also add a row to the "## Available skills" guide section in the agent body with the loading trigger.

**Returns between agents:** an agent launched by another inlines `shared/handoff-envelope` (common envelope) and its payload `<family>/<agent>-payload` (Bucket A, `when: role=subagent`, 200 to 400 tokens, statuses in the `statuses:` frontmatter). The caller inlines no format: the statuses of the agents it launches are generated in its session contract. Never write `## Retour vers <agent>` with an agent name in a skill: the title comes from the contract. Examples: `skills/developer/developer-payload.md`, `skills/reviewer/reviewer-payload.md`.

---

## Session bundle

Since v5, nothing is deployed into the project (`oh deploy` / `oh sync` removed in v5). Each session starts from a session bundle built at launch outside the project (`~/.oh/bundles/<hash>/`) from its workflow: agents translated from the hub format to the opencode format, skills, permissions, MCP servers.

The bundle logic is implemented in Go in `cli/internal/bundle/`.

### Quick steps

1. Add or modify agents/skills in the `agents/` and `skills/` directories
2. Run `oh bundle build <workflow>` to build the bundle of a workflow that uses them
3. Run `oh bundle show <workflow> --budget` to inspect its content and context budget

---

## Contribution conventions

### Commits

**Conventional Commits** format required:

```
feat: add agent <name>
fix: fix <issue> in <file>
docs: update <section>
chore: <maintenance>
refactor: <restructuring>
```

### File naming

| Type | Convention | Example |
|------|-----------|---------|
| Agent | `<domain>[-<speciality>].md` | `auditor-subagent.md` |
| Skill (in a sub-folder) | `<domain>-<topic>.md` | `audit-security.md` |

### Internal documentation

The `docs/` folder is organized as follows (for reference when adding documentation):

- `docs/architecture/` — architectural decisions (ADR), diagrams, agents and skills reference
- `docs/guides/` — practical guides (contributing, workflows, authoring)
- `docs/reference/` — CLI and configuration reference
- `docs/dev/` — internal technical notes (gotchas, shell patterns)

### ADR

Any significant architectural decision must be documented in an ADR:

```bash
touch docs/architecture/adr/<NNN>-<kebab-case-title>.md
```

Format: see [ADR-001](../architecture/adr/001-agent-skill-separation.en.md) as a template.

### PR

Before submitting a PR:

```bash
# Verify the session bundle builds correctly
oh bundle build <workflow>

# Review the bundle content
oh bundle show <workflow> --budget
```

---

## Pre-PR checklist

- [ ] The agent file follows the minimal structure (frontmatter + body)
- [ ] The skill has a frontmatter with `name` and `description`
- [ ] Bucket A skills are in `skills:`, Bucket B skills are in `native_skills:` — rationale documented in [ADR-010](../architecture/adr/010-hybrid-skills-architecture.en.md)
- [ ] If the agent uses `native_skills:`, `permission: skill: allow` is set; if it's a coordinator, `skill: deny` is set
- [ ] ctx permissions (`ctx_search`, `ctx_batch_execute`, etc.) are declared according to the agent type — they are NOT inherited and must be explicit (see frontmatter rules above)
- [ ] If the agent has native skills, the agent body has a "## Available skills" guide section listing them with loading triggers
- [ ] The agent is referenced in `README.md` and `docs/architecture/agents.en.md`
- [ ] The skill is referenced in `docs/architecture/skills.en.md` with its bucket marker (A) or (B)
- [ ] If the skill defines a structured return format: injected in both the producing agent AND the consuming agent (always Bucket A)
- [ ] If architectural decision: an ADR is created in `docs/architecture/adr/`
- [ ] The commit follows Conventional Commits
- [ ] `make test` passes without errors (or `make test-unit` at minimum)
- [ ] `make lint` produces no warnings
- [ ] `oh bundle build <workflow>` passes without errors for the affected workflows
- [ ] `oh bundle show <workflow>` shows no unexpected divergence

---

## Creating a release

> Reserved for maintainers with write access to `main`.
>
> **The procedure to follow, checked against the workflows, is the [release checklist](../dev/release-checklist.md)** (in French). What follows is a short reminder.

### Prerequisites

- Be on the `main` branch with a clean working tree
- [GoReleaser](https://goreleaser.com/) installed (`brew install goreleaser`)
- Push access to the remote repository

### Preparing the CHANGELOG

Before releasing, write the release content under `## [Unreleased]` in `CHANGELOG.md`.

Items ready for the next release include:
- Team kanban board (5 columns, wired c/x/t/s actions, visual labels `[AI]`)
- Claim lifecycle (5 statuses: `planned`, `in_progress`, `review`, `blocked`, `done`)
- `oh team claim --planned` and automatic `planned → in_progress` transition via `oh start --dev` (now a deprecated alias of `oh run ticket`)
- External tracker sync (`oh team sync-tracker`, `cli/internal/tracker/` package, GitLab + Jira)
- `agent-reviewed` label auto-applied on `review.ready`
- Async pull on all team views

```markdown
## [Unreleased]

### Added
- ...

### Changed
- ...

### Fixed
- ...
```

### Running the release

```bash
# From the cli/ directory
cd cli

# Preview without publishing (snapshot)
goreleaser release --snapshot --clean

# Create the release (triggered by a tag push)
git tag -a v1.2.0 -m "Release v1.2.0"
git push && git push --tags
```

GoReleaser is configured via `.goreleaser.yml` in the `cli/` directory and handles:
1. Cross-compilation of the `oh` binary
2. Changelog generation from commits
3. GitHub release creation with artifacts

### Tag convention

Tags follow the `vX.Y.Z` format (annotated):

```bash
git tag -a v1.2.0 -m "Release v1.2.0"
```
