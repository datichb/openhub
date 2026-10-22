> [Lire en francais](cli-projects.fr.md)

# CLI Reference — Projects

## Projects

`oh project` manages the projects registered in the hub. **Alias:** `oh p`.

### oh project list

Lists the registered projects.

**Alias:** `oh project ls`

```
oh project list [options]
```

| Flag | Short | Type | Description |
|------|-------|------|-------------|
| `--status` | `-s` | string | Filter by status (`active`, `archived`) |
| `--json` | | bool | Output in JSON format |

**Example:**

```bash
oh project list
oh project ls -s active
oh project list --json
```

---

### oh project add

Registers a project in the hub. Without flags, runs an interactive wizard.

**Alias:** `oh project register`

```
oh project add [options]
```

| Flag | Short | Type | Description |
|------|-------|------|-------------|
| `--name` | `-n` | string | Project name |
| `--path` | `-d` | string | Project path (default: current directory) |
| `--language` | `-l` | string | Main language |
| `--yes` | `-y` | bool | Beads zero impact: remove what `bd init` wrote without a confirmation and undo its commit when it was not pushed (otherwise: listed, the undo of the commit included, then a confirmation; without a terminal nothing changes) |

**Example:**

```bash
oh project add -n api -d ./services/api -l go
oh project register -n frontend --language typescript
oh project add
```

---

### oh project remove

Removes a project from the registry (files on disk are not deleted).

**Alias:** `oh project rm`

```
oh project remove [project-id] [options]
```

| Flag | Short | Type | Description |
|------|-------|------|-------------|
| `--yes` | `-y` | bool | Remove without confirmation (required without a terminal). Former names: `--force`, `-f` (deprecated, same effect, with a warning) |

**Example:**

```bash
oh project remove my-project
oh project rm -y legacy-app
```

---

### oh project rename

Changes the display name of a project. The ID does not change. Interactive when arguments are omitted.

```
oh project rename [project-id] [new-name]
```

**Example:**

```bash
oh project rename api api-v2
oh project rename
```

---

### oh project move

Updates the registered path of a project (after moving it on disk). Does not move the folder. Interactive when arguments are omitted.

```
oh project move [project-id] [new-path]
```

**Example:**

```bash
oh project move api ../new-location/api
oh project move
```

---

### oh project configure

Configures the settings of a project: LLM provider, model, language. They are used when sessions are launched (`oh run`). Without flags, runs an interactive wizard.

```
oh project configure [project-id] [options]
```

| Flag | Short | Type | Description |
|------|-------|------|-------------|
| `--provider` | `-P` | string | LLM provider: `bedrock`, `anthropic`, `openrouter`, `github-copilot` (value not checked) |
| `--model` | | string | LLM model (the former short form `-m` is still accepted, with a warning) |
| `--language` | `-l` | string | Main language |

