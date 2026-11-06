> [Lire en français](migration-v5.fr.md)

# Migrating to oh v5

oh v5 starts every session from a **workflow** (`oh run <workflow>`), in a **session bundle** built outside the project, on an **opencode V2** server. This guide lists what changes for a v4 user (4.2.0). The same breaking changes, grouped by theme with the action to take, are in [MIGRATION.md](../../MIGRATION.md) (in French). **Coming from 5.0?** Go to [From 5.0 to 5.1](#from-50-to-51). **Coming from 5.1?** Go to [From 5.1 to 5.2](#from-51-to-52).

## Step-by-step update

1. **Back up**: `oh export -o ~/oh-backup-4.x.tar.gz` (database, `hub.toml`, encrypted secrets), and keep the 4.2.0 binary (§19).
2. **Install opencode V2** (§1), then **update oh**: `brew upgrade openhub` or `oh upgrade oh`.
3. **Run `oh doctor` first.** The first launch migrates the database (27 → 49, with a copy, §19) and the former `[workflow]` overrides (§9); there is no migration command to run: it runs at the start of every command **except `oh init` and `oh purge`**. Do not edit `hub.toml` by hand before this first launch.
4. **Fix what `oh doctor` reports**: refused `hub.toml` (§13), former keys, former deployments (§8), `~/.oh/skills` (§10), opencode version. `oh doctor --fix` repairs after a summary and a confirmation (`--yes` without a terminal).
5. **Remove the former deployments**: `oh migrate deploy-cleanup` (§8).
6. **Allow the environment variables** your agents need: `oh config set execution.env_allow "SSH_AUTH_SOCK, MYCO_*"` (§14).
7. **In a team**: `oh mcp trust <service>` for each imposed URL (§20), and read about team mode (§17).
8. **Update scripts and CI**: `oh deploy`/`oh sync`, `oh start` → `oh run`, `--yes` (§12), exit code 2 (§11).
9. **Check your workflows**: `oh workflow validate --all` (§15).

## 1. opencode V2 required

opencode V1 is no longer supported, with no fallback. oh v5 requires an opencode version between **2.0.0** and **2.99.99**; the **recommended** version is **2.2.5** (the one of the CI). Outside that range, `oh run` refuses to start and says why:

```text
opencode 1.18.29 is not supported: oh v5 requires version 2.0.0 or later.
Install a supported release of opencode, then run oh doctor. Guide: docs/guides/migration-v5.en.md
```

Without opencode: "opencode was not found: oh v5 requires a supported release of it".

1. Install opencode V2 with its own tooling (`brew install opencode`, or https://opencode.ai).
2. Check with `oh doctor`: the **opencode** line must be green (✓).

The fingerprint of the opencode Linux packages is pinned in oh for versions **2.0.20**, **2.0.25** and **2.2.5**. Another version of the range works, but `oh doctor` warns about it: container images would download it without a verified fingerprint, and `execution.strict_isolation = true` refuses it ([SECURITY.md](../../SECURITY.md#supply-chain)).

`oh upgrade opencode`, the managed install in `~/.oh/bin` and the `[opencode] version`, `channel`, `auto_update`, `install_dir` keys of `hub.toml` are removed (ignored if they remain). `[llm] default_provider` stays.

## 2. Former commands: replaced by `oh run`

These commands were deprecated aliases during 5.0; **5.1 removed them** ([From 5.0 to 5.1](#from-50-to-51)). The table gives the command to use:

| Former command | v5 equivalent |
|---|---|
| `oh start` | `oh run feature` (alias of `spec`, or of `implement-spec` with `--tickets`, §15) |
| `oh start --agent <id>` | `oh run libre --agent <id>` |
| `oh start --dev [-t <id>]` | `oh run ticket --tickets <id>` |
| `oh start --onboard [--refresh]` | `oh run onboarding [-i refresh=true]` |
| `oh start --parallel --tickets a,b` | `oh run ticket --tickets a,b` |
| `oh start --sweep <goal>` | `oh run sweep -i goal=<goal>` |
| `oh start --resume <id>` | `oh session attach <id> --attach suspend` |
| `oh audit`, `oh review`, `oh debug` | `oh run audit`, `oh run review`, `oh run debug` |
| `oh review feedback <mr>` | `oh run review-feedback -i mr=<mr>` (the workflow reads the GitLab discussions itself); `oh review --publish`: `oh mr create` |
| `oh takeover-brief enrich` | `oh run brief-enrich --headless` |
| `oh service` (`setup`, `remove`) | `oh mcp status`, `oh mcp setup`, `oh mcp disable` |

No longer working (no equivalent): `--max-sessions`, `--priority`, `--sweep-branch-prefix`.

- The monitor and merge view of the parallel mode: follow the sessions in the **Sessions** view or `oh session list`.

## 3. Free session: the `libre` workflow

The former launch "by agent" becomes the shipped **`libre`** workflow: the agent of your choice (`orchestrator` by default) and the agents it may call, without checkpoints, in a closed world.

```bash
oh run libre --agent debugger -m "test X fails since yesterday"
```

In the TUI: the `libre` command (former name `coder`, still accepted), or opening a session from the Worktrees view. `free` is an alias of `libre`.

## 4. Plugins

`oh plugin` and the Plugins view (global opencode V1 plugins, RTK included) are removed. Plugins are declared per workflow (`plugins:`); see [Shipped workflows](../reference/workflows.en.md#plugins-and-code-mode).

## 5. Metrics

`oh metrics`, the dashboard and the Metrics view now read the oh session registry (`~/.oh/oh.db`), fed by the daemon for every v5 session. Sessions started outside oh, or before v5, are no longer counted.

## 6. No more deployment into projects

`oh deploy` and `oh sync` are removed: every session starts from a bundle built at launch, outside the project (`oh bundle show <workflow>` to see it). Both commands are hidden from the help and only print a migration message, with **exit code 2**. Also removed:

- the **Agents** and **Deployment** steps of the init wizard, `oh project add` and `oh project configure` (the bundle holds the agents of the workflow; the per-project agent selection, column `projects.agents`, is dropped by migrations v36 and v37);
- the Deployment section of the project mode, the `deploy`/`sync` omnibar commands and the drift toast;
- `[deploy] disable_native_agents` (the closed world of the bundle always hides the native agents); `[deploy] instruction_files` stays;
- the `.opencode` links of worktrees (and the `s` resync of the Worktrees view);
- the former Workflow view (replaced by the workflow catalogue and editor).

The `team` MCP server reads the team of the session from its environment (`OH_TEAM_ID`) instead of `.opencode/team.json`.

## 7. Brick catalogue

The former project "Agents" view (selection of the deployed agents) becomes the read-only **Brick catalogue**: agents and skills, origin (hub or team catalogue), estimated cost, loaded skills, dependencies (`requires`) and the workflows that ship them. Access: Project config › "Bricks", or the `bricks` omnibar command. To change the agents of a session, edit (or extend) its workflow.

## 8. Clean the former deployments: `oh migrate deploy-cleanup`

The files written by `oh deploy` stay in the projects until they are removed. After the update, oh offers it **once**: a message when a command starts (`oh: former deployments found in N project(s)…`) or the "Cleanup of the former deployments" screen in the TUI (Clean / Show the diff / Later; `cleanup` omnibar command to open it again). `oh doctor` reports them while some remain.

```bash
oh migrate deploy-cleanup --dry-run   # what would be removed, with the diff of opencode.json
oh migrate deploy-cleanup             # summary then confirmation
oh migrate deploy-cleanup -p web --yes
```

For every registered project, removed: `.opencode/agents`, `.opencode/skills`, `.opencode/.deploy-state`, `.opencode/context-manifest.json`, `.opencode/team.json` (the `.opencode` folder too when empty; other files, such as `package.json`, stay).

In `opencode.json`, only the keys **written by oh and unchanged since the last deployment** are removed: `agent.<id>` blocks of the hub agents and of the disabled native agents, `mcp.<name>` servers started by `oh mcp serve`, `context-mode` in `plugin`, instruction files added by oh, `compaction`, `subagent_depth`, `enabled_providers`, `model`, the block of the configured provider, `permission.websearch`/`webfetch`. The reference is the copy of `opencode.json` kept in `.deploy-state` (`config_snapshot`): a key you changed since is **kept** (and reported), your own keys are never touched, the order of the file is kept. Without `.deploy-state`, `opencode.json` is not changed (check it by hand). An `opencode.json` that only held oh keys is removed.

## 9. Former workflow overrides (migration v38)

The overrides of the former single workflow (`hub.toml [workflow]`, `[workflow]` of the team `config.toml`, project workflow configuration) are migrated **automatically** at startup to `implement-spec` workflows of the team-state (team, project; a solo space is created for a project without team). Nothing is lost: the original configuration is archived next to them (`workflows/migrated/…`, `~/.oh/migrated/` for `hub.toml`). An unreadable project configuration, or one in an unknown format, is not migrated: it is archived in `~/.oh/migrated/project-<id>-workflow-config.json`, with a message at startup. Details: [Team workflows › Migration of the former overrides](team-workflows.en.md). There is no dedicated command: the migration runs at the start of every command, except `oh init` and `oh purge`.

## 10. Community skills and `oh skill budget` disconnected

The former community skills registry (`oh skill add`, `list`, `remove`, `search`) and `oh skill budget` are **removed** ([ADR-051](../architecture/adr/051-community-skills-disconnection.en.md)); `oh skill check` stays.

- `~/.oh/skills` is no longer read: these skills are no longer shipped in sessions, and a workflow listing one in `skills.extra` becomes invalid ("unknown skill").
- Nothing is deleted: `oh doctor` lists the packages left in `~/.oh/skills`. To keep a skill, copy its `SKILL.md` into the team catalogue (`catalog/skills/<category>/<name>.md` of the team-state, or of your solo space `oh team init --solo`), then list it by its reference (`<category>/<name>`) in `skills.extra`. The `~/.oh/skills` folder can then be deleted.
- Budget of a session: `oh bundle show <workflow> --budget`.

## 11. Commands, options and exit codes

**Removed commands**

- `oh deploy` and `oh sync`: see §6 (hidden, exit code 2).
- `oh start`, `oh audit`, `oh review`, `oh debug`, `oh review feedback`, `oh takeover-brief enrich`: removed in 5.1 (§2); so is **`oh service`** (`setup`, `remove`): use `oh mcp status`, `oh mcp setup`, `oh mcp disable`. The name answers with its replacement, exit code 2.
- `oh plugin` (§4), `oh skill add|list|remove|search|budget` (§10) and `oh upgrade opencode` (§1) no longer exist: "unknown command", exit code 2.

**Options**

- **One letter, one meaning**: `-p` is always `--project`, `-m` always `--message`, `-y` always `--yes`… ([table of short options](../reference/cli.en.md#short-options)). The short forms deprecated in 5.0 (`oh serve -p`, `oh project configure -m`, `oh metrics -d`, `oh session attach --how` and the value `here`…) were **removed in 5.1**: use `--port`, `--model`, `--period`, `--attach` (value `suspend`).
- **Removed without an alias** (exit code 2): `oh config model … -j` (use `-p`) and `oh purge --include-opencode` (use `--include-tool-data`).
- `--force` / `-f` no longer skips a confirmation: use `--yes` / `-y`. On `oh purge`, `oh project remove` and `oh teams remove`, `--force` was removed in 5.1. `--force` stays where it overrides a safety: `oh daemon stop`, `oh remote setup`, `oh worktree remove`, `oh worktree cleanup` (the latter also deletes modified worktrees).
- `oh status --json`: `tool` and `tool_version` replace `opencode_version`.
- **Implicit project**: outside the folder of a project, oh no longer silently takes the only registered project. A command that writes asks for confirmation and refuses without a terminal ("run the command from the project directory or use -p <project>"); a command that reads announces the project it uses.
- **Help**: `oh --help` gives one line per top-level command (about 70 lines instead of 573), with an "Advanced" section (`bundle`, `fanout`, `migrate`, `takeover-brief`, `completion`). `oh help --all` (or `oh --help --all`) gives the full listing of before; `oh <command> --help` is unchanged. A script that parsed `oh --help` must read `oh help --all`. `oh --all` alone prints the full help instead of opening the TUI.
- **`oh init`**: three screens in solo mode (welcome with language and setup type, provider, optional first project) instead of eight. The MCP integrations are no longer asked: run `oh mcp setup` afterwards.
- `oh teams` (teams of this machine) and `oh team` (the active team) are two separate commands.

**Exit codes** ([reference](../reference/cli.en.md#exit-codes))

| Code | Meaning |
|---|---|
| 0 | Success (`oh doctor` too when there are only warnings) |
| 1 | Failure: `workflow validate` or `skill check` with errors, `run --headless` with a failed session, `doctor` on a mandatory check, ordinary error |
| 2 | Wrong usage: unknown command or option (**it was 1 in 4.2.0**), removed command (`oh deploy`, `oh sync`), confirmation asked without a terminal and without `--yes` |
| 3 | `oh run --headless` with a session waiting for a decision and none failed |

A script that tested `$? -eq 1` to detect an unknown command must test 2. `oh beads` returns the exit code of `bd`.

## 12. Confirmation without a terminal: `--yes`

An action that asks for confirmation is **refused** when oh has no terminal (script, CI, redirected input), unless `--yes` (`-y`) is given: "confirmation required: run again with --yes", exit code **2**. In 4.2.0, `oh workflow publish|archive|restore` answered "yes" by themselves and several commands refused silently or failed with "open /dev/tty".

Commands concerned: `workflow publish|archive|restore`, `brick publish`, `purge`, `project remove`, `teams remove`, `service remove`, `worktree cleanup`, `session delete|resolve`, `migrate deploy-cleanup`, `run --recap` (and `start --recap`). `--yes` is added to `run`, `purge`, `project remove`, `teams remove`, `service remove` and `worktree cleanup`; with `purge`, it does not offer the backup. Two exceptions: `oh repair --restore` asks for `--auto` without a terminal; `oh doctor --fix` and `oh gc` also refuse without a terminal and without `--yes`, but exit with code 1.

To do: add `--yes` to the scripts and CI that run these commands. An agent cannot run them for you (§20).

## 13. Configuration

### `hub.toml` read strictly

4.2.0 read `hub.toml` with a loosely typed reader that converted values without a word. 5.0 reads it with a strict TOML decoder: **every value must have the type of its setting**. A `hub.toml` written by oh (`oh init`, `oh config set`, the TUI) stays valid; only hand-edited files are concerned. Refused now:

| Written in `hub.toml` | Before | Now | To do |
|---|---|---|---|
| `setup_done = "true"`, `enabled = "false"`, `"1"`, or `setup_done = 1` (a quoted boolean, or a number for a boolean) | read as a boolean | **error** | `true` / `false` without quotes |
| `keep_images = "3"`, `runner_project_id = "42"`, `budget_usd = "12.5"` (a quoted number) | read as a number | **error** | `3`, `42`, `12.5` without quotes |
| `keep_images = 2.7` (a decimal for an integer) | truncated to `2` | **error** | an integer |
| `member_id = 123`, `language = true` (a number or a boolean for a text) | converted to text | **error** | `"123"` |
| `env_allow = "A,B"`, `instruction_files = "X.md"`, `models = "a,b"` (a text for a list) | split at commas | **error** | `["A", "B"]` |
| a key or table no setting uses (typo, key of a newer oh) | ignored silently | **warning**, ignored, **lost at the next write** of `hub.toml`; `oh doctor` lists it | fix or remove it |

The error stops oh and gives the file, line, column and key of the first faulty value. `oh doctor` prints the same message; `oh doctor --fix` restores the latest `hub.toml.bak-*` backup that loads (three copies kept). `hub.toml` is written atomically and durably. No environment variable sets `hub.toml`. Reference: [Strict reading of `hub.toml`](../reference/config.en.md#strict-reading-of-hubtoml).

### `./hub.toml` is no longer read

In 4.2.0, without `~/.oh/hub.toml`, oh read a `hub.toml` from the current directory: a cloned repository could inject a configuration. Only the hub folder counts now: `~/.oh`, or `OH_HOME`. To do: put your configuration in `~/.oh/hub.toml`.

### Renamed and removed keys, case

- **Case**: the keys of the `remote.projects` and `remote.targets.binaries` tables keep their case (they used to be lowercased). Write them as the project identifier.
- `[opencode] default_provider` → **`[llm] default_provider`**: the old key is still read, `oh config set opencode.default_provider` is accepted, and `hub.toml` is rewritten with the current key at the next save.
- `[execution] opencode_version` → **`[execution] tool_version`**: read as a fallback only; `oh config set execution.opencode_version` is **refused** ("unknown config key"). Use `oh config set execution.tool_version <version>`.
- Former keychain names of the MCP tokens (`<service>-token`, `<service>-token-<project-id>`) → `openhub.mcp.<service>.token[.<project-id>]`: read as a fallback and copied under the new name at first access. The old keys and names will **no longer be read in v6**; `oh doctor` lists them.
- Removed, ignored then lost on save: `[opencode]` (§1), `[deploy] disable_native_agents` (§6), `[workflow]` and `[workflow.overrides]` (§9), `[parallel]` (no effect).
- **`policies.toml` is rewritten in full** at every change (`oh policies add`, Policies view of the TUI), under the team-state lock: hand-written comments and formatting are lost.
- **Bedrock `auth_mode`** is honoured: `bearer` uses the stored API key only; `profile` and `env` sign with SigV4 and ignore any stored key. Check `provider.bedrock.auth_mode` (`oh provider setup bedrock --from-env`).

## 14. Environment of the agents: `execution.env_allow`

In 4.2.0, the opencode server and the shells of the agents received **all** of your environment. In 5.0, in a local session, only an **allow-list** goes through: `PATH`, `HOME`, `USER`, `SHELL`, `TERM`, `LANG`/`LC_*`, `TZ`, `TMPDIR`, proxies and certificates, `XDG_*`, toolchain locations, and the useful `OH_*`. So **`SSH_AUTH_SOCK`**, **`GH_TOKEN`**, `GEMINI_API_KEY`, `NPM_TOKEN`, database URLs… no longer pass. Typical consequence: `git push` over ssh, `gh` or a tool that reads a token from the environment fail in the session.

To allow what your agents need (by name or `PREFIX_*` pattern):

```bash
oh config set execution.env_allow "SSH_AUTH_SOCK, MYCO_*"
```

> **`GH_TOKEN` and `gh`.** Since 5.4.1, `GH_TOKEN` is removed even when listed: use `gh auth login` (`gh` stores the token itself) or the GitHub MCP server of oh ([GitHub MCP](../reference/mcp-github.en.md)).

- `oh doctor` lists the variables held back, by name only, and warns when a name that looks like a secret is allowed (the agent can read it and send it anywhere its tools reach).
- Still refused whatever you write: the credentials oh manages itself (`AWS_*`, `ANTHROPIC_API_KEY`, `OPENAI_API_KEY`, `GITLAB_TOKEN`, `GITHUB_TOKEN`, `GH_TOKEN`, `OH_PASSPHRASE`…). `ZDOTDIR`, `BASH_ENV` and `ENV` are not in the base list.
- Containers and remote jobs never inherit your environment.

Reference: [The environment of the agents](../reference/config.en.md#the-environment-of-the-agents).

## 15. Workflows, agents and layers

- **`feature` and `cadrage` are aliases** (no more `feature.yaml` or `cadrage.yaml`): `feature` and `cadrage` lead to `spec`; `feature` with `--tickets` leads to `implement-spec` (`-m` is refused there: no `request` input, pass `-i instructions=…`). The `cp-plan` checkpoint replaces `cp-0` and `cp-final` replaces `cp-feature`. A layer that extends `hub:feature` or `hub:cadrage` is **refused** (`extends_alias`): write `extends: hub:spec` or `extends: hub:implement-spec`. `oh doctor` lists the "Removed workflows".
- Other aliases read in the workflows: `secu` (→ `security-fix`), `perf` (→ `benchmark`), `dbg` (→ `debug`), `feedback` and `retours` (→ `review-feedback`), `free` (→ `libre`); `manual` is read as `manuel`. The terms `dev`, `start`, `fast`, `rev` and `orchestrator` are no longer recognised in the palette or in `oh run`.
- **A project layer can no longer add an agent or `skills.extra`** to an inherited workflow (errors `patch_agent_refused`, `patch_skills_extra_refused`, the addition is ignored). The team layer keeps a warning (`patch_agent_added`, `patch_skills_extra_added`). To do: declare the agent or skill in the team workflow, then extend it in the project.
- **Layers can only harden**: `skills.deny`, preconditions of `risk: publish` workflows, `mcp`, `plugins`, `after:`, `calls`, `circuit_breaker`, `writes`, `code_mode`, `modes.allowed`; `checkpoints.<id>.approver` (team layer only). A loosening gives the `loosening` error. Check with `oh workflow validate --all`.
- **5 agents removed**: `developer-refactor`, `developer-migrator`, `test-generator`, `infra`, `database`; **4 added**: `conductor`, `merger`, `operator`, `release-manager`. A team or project workflow that names a removed agent is refused (`agent_removed`) with the replacement; the replacement table is in [MIGRATION.md](../../MIGRATION.md#agents-supprimés-consolidation-c1) (in French).
- **Session behaviours that change without any action from you**: subagents closed by default (daemon stopped, lockable operations are refused; without an active oh plugin, a workflow that delegates no longer starts: keep the daemon running, `oh daemon status`); checkpoints controlled by oh (`verify:` run by oh, `unlocks:` locks applied to every agent); `risk: read` really read-only on the Beads side; with `catalog.lock`, a brick edited by hand or committed without oh is ignored (publish with `oh brick publish`); budgets raised as soon as the overrun is seen, each fan-out child getting a share of the remaining budget; `spec`: the request is optional.

## 16. TUI keys

The TUI follows a single key contract ([TUI reference](../reference/tui.en.md#key-convention)).

- **Typing a letter no longer opens the palette**: `Ctrl+P` (or `/` outside a filter). `Enter` and letters are no longer taken from the list.
- Help: `?` or `F1` (`F1` works everywhere, forms included). `d` no longer closes toasts: `Esc` does. In a wizard, `Esc` asks "Quit the wizard?". Launch card: `Ctrl+S` goes to the summary then launches.
- Convention: `n` creates everywhere, `d` deletes (always confirmed), `x` archives (never destructive), `R` resumes or restores, `c` compares; decision: `y` accept, `n` refuse, `e` edit, `s` leave pending, `A` accept all.
- A single **Cleanup** view (former deployments and `oh gc`); palette commands `cleanup`, `gc`, `nettoyage`, `deploy-cleanup`. The `coder` command is called `libre`.

The **23 key changes**. The "4.2.0" column says whether the view already existed; if not, the change only concerns development builds.

| View | Before → now | 4.2.0 |
|---|---|---|
| Sessions | `D` → `d` delete · `c` → `R` resume · `g` → `R` recover a remote session · new `B` batch validation | no |
| Recap of an existing session | `r` → `R` resume · `t` → `T` transfer | no |
| Team wiki proposals | `a` → `y` accept · `x` → `n` refuse | yes |
| Workflows | `D` → `c` compare · `x` → `d` delete a draft (`x` only archives published ones) | no |
| Workflow history | `r` → `R` restore | no |
| Workflow editor | `x` → `d` remove · `y` → `Y` raw YAML | no |
| Patterns | `a` → `n` create · `v` → `y` validate | yes |
| Projects | `a` → `n` create · `n` → `e` rename | yes |
| Team board | `f` → `/` filter · `x` → `L` release | yes |
| Worktrees | `a` → `n` create · `x` → `M` clean up the merged ones | yes |
| Team detail | `y` → `T` discover the tracker | yes |
| Other lists (models, policies, secrets, teams…) | `a` → `n` create | yes |

Nothing to configure: a "New keys" message is shown once at the first launch after the update, then stays available from the help (`?`).

## 17. Team mode

- **No longer a beta (5.1).** Team mode no longer has a warning on stderr, a "beta mode" check in `oh doctor` or a "Beta" line at the top of the team board and team configuration. What 5.0 announced as limits is fixed: a claim in review (`ready-for-review` label or `review` status) is **kept** when the session ends, and so is a claim whose checkpoint (`cp-2`) is waiting or was put off with work left (label `session-ended`); it has a **lease** (`lease_hours`, 24 h by default) that the daemon renews while an open session works on the ticket, after which another member can take it over with `oh team claim <ticket>` (common confirmation; a takeover is no longer a silent transfer); what the end of a session decides goes through a local queue (`.git/oh-claims-queue.json`) applied as soon as the team repository answers, so a release is no longer lost offline. Details: [Lease and end of session](team-setup.en.md#lease-and-end-of-session) and [Status model](team-setup.en.md#status-model). The remote mode remains **experimental**.
- **Single GitLab token.** One search order for the tracker sync, `oh mr create`, the `review-feedback` inputs, the identity check of `oh team init|rejoin` and `oh mcp enable --project`: `GITLAB_TOKEN`, `tracker_token_key`, project token, team token, hub MCP token (key of `[mcp.gitlab] token`, then `openhub.mcp.gitlab.token`, then `gitlab-token`), then the former key `openhub.tracker.gitlab.token`. For `oh mr create`, the token of the project's team comes before the hub MCP token.
- **Disabled tracker.** The TUI no longer syncs a tracker with `[tracker] enabled = false`. Before, it synced it anyway.
- `oh mr create` reads the GitLab project and the reviewer from the project's team (falling back to the hub team).
- The REVIEW column of `oh board` and of the TUI Board is filled: a Beads `in_progress` ticket with the `ready-for-review` label is shown there. The MCP review vote moves an approved ticket through `review` then `done`.

## 18. Remote mode and fan-out (experimental)

- **Remote mode (GitLab CI)**: experimental and disabled by default (never validated on a real runner). Without `remote.experimental = true`, `oh remote setup`, `oh run --runtime remote`, the ☁ option of the launch card, `oh session fetch` and `oh session resolve` are refused. To try it, with test projects and tokens: `oh config set remote.experimental true`. 4.2.0 has no `oh remote`, so there is nothing to run again; `oh remote setup` is only run again after updating an intermediate build of 5.0.
- **Fan-out**: experimental and disabled by default. Without `fanout.experimental = true`, `oh run implement-spec -i fanout=true` (or `sweep`) is refused; refused in a remote session too ([guide](fan-out.en.md)).

## 19. Database and going back

The database goes from version **27 (4.2.0) to 49** at the first launch (21 migrations: 28 to 46, 48 and 49). A lock between processes prevents concurrent migrations, and a copy `~/.oh/backups/oh.db.v27-<date>.bak` is written **before** (three copies kept). `OH_SKIP_DB_BACKUP=1` skips the copy (the migration still happens: avoid it). A database written by a newer oh is refused (`OH_ALLOW_NEWER_SCHEMA=1` to override). `~/.oh` is 0700 and `oh.db` is 0600; the opencode server passwords are no longer in `oh.db` (`oh export` no longer carries them).

**There is no simple way back**: the database is migrated in place. The safe path:

1. `oh daemon stop` (refused while sessions run, unless `--force`).
2. `oh repair --restore ~/.oh/backups/oh.db.v27-<date>.bak` (with `--auto` without a terminal). The current database is set aside.
3. Reinstall the 4.2.0 binary **before** running another command: 5.0 would migrate the restored database again.
4. If needed, restore `hub.toml` too (`hub.toml.bak-*`, or the archive of `oh export` with `oh import`). Changes made with 5.0 are lost.

Not tested: opening a 5.0 database with a 4.2.0 binary. `oh repair` (no option) checks the database without migrating it; `--migrate` applies the remaining migrations after a copy.

## 20. Visible security

- **`oh mcp trust <service>`**: an MCP service URL **imposed by the team** is used only once approved (pinned in `mcp.<service>.trusted_url` of your own `hub.toml`); `https` required outside loopback. Without approval, the token is not sent to that URL and oh warns (`oh doctor`, `oh mcp status`, `oh team config`, MCP view of the TUI, `T` key). Each member runs `oh mcp trust <service>` after checking the URL and host shown; `oh mcp untrust <service>` withdraws the approval.
- **Global guard of the agents**: an agent can no longer run, from its shell, `oh mcp trust|untrust|setup|enable|disable|reset|serve`, `oh config set|unset|language|websearch`, `oh config model default|family|agent|unset`, `oh secrets set|delete|get|cleanup`, `oh budget set|unset|raise`, nor what publishes or deletes: `oh workflow publish|archive|restore`, `oh brick publish`, `oh purge`, `oh project remove` (aliases `rm`, `p`), `oh teams remove|archive|restore`, `--yes` included. Reads (`list`, `show`, `diff`) stay allowed. This is a guard rail, not a barrier: run these commands yourself.
- **`oh serve`**: random token per launch (URL `?token=…`, then a cookie), 401 without a token, 403 if `Host` is not `127.0.0.1`, `localhost` or `[::1]`. Open the URL shown and do not share it.
- **Isolation**: the plugins of your personal opencode configuration are reported at launch and by `oh doctor`; `execution.strict_isolation = true` hides them and refuses an opencode package without a pinned fingerprint (default: `false`).
- Without any action from you: the credential proxy no longer relays the agent's headers as is, the MCP gateway of containers is closed by default, writes of secrets, configuration, reports and wiki are atomic.

Accepted limits and maturity of the features: [SECURITY.md](../../SECURITY.md).

## From 5.0 to 5.1

5.1 **removes** what 5.0 had deprecated (breaking change, announced in the [CHANGELOG](../../CHANGELOG.md)). A removed command name does not vanish without an explanation: `oh` shows the replacement on the error output and exits with code **2** (a script sees it). A removed option gives the unknown-option error of the command line (code 2).

**Commands**

| Removed in 5.1 | Replacement |
|---|---|
| `oh start` | `oh run <workflow>`: `oh run libre --agent <agent>` (free launch), `oh run ticket --tickets <ids>`, `oh run onboarding`, `oh run sweep`, `oh run feature`; `oh session attach <id>` to resume a session |
| `oh audit` | `oh run audit -i type=<type>` |
| `oh review` | `oh run review -i review_mode=<mode> -i branch=<branch>`; `oh review --publish` → `oh mr create` |
| `oh review feedback <ref>` | `oh run review-feedback -i mr=<ref>` |
| `oh debug` | `oh run debug -i issue=<description>` |
| `oh service`, `oh service setup`, `oh service remove` | `oh mcp status`, `oh mcp setup`, `oh mcp disable` (delete the token with `oh secrets delete <key>`: `oh mcp disable` leaves the keychain alone) |
| `oh takeover-brief enrich <ticket>` | `oh run brief-enrich --headless -i ticket=<ticket>`, or the `e` key of the Takeover view of the TUI (which saves the enriched brief) |

**Options**

| Removed in 5.1 | Replacement |
|---|---|
| `oh serve -p`, `oh metrics -d`, `oh project configure -m` | `--port`, `--period`, `--model` |
| `oh session attach --how`, value `--attach here` | `--attach`, value `suspend` |
| `--force` / `-f` of `oh purge`, `oh project remove`, `oh teams remove` | `--yes` / `-y` |
| `oh repair --auto` | `--yes` (for `--restore` without a terminal) |
| `-w`, `-l`, `-t`, `-A`, `-r` of `oh start`, `-m`, `-b` of `oh review`, `-t` of `oh audit`, `-i` of `oh debug` | gone with their commands |

`--force` stays on `oh daemon stop`, `oh remote setup`, `oh worktree remove` and `oh worktree cleanup`, where it overrides a safety.

**To do**: search these names in your scripts, shell aliases and CI (`grep -rnE 'oh (start|audit|review|debug|service)\b|takeover-brief enrich|--force|repair --auto'`), then replace them following the tables. Details of the commands: [Commands removed in 5.1](../reference/cli-sessions.en.md#commands-removed-in-51).

**Deprecated in 5.1 (removed in v6)**

| Deprecated | Replacement |
|---|---|
| `oh dashboard` | The **Metrics** view of the TUI (`oh`, then `Ctrl+P` › Metrics). The command still opens it, with a warning |
| `oh metrics` | The same view. The command keeps printing its text (`--period`) on the standard output until v6, and warns on the error output |

Both are hidden from the help. To do: replace them in your habits; a script that reads `oh metrics` has until v6 (`oh budget report --json` gives the spending).

**Changed in 5.1**: `oh board`, `oh team board` and `oh dashboard` no longer draw a separate full-screen view: they open the TUI on the Board, the Team board and the Metrics view, and `Esc` goes back to the landing of the mode. Without a terminal (script, CI) they exit with code **2**. `oh board` shows the project of the current directory (or `--project <name>`, or the only registered project) instead of the tickets of the current directory; `--watch` is accepted and has no effect (the board refreshes every 5 s).

## From 5.1 to 5.2

5.2 changes the meaning of a team mode setting and adds a refusal (two breaking changes announced in the [CHANGELOG](../../CHANGELOG.md)). The database does not change. 5.2.1 is a corrective release: no breaking change, no migration. Without team mode, nothing to do.

**1. `max_auto_plan_per_member` becomes `max_in_progress_per_member`**

| In 5.1 | Since 5.2 |
|---|---|
| `max_auto_plan_per_member` capped the claims that the tracker sync created for a member | `max_in_progress_per_member` caps the number of tickets a member has **in progress** (5 by default, `0` = no limit) |
| Issues beyond the cap stayed "held", off the board | The sync puts **all** the assigned tickets on the board, with no cap, and never refuses anything; a ticket already in progress on the tracker arrives in progress, even beyond the limit |

Tickets in review (status `review`, or label `review` / `ready-for-review`) and in validation do not count; on a custom board, the columns of role `active` do. The old key is **deprecated**: still read when the new one is absent (a `hub.toml` value of 0 used to mean "default", it is ignored), announced once per process, reported by [`oh doctor`](../reference/cli-infra.en.md#oh-doctor) with the command to run, and removed on the next write of the file. `oh config set|get|unset tracker.max_auto_plan_per_member` is still accepted as an alias of the new key. Support ends in v6.

**To do**: write the new key, `oh config set tracker.max_in_progress_per_member <n>` for your machine (`hub.toml`) or `oh team config` for the team (`config.toml`). The TUI field is now called "Tickets in progress per member (max)". The "held" issues mechanism is gone (sync warning, "N issue(s) off the board" line of the team board); a 5.1.0 sync state that contains some makes the next run sweep everything again, which puts these issues on the board.

**2. Beyond the limit, oh refuses to take or start a ticket**

A member who reached the limit can no longer take or start one more ticket; oh refuses, giving the count and the key:

- `oh team claim`, also the takeover of an expired claim and `oh team claim transfer` to yourself;
- the claim key (`c`) of the team board, and a card moved to in progress or transferred to yourself on the board;
- launching a session on team tickets, local or remote, before anything is created.

**To do**: a script or a member that had more than 5 tickets in progress is refused at the next ticket. Raise the limit (`oh config set tracker.max_in_progress_per_member <n>` for yourself, `oh team config` for the team, `0` to remove it) or move tickets to review. Details: [team mode](team-setup.en.md) and [`[tracker]`](../reference/config.en.md).

## From 5.2 to 5.3

5.3 changes four team mode behaviours, two of them breaking changes announced in the [CHANGELOG](../../CHANGELOG.md) (`unassigned_labels`, card menu). The database does not change. It recommends opencode **2.2.5** (the supported range does not move: 2.0.0 to 2.99.99) and expects bd **1.3.3**: [`oh doctor`](../reference/cli-infra.en.md#oh-doctor) reports another bd version. Without team mode, nothing else to do.

**1. `unassigned_labels` becomes an OR, and the pool is swept entirely**

| In 5.2 | Since 5.3 |
|---|---|
| An issue entered the pool only if it carried **all** the labels of `unassigned_labels`: with several labels, the pool was almost always empty | An issue that carries **any** of the labels enters the pool (one request per label, each issue counted once); Jira already worked that way |
| Only the issues updated since the last sync were read: old issues never touched again never arrived | The pool is first swept entirely, over as many runs as needed (at most `max_unassigned_issues` claims per run, 20 by default), then incrementally; changing `unassigned_labels` starts the sweep again |

A pool of 70 issues therefore takes four sync runs. A claim released by hand is not recreated by the sweep.

**To do**: if you relied on the AND to narrow the pool, keep a single label or create a dedicated one. Raise `max_unassigned_issues` to go faster.

**2. `'todo'` is not a claim status**

The 5.2 discovery assistant wrote `'todo'` as a target of `label_status_mapping`. The sync refused it silently and put these issues in IN PROGRESS. 5.3 reads it as `planned` (status of the initial column) and reports it; a value that names no column is ignored and the sync says so. **Saving** the team config is refused, with the offending value and the valid statuses, while such a value is in it.

**To do**: run `oh doctor` ("Tracker mappings" check), then `oh doctor --fix`, which rewrites `config.toml` in one commit of the team repository. A value that names nothing is fixed by hand. On the first run, a claim whose only status label is a "to do" label moves to TODO.

**3. Several mapped labels: blocked first, then the most advanced**

When an issue carries several labels mapped to statuses, a **blocked** column (role `blocked`, or `blocked` on the default board) always wins; otherwise the **most advanced** column (by default DONE > VALIDATION > REVIEW > IN PROGRESS > TODO); on a tie, the alphabetical order of the labels. Before, it was the first label of the issue in the tracker's order. An issue "Blocked" + "READY TO DEV" is therefore blocked, and an old "READY TO DEV" left next to "DEV DOING" no longer moves it back.

**To do**: nothing, unless a column must win over the others: give it the role `blocked`, or place it further in the board. Details: [`[tracker]`](../reference/config.en.md#tracker-section).

**4. The `a` menu of a card follows its column**

| Column | Actions |
|---|---|
| todo (role `initial`) | *Frame and write the spec* (`spec`), *Implement the ticket* |
| in progress (first `active` column) | *Implement the ticket*, *Give an instruction to the running session* |
| review (role `review`) | *Code review* (branch of the claim), *Apply review feedback* (if the claim has an MR) |
| validation, blocked, done… | detail, status, transfer |

On a GitLab issue, *Implement the ticket* imports the issue as a Beads ticket **when you confirm the launch** (if the project has Beads), then launches `ticket`; otherwise `quick`. The claim of the card moves to in progress at launch, within the work-in-progress limit, and it is settled at the end of the session like those of the tickets. A board whose columns have no role keeps the former menu.

The new column role `review` is optional: without it, the column whose ID is `review` stands in. Its claims do not count in the work-in-progress limit and do not expire, whatever the ID of the column.

**To do**: to launch another workflow on a card, use the command bar or the home view. Details: [the `a` menu of a card](../reference/tui.en.md#the-a-menu-of-a-card) and [`[board]`](../reference/config.en.md#board-section).

## From 5.3 to 5.3.2

5.3.2 includes 5.3.1, which was never published. It mostly fixes creating and joining a team, with two minor breaking changes. The database does not change. Without team mode, nothing to do.

**1. Access to the team repository before the clone**

| In 5.3.0 | Since 5.3.2 |
|---|---|
| The clone started right after the URL was entered, with only the access git had on the machine; a private repository over HTTPS failed and no token could be entered | Repository, access probe, **Access** step when git is refused (HTTPS token, or an `ssh-add` reminder over SSH), identity, then clone. The error screen offers Retry, Set up access, Edit the URL, Skip the team |
| The token entered was only handed to the credential helper of the machine (`git credential approve`), and nothing was kept without one | The token is checked, kept in the keychain of `oh` under `openhub.git.<host>/<repository>.token` (`.user`: `oauth2` by default) and given to git through a one-time `GIT_ASKPASS` for the clone, pull, push and the daemon |

**To do**: nothing if access already works (SSH, credential helper of the machine). Otherwise, "Rejoin a team" in the palette runs the Access step again, or `oh secrets set --global openhub.git.<host>/<repository>.token`. Details: [team mode](team-setup.en.md) and [SECURITY](../../SECURITY.md).

**2. One place for the clone**

"Create a team" and `oh team init` cloned into `~/.oh/team-state/`, "Rejoin" into `~/.oh/team-states/<host>/<repository>/`. All paths now use the latter. A team already configured keeps its `state_path`, old path included: nothing is moved. A folder that holds the clone of another repository is refused instead of being reused.

**To do**: nothing, unless oh reports that a folder holds the clone of another repository: delete it, or choose another repository.

**3. Member identity**

The form is the same in every path: ID, display name, GitLab handle (optional, unique in the team), role (`lead` for whoever creates the team, `dev` for whoever joins it), mode `semi-auto`. An ID or a handle already held by a colleague is **refused**; before, "Create" overwrote their card and `oh team init` ignored it. If it is your card, "That's me" attaches it without overwriting anything (only empty fields are filled). "Rejoin a team" in the palette can now create your card, and `n` in the Teams view launches that wizard. An empty team repository (new GitLab project) is accepted by "Create".

**To do**: nothing, except for a script that reused the ID of another member: it now fails.

## From 5.3.2 to 5.3.3

5.3.3 fixes team mode defects that could lose or damage data. The database does not change. Without team mode, nothing to do.

**1. The tracker sync follows the changes of the issue, not its state at each run**

| In 5.3.2 | Since 5.3.3 |
|---|---|
| An open issue without a status label gave `in_progress` at each run, for any member: a claim moved to review, validation or done in the hub went back | The status is rewritten only if the state of the issue, its status or its labels mapped to a column changed since the previous run (remembered on the claim: `tracker_state`, `tracker_labels`). A closed issue moves to terminal, a reopened one back to work |
| The labels set by the hub (`session-ended`, `review`…) were removed at each sync | Only the labels removed on the tracker side are removed on the hub side |
| Nothing was pushed to the tracker | With `push_labels`, the label of the column of the hub status is set on the issue and the other status labels removed |

On the first run after the update, an existing claim is not moved: oh remembers the issue. A sync whose push to the team-state fails is now an error, and `sync-state.json` is not written.

**To do**: nothing.

**2. Git token and GitLab API token kept apart**

The git token entered at the Access step (`read_repository`, `write_repository`) is no longer copied to `openhub.team.<team>.gitlab.token`. "Rejoin" offers the "GitLab token" step again (it can be skipped), and an identity GitLab does not confirm (`GET /user` refused or unreachable) becomes a warning instead of blocking. A copy made by a 5.3.x stays in place.

**To do**: for the tracker and the identity check, an API token with `api` (or `read_api` + `read_user`).

**3. The team `config.toml` and `oh team config`**

A change of `config.toml` (TUI, wizards, `oh team config`) is applied on the latest version of the file: if a colleague changed the same setting meanwhile, nothing is overwritten and oh offers to reload. An unreadable `config.toml` is no longer shown as empty: saving is refused. `oh team config` keeps the current value on Enter, offers "inherit" to remove a local setting, and refuses without a terminal. `oh team config status` writes nothing any more: it no longer removes colleagues from `members.toml`.

**To do**: a script that drove `oh team config` through the standard input fails; set the keys in `config.toml` (team) or with `oh config set` (machine).

**4. Team paths and IDs**

A team without `state_path` now gets the path of its clone when `hub.toml` is read (the old clone of the right repository if it exists): no git command runs in the current folder any more. New clones go to `~/.oh/team-states/<host>/<group>/<repository>`. Two repositories with the same name (`acme/team-state`, `beta/team-state`) coexist: the second gets the ID `beta-team-state`. An ID chosen with `--id` is no longer rewritten, and a team never replaces another one. No clone is moved or cloned again.

**To do**: nothing; if a script assumed the ID of a team is the name of its repository, read `oh teams list`.

## From 5.3 to 5.4

5.4 makes team mode consistent from end to end: one rule to choose the team, access checked before the clone, automatic tracker sync, governed configuration, adjustable notifications and a real life cycle (leaving a team, removing a member). Several commands that a script may call change behaviour (**light breaking changes** flagged below). The database does not change and no file is migrated. Without team mode, nothing to do.

**1. Teams: one rule for "which team?"**

| In 5.3 | Since 5.4 |
|---|---|
| Commands and the TUI each aimed at "the active team" or the first team of `hub.toml`; the repository and the member could come from two different teams | `--team`, then the team chosen in the TUI, then the team of the **project** (`-p`, otherwise the current folder), then the default team, then the first enabled team. The repository and the member always come from the same team. `--team` is accepted by every subcommand of `oh team` (`oh team profile --team` and `oh team promote --team` are unchanged) |
| The default team was the first one of `hub.toml`, an archived one if there was no other | `oh teams default <id>` writes `default = true` in `[[teams]]`; without it, the first **enabled** team, as before. An archived team is never the default team any more (all archived: no default team) |
| `oh team init` configured the existing team, or created one | **Light breaking change**: `oh team init` always creates a new team on a new repository. To reconfigure: `oh team init --reconfigure [--team <id>]` (palette: "Reconfigure the team"). `oh team init` and `oh team rejoin` **refuse** `--team` (it was ignored) |

In the folder of a project of team B, `oh team status` now acts on B, not on the first team. A project is attached to a team with `oh project set-team <project> <team>` (or `oh project add --team`), or in the TUI with "Attach the project to a team".

**To do**: with several teams, `oh teams default <id>`. A script that reran `oh team init` to reconfigure adds `--reconfigure`; a script that aimed at the first team from a project folder adds `--team <id>`.

**2. Team-state access and SSH**

| In 5.3 | Since 5.4 |
|---|---|
| `oh team rejoin --repo` cloned before any probe and only asked for the API token | It probes access **before** cloning: for HTTPS, user name and git token are asked, checked and kept in the keychain. It can create your card (`--new-member` with `--member-id`, `--display-name`, `--gitlab-username`, `--role`) |
| `oh teams add` only wrote `hub.toml`, with an unchecked `member_id` | Alias of `oh team rejoin` (probe, clone, checked card); `--member-id` is optional. `oh teams set <id> repo <url>` follows a moved or renamed repository, after checking it |
| "Create" and `oh team promote` probed read-only: a `read_repository` token passed, the push failed | A dry-run push checks the right to write, from the probe |
| An expired git token filled the daemon log without any alert | One "To handle" alert per repository, no more attempts until the token changes; `oh doctor` check "Team-state access" |
| An SSH server missing from `~/.ssh/known_hosts` was accepted silently (`accept-new`) | **Light breaking change**: the wizards (`oh init`, `oh team init`, `oh team rejoin`, `oh teams add`, `oh teams set <id> repo`, `oh team promote --remote`, palette) show the SHA256 fingerprint and ask for confirmation, once per server. **Without a terminal the command is refused.** The daemon no longer connects to such a server (team added by hand in `hub.toml`): "To handle" alert, then it resumes as soon as the server is known |

**To do**: in CI or in a script, check the fingerprint of the server then run `ssh -T git@<host>` once (or `ssh-keyscan <host> >> ~/.ssh/known_hosts`). `oh doctor` reports unknown servers ("Known SSH server") without contacting them. Hosts that are already known, `@cert-authority` and everyday `pull`/`push` do not change. Limits: `ProxyJump`/`ProxyCommand` are not handled by the confirmation (since 5.5 `GIT_SSH_COMMAND` and the `core.sshCommand` of your global git configuration are followed, not the one of a repository, and `HashKnownHosts yes` is applied: see [team-setup](team-setup.en.md#first-contact-with-an-ssh-server)). An expired git token is renewed with `oh team rejoin --repo <url>`.

**3. Tracker sync**

| In 5.3 | Since 5.4 |
|---|---|
| `auto_sync` and `sync_interval_minutes` were only read for display | The daemon syncs every `sync_interval_minutes` (5 by default) and the board when it opens. One member per interval (`sync.lock` at the root of the team-state, ignored by older `oh`) |
| Every active project of the hub was linked to the tracker of the team | **Light breaking change**: only the projects attached to the team (`team_id`) are. "No hub project is linked to the tracker": `oh project set-team`. Existing claims stay |
| A ticket was called `42` | It carries the identifier written by `ticket_pattern` (`SRU-42`); existing claims keep theirs, without duplicates |
| A 403 stopped the whole sync and asked to replace the token | A 403 is an error of the tracker project: the other projects are synced |
| Default TODO column of the board: `todo` (move refused) | Status `planned`; the `s` menu only offers the allowed moves; the MCP verdict follows the columns of the board and is signed by the member |

**To do**: if `auto_sync = true` was set without effect, the sync starts running: check the setting. Attach projects to their team.

**4. TUI**

The home screen always has a Teams section with "➕ New team" (Create, Join, Solo space). Teams view: `p` (Team view), `*` (default), `e` (rename), `x` (archive), `R` (restore); **`d` also detaches the projects** of the removed team (light breaking change) and, for a team that has a team-state, offers to leave it. "Team of the project" becomes "Attach the project to a team" (Project view) and the Team field of the project configuration is a list. "Team history" is gone: use "Activity". Team commands are in the "Team" category and available without a restart after a creation or a join. `Ctrl+T` no longer offers solo spaces.

**To do**: nothing.

**5. Team configuration**

| In 5.3 | Since 5.4 |
|---|---|
| Webhook: `mattermost_webhook`, shown in clear, sometimes without effect | `webhook_url` + `type` + `enabled = true`; without `enabled = true`, nothing is sent. `mattermost_webhook` is still read, `oh doctor --fix` migrates it. Webhook masked everywhere; `oh doctor` warns if the team-state repository is public |
| Any member could change the enforced settings, without a trace | **Light breaking change**: `*_enforced`, `[limits.enforced]` and `[governance]` are reserved to the `lead` role (`role` in `members.toml`); only a lead changes a role. A team without a lead is not locked; `[governance] config = "any_member"` reopens it to everyone. Every publication leaves a `config_changed` event (keys and author, never the values) in `oh team activity` |
| The `[tracker]` of `hub.toml` applied to every team; a member could exceed or lift `max_in_progress_per_member` | **Light breaking change**: `[teams.tracker]` under a `[[teams]]` wins over `[tracker]` (which stays the default); wizards and the detail write the team's section. The team cap can no longer be exceeded or lifted; a team that chose nothing leaves the choice |
| `config.toml` lost its unknown keys and kept default values, `[parallel]` and `type_enforced` | Unknown keys are kept; default values, `[parallel]` and `type_enforced` are no longer written. A 5.4 that publishes removes these keys from a file written by a 5.3 |
| TUI: several ways of saving in one screen | Shared: draft published with `Ctrl+S`; personal: saved on its own; `Esc` with a draft asks what to do |
| `oh teams set <id> repo <url>` only | `oh teams set <id> repo|name|member-id|enabled|default [value]` (2 or 3 arguments); `oh team config get|set|unset|show [--origin]` sets any key without a terminal |

`max_ticket_wip` no longer counts finished claims. `oh doctor` gains a "Team configuration" check (former keys, out-of-bounds values, `--fix`).

**To do**: add `enabled = true` to webhooks; give the `lead` role to whoever sets the enforced keys (or `any_member`); a script that reads `oh team activity` tolerates the `config_changed` type.

**6. Notifications**

| In 5.3 | Since 5.4 |
|---|---|
| A webhook could (in theory) receive every event | **Light breaking change**: by default only review (`review.*`), wiki (`wiki.*`) and agent messages. For `claim.*`, `session.complete` or `config_changed`: `events = [...]` on `[notification]` or on the destination (`oh team config set notification.events "review.*, claim.*"`). These events were in practice never sent |
| No personal recipient | Mentions with the `mattermost_username` of the cards (Mattermost, Slack); `mention = false` turns them off |
| Webhook text in French, with identifiers | The sender's language, names of members and projects |
| A single `[session] notify` switch | `hub.toml [notifications]` (Settings › Notifications), reloaded live; `[session] notify = "off"` still turns system notifications off as long as `[notifications] system` is not written |
| End of a step: system notification; each notification replaced the previous one | End of a step: toast (`events.step_done = "system"` to get it back); one notification per family (decisions, budget, errors, alerts…) |
| Decision open when going to sleep: lost (`gone`) | Kept (marked asleep), notification "asleep with a pending decision", resumed on wake-up |
| A failing webhook lost the message | Message kept in `~/.oh/notify/outbox/` and resent by the daemon |
| MCP tool `team_notify`: "Notification sent." even when nothing was sent | Error when nothing left; message signed, limited to 1000 characters and 5 per 10 minutes |

**To do**: list in `events` the families the webhook must receive.

**7. Team and claim life cycle**

| In 5.3 | Since 5.4 |
|---|---|
| No way to leave a team: `oh teams remove` only touched `hub.toml` | `oh teams leave <id>` releases your claims, deletes the clone and the keychain keys, removes the team from `hub.toml`; the card stays unless `--remove-member`. `oh teams remove` still only touches `hub.toml` |
| Removing a member: impossible | `oh team member remove <member> [--to <member> \| --release]` (`lead` role; a team without a lead is not locked). `oh team claim transfer --from … (--to \| --release)` moves all the claims of a member |
| Releasing or transferring the claim of someone else: unchecked | **Light breaking change**: confirmation naming the owner; without a terminal, `--force`. A transfer to a member without a card, or of a finished ticket, is refused |
| `claim.transferred` signed by the former owner; few events | Signed by whoever transfers. New events in `oh team activity`: `member.removed`, `claim.released` and `claim.taken` (sync, board, launches, session ends) |

**To do**: scripts that release or transfer the claim of another member add `--force`; those that read the activity tolerate the new types.

## From 5.4 to 5.4.1

5.4.1 is a bug-fix release (notifications, hub home, guards). Only one change needs an action: a security fix to the environment of the agents. The database does not change.

| In 5.4 | Since 5.4.1 |
|---|---|
| `GH_TOKEN` listed in `execution.env_allow` (or matched by `GH_*`) reached the shells of the agents; only `GITHUB_TOKEN` was removed | **Security**: `GH_TOKEN` is removed even when listed, like `GITHUB_TOKEN`. So are `GH_ENTERPRISE_TOKEN`, `GITHUB_ENTERPRISE_TOKEN`, `GITLAB_ACCESS_TOKEN`, `GITLAB_PRIVATE_TOKEN`, `GITLAB_OAUTH_TOKEN` and `ANTHROPIC_AUTH_TOKEN` ([§ 14](#14-environment-of-the-agents-executionenv_allow)) |
| `audit.finding` was declared, but nothing emitted it | Emitted at the end of an audit session that deposited its report; a webhook gets it when its destination asks for it in `events` |
| `notifications.jsonl` could lose lines when two TUIs or the daemon wrote during a rotation | Writes and rotation under a lock (`notifications.jsonl.lock`) |

**To do**: for `gh` in a session, run `gh auth login` once (`gh` stores the token itself) or use the GitHub MCP server of oh ([GitHub MCP](../reference/mcp-github.en.md)); remove `GH_TOKEN` and the other names above from `env_allow`. `oh doctor` reports them as managed by oh.
