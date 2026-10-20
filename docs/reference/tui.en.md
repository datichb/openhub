> [Lire en français](tui.fr.md)

# TUI reference — OpenHub

> Reference of the terminal interface (TUI) of oh: keys, palette commands, views and their actions. For step-by-step walkthroughs, see the [TUI guide](../guides/tui-usage.en.md).

## Launching

```bash
oh          # Starts the TUI (interactive terminal detected)
oh --no-tui # Forces the classic CLI mode
```

The TUI starts when `oh` runs without a subcommand in an interactive terminal. Variables that disable it: `CI=true`, `TERM=dumb`, `OH_RICH_TUI=0`.

When no provider is configured (first launch), the initial setup wizard opens in the TUI. See [InlineWizardView](tui-inline-wizard.en.md).

## Layout

```
┌──────────────────────────────────────────────────────────────────┐
│                         CONTENT AREA                             │
│                   (adapts to the active view)                    │
├──────────────────────────────────────────────────────────────────┤
│  hints line of the view (keys) or command palette                │ Omnibar
│  🏠 Hub · ● 2 ⏸ 1                                                │ Mode bar
└──────────────────────────────────────────────────────────────────┘
```

The mode bar shows the sessions badge **`● N ⏸ M`** (N sessions working, M decisions waiting); it turns to the alert colour when a decision waits.

**Small terminals**: under 32 rows, compact layout (one-line banner, omnibar and mode bar on 3 rows); under 60×16, a « Terminal too small » message and only `Ctrl+Q` works. Setting `[cli] compact = auto | always | never`. Switching layouts keeps the selected element.

## Global keys

| Key | Action |
|-----|--------|
| `Ctrl+P` | Command palette |
| `/` | Filter the list of the view; when the view does not filter: palette |
| `?` / `F1` | Help: global keys, convention, keys of the active view, legend, commands |
| `Esc` | Closes the toasts first; else goes back (previous view); at the root of a Project or Team mode: back to the Hub |
| `Ctrl+T` | Team mode (choice when there are several teams); from the Team mode: back to the Hub |
| `Ctrl+Q` / `Ctrl+C` | Quit (when sessions work: finish the step then sleep, background, or stop) |
| `Enter` | Action of the selected list, button or field; palette only when nothing uses it |

**Key order**: windows (forms, confirmations) and the palette get every key; then the global keys; then the keys of the view; then the selected list or field. Typing a letter « anywhere » does not open the palette.

**Views that keep the keys**: the launch form, the wizards, the workflow editor and the publication keep `Esc` (close, « Quit the wizard? ») and do not close the toasts; `F1` opens the help there. The tabs of the detail of a session and the batch validation also keep `Esc` (back to the detail, cancel the batch), but `?` and `Ctrl+P` stay available there, and the hints line shows the keys of the tab or of the batch.

**Buttons**: one style in windows, forms, wizards and the recap. The main action is bold; the selected button is framed `▸ label ◂`. On opening, the main action is selected (« Cancel » for a dangerous confirmation); `Tab` / `Shift+Tab` (and `←` / `→`) go through the buttons; `↓` / `↑` scroll the text of a window.

## Key convention

A key has the same meaning in every view. A view may give a free letter to an action of its own (`a` attach, `p` relaunch the driver, `B` batch…), but never gives a key of the convention another meaning, nor an action of the convention another key. The help (`?`) shows this table and offers to see the « New keys » message again, shown once at the first launch after the update.

| Context | Key | Action |
|---------|-----|--------|
| Everywhere | `↑` `↓` / `j` `k` | Move up / down |
| Everywhere | `g` / `G` (`Home` / `End`) | First / last element |
| Everywhere | `/` | Filter the list; without a filter in the view: palette |
| Everywhere | `r` | Refresh |
| Everywhere | `?` / `F1` | Help and keys |
| Everywhere | `Enter` | Open the selected element |
| Everywhere | `Esc` | Back (closes the toasts first) |
| List | `n` | Create |
| List | `e` | Edit |
| List | `d` | Delete, **always confirmed** |
| List | `x` | Archive or file away, **never destructive** |
| List | `R` | Resume or restore (sleeping session, remote session, version of a workflow) |
| List | `c` | Compare (diff) |
| Decision | `y` | Accept (checkpoint, wiki proposal, element of a batch) |
| Decision | `n` | Refuse, with an instruction; the validated instruction is the confirmation |
| Decision | `e` | Modify before accepting |
| Decision | `s` | Leave pending |
| Decision | `A` | Accept all |
| Reader | `/` | Search the text (trace, reports) |
| Reader | `n` / `N` | Next / previous match |

