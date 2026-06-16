> 🇫🇷 [Lire en français](cli.fr.md)

# CLI Reference — `oc` commands

All commands available via the `oc.sh` entry point (recommended alias: `oc`).

---

## Global synopsis

```
oc <command> [sub-command] [options] [arguments]
```

---

## `oc install`

Installs tools and creates the hub structure.

```bash
oc install
```

**Behaviour:**
- Checks and **requests confirmation** before installing each dependency (Node.js, opencode, Beads, bun)
- If `config/hub.json` already exists, requests confirmation before overwriting

---

## `oc uninstall`

Uninstalls opencode-hub and cleans up artefacts created during installation.

```bash
oc uninstall
# equivalent to:
bash ~/.opencode-hub/uninstall.sh
```

**Behaviour:**

Guides the uninstallation through 4 optional steps, all with explicit confirmation:

| Step | Action | Default |
|------|--------|---------|
| 1 | Clean up deployed agents in projects (`.opencode/agents/`, `opencode.json`, `.opencode/agents/`) | `[y/N]` |
| 2 | Remove the hub (`~/.opencode-hub`) | `[y/N]` |
| 3 | Remove the `oc` alias and bun exports from the shell rc file | `[Y/n]` |
| 4 | Uninstall system tools: `opencode`, `beads`, `bun` (separately) | `[y/N]` |

> `jq` and `node` are not offered for uninstallation (general use, risk of breaking other tools).
>
> A `.bak` backup is automatically created before any modification of the rc file.

---

## `oc deploy`

Generates agent files for a project. When a `PROJECT_ID` is provided, **automatically detects the project's stack** and injects the corresponding stack-specific skills into developer agents (in addition to their statically declared skills).

```bash
oc deploy [PROJECT_ID]
oc deploy --check [PROJECT_ID]
oc deploy --diff  [PROJECT_ID]
```

**Arguments:**

| Argument | Values | Description |
|----------|--------|-------------|
| `[PROJECT_ID]` | ID of a registered project | Optional — deploys at hub level if absent (no stack detection) |

**Options:**

| Option | Description |
|--------|-------------|
| `--check` | Checks if files are up to date without deploying |
| `--diff` | Compares sources with deployed files; offers deployment if a difference is detected |

**Stack detection:**

When `PROJECT_ID` is provided, `oc deploy` reads the project's dependency files (`package.json`, `pyproject.toml`, `requirements.txt`, `Gemfile`, `build.gradle`, infrastructure files, etc.) to detect the active stack. The corresponding skills from `skills/developer/stacks/` are then injected into developer agents based on the mapping in `config/stack-skills.json`.

This means a `developer-frontend` agent deployed on a React/Vitest/Playwright project will automatically receive `dev-standards-react`, `dev-standards-vitest`, and `dev-standards-playwright` — without any agent configuration changes.

**Examples:**

```bash
oc deploy                       # deploy at hub level (no stack detection)
oc deploy MY-APP                # deploy agents to MY-APP (with stack detection)
oc deploy --check               # check hub agents
oc deploy --check MY-APP        # check MY-APP agents
oc deploy --diff MY-APP         # show diff sources → deployed for MY-APP
```

**Generated outputs:**

| Target | Generated files |
|--------|----------------|
| `opencode` | `.opencode/agents/*.md` + `opencode.json` (regenerated if an API key or PROJECT_ID is defined) |

**`--check` exit codes:**
- `0`: everything is up to date
- `1`: at least one file is outdated or missing

> An animated spinner (`⠋⠙⠹…`) is displayed while deploying.

---

## `oc sync`

Redeploys agents on all registered projects that have a defined local path.

```bash
oc sync [--dry-run]
```

**Options:**

| Option | Description |
|--------|-------------|
| `--dry-run` | Checks freshness without deploying (equivalent to `oc deploy --check` on each project) |

**Examples:**

```bash
oc sync             # redeploy on all projects
oc sync --dry-run   # check without deploying
```

---

## `oc start`

Launches the default tool in a project's directory.

```bash
oc start [PROJECT_ID] [prompt]
         [--dev [--label <label>] [--assignee <user>]]
         [--onboard [--refresh]]
         [--parallel]
         [--worktree [<branch>]]
         [--agent <name>]
         [--provider <p>]
```

**Arguments:**

| Argument | Description |
|----------|-------------|
| `[PROJECT_ID]` | Project ID — interactive selection if absent |
| `[prompt]` | Startup prompt passed to the tool |

**Options:**

