> [Lire en français](migration-v5.fr.md)

# Migrating to oh v5

oh v5 starts every session from a **workflow** (`oh run <workflow>`), in a **session bundle** built outside the project, on an **opencode V2** server. This guide lists what changes for a v4 user (4.2.0). The same breaking changes, grouped by theme with the action to take, are in [MIGRATION.md](../../MIGRATION.md) (in French).

## Step-by-step update

1. **Back up**: `oh export -o ~/oh-backup-4.x.tar.gz` (database, `hub.toml`, encrypted secrets), and keep the 4.2.0 binary (§19).
2. **Install opencode V2** (§1), then **update oh**: `brew upgrade openhub` or `oh upgrade oh`.
3. **Run `oh doctor` first.** The first launch migrates the database (27 → 49, with a copy, §19) and the former `[workflow]` overrides (§9); there is no migration command to run: it runs at the start of every command **except `oh init` and `oh purge`**. Do not edit `hub.toml` by hand before this first launch.
4. **Fix what `oh doctor` reports**: refused `hub.toml` (§13), former keys, former deployments (§8), `~/.oh/skills` (§10), opencode version. `oh doctor --fix` repairs after a summary and a confirmation (`--yes` without a terminal).
5. **Remove the former deployments**: `oh migrate deploy-cleanup` (§8).
6. **Allow the environment variables** your agents need: `oh config set execution.env_allow "SSH_AUTH_SOCK, GH_TOKEN"` (§14).
7. **In a team**: `oh mcp trust <service>` for each imposed URL (§20), and read about team mode in beta (§17).
8. **Update scripts and CI**: `oh deploy`/`oh sync`, `oh start` → `oh run`, `--yes` (§12), exit code 2 (§11).
9. **Check your workflows**: `oh workflow validate --all` (§15).

## 1. opencode V2 required

opencode V1 is no longer supported, with no fallback. oh v5 requires an opencode version between **2.0.0** and **2.99.99**; the **recommended** version is **2.0.25** (the one of the CI). Outside that range, `oh run`, `oh start` and its aliases refuse to start and say why:

```text
opencode 1.18.29 is not supported: oh v5 requires version 2.0.0 or later.
Install a supported release of opencode, then run oh doctor. Guide: docs/guides/migration-v5.en.md
```

Without opencode: "opencode was not found: oh v5 requires a supported release of it".