The default workflow and runtime of the project are set in the TUI (Project config › Execution), see [`oh run`](cli-workflows.en.md#oh-run).

**Example:**

```bash
oh project configure api -P anthropic -m claude-sonnet-4-20250514
oh project configure api -l go
oh project configure
```

---

## Merge requests

### oh mr create

Creates (or finds) the GitLab merge request of the current branch, against the base branch of the project, and optionally assigns a team reviewer. Replaces `oh review --publish`. The review itself is a workflow: [`oh run review`](cli-workflows.en.md#oh-run). Requires GitLab write access (`oh mcp setup gitlab`); merging remains a manual action.

```
oh mr create [-p <project>] [--reviewer <member>]
```

| Flag | Short | Type | Description |
|------|-------|------|-------------|
| `--project` | `-p` | string | Project (detected from the current folder) |
| `--reviewer` | | string | ID of the team member to assign as reviewer |

```bash
oh mr create
oh mr create --reviewer alice
```

---

## Project wiki

### oh wiki check

Checks the project wiki (`docs/wiki/` of the closest parent folder that has one): frontmatter (`updated` a YYYY-MM-DD date, `confidence` in `confirmed | mixed | uncertain` and consistent with the tags of the page, `agents`; no other key, but `synced` in `index.md`: unknown key `frontmatter_key`, a warning, an error in a spec of `specs/` as at `verify.spec`), relative links and anchors, tag lines (`` — `TAG` · agent · YYYY-MM-DD[ · source] ``, a source for `CONFIRMÉ`), pages without a tag (but `specs/`, `decisions/`, `postmortems/`, `retros/`), layout (a page out of `index.md`, `glossary.md`, `technical/`, `business/`, `specs/`, `decisions/`, `postmortems/`, `retros/`), freshness (a page older than the last commit of a file it cites; a missing cited file) and god nodes computed by oh against the table of `index.md`. Also shows the computed god nodes. Non-zero exit code on errors (usable in CI).

```
oh wiki check [dir] [--fix] [--json] [--path <file>]… [--base <ref> [--head <ref>]] [--since-sync]
```

| Flag | Type | Description |
|------|------|-------------|
| `--fix` | bool | Repairs what oh can compute: `confidence` of the pages, god nodes table of `index.md` (and its `updated`) |
| `--json` | bool | JSON report (`pages`, `god_nodes`, `issues` with `page`, `line`, `kind`, `severity`, `message`, `fixable`; `scope` for a change) |
| `--path` | string, repeatable | Restricts to a change: the changed files (relative to the root) |
| `--base`, `--head` | string | Restricts to the files changed from the merge base of `--base` and `--head` to `--head` (without `--head`: the working tree, uncommitted files included) |
| `--since-sync` | bool | Restricts to the commits since the last synchronization (`synced:` of `index.md`, else the last commit of `docs/wiki/`) |

Restricted to a change, the check keeps only the issues of the wiki pages it modifies, lists the commits, the files and the **pages concerned** (that cite a changed file or hold a touched god node), and adds the drift: `drift_confirmed` (a `CONFIRMÉ` page that cites a changed file, not updated) and `drift_god_node` (a touched god node none of whose pages is updated). It is the documentation axis of the `reviewer` and the starting point of [`doc-sync`](workflows.en.md#doc-sync).

oh never follows a symbolic link in `docs/wiki/`; a page larger than 256 KiB, or over 400 pages, is not read (warning).

### oh wiki pending

Lists the agents' proposals for the project wiki ([`wiki_propose`](cli-mcp.en.md#wiki-mcp-server)) still to apply: accepted on the commit card (oh applies them when the checkpoint is passed) or pending (workflows without a commit, for `doc-sync`).

```
oh wiki pending [dir] [--all] [--session <id>] [--json]
```

| Flag | Type | Description |
|------|------|-------------|
| `--all` | bool | Every proposal, applied and refused ones included |
| `--session` | string | Only those of a session |
| `--json` | bool | JSON output |

The project is the one oh knows for the folder (or a parent folder). A line replacement (`replaces`) is shown "old line" → "new line"; a proposal whose line is gone when applied is `stale` (not applied, shown with `--all`). With `--all`, a proposal refused with a reason adds a `REFUSAL REASON` column (the reason is also the `reason` field of `--json`). To decide on a proposal outside the TUI: [`oh wiki decide`](#oh-wiki-decide).

### oh wiki decide

Records the decision of the user on proposals of the queue, outside the TUI: the same choices as on the card of the commit checkpoint, the same service.

```
oh wiki decide <id>... (--accept | --edit | --content <text> | --file <file|-> | --refuse [--reason <reason>] | --pending) [--dir <folder>]
```

| Flag | Type | Description |
|------|------|-------------|
| `--accept` | bool | Accept: oh applies it when the commit checkpoint of its session is passed, else `doc-sync` |
| `--edit` | bool | Edit the content in `$EDITOR` (several lines), then accept; an emptied or unchanged content records nothing |
| `--content` | string | Edit without an editor: the new content |
| `--file` | string | Edit without an editor: file of the new content (`-`: standard input) |
| `--refuse` | bool | Refuse (alias `--reject`) |
| `--reason` | string | Reason of a refusal (optional, with `--refuse` only) |
| `--pending` | bool | Put back to pending |
| `--dir` | string | Folder of the project (default: the current folder) |

Exactly one decision; an edit is about one proposal. The edited content is cleaned like a proposal and the original is kept. The ids (`wp-<n>`) are those of the project, given by `oh wiki pending`; a proposal already applied or refused can no longer be changed. With several ids, oh decides on each one and reports those that fail (non-zero exit code). The decision is signed `oh` (`decided_by`).

**Reason of a refusal.** `--reason` records why the proposal is refused, like the optional reason asked by `n` in the TUI. The reason is cleaned (one line, no control characters) and bounded to 500 characters; empty, it is simply left out. It is kept with the proposal (`reason` column, shown by `oh wiki pending --all` and by the TUI on the refused proposal). An acceptance, an edit or a move back to pending takes no reason: `--reason` without `--refuse` is an error.

### oh wiki stats

Metrics of the wiki, computed by oh: pages (specs, decisions, postmortems among them), **coverage of the domains** (pages `business/<domain>.md` and domains of the map of the index; covered = the page exists and holds at least one tagged enrichment), **median age of the pages** (days since `updated`, and the oldest), **share of `INCERTAIN`** among the tagged enrichments, **proposals waiting** (queued and accepted not applied) and last synchronization (`doc-sync`). Same figures in the "Wiki" view of the project mode of the TUI.

```
oh wiki stats [dir] [--json]
```
