> [Lire en français](cli-workflows.fr.md)

# CLI Reference — Workflows

Declarative workflows (`apiVersion: oh/v1`) describe a use case: entry agent, agents, checkpoints, inputs, resources and allowed execution environments. They are read by layers (hub, then the team and project layers of the team-state, see [Team workflows](../guides/team-workflows.en.md)); the most specific layer extends the one below.

## oh run

```
oh run [workflow] [-i key=value]… [--tickets a,b] [--one-session] [--mode <mode>] [--runtime local|container|remote]
                  [--location base|new|<worktree>] [--allow-dirty | --stash] [--attach <opening>] [--recap] [--dry-run] [--draft] [-a <agent>]
                  [--headless [--output <file>] [--timeout <duration>]] [--parent <session>] [-p <project>] [-P <provider>]
                  [--on-existing resume|transfer|keep|delete|parallel [--discard-base-changes]]
```

| Flag | Short | Type | Default | Description |
|------|-------|------|---------|-------------|
| `--input` | `-i` | string (repeatable) | | Workflow input `key=value` |
| `--message` | `-m` | string | | Request: short for `-i request=…`, when the workflow has a `request` input |
| `--tickets` | | list (commas) | | Beads tickets (one session per ticket when the workflow allows it) |
| `--one-session` | | bool | `false` | Every ticket in a single session |
| `--mode` | | string | workflow default mode | `manuel`, `semi-auto` or `auto` (`manual` is read as `manuel`, also in `modes:` and in the `mode:` keys of the checkpoints of a workflow) |
| `--runtime` | | string | see below | `local`, `container` or `remote` |
| `--location` | | string | `base` | `base`, `new` (new worktree) or the path of an existing worktree |
| `--allow-dirty` | | bool | `false` | Launch a writing session in a directory with uncommitted changes anyway |
| `--stash` | | bool | `false` | Stash (`git stash`) the uncommitted changes of the directory before the launch |
| `--attach` | | string | Settings preference | Opening: `auto`, `iterm`, `terminal`, `tmux`, `browser`, `suspend`, `none` |
| `--recap` | | bool | `false` | Recap and confirmation before the launch |
| `--dry-run` | | bool | `false` | Prepare the launch and show it without starting anything (see below) |
| `--draft` | | bool | `false` | Run your draft of the workflow (local only) |
| `--agent` | `-a` | string | | Entry agent (workflows with a selectable entry, e.g. `libre`) |
| `--headless` | | bool | `false` | Without interface: wait for the end of the turn, print the answer, stop the session |
| `--output` | | string | standard output | With `--headless`: file of the answer |
| `--timeout` | | duration | `30m` | With `--headless`: maximum wait (`0` = none) |
| `--parent` | | string | | Previous session (chaining) |
| `--on-existing` | | string | ask (terminal) | Open session already on a ticket of the launch: `resume`, `transfer`, `keep`, `delete` or `parallel` (see below) |
| `--discard-base-changes` | | bool | `false` | With `delete`: also discard the uncommitted changes of a session that worked in the project directory |
| `--project` | `-p` | string | project of the current folder | Project |
| `--provider` | `-P` | string | project provider | LLM provider |

Launches a workflow: layer resolution and validation, session bundle, session plan, initial prompt rendering, then start through the RunService (one server per group: bundle version, project, runtime).