1. Install opencode V2 with its own tooling (`brew install anomalyco/tap/opencode`, or https://opencode.ai).
2. Check with `oh doctor`: the **opencode** line must be green (✓).

The fingerprint of the opencode Linux packages is pinned in oh for versions **2.0.20** and **2.0.25**. Another version of the range works, but `oh doctor` warns about it: container images would download it without a verified fingerprint, and `execution.strict_isolation = true` refuses it ([SECURITY.md](../../SECURITY.md#supply-chain)).

`oh upgrade opencode`, the managed install in `~/.oh/bin` and the `[opencode] version`, `channel`, `auto_update`, `install_dir` keys of `hub.toml` are removed (ignored if they remain). `[llm] default_provider` stays.

## 2. Former commands: aliases of `oh run`

They still work during v5.x, with a warning that gives the command to use:

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
| `oh review feedback <mr>` | `oh run review-feedback -i mr=<mr>` (deprecated: `oh review feedback` still reads the GitLab discussions, previews them, then launches this workflow); `oh review --publish`: `oh mr create` |
| `oh takeover-brief enrich` | `oh run brief-enrich --headless` |
| `oh service` (`setup`, `remove`) | `oh mcp status`, `oh mcp setup`, `oh mcp disable` |

No longer working:

- `oh start --parallel` **without** `--tickets` (refused); `--max-sessions`, `--priority`, `--sweep-branch-prefix` have no effect.
- `oh start --agent` combined with `--dev`, `--onboard`, `--parallel` or `--sweep` (refused).
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

**Removed or deprecated commands**

- `oh deploy` and `oh sync`: see §6 (hidden, exit code 2).
- `oh start`, `oh audit`, `oh review`, `oh debug`, `oh review feedback`, `oh takeover-brief enrich`: deprecated aliases of `oh run` (§2). **`oh service`** (`setup`, `remove`) is deprecated too: use `oh mcp status`, `oh mcp setup`, `oh mcp disable`.
- `oh plugin` (§4), `oh skill add|list|remove|search|budget` (§10) and `oh upgrade opencode` (§1) no longer exist: "unknown command", exit code 2.

**Options**

- **One letter, one meaning**: `-p` is always `--project`, `-m` always `--message`, `-y` always `--yes`… ([table of short options](../reference/cli.en.md#short-options)). The former forms are still accepted for **one version**, with a warning: `oh serve -p` (→ `--port`), `oh project configure -m` (→ `--model`), `oh metrics -d` (→ `--period`), `oh review -m` / `-b` (→ `--mode` / `--branch`), `oh debug -i` (→ `--issue`), `oh audit -t` (→ `--type`), `oh start -w` / `-l` (→ `--worktree` / `--label`), `oh session attach --how` (→ `--attach`; the value `here` becomes `suspend`).
- **Removed without an alias** (exit code 2): `oh config model … -j` (use `-p`) and `oh purge --include-opencode` (use `--include-tool-data`).
- `--force` / `-f` no longer skips a confirmation: use `--yes` / `-y`. On `oh purge`, `oh project remove`, `oh teams remove` and `oh service remove`, `--force` is still accepted for one version (same effect, warning, hidden from the help). `--force` stays where it overrides a safety: `oh daemon stop`, `oh remote setup`, `oh worktree remove`, `oh worktree cleanup` (the latter also deletes modified worktrees).
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
oh config set execution.env_allow "SSH_AUTH_SOCK, GH_TOKEN, MYCO_*"
```

- `oh doctor` lists the variables held back, by name only, and warns when a name that looks like a secret is allowed (the agent can read it and send it anywhere its tools reach).
- Still refused whatever you write: the credentials oh manages itself (`AWS_*`, `ANTHROPIC_API_KEY`, `OPENAI_API_KEY`, `GITLAB_TOKEN`, `GITHUB_TOKEN`, `OH_PASSPHRASE`…). `ZDOTDIR`, `BASH_ENV` and `ENV` are not in the base list.
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

## 17. Team mode (beta)

- **Beta.** Team mode is published with a single warning per process on stderr (`oh team …` commands, first claim of a launch), a warning in `oh doctor` (no effect on the exit code) and a "Beta" line at the top of the team board and team configuration. Known limits: the reservation (claim) is released when the session stops even with `cp-2` pending; **no expiry** (manual takeover after `takeover.stale_days`, 3 by default, `oh team claim transfer`); a release can be lost offline or if the daemon stops before the push (`oh team release <ticket> -p <project>`); `oh doctor` reports orphans. Details: [known limits](team-setup.en.md#known-limits). Full fix planned for 5.1.
- **Single GitLab token.** One search order for the tracker sync, `oh mr create` and `oh review --publish`, `oh review feedback`, the identity check of `oh team init|rejoin` and `oh mcp enable --project`: `GITLAB_TOKEN`, `tracker_token_key`, project token, team token, hub MCP token (key of `[mcp.gitlab] token`, then `openhub.mcp.gitlab.token`, then `gitlab-token`), then the former key `openhub.tracker.gitlab.token`. For `oh mr create`, the token of the project's team comes before the hub MCP token.
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
- **Global guard of the agents**: an agent can no longer run, from its shell, `oh mcp trust|untrust|setup|enable|disable|reset|serve`, `oh config set|unset|language|websearch`, `oh config model default|family|agent|unset`, `oh service setup|remove`, `oh secrets set|delete|get|cleanup`, `oh budget set|unset|raise`, nor what publishes or deletes: `oh workflow publish|archive|restore`, `oh brick publish`, `oh purge`, `oh project remove` (aliases `rm`, `p`), `oh teams remove|archive|restore`, `--yes` included. Reads (`list`, `show`, `diff`) stay allowed. This is a guard rail, not a barrier: run these commands yourself.
- **`oh serve`**: random token per launch (URL `?token=…`, then a cookie), 401 without a token, 403 if `Host` is not `127.0.0.1`, `localhost` or `[::1]`. Open the URL shown and do not share it.
- **Isolation**: the plugins of your personal opencode configuration are reported at launch and by `oh doctor`; `execution.strict_isolation = true` hides them and refuses an opencode package without a pinned fingerprint (default: `false`).
- Without any action from you: the credential proxy no longer relays the agent's headers as is, the MCP gateway of containers is closed by default, writes of secrets, configuration, reports and wiki are atomic.

Accepted limits and maturity of the features: [SECURITY.md](../../SECURITY.md).