`e`, `s`, `A` and `c` carry an action of the view when it has not the action of the convention (`e` chain in Sessions, `s` stop a session, `A` open with…).

### Changed keys

| View | Before → now |
|------|--------------|
| Sessions | `D` → `d` delete · `c` → `R` resume · `g` → `R` recover a remote session · new: `B` batch validation |
| Recap of an existing session (launch form) | `r` → `R` resume · `t` → `T` transfer |
| Wiki proposals | `a` → `y` accept · `x` → `n` refuse |
| Workflows | `D` → `c` compare · `x` → `d` delete a draft (`x` only archives published workflows) |
| History of a workflow | `r` → `R` restore (`r` refreshes) |
| Workflow editor | `x` → `d` remove or go back to the inherited value · `y` → `Y` raw YAML |
| Patterns | `a` → `n` create · `v` → `y` validate |
| Projects | `a` → `n` create · `n` → `e` rename |
| Team board | `f` → `/` filter · `x` → `L` release |
| Worktrees | `a` → `n` create · `x` → `M` clean up merged ones |
| Team detail | `y` → `T` discover the tracker |
| Other lists (models, policies, secrets, teams…) | `a` → `n` create |

## The omnibar

At the bottom of the screen. At rest, it shows the hints line of the view (or of the tab); `Ctrl+P` (or `/` outside a filter) opens it for input, with the suggestions above:

```
│  ● run audit           Project audit (security, performance…)     │
│  ○ run review          Code review of a branch…                   │
├───────────────────────────────────────────────────────────────────┤
│  > aud_                                                           │
```

| Key | Action |
|-----|--------|
| Typing | Filters the commands (fuzzy, on the label and the aliases) |
| `↓` / `Tab`, `↑` / `Shift+Tab` | Go through the suggestions |
| `Enter` | Run the selected command |
| `Esc` | Close; the focus goes back where it was (tab included) |

## Palette commands

The Command column gives the id; the label or an alias can be typed too. Each term has one meaning. « Project »: in the Project mode only; « Team »: in the Team mode only; else everywhere.

### Workflows and sessions

| Command | Aliases | Mode | Description |
|---------|---------|------|-------------|
| `run.<workflow>` | the id of the workflow and its `aliases:`: `cadrage`, `feature` (→ `spec`), `secu` (→ `security-fix`), `perf` (→ `benchmark`), `dbg` (→ `debug`), `feedback`, `retours` (→ `review-feedback`) | — | Launch form of the workflow (one command per workflow of the catalogue) |
| `run.feature.implement-spec` | « run feature → implement-spec (with tickets) » | — | Form of `implement-spec` under the alias `feature` with tickets |
| `coder` | session, code, free | Project/Team | Form of `libre` (default agent `orchestrator`) |
| `coder.agent` | libre agent, free agent, session agent | Project/Team | « Free session with an agent… »: choose the entry agent, then the form |
| `sessions` | parallel, par, multi, inbox, à traiter, decisions | — | Sessions view |
| `workflows` | catalogue, catalog, wf, workflow | — | Workflow catalogue |
| `bricks` | briques, agents, skills, catalogue des briques | — | Bricks (agents and skills) |
| `review.publish` | publish, mr | Project/Team | Create the MR of the current branch; only when GitLab writing is enabled |

The aliases come from the YAML of the workflows (`aliases:`, see the [workflows reference](workflows.en.md)): they are valid for `oh run` too. The former hard-coded terms (`dev`, `start`, `fast`, `rev`…) are no longer recognised.

### Projects and configuration