| Option | Description |
|--------|-------------|
| `--dev` | Development mode — loads open `ai-delegated` tickets into the startup prompt. Automatically performs a tracker sync `--pull-only` before launch. Runs in the project's main directory. |
| `--dev --label <label>` | Like `--dev`, but filters tickets with label `<label>` |
| `--dev --assignee <user>` | Like `--dev`, but filters tickets assigned to `<user>` |
| `--onboard` | Injects a project discovery prompt to onboard the agent on the codebase |
| `--onboard --refresh` | Resets the onboarding context before re-running discovery |
| `--parallel` | Launches an `orchestrator-dev` in an **isolated worktree** (`parallel/TIMESTAMP`) to process multiple `ai-delegated` tickets simultaneously in `auto` mode. Use when several independent tickets are ready. Requires `Worktree: enabled` in `projects.md`. |
| `--worktree [<branch>]` | Opens a **free session in an isolated worktree** on a named branch. If the branch is omitted, it is prompted interactively. Ideal for starting independent development in parallel with an ongoing session. Creates the worktree if absent, reuses it otherwise. |
| `--agent <name>` | Force the startup agent (e.g. `orchestrator`, `developer-fullstack`) |
| `--provider <p>` | Override the LLM provider for this session (e.g. `anthropic`, `openai`). Regenerates `opencode.json` if agents are already deployed. |

> **Mutual exclusivity:** `--dev`, `--parallel` and `--worktree` are mutually exclusive with each other, and all are incompatible with `--onboard`. `--label` and `--assignee` are mutually exclusive. `--refresh` requires `--onboard`.

> **Choosing between `--dev`, `--parallel` and `--worktree`:**
> - `--dev`: sequential session in the main repo, tickets processed one by one
> - `--parallel`: multi-ticket orchestrator in a dedicated worktree (auto mode, filesystem isolation)
> - `--worktree`: free session on an isolated branch, no mandatory Beads link — for independent parallel development

**Examples:**

```bash
oc start                                        # interactive project selection
oc start MY-APP                                 # launch tool in MY-APP
oc start MY-APP "explain the architecture"      # with startup prompt
oc start MY-APP --dev                           # load ai-delegated tickets
oc start MY-APP --dev --label ai-delegated      # filter by label
oc start MY-APP --dev --assignee alice          # filter by assignee
oc start MY-APP --onboard                       # project discovery prompt
oc start MY-APP --onboard --refresh             # re-discovery with context reset
oc start MY-APP --parallel                      # multi-ticket orchestrator in isolated worktree
oc start MY-APP --worktree feat/my-feature      # free session on isolated branch
oc start MY-APP --worktree                      # free session, branch name prompted interactively
oc start MY-APP --agent developer-fullstack     # force startup agent
oc start MY-APP --provider openai               # override LLM provider
```

**Launch display:**

```
◆  MY-APP
│  Path       /Users/alice/workspace/my-app
│  Target     opencode
│
│  → New to this project? Invoke the onboarder agent
│    "Onboard yourself onto this project"
│  → Or launch directly: ./oc.sh start --onboard MY-APP
│
└  Launching opencode…
```

> Warns in the context block if agents are not deployed (`◆` yellow) or if `.beads/` is absent.

---

## `oc audit`

Launches an AI audit on a project by invoking the `auditor` agent (and its specialised sub-agent if `--type` is specified).

```bash
oc audit [PROJECT_ID] [--type <type>]
```

**Arguments:**

| Argument | Description |
|----------|-------------|
| `[PROJECT_ID]` | Project ID — interactive selection if absent |

**Options:**

| Option | Values | Description |
|--------|--------|-------------|
| `--type <type>` | `security`, `accessibility`, `architecture`, `ecodesign`, `observability`, `performance`, `privacy` | Targets the audit on a specific domain. If absent: global audit via `auditor` |

**Behaviour:**

1. **Validation** — verifies the `--type` is among the 7 recognised domains (if provided)
2. **Project resolution** — normalises the ID and resolves the local path
3. **projects.md check** — if the project has a restrictive agent selection (not `all`), verifies that `auditor` (and `auditor-<type>` if specified) are included:
   - If missing → offers to add them + redeploy
   - If refused → displays physically deployed audit agents and offers a selection menu
4. **Physical deployment check** — if the agents folder is absent or files are missing, offers `oc deploy`
5. **Launch** — builds the bootstrap prompt and opens the tool with `--agent auditor` (or the selected agent)

**Examples:**

```bash
oc audit                          # interactive project selection, global audit
oc audit MY-APP                   # global audit on MY-APP
oc audit MY-APP --type security   # security audit only
oc audit MY-APP --type privacy    # GDPR/privacy audit only
```

**Injected prompt:**

```
Perform a complete audit of the project.

Project: MY-APP
Path: /Users/alice/workspace/my-app
Scope: security audit only.   ← present only if --type

Workflow:
1. Announce the audit scope and methodology
2. Explore relevant files according to the audit type
3. Identify and classify points of attention (🔴 critical, 🟠 important, 🟡 improvements)
4. Produce the structured audit report with prioritised recommendations
```

> For a complete multi-domain audit, invoke the `auditor` agent directly without `--type`.

---

## `oc review`

Launches an AI code review on a branch by invoking the `reviewer` agent with the branch name in the prompt — the reviewer fetches the diff itself via `git diff`.

```bash
oc review [PROJECT_ID] [--branch <branch>]
```

