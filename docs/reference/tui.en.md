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
│  🏠 Hub · ● 2 ◐ 1                                                │ Mode bar
└──────────────────────────────────────────────────────────────────┘
```

The mode bar shows the sessions badge **`● N ◐ M`** (N sessions working, M decisions waiting); it turns to the alert colour when a decision waits. The legend of the symbols (states, decisions) and `NO_COLOR` (no color) are in the [CLI reference](cli.en.md#symbols-and-colors).

**Small terminals**: under 32 rows, compact layout (one-line banner, omnibar and mode bar on 3 rows); under 60×16, a « Terminal too small » message and only `Ctrl+Q` works. Setting `[cli] compact = auto | always | never`. Switching layouts keeps the selected element.

**Landings.** The three landings draw their name in the banner as it was typed (a team « Core » is not « CORE »; the Hub draws the logo « OPENHUB »).

- **Hub**: under the banner, the tool line `opencode v2.2.5 · adapter opencode-v2 ✔` (the tool and the adapter that launch the sessions). The System section lists Configuration, Metrics, Doctor, Secrets, **Notifications** and the **Brick catalogue**.
- **Project**: the header gives the path and the branch, the tool line and the execution environments with their state, `execution: ⌂ local ✔  ▣ container · colima ✔  ☁ remote ✗` (read when the landing opens, not on the event loop).
- **Team**: besides *Team sessions*, the section **Sessions of the other members** lists, from the claims of the team-state, who works on which ticket (member, ticket, session, since when; `●` the claim is alive, `○` its lease is past). It is **read only**: `Enter` opens a window with the details and a single button *Close*; a session runs on the machine of its member, oh does not act on it.

## Global keys

| Key | Action |
|-----|--------|
| `Ctrl+P` | Command palette |
| `/` | Filter the list of the view; when the view does not filter: palette |
| `?` / `F1` | Help: global keys, convention, keys of the active view, legend, commands |
| `Esc` | Closes the toasts first; else goes back (previous view); at the root of a Project or Team view: back to the Hub view |
| `Ctrl+T` | Next view of the cycle **Hub → Project → Team → Hub**, skipping what is not configured: from the Hub, the project you left (the only one, or a choice among several), else the team; from the Project, the team (choice when there are several), else the Hub; from the Team, the Hub |
| `Ctrl+Q` / `Ctrl+C` | Quit (when sessions work: finish the step then sleep, background, or stop) |
| `Enter` | Action of the selected list, button or field; palette only when nothing uses it |

**Key order**: windows (forms, confirmations) and the palette get every key; then the global keys; then the keys of the view; then the selected list or field. Typing a letter « anywhere » does not open the palette.

**Windows**: a window opened while another one is shown stacks over it, up to three levels (the backdrop gets darker each time). `Esc` closes the top one, with or without a button, and gives control back to the one under it; closing a window also closes the ones over it. The focus goes back to where the window was opened from: the button of the window under it, or the component of the view that had it. A question asked from a card ("Decide", "Refuse", "Override"…) opens over the card: "Cancel" or `Esc` gives the card back as it was, answering closes the question and the card with it. A required field left empty (instruction, reason) is refused without closing the window: what was typed stays, with a warning. `Tab` and `Shift+Tab` do not close an input.

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
| Everywhere | `Ctrl+S` | Save: the team pages and the workflow editor, which do not save on their own (in a launch form or a wizard: next step or launch) |
| List | `n` | Create |
| List | `e` | Edit |
| List | `d` | Delete, **always confirmed** |
| List | `x` | Archive or file away, **never destructive** |
| List | `R` | Resume or restore (sleeping session, remote session, version of a workflow) |
| List | `c` | Compare (diff) |
| List | `Space` | Tick / untick, switch a setting, choose the option under the cursor |
| Decision | `y` | Accept (checkpoint, wiki proposal, element of a batch) |
| Decision | `n` | Refuse, with an instruction; the validated instruction is the confirmation |
| Decision | `e` | Modify before accepting |
| Decision | `s` | Leave pending |
| Decision | `A` | Accept all |
| Reader | `/` | Search the text (trace, reports) |
| Reader | `n` / `N` | Next / previous match |

`e`, `s`, `A` and `c` carry an action of the view when it has not the action of the convention (`e` chain in Sessions, `s` stop a session, `A` open with…).

`w` has one meaning only: open in the browser (Sessions). Saving is `Ctrl+S`.

**Saving**: the Settings and the Project pages (config, models, MCP) save by themselves after **one delay** (500 ms, the same everywhere) and say so in the title of the page: `✎ saving…` then `✔ saved` (`✗ not saved` when the save failed). The team pages (team detail, team models, team MCP) publish to the team-state, shared with the team: they do not save on their own, the title says `✎ not published — Ctrl+S` as long as a change waits, `✔ saved` once `Ctrl+S` published it.

### Changed keys

| View | Before → now |
|------|--------------|
| Sessions | `D` → `d` delete · `c` → `R` resume · `g` → `R` recover a remote session · new: `B` batch validation |
| Recap of an existing session (launch form) | `r` → `R` resume · `t` → `T` transfer · `k` → `K` replace, keeping the work |
| Wiki proposals | `a` → `y` accept · `x` → `n` refuse |
| Workflows | `D` → `c` compare · `x` → `d` delete a draft (`x` only archives published workflows) |
| History of a workflow | `r` → `R` restore (`r` refreshes) |
| Workflow editor | `x` → `d` remove or go back to the inherited value · `y` → `Y` raw YAML |
| Patterns | `a` → `n` create · `v` → `y` validate |
| Projects | `a` → `n` create · `n` → `e` rename |
| Team board | `f` → `/` filter · `x` → `L` release |
| Worktrees | `a` → `n` create · `x` → `M` clean up merged ones |
| Team detail, team models, team MCP | `y` → `T` discover the tracker (detail) · `w` → `Ctrl+S` save |
| Workflow editor | also `w` → `Ctrl+S` save |
| Team activity | `w` → `s` week (`t` today, `0` all) |
| MCP | `w` → `m` write mode of a service |
| Launch form (modified directory) | `w` → `t` worktree (`s` stash, `b` allow) |
| Switching views | `Ctrl+T`: Hub → Project → Team → Hub (it only went between the Hub and the Team) |
| Other lists (models, policies, secrets, teams…) | `a` → `n` create |
| Forms (multiple choice) | `x` → `Space` tick (`x` no longer ticks: it archives in lists of objects) |
| Board columns editor | `a` → `n` add · `r` → `o` change the role · new: `e` rename, `d` asks for a confirmation (see [the editor](#board-columns-editor)) |

## Legend of the symbols

A symbol has **one meaning**, on every screen of the TUI and in the commands. The help (`?`) shows the legend under the key convention; the source is the table of meanings of the theme (`theme.Meanings`, see [Aurum](../design/aurum.md#table-des-sens-themeiconsgo-glyphsymbolsgo)).

| Symbol | Meaning |
|--------|---------|
| `✔` | worked, valid (also a finished step) |
| `✗` | failed, invalid: a one-off error, a refused input |
| `○` | off, at rest (also a step to come, a choice not taken) |
| `–` | missing, not set (also a skipped step) |
| `⚠` | works, but needs attention |
| `●` | running right now |
| `◆` | current element: view, step, team |
| `✎` | changed, neither saved nor published |
| `★` | chosen by default, pinned element |
| `▸` | row under the cursor, start of an action |
| `◉` | chosen in a single choice |
| `☑` / `☐` | ticked / not ticked in a multiple choice |

The states of a session have symbols of their own (`⊗` a session that stopped on a failure, `◌` a sleeping session, see the [CLI reference](cli.en.md)): `✗` is a one-off error, `⊗` is how a run ended.

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

The Command column gives the id; the label or an alias can be typed too. Each term has one meaning. « Project »: in the Project view only; « Team »: in the Team view only; else everywhere. The word « mode » is reserved for the workflow mode: the three home screens are called « Hub / Project / Team view ».

### Workflows and sessions

| Command | Aliases | View | Description |
|---------|---------|------|-------------|
| `run.<workflow>` | the id of the workflow and its `aliases:`: `cadrage`, `feature` (→ `spec`), `secu` (→ `security-fix`), `perf` (→ `benchmark`), `dbg` (→ `debug`), `feedback`, `retours` (→ `review-feedback`) | — | Launch form of the workflow (one command per workflow of the catalogue) |
| `run.feature.implement-spec` | « run feature → implement-spec (with tickets) » | — | Form of `implement-spec` under the alias `feature` with tickets |
| `libre` | coder, session, code, free | Project/Team | Form of `libre` (default agent `orchestrator`) |
| `libre.agent` | coder.agent, libre agent, free agent, session agent | Project/Team | « Free session with an agent… »: choose the entry agent, then the form |
| `sessions` | parallel, par, multi, inbox, à traiter, decisions | — | Sessions view |
| `workflows` | catalogue, catalog, wf, workflow | — | Workflow catalogue |
| `bricks` | briques, agents, skills, catalogue des briques | — | Bricks (agents and skills) |
| `review.publish` | publish, mr | Project/Team | Create the MR of the current branch; only when GitLab writing is enabled |

The aliases come from the YAML of the workflows (`aliases:`, see the [workflows reference](workflows.en.md)): they are valid for `oh run` too. The former hard-coded terms (`dev`, `start`, `fast`, `rev`…) are no longer recognised.

### Projects and configuration

| Command | Aliases | View | Description |
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
| `home`, `project.mode`, `hub.mode`, `quit` | accueil · projet, focus · hub, retour · exit, q | Home, Project view, Hub view, quit |

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

Sections: **To handle** (sessions with waiting decisions, each followed by its decisions: `⏸` checkpoint, `?` question, `!` permission, `$` budget, `✗` error, `⊘` circuit breaker), **Running**, **To recover** (remote sessions), **Sleeping**, **Finished, 7 days**. The children of a fan-out (`↳`) are shown as a tree under their parent (`⑂`); the group goes to « To handle » as soon as a child waits.

```
┌ Sessions ─────────────────────────────┐┌ implement-spec · ses_8f2a ────────────┐
│ ● ⑂ implement-spec  spec auth   $1.84 ││ [Detail]  Trace  Reports  Fan-out     │
│   ├ ✔ ↳ ticket  oh-12  merged   $0.41 ││ ⏸ cp-merge: 2/3 children merged       │
│   └ ◐ ↳ ticket  oh-14  cp-2     $0.22 ││ Outputs recorded by oh                │
│ ✔   spec        auth OAuth      $0.93 ││ ↪ Suggested next steps (next:)        │
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