| Command | Aliases | Mode | Description |
|---------|---------|------|-------------|
| `board` | kanban, tasks, project board | Project | Beads board of the project |
| `project-wiki` | wiki, wiki projet, project wiki, doc metrics | Project | Wiki of the project: queue of proposals and metrics |
| `board.init` | beads init, init board, init tickets | — | Initialise Beads |
| `projects`, `project.add` | proj, list · project add, nouveau projet | — | Projects, add wizard |
| `settings`, `project-config` | config, cfg · config projet | — / Project | Hub settings, project configuration (Execution included) |
| `models`, `provider`, `mcp`, `secrets` | mod, llm · prov, api · servers · tokens, credentials | — | Models, provider, MCP servers, secrets |
| `teams`, `team-detail`, `team.discover` | team, équipes · tracker · discover | — / Team | Teams, team detail, tracker discovery |
| `init` | setup, reconfigure | — | Setup wizard |

### System and navigation

| Command | Aliases | Description |
|---------|---------|-------------|
| `status`, `doctor`, `metrics`, `notifications` | stat · doc, health · stats, usage · notif, logs | Status, diagnosis, metrics, notifications |
| `cleanup` | nettoyage, gc, oh gc, deploy-cleanup, migrate, garbage, espace disque, disk | **Cleanup** view (former deployments and data of oh) |
| `history.export`, `history.import` | export history, import history | Export / import of the session history |
| `help` | ?, aide, shortcuts | Help |
| `home`, `project.mode`, `hub.mode`, `quit` | accueil · projet, focus · hub, retour · exit, q | Home, Project mode, Hub mode, quit |

### Team

| Command | Aliases | Description |
|---------|---------|-------------|
| `team.board`, `team.status`, `team.activity`, `history.team` | team board, team stat, activity, team history | Board, status, activity, history of the team |
| `team.briefs`, `team.patterns`, `team.policies` | takeover, reprises · pat · pol, rules | Briefs, patterns, policies |
| `team.wiki` | wiki, proposals, pending, team-state | Team proposals (team-state) |
| `team.sync`, `team.configure`, `team.init`, `team.rejoin` | sync tracker · team projet · team init · rejoin | Sync the tracker, team of the project, create or join a team |
| `worktrees` | wt, git worktree | Git worktrees |

## Views and keys

Each view shows its keys in the hints line; `?` repeats them. The keys of the convention (`j`/`k`, `g`/`G`, `r`, `Esc`…) are not repeated when they have their usual meaning.

### Sessions

Sections: **To handle** (sessions with waiting decisions, each followed by its decisions: `⏸` checkpoint, `?` question, `!` permission, `$` budget, `✗` error or circuit breaker), **Running**, **To recover** (remote sessions), **Sleeping**, **Finished, 7 days**. The children of a fan-out (`↳`) are shown as a tree under their parent (`⑂`); the group goes to « To handle » as soon as a child waits.

```
┌ Sessions ─────────────────────────────┐┌ implement-spec · ses_8f2a ────────────┐
│ ● ⑂ implement-spec  spec auth   $1.84 ││ [Detail]  Trace  Reports  Fan-out     │
│   ├ ✓ ↳ ticket  oh-12  merged   $0.41 ││ ⏸ cp-merge: 2/3 children merged       │
│   └ ⏸ ↳ ticket  oh-14  cp-2     $0.22 ││ Outputs recorded by oh                │
│ ✓   spec        auth OAuth      $0.93 ││ ↪ Suggested next steps (next:)        │
└───────────────────────────────────────┘└───────────────────────────────────────┘
```

| Key | Action |
|-----|--------|
| `Enter` | On a decision: open its card; on a session: show / hide the feed |
| `y` / `n` | Permission: allow once / refuse; checkpoint: accept (card when the verification fails) / « Fix first » form (expected approver shown) |
| `x` | File away an error, a budget overrun or a circuit breaker |
| `$` | Raise the budget (`$` decision) |
| `a` / `A` | Attach (open opencode) / open with… (automatic, iTerm2, Terminal.app, tmux, browser, here) |
| `t` | Show / hide the live feed |
| `m` / `i` / `M` | Send an instruction / interrupt the step / change the model of the next steps |
| `s` | Stop the session (confirmation) |
| `e` | Chain with… (`next:` steps first) |
| `B` | Batch validation (see below) |
| `d` | Delete the session (confirmation: what is deleted and lost) |
| `R` | Resume a sleeping session; recover a ready remote session |
| `o` / `w` | Results (MR description) / open in the browser |
| `f` | Active project / all projects |
| `Tab` / `Shift+Tab` | Next / previous tab of the detail |