**Arguments:**

| Argument | Description |
|----------|-------------|
| `[PROJECT_ID]` | Project ID — interactive selection if absent |

**Options:**

| Option | Description |
|--------|-------------|
| `--branch <branch>` | Branch to review. If absent: uses the project's current git branch |

**Behaviour:**

1. **Branch resolution** — if `--branch` is not provided, detects the current branch via `git branch --show-current` in the project directory
2. **Git fetch** — runs `git fetch` to update remote refs; if it fails (no network, auth), prompts for confirmation before continuing
3. **Pull base branch** — runs `git pull --ff-only origin <base>` where `<base>` is read from `- Worktree base branch :` in `projects.md` (default: `main`); if it fails (diverged branch), prompts for confirmation
4. **projects.md check** — if the project has a restrictive agent selection (not `all`), verifies that `reviewer` is included:
   - If missing → offers to add it + redeploy
5. **Physical deployment check** — if the agents folder is absent or `reviewer.md` is missing, offers `oc deploy`
6. **Diff instruction** — injects the exact `git diff <base>...<branch>` command into the prompt; the agent executes it itself and analyses the result progressively, avoiding context window overflow on large branches
7. **Launch** — opens the tool with `--agent reviewer` and the prompt containing the diff instruction

**Examples:**

```bash
oc review                              # interactive project selection, current branch
oc review MY-APP                       # review current branch of MY-APP
oc review MY-APP --branch feat/login   # review branch feat/login
```

**Injected prompt:**

```
Perform a code review of branch `feat/login`.

Project: MY-APP
Path: /Users/alice/workspace/my-app
Branch reviewed: feat/login
Base branch:     main

→ Read CONVENTIONS.md at the project root before reviewing   ← if the file exists

To get the diff, run:
  git diff main...feat/login

Workflow:
1. If CONVENTIONS.md exists at the root → read it to apply real project conventions
2. Run `git diff main...feat/login` to get the changes
3. Analyse the diff according to the systematic checklist in the review-protocol skill
4. Produce the structured report by severity: Critical → Major → Minor → Suggestion → Positive points
```

> The `reviewer` agent does not modify any files — it only produces an analysis report.
> The agent fetches the diff itself via `git diff` — this avoids context window overflow on large branches.
> For an empty diff (branch up to date with the base branch), the agent detects and reports this.
> The base branch used for the diff is read from `- Worktree base branch :` in `projects.md` (default: `main`).

---

## `oc conventions`

Generates or updates the `CONVENTIONS.md` file at the root of a project by
invoking the `onboarder` agent in conventions mode.

```bash
oc conventions [PROJECT_ID] [--force]
```

**Arguments:**

| Argument | Description |
|----------|-------------|
| `[PROJECT_ID]` | Project ID — interactive selection if absent |

**Options:**

| Option | Description |
|--------|-------------|
| `--force` | Overwrites `CONVENTIONS.md` without asking for confirmation if it already exists |

**Behaviour:**

1. Resolves the project (interactive if `PROJECT_ID` absent)
2. If `CONVENTIONS.md` already exists in the project → displays the generation date and requests confirmation before overwriting (unless `--force`)
3. Injects the conventions bootstrap prompt and opens the tool with the `onboarder` agent
4. The agent explores the codebase, detects real conventions (9 categories) and generates `CONVENTIONS.md`
5. Adds `CONVENTIONS.md` to the project's `.git/info/exclude` if not already there (local exclusion, invisible to other devs)

**Examples:**

```bash
oc conventions                   # interactive project selection
oc conventions MY-APP            # generate CONVENTIONS.md for MY-APP
oc conventions MY-APP --force    # regenerate without confirmation
```

**Generated file:**

`CONVENTIONS.md` documents real conventions observed in the codebase:
formatting, naming, architecture, tests, Git, error handling, security,
performance, and project-specific conventions. This file is read by all developer
and quality agents at the start of a session to code respecting the project's
conventions rather than generic standards.

> `CONVENTIONS.md` is excluded via `.git/info/exclude` — it stays local to the workstation, invisible to other devs.
> To regenerate it after a project evolution: `oc conventions MY-APP --force`.

---

## `oc debug`

Launches a bug debugging session on a project by invoking the `debugger` agent.

```bash
oc debug [PROJECT_ID]
```

**Arguments:**

| Argument | Description |
|----------|-------------|
| `[PROJECT_ID]` | Project ID — interactive selection if absent |

**Behaviour:**

1. **Project resolution** — normalises the ID and resolves the local path
2. **projects.md check** — if the project has a restrictive agent selection (not `all`), verifies that `debugger` is included:
   - If missing → offers to add it + redeploy
3. **Physical deployment check** — if the agents folder is absent or `debugger.md` is missing, offers `oc deploy`
4. **Launch** — builds the bootstrap prompt and opens the tool with `--agent debugger`

**Examples:**

```bash
oc debug               # interactive project selection
oc debug MY-APP        # launch the debugger on MY-APP
```