**Checkpoint card** (`Enter` on `⏸`): expected approver (`approver:`, « ⚠ out of approver » when your git identity is not one), **verification run by oh** (commands and state), evidence of the agent, changes (`+N −M · K file(s)`), last messages, timeline, reports. Buttons: **Decide**, **Full diff**, **Read rp-N** (one per report), **Wiki proposals (N): decide…** (when there are some), **Attach**. "Full diff" and "Read rp-N" open the reader over the card: `Esc` or a click outside closes it and gives the card back, on the button it came from; if the decision is settled elsewhere while you read, the card and the reader close together.

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

**Existing session** (a session of the same workflow already works on the ticket): the recap compares both; a choice launches at once: `R` resume, `T` transfer (local ↔ container, model), `K` replace keeping the work, `d` delete and relaunch (confirmed), `p` in parallel.

**Ticket picker** (« Choose… »): `/` search, `f` label filter, `e` epic, `Space` tick (several tickets), `Enter` choose, `Esc` cancel.

### Wiki of the project

Command `project-wiki` (Project view). At the top, the summarised metrics (pages, coverage of the domains, share of CONFIRMÉ / INCERTAIN, last sync); below, the queue of the proposals outside a checkpoint: **Pending**, **Accepted, applied at the next commit**, **Stale** (replaced line gone: nothing is applied).