**Tabs of the detail** (`Tab`). Outside Detail, the keys go to the tab; `Esc` goes back to Detail and the list; `?` and `Ctrl+P` stay available.

| Tab | Content | Keys |
|-----|---------|------|
| Detail | Session (workflow, runtime ⌂ ▣ ☁, location, cost, decisions), **outputs recorded by oh**, **suggested next steps (`next:`)** | those of the list |
| Trace | Text of `oh session trace` | `/` search, `n` / `N`, `g` / `G`, `r` |
| Reports | Reports of the session, reader on the right; on the parent session of a fan-out, those of the children are added, sorted by date, with the child that filed them (column after `rp-N`) | `Enter` read (then `Esc` goes back to the list), `/`, `n` / `N`, `r` |
| Fan-out | State of the fan-out: parallelism, driver, budget, children (ticket, session, state, branch, cost, end), fix rounds, last refusal of `cp-merge`, integration branch | `Enter` open the child, `p` relaunch the driver, `d` clean up, `B` batch validation, `r` |

`p` refuses to relaunch a driver that runs (« already running »); an accepted relaunch writes the event « started again by the user » in the events of the fan-out (the daemon writes « by the daemon »); the log of the driver is `~/.oh/sessions/<id>/fanout.log`. `d` (after `cp-merge` or once the session stopped) shows what is deleted and kept, offers `--discard` (unmerged branches) to tick, then confirms; it is `oh fanout clean`.

**Checkpoint card** (`Enter` on `⏸`): expected approver (`approver:`, « ⚠ out of approver » when your git identity is not one), **verification run by oh** (commands and state), evidence of the agent, changes (`+N −M · K file(s)`), last messages, timeline, reports. Buttons: **Decide**, **Full diff**, **Read rp-N** (one per report), **Wiki proposals (N): decide…** (when there are some), **Attach**.

| « Decide » form | Effect | Message |
|-----------------|--------|---------|
| Validate | The checkpoint passes (out of approver: confirmation, traced) | optional |
| Fix first | The agent fixes then asks again | required |
| Other instruction | The agent follows the instruction then asks again | required |
| Override the verification | Only when the verification failed and the checkpoint allows it | reason required, traced |

Failed verification: the card opens on **Fix (instruction)** and **Override…** (reason, confirmation, traced in `oh session trace`); never an « Override » on the `cp-merge` of a fan-out.

**Wiki proposals of a checkpoint** (button of the card): list without a limit (before / after, source, author, tag), `y` accept, `n` refuse (optional reason, confirmed; `Enter` with nothing typed refuses without a reason), `e` modify (one line, or `$EDITOR`), `s` leave, `A` accept all, `Enter` decide the checkpoint, `Esc` back to the card. The reason typed is kept with the proposal and shows under it once refused (“Reason for the refusal: …”). The accepted ones are applied when the checkpoint passes, before the commit.

**Batch validation** (`B`): replaces the list; scope = the fan-out of the selected session (else every session), filter = the checkpoint of the selected decision.

| Key | Action |
|-----|--------|
| `y` / `n` / `s` | Accept / refuse (instruction) / leave the element |
| `A` | Accept all |
| `/` | Filter by checkpoint (like `--checkpoint`) |
| `Enter` | Open the card (the only way for a failed verification: it is not accepted in a batch) |
| `C` | Confirm: summary, then the answer of each element of the filter |
| `Esc` | Cancel the batch |

Only the elements of the filter are answered.

**Remote recovery** (`R` on a « To recover » line): import of the session, then « Replay the journal (N op.) »; a Beads conflict offers Keep local, Apply remote, Merge the notes or Later.

### Launch form

Generated from the YAML of the workflow (« Launch · <workflow> », or « Launch · feature → implement-spec (with tickets) » under an alias), in three steps:

