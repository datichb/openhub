> [Lire en francais](cli-mcp.fr.md)

# CLI Reference — MCP & Plugins

## MCP Management

MCP servers built into oh: `figma`, `github`, `gitlab`, `gslides`, `jira`, `linear`, `team`, plus the custom servers declared in `~/.oh/mcp/<name>/manifest.json`. A session gets the project MCP servers (or only those listed by the workflow `mcp:` field) in its session bundle. Per-server pages: [Figma](mcp-figma.en.md), [GitHub](mcp-github.en.md), [GitLab](mcp-gitlab.en.md), [Google Slides](mcp-gslides.en.md), [Jira](mcp-jira.en.md), [Linear](mcp-linear.en.md), [Team](mcp-team.en.md).

Every command below (except `list` and `serve`) accepts `-p, --project <name or ID>`.

### oh mcp enable

Enables an MCP service at hub level or for a project. With `--project` and no token, offers to inherit the hub token or to configure a new one.

```
oh mcp enable <service> [options]
```

| Flag | Short | Type | Description |
|------|-------|------|-------------|
| `--project` | `-p` | string | Project name or ID |

**Example:**

```bash
oh mcp enable figma
oh mcp enable gitlab --project my-project
```

---

### oh mcp disable

Disables an MCP service at hub level or for a project. With `--project`, the service is disabled for that project whatever the hub config.

```
oh mcp disable <service> [options]
```

| Flag | Short | Type | Description |
|------|-------|------|-------------|
| `--project` | `-p` | string | Project name or ID |

**Example:**

```bash
oh mcp disable figma
oh mcp disable gitlab --project my-project
```

---

### oh mcp reset

Removes the project override of a service (back to the hub config). Requires `--project`.

```
oh mcp reset <service> --project <name>
```

| Flag | Short | Type | Description |
|------|-------|------|-------------|
| `--project` | `-p` | string | **(required)** Project name or ID |

**Example:**

```bash
oh mcp reset figma --project my-project
```

---

### oh mcp setup

Configures an MCP service (interactive wizard: token, options). Services: `figma`, `gitlab`, `gslides`, `jira`; without argument, the service is chosen from a list. The write mode is offered for the services that have write tools (`gitlab`, `jira`). With `--project`, the token is stored in the keychain under a project-scoped key.

```
oh mcp setup [service] [options]
```

| Flag | Short | Type | Description |
|------|-------|------|-------------|
| `--project` | `-p` | string | Project name or ID |

**Example:**

```bash
oh mcp setup
oh mcp setup jira --project my-project
```

---

### oh mcp status

Shows the status of every MCP service. With `--project`, shows the effective config (project overrides included).

```
oh mcp status [options]
```

| Flag | Short | Type | Description |
|------|-------|------|-------------|
| `--project` | `-p` | string | Project name or ID |

**Example:**

```bash
oh mcp status
oh mcp status --project my-project
```

---

### oh mcp serve

Runs a built-in MCP server on stdin/stdout (JSON-RPC). Used by opencode, through the server declaration in the session bundle; rarely run by hand.

```
oh mcp serve <name> [--token-key <key>]
```

| Flag | Type | Description |
|------|------|-------------|
| `--token-key` | string | Keychain key to read the service token from (sets `FIGMA_TOKEN`, `GITLAB_TOKEN` or `GOOGLE_ACCESS_TOKEN` when not already set) |

Tools are exposed without the server name (`get_project`): the tool (opencode) prefixes it, the session sees `gitlab_get_project` (the name used by agents, skills and permission rules). A call under the former prefixed name (`gitlab_get_project` on the server side) is still accepted.

