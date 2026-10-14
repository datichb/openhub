> [Lire en francais](tui-usage.fr.md)

# Guide -- Using the OpenHub TUI

> Practical guide to the oh v5 TUI: launch workflows, drive sessions, take decisions, manage workflows and settings.

## Quick start

```bash
oh
```

The TUI is the **control tower**: you launch sessions there, follow them and take decisions. The agent's work happens in the opencode window, opened next to it (iTerm2 or Terminal.app tab, tmux, browser). Closing opencode does not stop the session. See [v5 sessions](sessions-v5.en.md).

On the first start without configuration, the `oh init` wizard opens in the TUI (see [Getting started](getting-started.en.md#initial-setup-oh-init)).

---

## Navigation

### Global shortcuts

| Key | Action |
|-----|--------|
| `Ctrl+P` or `/` | Open the omnibar (`/`: when the view does not use it to search) |
| `?` or `F1` | Help (global shortcuts, form and wizards, keys of the current view) |
| `Esc` | Close the toasts shown; otherwise back to the previous view; at the root of a project or team mode, back to Hub mode. In the launch form and the wizards, `Esc` keeps its meaning (close the form, « Quit the wizard? ») and does not close the toasts |
| `Ctrl+T` | Enter team mode (choice when there are several teams); from team mode, back to the Hub |
| `Ctrl+Q` (or `Ctrl+C`) | Quit (see [Quitting oh](#quitting-oh)) |
| `j` / `k`, `g` / `G` | Down / up, top / bottom of a list (in the Sessions view, `g` fetches a remote session) |
| `{` / `}` | Previous / next section in sectioned lists |
| `Tab` | Switch column or field (`h` / `l` on two-column landings) |
| `Enter` | action of the selected list, button or field (the omnibar opens only when nothing uses it) |

Action results show up as toasts (success, error, info) that disappear by themselves (`Esc` closes them, except in the launch form and the wizards). Each view shows its shortcuts at the bottom.

**Changed in v5.0**: typing a letter « anywhere » no longer opens the omnibar (use `Ctrl+P` or `/`); `Enter` and letters always go to the selected list or field; `d` no longer dismisses toasts (`Esc`). `Esc`, `Ctrl+P`, `Ctrl+T`, `?`, `F1` and `Ctrl+Q` mean the same in every view. Every destructive action (removing a worktree, cleaning up, releasing a ticket, rejecting a wiki proposal…) asks for a confirmation.

**Buttons** (windows, forms, wizards, recap): one style everywhere. The primary action is written in bold in the mode color; the focused button has a light background, framed `▸ label ◂`. On opening, the primary action is focused ("Cancel" for a dangerous confirmation, such as a deletion). `Tab` / `Shift+Tab` (or `←` / `→`) move from one button to the next, all of them included; `↓` / `↑` scroll the text of a window that is too long; `Enter` activates the focused button.

### Small terminals

The layout follows the terminal size:

| Size | Layout |
|------|--------|
| 32 rows and more | full: large banner, footer, 8-row bottom bar |
| 16 to 31 rows | **compact**: one-line banner, no footer nor spacing, 3-row bottom bar (≈ 19 list rows in 80×24) |
| under 60 columns or 16 rows | « Terminal too small (W×H) — 60×16 minimum » message; only `Ctrl+Q` works |

Switching between layouts keeps the selected item. Setting `[cli] compact = auto | always | never` (Settings › CLI › Compact mode, or `oh config set cli.compact always`): `auto` (default) follows the height, `always` forces the compact layout, `never` the full one (the « too small » message remains).

### Sessions badge

The bottom bar shows `● N ⏸ M` everywhere: `N` live sessions (neither sleeping nor finished), `M` waiting decisions. The badge turns to the alert color when `M > 0`, and a toast announces each new decision. System notifications come from the oh daemon, even with the TUI closed.

### Modes

| Mode | Landing content |
|------|-----------------|
| **Hub** | Start, Running sessions, System (Configuration = Settings, Metrics, Doctor, Secrets), Projects (known projects, then "Add a project"), Teams |
| **Project** | Start (with categories), Project sessions, Project (Board, Metrics, Status), Configuration (Project Config, Worktrees, Project workflows), Team |
| **Team** | Start (with categories), Team sessions, Board (team board, status, activity, takeover briefs), Configuration (patterns, policies, team detail, team workflows), Navigation |

Selecting a project or a team on the Hub landing opens its mode. Started from the directory of a registered project, `oh` opens in Project mode. The omnibar only shows the commands of the active mode. On the landings, texts that are too long are cut with "…" and the two columns widen with the terminal (up to 180 columns).

---

## Start

The **Start** section is the same on the three landings:

| Line | Content |
|------|---------|
| `◆` | the project default workflow (Project config › Execution), if any |
| `★` | pinned workflows (at most 5 per scope) |
| `·` Recent / `↺` | the last 3 launched workflows (pinned ones excluded), with their age |
| `▸ Develop (N)`, `Frame`, `Quality`, `Knowledge`, `Other` | categories (Project and Team modes): `Enter` lists their workflows |
| `… All workflows (N)` | opens the workflow catalogue |

| Key | Action |
|-----|--------|
| `Enter` | open the workflow's launch form |
| `*` | pin / unpin (scope of the landing: whole hub, this project or this team) |

When no workflow is available, a "Free session" line replaces the list.

---

## Launch form

Opened from "Start", the catalogue, the omnibar (`run <workflow>`) or the board (`a` on a ticket). It is generated from the workflow and has three steps:

| Step | Content |
|------|---------|
| **Inputs** | the workflow inputs: ticket (field + "Pick…" button), text, list (`enum`), checkbox (`bool`), branch… Required fields are marked |
| **Options** | Mode (allowed modes), Runtime (`⌂ local`, `▣ container`, `☁ remote`, with the reason when unavailable), Location (base, existing worktree, "+ new worktree"), Opening, and "A single session for every ticket" with several tickets |
| **Recap** | agents, skills and budget, MCP, Code Mode, allowed Beads commands, checkpoints in the chosen mode (mandatory ones marked), isolation, sessions ("N sessions · 1 server · one worktree per writing session"), warnings (decisions already waiting, preconditions). When a writing session would start in a directory with uncommitted changes: `[w]` new worktree (default, your changes stay untouched) · `[s]` stash · `[b]` launch anyway |

| Key | Action |
|-----|--------|
| `Tab` | next field |
| `Ctrl+S` | go to the recap (from any step); on the loaded recap: launch |
| `Alt+←`, `PgUp` or `Ctrl+B` | previous step |
| `Alt+→` or `PgDn` | next step |
| `Esc` | close the form |

A launch always starts from the recap: a first `Ctrl+S` shows it, a second one launches once the recap is computed. The "Next", "Back" and "Launch" buttons do the same. While launching, the form shows "Launching…" (a second `Ctrl+S` is ignored); in a container, the image build progress shows below. If the provider has no credentials, the recap says so with a "Configure the provider" button: it opens the Provider view, and back here (`Esc`) the recap checks again. "Launch" is the primary action of the recap.

**Ticket already worked on**: when an open session already works on the ticket, the recap compares it with the launch ("Existing session / New launch" table: state, workflow, runtime, mode, model, directory / branch, work; differences marked ⚠). A session of another workflow is shown for information. For a session of the same workflow, a choice is needed (a choice launches at once; "Launch" / `Ctrl+S` are no longer enough):

| Key | Button | Effect |
|-----|--------|--------|
| `r` | Resume | open the existing session again, no new session |
| `t` | Resume locally / in a container / with these settings | transfer the session and its conversation to the chosen runtime and model; on failure ("Transfer failed"), the session is unchanged and oh offers "Replace" |
| `k` | Replace (keep the work) | stop the session; the new one continues in the same directory and branch, with a handoff brief |
| `d` | Delete and relaunch | delete the session and its work (confirmation: what will be deleted and lost, then, when it worked in the project directory, keep or discard its changes), then launch again |
| `p` | Launch in parallel | new session beside it, in another worktree |

See [Sessions v5 › Launching on a ticket already worked on](sessions-v5.en.md#launching-on-a-ticket-already-worked-on).

### Beads ticket picker

The "Pick…" button of a ticket input opens the picker inside the form:

| Key | Action |
|-----|--------|
| `/` | search (id, title, label); `Enter` or `↓` goes back to the list, `Esc` clears |
| `f` | switch the label filter (by default the workflow's, e.g. `ai-delegated`) |
| `e` | switch the epic |
| `Space` | select / unselect (when the workflow takes several tickets) |
| `Enter` | pick (or confirm the selection) |
| `Esc` | cancel |

A preview of the ticket (acceptance criteria) shows below the list; a ticket claimed by another member is flagged. Several tickets = one session per ticket, unless "A single session for every ticket" is checked.

---

## Sessions view

`sessions` command (aliases `parallel`, `inbox`), or the Sessions section of the landings (at most 3 lines, then "All sessions").

| Section | Content |
|---------|---------|
| **To handle** | sessions with waiting decisions, each followed by its decisions: `⏸` checkpoint, `?` question, `!` permission, `$` budget, `✗` error or circuit breaker (a session shows only once in the view, with its short ID) |
| **Running** | live sessions (working, waiting, idle) |
| **To fetch** | remote sessions (GitLab CI): pipeline running, MR ready, Beads journal to replay |
| **Sleeping** | sessions whose server sleeps; they can be resumed |
| **Finished, 7 days** | sessions stopped or finished less than 7 days ago |

The **Detail** pane shows the selected session: workflow, mode, runtime, directory, bundle, state, agent, cost, the checkpoint **timeline** (`✔ cp-1 10:03 → developer (3) → reviewer → ⏸ cp-2 → ○ cp-3`), the open decisions and, when a follow-up is known, "↪ Chain with <workflow> (e)".

| Key | Action |
|-----|--------|
| `Enter` | on a decision: open its card; on a session: show / hide the feed |
| `y` | approve (permission: once; checkpoint: Validate) |
| `n` | reject a permission; on a checkpoint: card on "Fix first" |
| `x` | dismiss an alert (`✗` error, `$` budget, `✗` circuit breaker) |
| `a` | attach (open the opencode window; resumes a sleeping session) |
| `A` | open with…: automatic, iTerm2, Terminal.app, tmux, browser, here (oh suspended) |
| `t` | live feed |
| `m` | send an instruction (taken at the next step) |
| `i` | interrupt the current step |
| `M` | switch the model for the next steps (`provider/model`) |
| `s` | stop the session (confirmation) |
| `B` | validate by batch: the waiting checkpoints of the fan-out of the selected session (else all of them), each one to validate, leave waiting or refuse with an instruction; confirmation, then the result of each one (as `oh session approve --batch`) |
| `D` | delete the session for good: worktree, local branch (never the remote one), decisions, files (confirmation, "Cancel" focused) |
| `c` | resume a sleeping session |
| `o` | results and MR description |
| `w` | open in the browser (one-time code) |
| `e` | chain with another workflow |
| `g` | fetch a remote session |
| `f` | filter: active project / every project |
| `r` | refresh |

### Checkpoint card

`Enter` on a `⏸` opens the card: title `⏸ cp-2 · Commit or fix · ticket · my-app`, the agent's summary, **Changes** (`+a −d · N file(s)`, 8 files listed), **Last messages**, **Timeline**. Actions:

- **Decide**: form with three choices and a "Message to the agent":

| Choice | Effect | Message |
|--------|--------|---------|
| **Validate** | the checkpoint passes | optional |
| **Fix first** | the agent fixes, then asks for the checkpoint again | required |
| **Other instruction** | the agent follows the instruction, then asks again | required |

- **Full diff**: the whole session diff (when it has changes).
- **Attach**: open the opencode window.

CLI equivalent: `oh session approve <id> --decision once|fix|other|reject [-m "…"]`.

### Other cards

- **Permission** (`!`): action, resource, agent; once / always / reject, with a message. "Always" is refused when the workflow requires strict isolation.
- **Question** (`?`): form generated from the agent's fields (choices, "other answer" when allowed, boolean, multiple choice, text).
- **Alert** (`✗`, `$`): dismiss or attach. The circuit breaker (`✗`) suspends delegations after N delegations in a row; dismissing it unblocks them.

When the decision was already taken elsewhere (opencode window, browser, CLI), the card says so: the first answer wins.

### Live feed (`t`)

A pane opens on the right: current agent, called tools and their result, delegations, messages, cost in the title. It follows the selected session (moving to another line switches the feed). `t` closes it. Without the oh daemon, the feed is unavailable.

### Chain with… (`e`)

`e` offers the workflows with an input that takes an output of the session (branch, tickets, path), for example `review` after `ticket`. The choice opens the launch form prefilled, linked to the previous session. When a workflow session stops, a toast reminds you: "✔ … finished · $… — Sessions › e: chain with …".

### Fetching a remote session (`g`)

On a "To fetch" line: report (artifacts, session import, deferred checkpoint), then "Replay the journal" for Beads. A ticket changed in the meantime opens a conflict window: "Keep local", "Apply remote", "Merge the notes" or "Later". See [Remote execution](remote-runners.en.md).

---

## Quitting oh

`Ctrl+Q` (or the `quit` command). When sessions still work, the "Quit oh — N session(s) still working" window offers, **for each one**:

| Choice | Effect |
|--------|--------|
| **Finish step, sleep** (default) | the current step finishes, then the session goes to sleep |
| **Background** | the session goes on without the TUI |
| **Stop now** | the session is stopped |

Waiting or idle sessions are put to sleep right away. `Esc` cancels quitting. When you come back, a message sums up what happened while you were away (changed sessions, decisions to handle, cost).

---

## Workflow catalogue

`workflows` command (or "All workflows", "Project workflows", the team "Workflow" entry). Workflows are grouped by layer: **Hub · built in** (read only), **Team**, **Project**, **My drafts**. Each line shows the version, the risk, the possible runtimes (`⌂ ▣ ☁`), the validation state and the `extends` chain.

| Key | Action |
|-----|--------|
| `Enter` | launch (launch form); on a draft: test it |
| `n` | new workflow (empty, extension or copy) |
| `e` | edit; on a hub workflow: extend it; on a published workflow: create a draft |
| `v` | validate (findings) |
| `t` | test the draft (launch with `--draft`) |
| `p` | publish the draft |
| `D` | diff of the draft with the published version |
| `h` | version history |
| `x` | archive a published workflow, discard a draft |
| `*` | pin |
| `r` | reload |

**Publishing** (`p`): validation, impact (rights and controls changed), diff, required message; `Ctrl+S` publishes. Offline, the publication is queued and replayed at the next synchronization. **History** (`h`): `Enter` diff with the current version, `r` restore (published again as a new version). See [Team workflows](team-workflows.en.md).

### Editor

Sections (`Tab` / `Shift+Tab`): **General**, **Graph**, **Inputs & prompt**, **Resources**, **Bundle preview**. Each field shows its origin (inherited or written in the draft) and whether the parent workflow locks it.

| Key | Action |
|-----|--------|
| `Enter` | edit the field (Bundle preview: go to the field of the finding) |
| `x` | back to the inherited value; remove an element |
| `a` | add an agent (Graph) or an input (Inputs & prompt) |
| `c` | add a checkpoint (Graph) |
| `m` | change the shown mode (Graph) |
| `P` | edit the prompt template in `$EDITOR` |
| `y` | edit the YAML in `$EDITOR` |
| `u` / `U` | undo / redo |
| `w` or `Ctrl+S` | save the draft |
| `Esc` | close (asks what to do when changes remain) |

---

## Brick catalogue

`bricks` command (aliases `briques`, `agents`, `skills`). Read-only list of the agents and skills: origin (hub or team), family, estimated cost in tokens, dependencies, workflows using them.

| Key | Action |
|-----|--------|
| `Enter` | open a workflow that uses the brick |
| `/` | search (id, name, description) |
| `f` | agents / skills / all |
| `Esc` | back |

---

## Cleanup of former deployments

`cleanup` command (aliases `nettoyage`, `deploy-cleanup`). oh v5 no longer deploys anything into projects; the screen lists, per project, the leftovers of former deployments (`.opencode/agents`, `.opencode/skills`, keys written by oh in `opencode.json`…) and what is kept (your own keys). Buttons: **Clean**, **Show the diff**, **Later**. CLI equivalent: `oh migrate deploy-cleanup`.

---

## Settings

`settings` command. The v5-specific sections:

| Section | Fields |
|---------|--------|
| **Sessions** | Open sessions in (auto, iTerm2, Terminal.app, tmux, browser, suspend), iTerm2 style (tab, split, window), Sleep after (minutes, default 5) |
| **Session restrictions** (off when empty) | Max working sessions, Budget per session (USD), Daily budget (USD), Memory cap (MB), Allowed models |
| **Execution** | Default runtime, Container engine (auto = Colima, then Podman, then Docker), Image cache, Pinned opencode version, Strict isolation |
| **Remote (GitLab CI)** | per target: instance, `oh-runner` project, token key (read only), tag, image builder, architecture, maximum duration; applied by the next `oh remote setup` |

The other sections (General, CLI, Opencode, Workflows, MCP, Worktree, Tracker) keep their role. In configuration views: `j`/`k` to navigate, `Enter` to edit; changes are saved.

**Project config** (`project-config`) has its **Execution** section: Dev Dockerfile, Build args, Cache volumes, Default workflow, Default runtime. See [Container sessions](container.en.md).

---

## Omnibar

Fuzzy matching on names and aliases:

```
> tick        → run ticket (launch form)
> secu        → run audit
> brick       → brick catalogue
> inbox       → Sessions view
```

### Sessions and workflows

| Command | Description |
|---------|-------------|
| `run <workflow>` | launch form of a catalogue workflow. Aliases: `dev` → `run ticket`, `start` → `run feature`, `onboard` → `run onboarding`, `feedback` → `run review-feedback`, `secu`/`perf`/`archi` → `run audit`, `rev` → `run review`, `dbg` → `run debug`, `fast` → `run quick` |
| `run <workflow> ⟨bd-42⟩` | from a board: workflow on the selected ticket |
| `coder` | Free session (`libre` workflow, agent of your choice) — Project and Team modes |
| `sessions` | Sessions view (aliases `parallel`, `inbox`) |
| `workflows` | workflow catalogue (aliases `catalogue`, `wf`) |
| `bricks` | brick catalogue |
| `cleanup` | cleanup of former deployments |
| `review.publish` | publish a review (MR + notification), when GitLab write access is enabled |

### Projects and configuration

| Command | Description |
|---------|-------------|
| `projects` | project list (`a` add, `d` remove, `n` rename, `m` move, `p` Project mode, `b` initialize Beads, `r` refresh, `Enter` configure) |
| `project add` | project wizard |
| `board`, `board init` | project Beads board, initialization |
| `project-config` | project configuration (including Execution) |
| `worktrees` | git worktrees of the project (`a` add, `d` remove, `o` open a session, `p` prune the references, `x` remove the merged worktrees, never one used by a session; `d`, `p` and `x` ask for a confirmation) |
| `settings`, `models`, `provider`, `mcp`, `secrets` | settings, models, provider (Bedrock region checked as it is typed, message under the field), MCP servers, secrets |
| `teams`, `team-detail` | teams, team detail (`K` provider key for the team; solo space: "Switch to a team") |
| `init` | run the setup wizard again |

### Team (with a team-state repository)

| Command | Description |
|---------|-------------|
| `team board`, `team status`, `team activity` | team board, status, activity |
| `team briefs`, `team patterns`, `team policies`, `team wiki` | takeover briefs, patterns, policies, wiki |
| `team sync` | synchronize the claims with the tracker |
| `team init`, `team rejoin` | create, rejoin a team |

### System and navigation

| Command | Description |
|---------|-------------|
| `status`, `doctor`, `metrics`, `notifications`, `help` | status, diagnostics (`r` to rerun), metrics, notifications, help |
| `home`, `project mode`, `hub mode` | home, Project mode, Hub mode |
| `quit` | quit (same window as `Ctrl+Q`) |

Commands removed in v5: `deploy`, `sync`, `plugins`, `upgrade`, the Workflow view (replaced by the catalogue) and the former parallel view (replaced by Sessions).

---

## Inline wizards

`init`, `project add` and `team init` open wizards inside the TUI. The omnibar and toasts stay available.

| Key | Action |
|-----|--------|
| `Ctrl+S` | submit the step |
| `Alt+←`, `PgUp` or `Ctrl+B` | previous step |
| `Esc` | « Quit the wizard? »: **Quit**, **Skip the step** (when optional) or **Continue** (the default; `Esc` too) |

A summary screen ends the wizard: `Enter` opens the detail view, `Esc` goes back to the previous view.

---

## Tips

- **Launch without leaving the board**: `a` on a ticket lists the workflows that take a ticket; the form opens at the Options step, ticket prefilled.
- **Decide fast**: `y` on a "To handle" line approves a permission or a checkpoint without opening the card.
- **Several projects**: `f` in the Sessions view switches between the active project and every project.
- **Contextual help**: `?` lists the shortcuts of the current view.