1. **Inputs**: one line per input (ticket with « Choose… », check box for `bool`, list for `enum`, text); the help of the input under its field; « Integer expected » blocks, « Path not found » and « Branch not found: … » warn. A `from:` input shows « ⚙ computed by oh: … → ses_… » (it is not prefilled with its default). For `libre`: **Entry agent** (« selectable » agents).
2. **Options**: mode, execution (`⌂ local`, `▣ container`, `☁ remote`, with the reason when unavailable), location (base, worktrees, « + new worktree »), opening.
3. **Recap**: agents, first-turn budget, isolation, sessions and locations, warnings; « Launch ».

| Key | Action |
|-----|--------|
| `Tab` | Next field |
| `Ctrl+S` | Go to the recap; on the loaded recap: launch |
| `Alt+←` / `PgUp` / `Ctrl+B`, `Alt+→` / `PgDn` | Previous, next step |
| `Esc` | Close |

**Existing session** (a session of the same workflow already works on the ticket): the recap compares both; a choice launches at once: `R` resume, `T` transfer (local ↔ container, model), `k` replace keeping the work, `d` delete and relaunch (confirmed), `p` in parallel.

**Ticket picker** (« Choose… »): `/` search, `f` label filter, `e` epic, `Space` tick (several tickets), `Enter` choose, `Esc` cancel.

### Wiki of the project

Command `project-wiki` (Project mode). At the top, the summarised metrics (pages, coverage of the domains, share of CONFIRMÉ / INCERTAIN, last sync); below, the queue of the proposals outside a checkpoint: **Pending**, **Accepted, applied at the next commit**, **Stale** (replaced line gone: nothing is applied).

| Key | Action |
|-----|--------|
| `y` / `n` / `e` / `s` / `A` | Accept / refuse (optional reason, confirmed) / modify / leave / accept all, like `oh wiki decide` |
| `/` | Filter (page, content, agent, workflow) |
| `m` | Every metric |
| `r` | Refresh |

### Team proposals (team-state)

Command `team.wiki` (Team mode): wiki proposals of the team-state, then pages. `{` / `}` sections, `Enter` see (buttons Accept, Refuse confirmed, Close), `y` accept, `n` refuse (confirmed), `r` refresh.

### Bricks

Command `bricks`, read only. Columns: id, origin (`hub`, `team`, `team ⟳ hub` when a team brick replaces the one of the hub, `(not sealed)` without `catalog.lock`), version, state (`✓`, `!` problem, `✗`). The detail shows the replacement, the publication (author, date), the agents and workflows that use it, the cost, the problems and the write commands (`oh brick new`, `edit`, `publish`).

`/` search, `f` agents / skills / all, `Enter` open a workflow that uses it, `r` refresh.

### Cleanup

Command `cleanup` (aliases `nettoyage`, `gc`), and Doctor › `f` on the clean-up checks. One view, two sections (`Tab`):

```
 [Former deployments]  Data of oh
╔ Cleanup of the former deployments ═══════════════════════╗
║ demo  /work/demo                                          ║
║   − .opencode/agents/ (12 files)                          ║
╚═══════════════════════════════════════════════════════════╝
```

| Section | Content | Keys |
|---------|---------|------|
| Former deployments | What the former deployments left in the projects and what is kept (your personal keys), like `oh migrate deploy-cleanup` | `c` see the diff of the configuration, `d` clean up (confirmed), `r` look again |
| Data of oh | What `oh gc` can remove, by category and size (bundles, server data, stopped sessions, worktrees, fan-outs, temporary folders, orphan servers, history); the preview removes nothing | `d` remove the element, `A` clean up everything (confirmed), `r` refresh the preview; the result is shown above |

The palette opens « Data of oh »; Doctor opens the section of its check. The « Cleanup of the former deployments » screen is also offered once at startup when leftovers are found. While the daemon cleans up, « Data of oh » answers « busy ».

### Doctor

Command `doctor`. `f` fix the selected check (like `oh doctor --fix`, with a preview and a confirmation: configuration restored from a backup, Beads, orphan servers; opens the section of the Cleanup on its checks), `o` offline (no network call, like `--offline`), `r` check again.

### Workflows, editor, history

**Catalogue** (`workflows`): sections Hub (read only), Team, Project, **My drafts**, **⚠ Integrity**.