| Key | Action |
|-----|--------|
| `y` / `n` / `e` / `s` / `A` | Accept / refuse (optional reason, confirmed) / modify / leave / accept all, like `oh wiki decide` |
| `/` | Filter (page, content, agent, workflow) |
| `m` | Every metric |
| `r` | Refresh |

### Team proposals (team-state)

Command `team.wiki` (Team view): wiki proposals of the team-state, then pages. `{` / `}` sections, `Enter` see (buttons Accept, Refuse, Close), `y` accept, `n` refuse, `r` refresh. **Refusing asks for a reason** (required: an empty one refuses nothing); it is kept in the history of the team-state and in the `wiki.rejected` event, shown in the Activity view and in the notification.

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

Command `doctor`. `f` fix the selected check (like `oh doctor --fix`, with a preview and a confirmation: configuration restored from a backup, Beads, claims attached to the project created again, orphan servers; opens the section of the Cleanup on its checks), `o` offline (no network call, like `--offline`), `r` check again.

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

**Editor**: sections General, Graph, Inputs & prompt, Resources, Bundle preview (`Tab` / `Shift+Tab`). `Enter` modify, `d` (or `Delete`) remove an element or go back to the inherited value (confirmed), `a` agent or input, `c` checkpoint, `m` mode shown (Graph), `P` template in `$EDITOR`, `Y` raw YAML in `$EDITOR`, `u` / `U` undo / redo, `Ctrl+S` save, `Esc` (save and quit, drop or keep editing).