**Launch display:**

```
◆  oc debug  MY-APP
│  Path          /Users/alice/workspace/my-app
│  Target        opencode
│  Agent         debugger
│
└  Launching opencode…
```

> The `debugger` agent analyses the described bug, explores the codebase and produces a structured diagnostic with hypotheses and recommended fixes.

---

## `oc init`

Registers a project in the hub. Guides the user through **6 numbered steps** and displays a coloured summary at the end.

```bash
oc init [PROJECT_ID] [path]
```

**Arguments:**

| Argument | Description |
|----------|-------------|
| `[PROJECT_ID]` | Unique project identifier (letters, digits, `-`, `_`) |
| `[path]` | Absolute or `~`-expanded path to the project directory |

**Interactive wizard:**

| Step | Content |
|------|---------|
| 1 — Project information | PROJECT_ID, path, directory verification/creation, name, stack, labels, tracker |
| 2 — Beads & tracker | `bd init`, Git upstream, tracker configuration |
| 3 — Agents | Agent selection and native OpenCode agents to disable |
| 4 — MCP Services | Selection of MCP integrations to enable for this project (`none` by default) |
| 5 — LLM provider | Project-specific provider configuration (overrides hub) |
| 6 — Deployment | Immediate deployment proposal |

> Directory creation happens at the **end of step 1** — Beads is thus guaranteed accessible from step 2.

> **Step 4 — MCP Services:** By default, no MCP server is deployed (opt-in). Answer `Y` to open a multi-select picker listing available services from `config/services.json`. The selection is persisted as `- MCP :` in `projects/projects.md` and applied on every `oc deploy`. To change the selection later, edit `projects.md` directly or re-run `oc init`.

**Wizard display:**

```
◆  Project initialisation
│
│
◇  Step 1/6 — Project information
│
│  PROJECT_ID (e.g. MY-APP):
│  ...
│
◇  Step 2/6 — Beads & tracker
│
│  ...
│
◇  Step 4/6 — MCP Services
│
│  Enable MCP integrations for this project? [y/N]:
```

**Final summary:**

```
┌─ MY-APP initialised ──────────────────────────────┐
│  Path         /Users/alice/workspace/my-app        │
│  Name         My Application                       │
│  Stack        Vue 3 + Laravel                      │
│  Tracker      jira                                 │
│  Beads        ◆ initialised                        │
│  MCP          figma-mcp                            │
│                                                    │
│  Next → ./oc.sh start MY-APP                       │
└────────────────────────────────────────────────────┘

└  Project MY-APP ready — ./oc.sh start MY-APP
```

**Examples:**

```bash
oc init                              # full interactive mode
oc init MY-APP ~/workspace/my-app    # pre-fills ID and path (remaining questions interactive)
```

---

## `oc status`

Displays a dashboard of the state of all registered projects.

```bash
oc status [--short]
```

**Options:**

| Option | Description |
|--------|-------------|
| `--short` / `-s` | Compact view: id / path / status table (replaces the former `oc list`) |

**Without option — detailed view.** For each project, checks:
- Local path accessible
- Beads initialised (`.beads/`)
- API key configured (provider + model)
- Tracker configured
- Agents deployed for the default target

**Example detailed output:**

```
  MY-APP
    ·  Path: /Users/alice/workspace/my-app
    ✔  Beads initialised
    ✔  API configured (anthropic / claude-sonnet-4-5)
    ·  Tracker: none
    ✔  Agents deployed (opencode): 12 file(s)
```

**Examples:**

```bash
oc status          # detailed view of all projects
oc status --short  # compact list (id, path, status)
```

---

## `oc project`

Operations on registered projects: renaming and moving.

```bash
oc project rename <OLD_ID> <NEW_ID>
oc project move   <PROJECT_ID> <new_path>
```

### `oc project rename`

Renames a project in **all registry files** (`projects.md`, `paths.local.md`, `api-keys.local.md`).

```bash
oc project rename MY-APP MY-APP-V2
```

- Requests confirmation before any modification
- Updates all three files atomically
- Reminds you to redeploy agents after renaming if necessary

### `oc project move`

Changes a project's local path in `paths.local.md`.

```bash
oc project move MY-APP ~/workspace/my-app-new
```

- Accepts `~` paths and relative paths (resolved from `$PWD`)
- Warns if the destination folder does not exist yet (can continue anyway)

**Examples:**

```bash
oc project rename OLD-NAME NEW-NAME           # rename across all registries
oc project move MY-APP ~/workspace/my-app     # update the local path
```

---

## `oc remove`

Removes a project from the registry (with confirmation).

```bash
oc remove <PROJECT_ID> [--clean]
```

**Options:**

| Option | Description |
|--------|-------------|
| `--clean` | Also removes deployed agent files in the project directory (`.opencode/agents/`, `opencode.json`) |

**Examples:**

```bash
oc remove MY-APP           # remove from registry only
oc remove MY-APP --clean   # remove from registry + clean deployed files
```