| Key | Action |
|-----|--------|
| `Enter` | Launch; on a draft: test it |
| `n` / `e` | New (empty, extension, copy) / edit (on the hub: extend) |
| `v` / `t` / `p` | Check / test the draft / publish |
| `c` | Compare: diff of the draft with the impact |
| `h` | History |
| `x` | Archive a published workflow (reason) |
| `d` | Delete a draft (confirmed) |
| `*` | Pin |

**Publication**: version, validation, impact (widenings ⚠), diff, required message; `Ctrl+S` publishes; offline, queued and replayed at the next sync.

**History**: `Enter` or `c` diff with the current version, `R` restore (confirmed, republished as a new version), `r` refresh.

**Editor**: sections General, Graph, Inputs & prompt, Resources, Bundle preview (`Tab` / `Shift+Tab`). `Enter` modify, `d` (or `Delete`) remove an element or go back to the inherited value (confirmed), `a` agent or input, `c` checkpoint, `m` mode shown (Graph), `P` template in `$EDITOR`, `Y` raw YAML in `$EDITOR`, `u` / `U` undo / redo, `w` or `Ctrl+S` save, `Esc` (save and quit, drop or keep editing).

### Boards

| View | Keys |
|------|------|
| Project board | `h` / `l` columns, `Enter` detail, `a` actions (launch a workflow on the ticket), `L` link to the tracker, `i` initialise Beads |
| Team board | `h` / `l` columns, `[` / `]` projects, `Enter` detail, `a` actions, `/` filter (menu: text, assignee, label), `c` claim, `L` release (confirmed), `t` transfer, `s` status |

### Other views

| View | Keys |
|------|------|
| Projects | `Enter` configure, `n` add, `e` rename, `m` move, `d` delete (confirmed), `p` Project mode, `b` initialise Beads |
| Worktrees | `Enter` / `o` open in a terminal, `n` create, `d` delete, `p` prune the references, `M` delete the merged worktrees (never those of a session not stopped); confirmed |
| Settings, Project config | `{` / `}` sections, `Space` toggle, `Enter` modify, `u` undo, `r` reload (confirmed when changes are unsaved) |
| Models (hub, project, team) | `Enter` modify, `n` create, `d` delete (confirmed), `u` undo, `w` save (team) |
| Provider | `Space` default, `Enter` / `e` modify, `s` set up, `d` clear the configuration and the key of the keychain (confirmed) |
| MCP (hub, project, team) | `Space` enable, `Enter` modify, `t` token or test, `w` save, `u` undo, `T` approve the URL imposed by the team (hub view; confirmed, see [`oh mcp trust`](cli-mcp.en.md#oh-mcp-trust)) |
| Secrets | `Enter` / `e` modify, `n` create, `d` delete (confirmed) |
| Teams | `Enter` open, `n` create, `d` delete (confirmed), `s` sync, `u` undo |
| Team detail | `Space` toggle, `Enter` modify, `w` save, `s` sync, `t` test, `u` undo, `T` discover the tracker, `K` provider key of the team |
| Patterns | `Enter` see, `n` create, `y` validate, `d` delete (confirmed) |
| Policies | `Enter` detail, `n` create, `t` template, `e` edit, `d` delete (confirmed), `c` check |
| Status, Activity, Metrics | `c` check the conventions · `t` / `w` / `0` today, week, all · `7` / `3` / `0` period, `Tab` usage by agent |
| Briefs | `Enter` see the brief, `e` enrich (workflow `brief-enrich`) |

## Wizards

`init`, `project add` and `team init` open wizards in the TUI (`shell.PushView`). `Ctrl+S` validate the step, `Ctrl+B` / `Alt+←` / `PgUp` back, `Esc` → « Quit the wizard? » (**Quit**, **Skip the step** when it is optional, **Continue**). See [InlineWizardView](tui-inline-wizard.en.md).

## opencode sessions

When a session is launched, oh builds its bundle and starts it on the `opencode serve` server of its group; the opencode client opens next to it (tab or window, tmux, browser) and the TUI stays usable. Closing opencode does not stop the session: it is followed and reopened from **Sessions**. See [Sessions v5](../guides/sessions-v5.en.md).

## Sync of the team views

The team views (board, status, activity, briefs, patterns, policies) pull the git data of the team on opening and on `r`. A « Synchronising… » toast shows when the pull takes more than one second; on a network error, a warning and the local data.