### Boards

`oh board`, `oh team board` and `oh dashboard` (deprecated) open the TUI directly on the **Board**, **Team board** and **Metrics** views below, over the landing of the mode (project, team, hub): `Esc` goes back to it, `Ctrl+Q` quits. Without a terminal they exit with code 2. There is no separate full-screen board any more: what the command opens is the view of the shell, with its keys.

| View | Keys |
|------|------|
| Project board | `h` / `l` columns, `Enter` detail, `a` actions (launch a workflow on the ticket), `L` link to the tracker, `i` initialise Beads |
| Team board | `h` / `l` columns, `[` / `]` projects (the tabs show the project name, sorted by name; an ID that no local project knows shows as "(unknown project)"), `Enter` detail, `a` actions (workflows taking a ticket; on a GitLab issue, also the workflows taking a free request, with `#<iid>`), `/` filter (menu: text, **My tickets**, **Unassigned** — the pool, to take with `c` —, assignee, label, clear), `c` claim (refused at `max_in_progress_per_member`), `L` release (confirmed), `t` transfer, `s` status, `C` edit the columns of the board (the editor below) |

### Board columns editor

Team home → **Board Columns**, or `C` on the team board (an active team with a cloned team-state is needed; back from the editor, the board reads its tickets again with the new columns). The editor sets the columns of the team board (the `[board]` section of the team configuration, see [`config.en.md`](config.en.md#board-section)) and the rules that put a tracker label or status in a column. It is a wizard step: `Esc` asks « Quit the wizard? », and nothing is written until `Ctrl+S` is pressed.

List of the columns:

| Key | Action |
|-----|--------|
| `↑` `↓` / `j` `k` | choose a column |
| `n` | add a column after the one under the cursor (the name is asked; the id is derived from it, the role is suggested from the name) |
| `e` / `Enter` | rename the column (its id does not change) |
| `d` | delete the column, **after a confirmation** (the focus is on « Cancel »); the labels and statuses that lead to it go with it, and the confirmation gives their number. Two columns at least stay: below that, a message refuses without asking |
| `o` | change the role: `initial` → `active` → `terminal` → `blocked` |
| `K` / `J` | move the column up / down |
| `m` | open the mappings of the column (when the team has a tracker) |
| `Ctrl+S` | save: at least two columns, one of them `terminal`, else a message says what is missing |

Mappings of a column (`m`): the list of the labels, then of the statuses (Jira), that put a ticket in the column.

| Key | Action |
|-----|--------|
| `↑` `↓` / `j` `k` | choose a label or a status (titles are skipped) |
| `n` | add a label (the name is asked) |
| `e` / `Enter` | rename the label or the status |
| `d` | remove it from the column, **after a confirmation** |
| `v` | move it to another column: the list of the other columns is offered, the label or status then leaves this column (shown in the target's mappings) |
| `q` | back to the list of the columns |

The tracker discovery wizard (`Discover Tracker`) uses the same editor for its columns, without the `m` key. Before 5.1: `a` added and `r` changed the role; they are now `n` and `o` (`r` refreshes everywhere else), and `d` asks for a confirmation (see [Changed keys](#changed-keys)).

### Other views

| View | Keys |
|------|------|
| Projects | `Enter` configure, `n` add, `e` rename, `m` move, `d` delete (confirmed), `p` Project view, `b` initialise Beads |
| Worktrees | `Enter` / `o` open in a terminal, `n` create, `d` delete, `p` prune the references, `M` delete the merged worktrees (never those of a session not stopped); confirmed |
| Settings, Project config | `{` / `}` sections, `Space` toggle, `Enter` modify, `u` undo, `r` reload (confirmed when changes are unsaved) |
| Models (hub, project, team) | `Enter` modify, `n` create, `d` delete (confirmed), `u` undo, `Ctrl+S` save (team) |
| Provider | `Space` default, `Enter` / `e` modify, `s` set up, `d` clear the configuration and the key of the keychain (confirmed) |
| MCP (hub, project, team) | `Space` enable, `Enter` modify, `t` token or test, `m` write mode (hub view), `Ctrl+S` save (team), `u` undo, `T` approve the URL imposed by the team (hub view; confirmed, see [`oh mcp trust`](cli-mcp.en.md#oh-mcp-trust)) |
| Secrets | `Enter` / `e` modify, `n` create, `d` delete (confirmed) |
| Teams | `Enter` open, `n` create, `d` delete (confirmed), `s` sync, `u` undo |
| Team detail | `Space` toggle, `Enter` modify, `Ctrl+S` save, `s` sync, `t` test, `u` undo, `T` discover the tracker, `K` provider key of the team |
| Patterns | `Enter` see, `n` create, `y` validate, `d` delete (confirmed) |
| Policies | `Enter` detail, `n` create, `t` template, `e` edit, `d` delete (confirmed), `c` check |
| Status, Activity, Metrics | `c` check the conventions · `t` / `s` / `0` today, week, all (Activity) · `7` / `3` / `0` period, `Tab` usage by agent |
| Briefs | `Enter` see the brief, `e` enrich (workflow `brief-enrich`) |

## Wizards

`init`, `project add` and `team init` open wizards in the TUI (`shell.PushView`). `Ctrl+S` validate the step, `Ctrl+B` / `Alt+←` / `PgUp` back, `Esc` → « Quit the wizard? » (**Quit**, **Skip the step** when it is optional, **Continue**). See [InlineWizardView](tui-inline-wizard.en.md).

## opencode sessions

When a session is launched, oh builds its bundle and starts it on the `opencode serve` server of its group; the opencode client opens next to it (tab or window, tmux, browser) and the TUI stays usable. Closing opencode does not stop the session: it is followed and reopened from **Sessions**. See [Sessions v5](../guides/sessions-v5.en.md).

## Sync of the team views

The team views (board, status, activity, briefs, patterns, policies) pull the git data of the team on opening and on `r`. A « Synchronising… » toast shows when the pull takes more than one second; on a network error, a warning and the local data.