- **Inputs** (`-i`, repeatable): values of the workflow `inputs` (`true`, `3`, `a,b` are converted to the input type). Defaults may depend on other inputs (`branch: feat/{{ .ticket }}`). In the prompt template, text inputs go through the `data` function: truncated (`max_length`, else 20,000 characters), and wrapped in `<oh:data-<fingerprint> name="…">…</oh:data-<fingerprint>>` tags when oh computed them from an outside source (`from:`); what you type is written as it is; see [Workflow schema](workflow-schema.en.md).
- **Request** (`-m "…"`): fills the `request` input (`quick`, `spec` and its aliases `feature` and `cadrage`, `libre`, `infra-change`). Refused, with the list of its inputs, when the launched workflow (alias resolved) has no `request` input: `oh run feature --tickets … -m …` launches `implement-spec`, which takes `-i instructions=…`. `-m` and `-i request=…` together are refused.
- **Tickets** (`--tickets`): fill the first `beads-id` input; with `picker.multi`, **one session per ticket**, all in the same server group, each in its own worktree when the workflow writes. An **epic** stands for its open or in-progress children (one session each, or a single session with all of them with `--one-session`; the recap shows `Epic  <epic> → <children>`); an epic without such a child is refused. A `beads-ids` input receives the list in a single session. In a team project, each ticket is claimed for the active member when its session starts, or moves from « planned » to the work status (for any workflow, not only `oh start --dev`); a ticket claimed by another member is reported. When the session stops, the claim of a closed ticket moves to the terminal status of the board (then goes away after the retention), the claim of a ticket still open is released; a sleeping session keeps its claims. `oh doctor` reports the orphan claims (ticket closed, or every session on the ticket ended) with the command to release them.
- **Computed inputs** (`from:`): an empty input that declares `from:` is computed by oh at launch (e.g. `review-feedback`: branch, target branch and discussions read on the MR of the `mr` input; `brief-enrich`: the takeover brief of the ticket). Sources: [workflow schema](workflow-schema.en.md#computed-inputs-from).
- **Location** (`--location`): `base` (default, project directory), `new` (a new worktree per session, branch = `branch` input or `oh/<workflow>-<ticket>`), or the path of an existing worktree. A writing session (`risk` other than `read`) **automatically gets a worktree** when another writing session (running or asleep) uses the same directory. Replaces `--worktree`.
- **Uncommitted changes**: a writing session never starts in a base directory with uncommitted changes (untracked files included) without a choice. By default it works in a **new worktree** of its branch and your changes stay untouched; `--stash` puts them aside first (`git stash apply <commit>` brings them back, the commit is printed), `--allow-dirty` launches there anyway. An existing worktree chosen with `--location <path>` is only reported. Whatever the choice, no agent may run the git commands that throw away work (`git checkout <…>` except `-b`, `git restore`, `git reset --hard|--merge|--keep`, `git clean`, `git stash` except `list`/`show`, `git switch -f|--discard-changes`, `git worktree remove`, `git branch -D`, `git rm` except `--cached`, defining a git alias, also as `git -C <dir> …`, after `env` or in `sh -c '…'`): branch changes go through `git switch`. Only the subcommand of git is aimed at: a commit or tag message that holds "git" then "clean" ("cleanup") runs.
- **Ticket already worked on** (`--on-existing`): oh looks for the open sessions (neither stopped nor failed) of the project working on a ticket of the launch, whatever their workflow. In a terminal, it shows for each one an "Existing session / New launch" table (state, workflow, version, bundle, runtime, mode, provider, model, location, branch, work, open decisions, cost, last activity; differences marked ⚠) then, for a session of **the same workflow**, asks what to do. Choices: `resume` resumes it (no new session); `transfer` moves the session and its conversation to the runtime (local ↔ container) and model of the launch — on failure, the session is unchanged and oh offers `keep`; `keep` stops it and starts the new session in the same directory and branch, with a handoff brief (`~/.oh/sessions/<id>/handoff.md`); `delete` deletes it with its work (as [`oh session delete`](cli-sessions.en.md#oh-session-delete), confirmed in a terminal) then launches afresh; `parallel` starts the new session beside it, in another worktree. **Without a terminal**, a session of the same workflow requires `--on-existing` (refused otherwise: "choose what to do with --on-existing=…"); a session of another workflow is reported and the new one starts beside it. A remote session only offers `parallel`. A transfer does not carry the subagent sessions.
- **Default workflow**: without argument, `oh run` launches the project default workflow (Project config › Execution).
- **Runtime** (`--runtime`): `local`, `container` or `remote` (must be in `runtime.allowed`; refused with the reason when the container engine is unavailable or the remote target is not set up; `remote` is **experimental** and refused without `oh config set remote.experimental true`); without `--runtime`, the project default runtime, then the Settings one, then the workflow one, when allowed. See [Container](../guides/container.en.md) and [Remote execution](../guides/remote-runners.en.md) (`oh remote setup`, then `oh session fetch` / `oh session resolve` on return).
- **Recap** (`--recap`): agents, first-turn budget, MCP, Code Mode, allowed Beads commands, checkpoints in the session mode (pause, auto, skipped, conditional; mandatory ones marked), isolation, sessions and locations (with the choice applied to an existing session), existing sessions of the tickets, warnings (uncommitted changes, automatic or already existing worktree, worktree shared with another writing session, ticket worked on in a worktree while this session takes the project directory, pending decisions on the ticket, in the project and in total), then confirmation.
- **See without launching** (`--dry-run`): prepares the launch like a real one (resolution, validation, session bundle, session plan, prompt), shows the recap, the planned pauses and the warnings, then the line « dry-run: nothing was started »; exit code 0 when the preparation succeeds, a preparation error (invalid workflow, blocking precondition, missing credentials…) exits as for a real launch. It has **no effect** on the project or the team: no session, no worktree (the planned one is shown), no ticket claim, no shell file for local sessions, no question (no choice about an existing session, no suggested workflow, no import of the Bedrock key of the environment: missing credentials are an error). **The only write**: the session bundle, in `~/.oh/bundles/<hash>`, addressed by its content (same for the same launch, reused by the real launch) and cleaned by `oh gc` when no session uses it. Refused for remote execution (`--runtime remote`): the target and the pipeline are only checked when it is sent. With `--headless`, it also tells whether the launch could run without interface (a waiting checkpoint → refused).
- **Single session** (`--one-session`): every ticket in the same session, instead of one session per ticket.
- **MCP**: without an `mcp:` field in the workflow, the session gets the project MCP servers; with `mcp:` (even empty), only the listed ones (a listed server missing from the project is reported).
- **Preconditions**: a blocking precondition refuses the launch; a suggestion (e.g. no wiki → `onboarding`) offers to run the suggested workflow first. With `resume: true`, the initial launch is remembered and offered again when that session ends ("Chain with…" in the TUI).
- **Without interface** (`--headless [--output <file>] [--timeout 30m]`): no window is opened; oh waits for the end of the turn, writes the answer (standard output or file, one file per session: `<file>.<ticket>`) then stops the session. Refused when a checkpoint waits for a validation in the chosen mode. A session asking for a decision (permission, question) stays open: `oh session inbox`, `oh session approve`. Beyond `--timeout`, the session is stopped and oh exits with an error (code 1). `oh takeover-brief enrich` goes through `oh run brief-enrich --headless`.
- **Chaining** (`--parent <session>`): links the new session to the previous one ("Chain with…" in the TUI).
- Two simultaneous launches in the same directory are refused ("launch in progress").
- **Draft** (`--draft`): runs the version being edited (your draft, team or project layer) instead of the published one. Refused on a remote runtime and when the draft **widens** the published version (risk, checkpoints, runtimes, Beads, budget…): publish it to apply these changes. See [Team workflows](../guides/team-workflows.en.md).
- **Entry agent** (`--agent`, `-a`): for a workflow with a selectable entry (`libre`), the starting agent; the members are this agent and those it may call. Refused on other workflows.
- Requires opencode V2 (minimum version in `oh doctor`).

**Examples:**

```bash
oh run                                              # project default workflow
oh run feature -m "Add a /health endpoint"
oh run ticket --tickets bd-42,bd-43                 # one session per ticket
oh run ticket --tickets bd-42,bd-43 --one-session
oh run libre --agent debugger -m "Test X fails"
oh run audit -i type=security --runtime container
oh run review --headless --output review.md --timeout 20m
oh run ticket --tickets bd-42 --runtime remote
oh run ticket --tickets bd-42 --runtime container --on-existing=transfer   # resume the ticket's session in a container
```

## oh workflow list

Lists the catalogue workflows, one per id (its most specific layer), with version, risk, execution environments and validity (`✓` valid, `!` warnings, `✗` errors). With a team-state: then **your drafts** (`✎`, error count, "new brick"), the files skipped by the integrity check and the publications waiting for the network (`⏳`). An unreadable team file is listed invalid in its own layer (team or project).

| Flag | Type | Description |
|------|------|-------------|
| `-p, --project` | string | Team-state and layer of the project (same context as the editing commands) |
| `--team` | string | Team or solo space, without a project |
| `--json` | bool | JSON output (array of summaries: id, layer, chain, risk, entry, modes, runtimes, Beads input, diagnostics; drafts are included with `"draft": true`, plus `queued`, `team_bricks`, `new_bricks` when relevant) |

## oh workflow show

```
oh workflow show <id>|<layer>:<id> [-p <project>] [--team <id>] [--origin] [--json]
```

Shows a workflow once its `extends` are resolved, in the same context as a launch: team and project layers of the project of `-p` or of the current folder (else the active team; `--team` picks a team or solo space). `project:<id>` is accepted (outside a project, the message asks for `-p`). It shows: header (chain, version, risk, isolation, entry agent, modes, runtimes, Code Mode), inputs, agents, checkpoints (behavior per mode), resources (skills, MCP, Beads, plugins, outputs, models) and diagnostics. The command fails when the workflow is invalid.

| Flag | Type | Description |
|------|------|-------------|
| `--origin` | bool | Shows the layer that set each value (`default` when no document writes it) |
| `--json` | bool | JSON output (resolved `spec`, `origins` by field path, `diagnostics`) |
| `-p`, `--project` | string | Project (id or name); default: the one of the current folder |
| `--team` | string | Team or solo space, without a project |

## oh workflow validate

```
oh workflow validate <file>|<id>|<layer>:<id> [--layer hub|team|project] [--project <project>] [--json]
oh workflow validate --all [--json]
```

Validates a file or a catalogue workflow: strict parsing, `extends` resolution, security rules, references to the brick catalogue. Non-zero exit on errors.

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--all` | bool | `false` | Validates every hub workflow |
| `--layer` | string | `hub` | Layer of the validated file: `hub`, `team` or `project` |
| `--project` | string | | Project (id or name) whose layer is loaded with its team layer (no short form) |
| `--team` | string | | Team or solo space, without a project |
| `--json` | bool | `false` | JSON output |

The team and project layers loaded are those of the project of `--project` or of the current folder (else the active team, or the only solo space); a bare id means the most specific layer holding it. An unpublished workflow (`team:<id>`, `project:<id>`) is looked up in **your draft**, flagged "(your draft, not published)". A team or project workflow without prompt (nor `extends`) gives a warning: the session would start without any initial instruction; published files changed outside a publication are skipped with a warning. Brick references are checked against the catalogue **merged** with the team bricks (`catalog/`), as at launch.

## Editing team and project workflows

Commands of the draft → publication cycle (see [Team workflows](../guides/team-workflows.en.md)). All accept `-p, --project <project>`: team-state and layer of the project; by default, the project of the current folder (when it has a team or a solo space), else the active team (or, without a team, your only solo space); `--team <id>` picks a team or solo space without a project. A bare id means the project layer when it holds the workflow, else the team layer; `team:<id>` / `project:<id>` set it. The `-p, --project` and `--team` flags are not repeated in the tables below.

### oh workflow new

```
oh workflow new <id> [--layer team|project] [--extends <ref> | --copy <ref>] [--file <file>|-] [--no-edit]
```

Creates your draft: empty (valid) skeleton, patch of a workflow (`--extends hub:ticket`) or copy of a document under the new id (`--copy hub:review`, prompt template included). Opens `$VISUAL`/`$EDITOR` (else `vi`), then validates: on errors, the editor can be reopened, otherwise the edited file is kept.

> With `--file`, a document whose `prompt.template` points to a missing template gets a starter template (mode, language, one line per input), or the `--prompt-file` one. A document **without prompt** (nor `extends`) gets `prompt.template: prompts/<id>.md.tmpl` and the same starter template, with a message: complete it with `oh workflow edit <id> --prompt` (or the TUI editor, key `P`).

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--layer` | string | `team` | Draft layer: `team` or `project` |
| `--extends` | string | | Workflow to extend (e.g. `hub:ticket`): the draft is a patch |
| `--copy` | string | | Document to duplicate (e.g. `hub:review`) under the new id |
| `--file` | string | | Read the document from this file (`-`: standard input) instead of opening the editor |
| `--no-edit` | bool | `false` | Save the initial draft without opening the editor |

### oh workflow edit

```
oh workflow edit <id> [--layer team|project] [--prompt] [--file <file>] [--prompt-file <file>]
```

Edits your draft (without a draft: a copy of the published version becomes your draft). `--prompt` opens the draft's own prompt template; `--file` / `--prompt-file` replace the content without an editor.

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--layer` | string | project when it holds the workflow, else team | `team` or `project` |
| `--prompt` | bool | `false` | Edit the prompt template instead of the document |
| `--file` | string | | Replace the document with this file (`-`: standard input) |
| `--prompt-file` | string | | Replace the draft's prompt template with this file |

### oh workflow diff

```
oh workflow diff <id> [--against published|<version>] [--json]
```

Diff of the document and prompt template between your draft and the published version (or a history version), then the **impact summary** (⚠ = widening: risk, writing agents, remote, checkpoints, Beads, budget, MCP…) and new team bricks.

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--against` | string | `published` | `published` or a version number |
| `--layer` | string | project when it holds the workflow, else team | `team` or `project` |
| `--json` | bool | `false` | JSON output (diff, impact, next version) |

### oh workflow publish

```
oh workflow publish <id> -m "<message>" [--yes]
oh workflow publish --retry
```

Shows the next version and the impact, asks for confirmation when the workflow is widened (unless `--yes` or without a terminal), then publishes (sync, revalidation, version + 1, history, `workflows.lock`, commit + push, redone if another member published in the meantime). Offline: queued; `--retry` replays the pending publications (the TUI also replays them automatically at each team-state synchronization). Team members only (`[governance] publish`).

| Flag | Short | Type | Description |
|------|-------|------|-------------|
| `--message` | `-m` | string | Publication message (required) |
| `--retry` | | bool | Replay the pending (offline) publications |
| `--yes` | `-y` | bool | Do not ask for confirmation |

### oh workflow history

```
oh workflow history <id> [--json]
```

Published versions, most recent first: version, date, author, message.

| Flag | Type | Description |
|------|------|-------------|
| `--json` | bool | JSON output |

### oh workflow restore

```
oh workflow restore <id> <version> [--yes]
```

Publishes the content of a version (document and template) again as a **new** version, after revalidation.

| Flag | Short | Type | Description |
|------|-------|------|-------------|
| `--yes` | `-y` | bool | Do not ask for confirmation |

### oh workflow archive

```
oh workflow archive <id> [-m "<reason>"] [--yes]
```

Withdraws the published workflow; its last version stays in history (can be restored).

| Flag | Short | Type | Description |
|------|-------|------|-------------|
| `--message` | `-m` | string | Reason for archiving |
| `--yes` | `-y` | bool | Do not ask for confirmation |

## oh bundle build / show

```
oh bundle build <workflow> [-p <project>] [-P <provider>] [--json]
oh bundle show <workflow>|<hash> [-p <project>] [-P <provider>] [--budget] [--json]
```

Compiles (idempotently, by hash) the session bundle of a workflow into `~/.oh/bundles/<hash>/` and shows it: agents (entry agent first), delegations, on-demand skills, MCP, plugins, default model, delegation depth, isolation (`strict` when the workflow requires it), global permissions and the estimated cost of the first turn (entry agent + its skill catalogue; with `--json`, the skill count and catalogue cost of each agent: `skills`, `catalog_tokens`). Stack skills only enter the catalogue of the `stack_skills: true` agents. Without `-p`, the project is the one of the current directory (otherwise a hub-only bundle, without project instructions or MCP); the workflow is resolved with the team and project layers of that project. Stack skills follow the detected stack; the frontend domain skills (`dev-standards-frontend*`) are shipped only when the project has a frontend (`package.json` of a web framework, `.tsx`/`.jsx`/`.vue`/`.svelte` sources, `index.html`, HTML templates), a skill tied to a plugin (`plugin:`) only when the workflow loads that plugin (`plugins:`), and a skill tied to an MCP server (`mcp:`, GitLab and GitHub adapters) only when the session has that server.

| Flag | Short | Type | Description |
|------|-------|------|-------------|
| `--project` | `-p` | string | Project (instructions, models, MCP); default: detected from the current folder |
| `--provider` | `-P` | string | LLM provider (model normalization) |
| `--budget` | | bool | `show` only: details the estimated budget, tokens per agent and per skill (≈ 4 characters per token) |
| `--json` | | bool | JSON output (`bundle.Report`) |

`oh bundle show` accepts a workflow name or the hash of an already built bundle.

```bash
oh bundle build ticket -p my-app
oh bundle show ticket --budget
oh bundle show 3f9c2a… --json
```

The former `oh skill budget` alias is removed ([ADR-051](../architecture/adr/051-community-skills-disconnection.en.md)): the budget of a session is read with `oh bundle show <workflow> --budget`.

## Environment variables

| Variable | Effect |
|----------|--------|
| `OH_WORKFLOWS_DIR` | Replaces `~/.oh/hub/workflows` as the hub layer (workflows under development, tests) |