> Requests confirmation in both cases. Also removes the entry from `paths.local.md` and `api-keys.local.md`.

---

## `oc update`

Updates **installed tools**: opencode, Beads (`bd`) and registered external skills.

```bash
oc update
```

> Does not update the hub scripts themselves. For that, use `oc upgrade`.

---

## `oc upgrade`

Updates the **hub sources themselves** (`git pull` on the local repo). With an optional version argument, checks out a specific release tag.

```bash
oc upgrade              # pull latest main
oc upgrade v1.1.0       # checkout tag v1.1.0
```

After a successful update, offers to re-run `oc sync` to redeploy agents on all registered projects.

> **Summary of the distinction:**
> - `oc update` → updates installed tools (opencode, bd, external skills)
> - `oc upgrade` → updates hub scripts and agents via git

---

## `oc version`

Displays the hub version (read from `config/hub.json`).

```bash
oc version
```

---

## `oc config`

Manages API keys and AI models per project, as well as hub-level LLM provider configuration. Project data is stored in `projects/api-keys.local.md` (not versioned); hub configuration in `config/hub.json`.

```bash
oc config <sub-command> [options]
```

| Sub-command | Description |
|-------------|-------------|
| `set [PROJECT_ID] [options]` | Configure the API key, model and provider (project or hub) |
| `get <PROJECT_ID>` | Display a project's configuration (masked key) |
| `list [--providers]` | List all registered configurations, or all providers in the catalogue |
| `unset <PROJECT_ID>` | Delete a project's configuration (with confirmation) |
| `init-providers [--force]` | Initialise switcher configuration files in `config/providers/` |

**`oc config set` options:**

| Option | Description |
|--------|-------------|
| `--model <model>` | AI model (default: `claude-sonnet-4-5`) |
| `--provider <provider>` | LLM provider — in interactive mode, a numbered menu is shown from the `providers.json` catalogue |
| `--api-key <key>` | API key (masked input in interactive mode) |
| `--base-url <url>` | Base URL (OpenAI-compatible providers) |
| `--family-model <model>` | AI model for `family`-type agents |
| `--agent-model <model>` | AI model for agents |

**`oc config set` behaviour depending on arguments:**

- **`oc config set <PROJECT_ID>`** — interactive, configures the provider and key for that project
- **`oc config set`** (no `PROJECT_ID`) — interactive **hub** provider setup wizard (replaces the former `oc provider set-default`)
- **`oc config set --provider anthropic --api-key sk-...`** — non-interactive hub provider configuration
- **`oc config set --provider bedrock`** — hub provider without API key (e.g. Bedrock with AWS auth)
- **`oc config set --model claude-opus-4`** — update hub default model only
- **`oc config set --provider p --api-key k --model m`** — configure provider, key and hub model in one command

> After a `set` with `PROJECT_ID`, offers to re-deploy agents in the project if the path is known.

**`oc config list --providers`:**

Lists all providers in the catalogue with their hub configuration status.

**`oc config init-providers [--force]`:**

Creates the `config/providers/` directory and generates the JSON files used by `ocp`: `mammouth.json`, `copilot.json`, `openrouter.json`, `ollama.json`, `bedrock.json`. Also creates `config/providers/.gitignore` to protect API keys. Without `--force`, existing files are not overwritten.

**Examples:**

```bash
oc config set                                         # interactive hub wizard (default provider)
oc config set --provider anthropic --api-key sk-ant-... # configure hub provider
oc config set --provider bedrock                      # hub provider without API key
oc config set --model claude-opus-4                   # update hub default model only
oc config set MY-APP                                  # interactive mode for MY-APP
oc config set MY-APP --model claude-opus-4-5 --provider anthropic --api-key sk-ant-...
oc config set MY-APP --provider litellm --api-key sk-... --base-url https://api.example.com/v1
oc config get MY-APP                                  # display config (masked key)
oc config list                                        # list all project entries
oc config list --providers                            # list all providers in the catalogue
oc config unset MY-APP                                # delete (with confirmation)
oc config init-providers                              # initialise ocp switcher files
oc config init-providers --force                      # reinitialise all switcher files
```

---

## `oc agent`

Manages the hub's canonical agents.

```bash
oc agent <sub-command>
```

| Sub-command | Description |
|-------------|-------------|
| `list` | List all agents with their id, label and skills |
| `create` | Create a new agent (interactive workflow) |
| `edit <id>` | Modify skills and metadata of an existing agent |
| `info <id>` | Display the full detail of an agent (frontmatter + body) |
| `select <PROJECT_ID>` | Choose which agents to deploy for a project |
| `mode <PROJECT_ID>` | Display / override `primary`/`subagent` modes per project |
| `validate [agent-id]` | Validate agent consistency (required fields, existing skills, id uniqueness) |
| `deploy <agent-id> [PROJECT_ID]` | Deploy **a single agent** |
| `discover <PROJECT_ID>` | Discover existing project agents and propose to integrate them |