`oh mcp serve workflow` is internal: the `workflow` MCP server (see [workflow MCP server](#workflow-mcp-server)) injected into every session bundle, absent from `list`, `setup` and `enable`. Likewise, `oh mcp serve wiki` serves the [wiki MCP server](#wiki-mcp-server).

**Example:**

```bash
oh mcp serve gitlab
oh mcp serve figma --token-key openhub.mcp.figma.token
```

---

### workflow MCP server

Injected into every session started from a workflow; stateless, it passes each call to the oh daemon for the calling session (a subagent belongs to the oh session that delegated it). Agents see its tools as `workflow_<tool>`.

| Tool | Parameters | Role |
|------|------------|------|
| `workflow_status` | — | Workflow state: mode, checkpoints, next checkpoint, locked agents and operations, outputs |
| `workflow_checkpoint` | `id`, `summary`, `reports` (optional) | Passes a checkpoint (waits for the validation when it pauses in the mode); `reports` cites reports `rp-…` |
| `workflow_outputs` | `type`, `value`, `id` | Declares a typed output |
| `workflow_report` | `title`, `body` | Files the whole report of the agent; oh answers with its id `rp-<n>` |
| `workflow_report_get` | `id`, `section` (optional) | Reads a report, whole or one section (Markdown heading) |
| `workflow_reports` | — | Lists the reports of the session (id, author, title, size, time, sections) |

**Reports (`workflow_report`).** The agent files its whole report; oh keeps it and the coordinators only pass its id on. The author is the agent oh sees calling the tool (an `agent` parameter would be ignored). The reports filed since the previous checkpoint, and those the agent cites in `reports`, are attached to the checkpoint card; each filing enters the timeline (`▤ rp-2 (developer)`). Limits: 64 KiB per report, 100 reports and 4 MiB per session; beyond them the tool refuses and the agent includes the report in its block. Without the daemon (or when it keeps no report: `OH_WORKFLOW_REPORTS=off`), the tool answers "not recorded" without an error and the agent falls back on the whole report in its block. Agents never read nor write the reports other than through these tools (`~/.oh/sessions` refused by the oh guards, before the exceptions of the session scratch folders). User side: [`oh session report`](cli-sessions.en.md#oh-session-report).

When a checkpoint that unlocks the commit is passed, the answer of `workflow_checkpoint` may carry a "Wiki: …" line: the wiki proposals accepted on the card, applied by oh, and the files to commit with the code (see below).

---

### wiki MCP server

The project wiki (`docs/wiki/`), modernisation LD-J. Injected into the sessions started from a workflow when the project has a `docs/wiki/`; stateless, like `workflow`, it passes each call to the oh daemon (token of the server group) for the calling session. The wiki read is the one of the session folder (project or worktree); the proposal queue is the project's. Agents see its tools as `wiki_<tool>`.

| Tool | Parameters | Role |
|------|------------|------|
| `wiki_index` | — | Title and sections of the index, pages (title, `updated`, `confidence`), god nodes **computed by oh**, pending proposals (project, session) |
| `wiki_read` | `page`, `section` (optional) | Reads a page (path relative to `docs/wiki/`), or one section |
| `wiki_propose` | `page`, `content`, `tag`, `source` (required for `CONFIRMÉ`), `section`, `replaces` (optional) | Queues a proposal (an addition, or the replacement of a line with `replaces`); oh answers with its id `wp-<n>` |
| `wiki_pending` | `session_only` (optional) | Pending proposals (accepted or not), of the project or of the session |
| `wiki_apply` | `ids` | Applies proposals: **documentarian only** |
| `wiki_check` | `paths`, `base`, `head`, `since_sync` (optional) | Checks the wiki (frontmatter, links, tags, freshness, layout, god nodes), like [`oh wiki check`](cli-projects.en.md#oh-wiki-check); restricted to a change: commits, files, pages concerned and drift (`drift_confirmed`, `drift_god_node`) |

**Proposals (`wiki_propose`).** The author is the agent oh sees calling the tool (an `agent` parameter would be ignored); the tag line is written by oh (tag, author, date, source) and those the content carries are removed. The content is cleaned (control characters and terminal sequences removed) and neutralised: headings, frontmatter markers, HTML and comments (`oh:when` markers among them) escaped, an unclosed code fence closed. Any reference to the oh session data (`~/.oh/sessions`, the `scratch/` folders among them, or the sessions folder actually in use) is refused. Limits: 4 KiB and 60 lines per proposal, 50 open proposals per session, 500 per project. The same open proposal of the same session is not duplicated. Without the daemon, the tool answers "not recorded" without an error and the agent keeps the proposal in its report.

**Replacing a line (`replaces`, p5-fixes).** To correct an entry instead of adding a corrected one: `replaces` is the line of the page copied exactly (one line, spaces around it aside), `content` the new line (one line, cleaned and neutralised like an addition). The line must be in the page once (in the section `section` when given), outside the frontmatter and the code fences; a heading, a separator, a code fence or a tag line cannot be replaced. oh checks the line when the proposal is filed (missing or twice: refused) and again when it is applied: when it is gone, the proposal becomes `stale` and is not applied. Once applied, the new line takes the place of the old one (same indentation, same bullet), followed by its tag line written by oh; the tag line of the old entry, right after it, is removed. The card of the commit checkpoint (TUI), `oh wiki pending` and `wiki_pending` show the old line, then the new one; an edit on the card changes the new line.

**Validation and application.** The open proposals of the session show on the card of the commit checkpoint (TUI): accept, edit (on one line in the form, or on several in `$EDITOR`; content cleaned again; the original is kept) or refuse, one by one or all at once. Outside the TUI: [`oh wiki decide`](cli-projects.en.md#oh-wiki-decide). When the checkpoint is passed, oh applies the accepted proposals before giving control back to the agent (`✎ wp-…` entry in the timeline, status `applied`, `applied_by: oh`). The others stay queued ([`oh wiki pending`](cli-projects.en.md#oh-wiki-pending)), like every proposal of the workflows without a commit.

**`wiki_apply`.** Refused by a permission rule to every agent but the `documentarian`, for which it is asked to the user (`ask`); the daemon also refuses it (403) to any other agent it sees calling, whatever the rules of the session. It applies the pending or accepted proposals of the project.

**Remote session.** Same tools, served by the daemon of the job on the wiki of its clone; `wiki_apply` is refused there to every agent ("the proposals go back to the machine") and the commit checkpoint applies nothing. The proposals come back with the `wiki.json` artifact and enter, pending, the queue of the project at `oh session fetch` (schema 2: a proposal that replaces a line travels apart, which an oh of schema 1 leaves out instead of adding it; a `wiki.json` of a newer schema fails the import: update oh, then run `oh session fetch` again); the `wp-…` ids of the remote transcript are not renumbered (known limit); see [Living wiki › Remote sessions](../architecture/living-wiki.en.md#remote-sessions).

---

### oh mcp list

Lists every oh MCP server (`figma`, `github`, `gitlab`, `gslides`, `jira`, `linear`, `team`, then the custom servers of `~/.oh/mcp/`) and their command.

**Alias:** `oh mcp ls`

```
oh mcp list [options]
```

| Flag | Type | Description |
|------|------|-------------|
| `--json` | bool | Output in JSON format |

**Example:**

```bash
oh mcp list
oh mcp ls --json
```

---

### oh service (deprecated)

`oh service`, `oh service setup` and `oh service remove` still exist but are deprecated and hidden from the help: use `oh mcp status`, `oh mcp setup` and `oh mcp disable`.

---

## Plugin management

`oh plugin` (global opencode V1 plugins, RTK included) was removed in v5: plugins are declared per workflow (`plugins:`), see [Shipped workflows](workflows.en.md#plugins-and-code-mode).