### `oc agent create` — interactive workflow

1. **Identifier** — unique slug (e.g. `reviewer`)
2. **Label** — short name displayed in the tool (e.g. `CodeReviewer`)
3. **Description** — short phrase describing the role
4. **Skills** — interactive selector ↑↓/space with description panel
5. **Body** — if `opencode` is available, offer to auto-generate via `opencode run`
6. **Preview** — display of the complete `.md` file before writing
7. **Confirmation** — `Y/n` to create the file

### `oc agent validate`

```bash
oc agent validate             # validate all canonical agents
oc agent validate <agent-id>  # validate only the specified agent
```

Verifies for each agent:
- Required fields present (`id`, `label`, `description`, `skills`)
- `id` uniqueness across all agents
- Valid `mode` (`primary` | `subagent` | `all`) if present
- All referenced skills exist (local or external)

Returns exit code 1 if at least one error is detected.

### `oc agent deploy`

```bash
oc agent deploy <agent-id>                # deploy to hub
oc agent deploy <agent-id> <PROJECT_ID>   # deploy to project
```

Deploys **a single agent** without redeploying everything. Useful after modifying an agent or a skill.

- Applies the project's language setting (if configured)

**Examples:**

```bash
oc agent deploy planner            # deploy planner in the hub
oc agent deploy planner MY-APP     # deploy planner in MY-APP only
```

### `oc agent discover`

```bash
oc agent discover <PROJECT_ID>
```

Scans `.opencode/agents/` in the project, detects agents **not generated by the hub**, resolves their semantic similarity with hub agents, and interactively proposes to integrate them.

**Two integration modes:**

| Mode | Behavior | Example |
|------|----------|---------|
| `substitute` | The project agent **replaces** the corresponding hub agent during deploy | `my-planner.md` replaces the hub's `planner` |
| `complement` | The project agent **is added** alongside all hub agents | `custom-agent.md` coexists with all hub agents |

**Similarity resolution (3 levels):**
1. Exact ID match (e.g. `planner` → `planner`)
2. Lookup in `config/agent-aliases.json` (e.g. `plan` → `planner`, `frontend` → `developer-frontend`)
3. Advanced normalization with common prefix stripping (`dev-`, `my-`, `agent-`)

**Persistence:** choices are written to the `External agents` field in `projects.md`:
```markdown
- External agents : .opencode/agents/my-planner.md:substitute:planner|.opencode/agents/custom.md:complement
```

**At deploy time:** `oc deploy PROJECT_ID` automatically triggers discovery if new non-hub agents are found (disabled in non-interactive mode `OC_NON_INTERACTIVE=1`).

**Examples:**

```bash
oc agent discover MY-APP           # interactive discovery
oc deploy MY-APP                   # auto-discovery + deploy
```

> The interactive selector (agents) uses the alternate screen (`smcup`/`rmcup`) — the parent terminal content is fully preserved on close.
> `oc agent keytest` is available to diagnose terminals where navigation doesn't work (not in help, type `oc agent keytest`).

---

## `oc skills`

Manages external skills downloaded via context7.

```bash
oc skills <sub-command>
```

| Sub-command | Description |
|-------------|-------------|
| `search <query>` | Search for available skills |
| `add /owner/repo [name]` | Add an external skill |
| `list` | List all skills (local + external) |
| `update [name]` | Update an external skill (or all if absent) |
| `info /owner/repo` | Preview available skills in a repository |
| `used-by <skill>` | List agents that use this skill |
| `sync` | Re-download all external skills (useful after clone) |
| `remove <name>` | Remove an external skill |
| `validate [name]` | Validate skill consistency (frontmatter, sources) |

### `oc skills validate`

```bash
oc skills validate          # validate all skills (local + external)
oc skills validate <name>   # validate only the specified skill
```

Verifies for each skill `.md` file:
- Required frontmatter fields present (`name`, `description`)
- Consistency between the `name` field and the filename
- For external skills: presence of their source in `.sources.json`

Returns exit code 1 if at least one error is detected.

---

## `oc beads`

Manages the Beads (`bd`) integration in registered projects.

```bash
oc beads <sub-command>
```

| Sub-command | Description |
|-------------|-------------|
| `status [PROJECT_ID]` | Check Beads on all projects (or just one) |
| `init <PROJECT_ID>` | Initialise `.beads/` in the project |
| `list <PROJECT_ID>` | List open tickets in the project |
| `show <PROJECT_ID> <TICKET_ID>` | Show the full detail of a ticket |
| `create <PROJECT_ID> [title] [--label <l>] [--type <t>] [--desc <d>]` | Create a ticket in the project |
| `open <PROJECT_ID>` | Display the path to use `bd` manually |
| `sync <PROJECT_ID> [options]` | Synchronise with an external tracker |
| `tracker status <PROJECT_ID>` | Display the tracker connection status |
| `tracker setup <PROJECT_ID>` | Configure the tracker (interactive) |
| `tracker switch <PROJECT_ID>` | Switch provider (jira ↔ gitlab ↔ none) |
| `tracker set-sync-mode <PROJECT_ID> [mode]` | Set default sync direction for the project |

### `oc beads create`

```bash
oc beads create <PROJECT_ID> [title] [--label <label>] [--type <type>] [--desc <description>]
```

| Argument / Option | Description |
|-------------------|-------------|
| `<PROJECT_ID>` | Project in which to create the ticket |
| `[title]` | Ticket title — interactive mode if absent |
| `--label <label>` | Ticket label |
| `--type <type>` | Ticket type (`feature`, `fix`, `chore`, …) |
| `--desc <description>` | Long description |

**Examples:**

```bash
oc beads create MY-APP                                              # interactive mode
oc beads create MY-APP "Add role management"                        # direct title
oc beads create MY-APP "Fix race condition" --type fix --label bug  # with flags
```

**`oc beads sync` options:**

| Option | Description |
|--------|-------------|
| `--pull-only` | Import only from the tracker (overrides the project `Sync mode`) |
| `--push-only` | Export only to the tracker (overrides the project `Sync mode`) |
| `--dry-run` | Simulate without modifying |

> The default direction of `oc beads sync` is controlled by the `Sync mode` field in `projects.md`
> (set with `oc beads tracker set-sync-mode <PROJECT_ID>`). Default value: `bidirectional`.
> A CLI flag always takes precedence over the configured mode.

> `oc start` automatically warns if `.beads/` is not present in the project.

### `oc beads board` — Terminal kanban board

Displays a real-time kanban board in the terminal with 4 columns: **OPEN**, **IN PROGRESS**, **REVIEW**, **BLOCKED**.
No external dependency — pure shell + `bd`.

```bash
oc beads board [PROJECT_ID] [--watch] [--interval <sec>]
```

| Option | Description |
|--------|-------------|
| `[PROJECT_ID]` | Project to display (auto-discovered from current directory if absent) |
| `--watch` | Auto-refresh mode (Ctrl+C to quit) |
| `--interval <sec>` | Refresh interval in seconds (default: 5) |

**Examples:**

```bash
oc beads board MY-APP              # display board once
oc beads board MY-APP --watch      # live refresh every 5s
oc beads board MY-APP --watch --interval 10   # refresh every 10s
```

> The board adapts to your terminal width. Ticket titles are truncated to fit.
> Priority badges: **P0** in red, P1 in yellow, P2/P3 dimmed.
> Column borders are colour-coded: dim (open), blue (in progress), yellow (review), red (blocked).

---

## `oc service`

Manage external services and MCP integrations. See the [full reference](services.en.md).

```bash
oc service [setup|status|list|remove] [service-name]
```

**Examples:**

```bash
oc service list                     # list available services
oc service setup figma              # configure Figma (interactive wizard)
oc service status                   # status of all services
oc service remove gitlab            # remove GitLab config

# Short aliases
oc figma setup                      # = oc service setup figma
oc gitlab status                    # = oc service status gitlab
```

---

## `oc metrics`

Displays velocity metrics, costs and usage data for the OpenCode hub.

```bash
oc metrics                  # last 7 days (default)
oc metrics --period today   # today only
oc metrics --period week    # last 7 days
oc metrics --period month   # last 30 days
```

**Data sources:**

| Source | Data collected | Prerequisite |
|--------|---------------|-------------|
| `~/.local/share/opencode/opencode.db` | Sessions, costs, tokens, models, agents | `sqlite3` |
| `bd list` per project | Tickets by status | `bd` (optional) |
| `.opencode/metrics.jsonl` | Workflow velocity (legacy) | — |
| `~/.claude/context-mode/sessions/` | Tokens saved by context-mode | context-mode (optional) |
| `rtk gain` | Tokens saved by RTK | RTK 0.42+ (optional) |

**Displayed sections (in order):**

1. **Overview**: active sessions (including created), exact cost for the period (based on executed steps — includes multi-day sessions), input/output tokens, cache write/read, cache hit rate + estimated savings
   - **Plugin savings** *(if context-mode or RTK is installed)*: tokens saved, dollars saved and context reduction. The period matches the `--period` filter for context-mode; RTK always shows global statistics.
2. **Total cost**: lifetime (all sessions) + breakdown Today / 7 days / 30 days by steps. The line matching the active `--period` is highlighted. Always shown with all 3 fixed windows regardless of the chosen period.
3. **Cost**: sub-sections by project, by agent, by model (merged view)
4. **Activity**: session breakdown by usage category (code, exploration, planning, review, debug, conversation) with cost and percentage
5. **Recent sessions**: last 5 sessions with title, agent, cost and date
6. **Tickets per project** *(if `bd` available)*: status counters for each Beads project
7. **Workflow velocity** *(if `metrics.jsonl` present)*: completed tickets, average time, review cycles

> **Note on exact cost**: cost values are calculated from steps (`part.step-finish`) by execution date, not by session creation date. A session started yesterday but still active today contributes to the current day's cost (`--period today`).

**Options:**

| Option | Description |
|--------|-------------|
| `--period today` | Current day only |
| `--period week` | Last 7 days (default) |
| `--period month` | Last 30 days |

**If `sqlite3` is absent:** Help message displayed, exit 0 (non-blocking). Tickets and Velocity sections remain available.

**Prerequisite:** `sqlite3` (native on macOS — `sudo apt-get install sqlite3` on Linux)

---

## `oc dashboard`

Displays a synthetic view of the OpenCode hub — designed for a quick daily check (10 seconds). Financial information appears first.

```bash
oc dashboard
```

**Data sources:**

| Source | Data collected | Prerequisite |
|--------|---------------|-------------|
| `~/.local/share/opencode/opencode.db` | Session budget, recent sessions | `sqlite3` |
| `bd list` per project | Active, blocked, completed tickets | `bd` (optional) |
| `.opencode/session-state.json` | Active orchestrator session (legacy) | `jq` |
| `~/.claude/context-mode/sessions/` | Tokens saved by context-mode (lifetime) | context-mode (optional) |
| `rtk gain` | Tokens saved by RTK (global) | RTK 0.42+ (optional) |

**Displayed sections (in order):**

1. **Budget**: exact cost by steps (today since calendar midnight / this week / this month), active sessions (including created), inline cache hit rate, and **Total lifetime** line. Costs are based on executed steps — a session opened yesterday but still active contributes to today's cost.
2. **AI savings** *(if context-mode or RTK is installed)*: tokens saved (lifetime), dollars saved, context reduction. Silently absent if no plugin is installed.
3. **Active session** *(if orchestrator is running via `oc start`)*: agent, current ticket, action, start time
4. **Projects**: for each Beads project — current ticket and status counters (✅ / 🔄 / ⏳ / 🚫)
5. **Recent sessions**: last 5 sessions (title, agent, cost, date)

**If `sqlite3` is absent:** Budget and Recent sessions sections display a help message. Projects (bd) and AI savings sections remain available.

**If `bd` is absent:** Projects section displays a help message. Other sections remain available.

**Prerequisites:** `sqlite3` for cost/session sections; `bd` (optional) for tickets

> For detailed metrics over a custom period, use `oc metrics --period month`.

---

## `oc optimize`

Scans token waste patterns and produces a graded report.

```bash
oc optimize                      # last 30 days (default)
oc optimize --period week        # last 7 days
oc optimize --period today       # today only
oc optimize --project T-SRU      # filter to one project
```

**9 deterministic analyses (no LLM calls):**

| Analysis | Level | Signal |
|----------|-------|--------|
| Unused MCP servers | Critical | Deployed server with 0 calls in period |
| Sessions without edits | Critical/Warning | Costly sessions with no edit/write tool calls |
| Low Read/Edit ratio | Critical/Warning | < 1.0 → critical, < 2.0 → warning (ideal ≥ 2.0) |
| High error rate | Critical/Warning | > 25% → critical, > 10% → warning |
| Repeatedly read files | Warning | Same file read 5+ times in one session |
| Heavy delegation | Info | > 40% of tool calls = `task` |
| Unused Bucket B skills | Warning | No `skill` loads in period |
| Pure conversation sessions | Info | Sessions with no tool calls |
| Low cache hit rate | Warning | < 30% — input tokens reloaded unnecessarily |

**Grading:**

| Grade | Score (criticals × 3 + warnings) |
|-------|----------------------------------|
| A | 0 |
| B | 1–2 |
| C | 3–5 |
| D | 6–9 |
| E | 10–14 |
| F | 15+ |

Each finding includes a description and a ready-to-paste fix suggestion.

**Prerequisite:** `sqlite3`

**Options:**

| Option | Description |
|--------|-------------|
| `--period today\|week\|month` | Analysis period (default: 30 days) |
| `--project PROJECT_ID` | Filter to a single project |

---

## `oc yield`

Correlates OpenCode sessions with git commits to measure real productivity.

```bash
oc yield                      # last 7 days (default)
oc yield --period today       # today only
oc yield --period month       # last 30 days
oc yield --project T-SRU      # filter to one project
```

**Session classification:**

| Category | Definition | Cost display |
|----------|-----------|-------------|
| **Productive** | At least one git commit within 24h after the session | Green |
| **Abandoned** | No commit found in the 24h window | Yellow |
| **Reverted** | Commit found but message starts with `Revert` | Red |

**Worktree resolution:** sessions in git worktrees (e.g. `~/workspace/t-sru-2/`) are automatically linked to the main repository via `git rev-parse --show-toplevel`.

**Prerequisites:** `sqlite3` + `git`

**Options:**

| Option | Description |
|--------|-------------|
| `--period today\|week\|month` | Analysis period (default: 7 days) |
| `--project PROJECT_ID` | Filter to a single project |

> Costly abandoned sessions → use `oc optimize` to identify root causes.
