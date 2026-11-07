# v5.1 — entrées CHANGELOG proposées par les lots

> Source de F1 (CHANGELOG `[5.1.0]` écrit une seule fois). Lots du 23/10 (0-A à A4, K, L) : leurs récapitulatifs n'ont pas été conservés ; F1 les reconstruit depuis `git log v5.0.0..` (messages détaillés) et le journal § J de `release-v5.1.md`. Lots du 02/11 ci-dessous, tels que proposés (EN), corrigés des décisions prises après coup.

## Vague 5

### A5 — vues de configuration
- **TUI – Configuration views.** Settings, models, provider, MCP, secrets, project configuration and team detail now share the common display (colours, symbol table, ✔ for success), read their data without freezing the screen, and show loading, empty and error states the same way. Settings shows « Token server » (`tracker_token_url`) next to « Tracker Token », hidden when the tracker is Jira (also in team detail). The URL field of Google Slides is removed from the team MCP view, as Figma's was (a recorded URL is ignored; `oh mcp trust` no longer lists `gslides`). Texts: « Entrée »/« Espace » in the detail panes (FR), the empty-team hint says `n`.

### B4 — mode distant (Fixed)
- Remote sessions: replaying the Beads journal compares labels as sets (a label added or removed by the session is no longer reported as a conflict because the ticket changed on the machine; it is one only if the replay would undo a local label change, and conflicts now list the labels); "Merge the notes" keeps the session's `ready-for-review` label; the job's fake `bd` refuses a status absent from the project's Beads configuration (as `bd` does) instead of recording an entry that fails at replay.

### C8b
- Aucune entrée (refactor interne : stores via `app.App`, `daemon/compat.go` supprimé).

## Vague 6

### A6 — accueils et vues d'équipe
- *Changed:* the Hub, Project and Team landings share one footer; the board, wiki, patterns, policies, activity and team-status views read the team-state and run git/`bd` calls off the event loop.
- *Changed:* a disabled setting or service shows a grey ○ instead of a red ✗; "validated" patterns use ✔.
- *Added:* on a GitLab issue card, `a` also offers `spike` and `incident`, with short titles (`retro` retiré par F6).
- *Fixed:* the Provider view now writes the default provider, provider fields and resets to `hub.toml`; they were only changed in memory and lost on the next write.

### M1 — modèles et divers
- *Changed:* the default model of a session (when no level of the cascade sets one) is now `claude-sonnet-5-5`.
- *Changed:* `oh config model` and the Models view refuse a short model name without a version number (`sonnet`, `claude-sonnet`) and show the expected form (`claude-sonnet-5-5`); `provider/model` is unchanged.
- *Added:* the launch recap (`oh run --recap`, TUI launch form) shows the model the session uses and, on a second row, the other models of the agents of the bundle.
- *Added:* `oh team init` and the init wizards warn when the commit of the team repository structure ("team: init structure") fails, instead of only logging it.
- *Fixed:* an instruction sent to a `queued` or `preparing` session (`oh session send`, `m` in the Sessions view) is refused with a clear message.

### B5 — tracker et claims (version corrigée par F6 pour la limite)
- *Added:* team claims: a claim taken by hand (no session: `oh team claim`, team board, tracker plan) has its own, longer lease (`[claim] lease_hours_manual`, 72 h) that `oh team claim <ticket>` renews on your own claim, expired or not, without confirmation.
- *Added:* team claims: the renewal of the leases has a ceiling — a session asleep for more than `sleeping_renewal_days` (3) or a remote session launched more than `remote_renewal_days` (7) days ago stops renewing its claims, which then expire normally (`oh doctor` reports them).
- *Added:* tracker sync: `[claim] release_expired_on_sync` (off by default) lets the synchronization release expired claims (a claim in review is kept).
- *Changed:* a workflow that takes a Beads ticket launched on a GitLab issue card gets `#<iid>` (refused by its Beads input) instead of the bare number.
- *Fixed:* tracker sync no longer recreates a claim released on purpose (`oh team release`, end of a session, board, retention of done claims).
- *Fixed:* the Identity step of `team init`/`rejoin` no longer resets `default_mode` of the member.

### F6 — correctifs après la vague 6
- *Changed:* auto-plan limit (`max_auto_plan_per_member`): a sync creates at most N `planned` claims per member, and the issues it leaves out come in first at the next syncs. Issues already in progress are always let in (outside the limit) and come first. The sync result and the team board say how many issues were left out (`N issue(s) outside the board, limit reached`). The first full scan of a project no longer filters on the update time, so an old issue already in progress is found.
- *Changed:* the agents that declared `claude-sonnet-4-6` as their floor now declare `claude-sonnet-5-5` (`reviewer` and `benchmarker` stay on Opus). The add-project wizards (CLI and TUI) no longer write a model when the field is left empty: the project inherits the model of the hub (they used to write `claude-sonnet-4-5`).

## Ruptures à annoncer (récapitulatif)
- `k` → `K` (remplacer en gardant le travail, A4) ; nom court de modèle sans version refusé (M1) ; modèle par défaut `claude-sonnet-5-5` (M1/F6) ; URL Google Slides retirée (A5) ; ○ gris pour « désactivé » (A6) ; `MCP GitLab list_issues/list_mrs` renvoient un objet paginé (L3) ; plus celles listées en § P F1 de `release-v5.1.md`.

## Vague 7

### F7 — Fixed
- The "new keys" notice is versioned by a digest of the key-change table: it shows again once after an update that moves keys (a 5.0 install sees it again).
- `oh history` no longer panics on short identifiers, and its headers are translated.
- `oh init` no longer proposes the home folder or the disk root as the first project.
- Closing a ticket after a change of the git configuration raises the same alert as a refused checkpoint.
- `[gc] log_max_mb` / `log_keep` now apply to the tool server log.
- `oh daemon stop` says it counts live tool servers (with their number), not sessions.
- French labels: Policies → Politiques, Sync Tracker → Synchro tracker; board hints, rejoin prompts, export, team config, team events and other CLI messages go through i18n.
- The banner draws accented names without accents instead of "?".
- Dev/-dirty builds re-extract the hub content when the embedded content changes.
- Takeover briefs in the team repository use ISO dates.
- (Le point `oh start` de F7 devient caduc avec R1 : `oh start` retiré.)

### CI1
- Aucune entrée (CI et tests). À mentionner éventuellement : CI Windows élargie (99 paquets), vrai `bd` et rendu Mermaid testés en CI.

### B6 — Changed / Fixed
- **Team mode is no longer a beta.** The single stderr warning, the first-claim note, the `oh doctor` "beta mode" check and the "Beta" header of the team board and team configuration are removed. **Breaking for scripts:** the `Team · beta mode` doctor result no longer exists. The limits announced for 5.0 are gone: a claim in review (`ready-for-review` label or `review` status) outlives its session, a claim has a lease (`lease_hours`, renewed by the daemon, with ceilings for sleeping and remote sessions), an expired claim can be taken over with `oh team claim`, and what the end of a session decides goes through a durable local queue. The remote mode stays experimental.
- The status model (REVIEW = `in_progress` + `ready-for-review` in Beads, `review` on the claim) and the lease and end-of-session contract are documented in the team-setup guide.
- *Fixed:* the team configuration (`config.toml` of the team-state) is written atomically, like `hub.toml`.

### D1 — Documentation
- ADRs 056 to 060: team claims (lease, heartbeat, end of session, durable queue), strict GitLab token per server, `mode: all` for a called primary agent, return envelope and `workflow_report`, living documentation wiki.
- ADR statuses brought up to date (005, 007, 009, 015, 016 superseded; 046 amended: the fake `bd` also runs locally).
- Twelve pre-v5 documents moved from `docs/dev/` to `docs/legacy/`; the documentation index lists the internal docs and the archives.
- Release checklist, contributing guide, config cascade and stack wiki pages updated; the install hint for `bd` is `brew install beads` (`oh doctor`, board, troubleshooting).

### R2
- *Fixed:* `Attest` now reports a plugin the bundle declares but the server does not run (a warning at launch, a failure under `execution.strict_isolation`); if `/api/plugin` cannot be read it says so.
- *Fixed:* `oh run --headless` prints the assistant text in the order it was written (messages by time then id, read in pages), with a blank line between the pieces.
- *Added:* remote: the session manifest carries the `unlocks` of each checkpoint (additive); the job refuses to start when its bundle does not lock them the same way.
- *Added:* Settings › Remote: `remote.experimental` can be switched in the TUI, with the warning of the mode.
- *Added:* `execution.container_memory` (default: none; `512m` to `1024g`) bounds the memory of a container (`--memory`); settable with `oh config set` and in Settings › Execution.

### R1 — Removed (breaking)

Everything deprecated in 5.0 is removed. A removed command name still answers with its replacement (exit code 2); a removed option gives "unknown flag" (exit code 2).

| Removed | Replacement |
|---|---|
| `oh start` | `oh run <workflow>` (`oh run libre --agent <agent>`, `oh run ticket --tickets <ids>`, `oh run onboarding`, `oh run sweep`, `oh run feature`); `oh session attach <id>` to resume |
| `oh audit` | `oh run audit -i type=<type>` |
| `oh review` | `oh run review -i review_mode=<mode> -i branch=<branch>` |
| `oh review --publish` | `oh mr create` |
| `oh review feedback <ref>` | `oh run review-feedback -i mr=<ref>` |
| `oh debug` | `oh run debug -i issue=<description>` |
| `oh service`, `service setup`, `service remove` | `oh mcp status`, `oh mcp setup`, `oh mcp disable` (the keychain token is deleted with `oh secrets delete <key>`) |
| `oh takeover-brief enrich <ticket>` | `oh run brief-enrich --headless -i ticket=<ticket>`, or the `e` key of the Takeover view of the TUI |
| `oh serve -p`, `oh metrics -d`, `oh project configure -m` | `--port`, `--period`, `--model` |
| `oh session attach --how`, `--attach here` | `--attach`, value `suspend` |
| `--force` / `-f` of `oh purge`, `oh project remove`, `oh teams remove` | `--yes` / `-y` |
| `oh repair --auto` | `--yes` |

`--force` stays where it overrides a safety (`daemon stop`, `remote setup`, `worktree remove|cleanup`).

## Vague 8

### A7
- **CLI/TUI display:** sizes, costs, dates, ages, durations and token counts now come from one place, so they read the same in every command and screen (KiB/MiB, costs to the cent with a "<$0.01" floor, local time). A disabled setting in `oh team config` is a grey ○. Writes to `hub.toml`, the team-state and the keychain from Settings, Provider, team detail, team MCP and team models no longer freeze the interface. The launch form reads the project settings again when they changed. In team MCP, an edit made during a reload is no longer lost without asking. The member list of the rejoin wizard is filterable with `/`.

### A8
- The wizards of `oh provider setup`, `oh project add|configure|remove`, `oh policies add`, `oh patterns add`, `oh mcp setup` and `oh team init` now run on the same engine as the TUI wizards (`oh init` already did): `Esc` asks "Quit the wizard?" (no more silent double `Esc`), a failing step shows its error and waits for a retry instead of ending the wizard, and what a step prints appears once the wizard has left the screen.
- *Fixed:* `oh project remove --yes` without an id deleted nothing; `oh provider setup` ignored the ticked "use detected credentials" box until it was toggled.

### B7
- *Changed:* a team claim in review (status `review` or label `review`/`ready-for-review`) no longer expires: it stays reserved until the merge or its release; `oh doctor` reports it when it has waited longer than `stale_days` (`claims_review`).
- *Changed:* Hub and project pages save after one delay (500 ms) and show it in their title ("saving…", "saved"); team pages show "not published — Ctrl+S" until saved. `Ctrl+S` saves in team pages and the workflow editor; `w` only opens a session in the browser (activity week → `s`, MCP write mode → `m`, worktree of a modified directory → `t`).
- *Changed:* refusing a proposal of the team wiki needs a reason (`oh team wiki review` and TUI), kept in the history and in the `wiki.rejected` event.
- *Changed:* `Ctrl+T` now cycles Hub → Project → Team → Hub, skipping what is not configured.
- *Changed:* landings follow the mock-ups: tool and adapter in the Hub header, Notifications and Brick catalogue under System, execution environments (⌂ ▣ ☁) in the Project header, sessions of the other members (read only) in the Team landing; names are drawn as typed.
- *Changed:* one table of team events (symbol, tone) and one phrase per event for `oh team activity` and the TUI.

### F8 — Fixed
- `oh team init` (CLI and TUI) at the first launch now runs the config, identity, notifications and policies steps after cloning the team-state repo; they were skipped.
- `oh project add` no longer initializes Beads when the "initialize beads" box is unticked.
- Ctrl+S in the wizards now presses the main button of the step like Enter does, so the MCP services of `oh project add` and the policies of `oh team init` are kept; the same goes for the button of the grouped layout (`oh init`).

### B8
- Aucune entrée (refactor : interfaces `teamstate` par sous-domaine).

### C9
- *Changed:* the shell completion now describes commands and options in the language of the hub. `oh gc --only` lists the `fanout` category, `oh config model family` lists all the agent families, `oh conventions` is described as a check, and the help of `oh daemon run`, `oh eval pass` and `oh session attach --exec` follows the language. Internal: the commands write through `cmd.OutOrStdout()` / `cliout`, and the help texts live in the locales only.

### A9
- *Changed:* `oh board`, `oh team board` and `oh dashboard` open the TUI on the Board, Team board and Metrics views (Esc goes back to the landing of the mode) instead of drawing a separate full-screen view. Without a terminal they exit with code 2. `oh board` takes `--project <name>` (default: the project of the current directory, else the only registered project); `--watch` is kept and has no effect (the board refreshes every 5 s). Epics are no longer shown on the board.
- *Changed:* the Metrics view shows the sessions of the day, the active projects (hub), the cache savings and the reasoning share.
- *Deprecated:* `oh dashboard` and `oh metrics` are deprecated and will be removed in v6: use the Metrics view of the TUI (`oh`, then Ctrl+P › Metrics). They warn on the error output; `oh metrics` keeps its text output until v6.

## Vague 9

### B9, A10
- Aucune entrée (refactors : vues sur interfaces fines, découpage du shell).

### A11
- *Changed:* **board column editor keys.** In `oh team` › board columns, the keys now follow the TUI convention: `n` adds a column (was `a`), `e` renames (`Enter` still works), and the role is on `o` (was `r`, which refreshes in the other views). `J`/`K`, `m`, `q`, `d` and `Ctrl+S` are unchanged. The first-launch notice lists the changes.
- *Fixed:* renaming and deleting a label or status in the mappings screen of the column editor, which did nothing, now work.

## Vague 10

### A12
- *Added:* the board column editor (`oh` › team › Board Columns) asks for a confirmation before it deletes a column (the dialog says how many labels and statuses go with it) or a label/status mapping; the focus starts on "Cancel". The refusal to go under two columns still comes first, without a question.
- *Added:* the help (`?`) shows a legend of the symbols (✔ ✗ ○ – ⚠ ● ◆ ✎ ★ ▸ ◉ ☑ ☐), one meaning each, under the key convention. The Space bar is now listed in the convention (List: tick, untick, switch a setting, choose the option under the cursor).
- *Changed:* no key changes. Internal: every letter key of the card columns, lists with sections, workflow graph, inline selects, Projects, Worktrees, multi-select lists and the Beads picker goes through the `keys` package (`keys.Toggle` for Space, `keys.Own` for `{` `}` `h` `l` `m` `p` `b` `o`); `Home`/`End` now jump to the first/last field of a list with sections (they could land on a title); `Alt+<letter>` no longer acts as the letter. With `NO_COLOR`, bold is dropped from the banners, the Projects detail title and the primary buttons, like the other styles.
- *Docs:* the symbol legend and table of meanings (`aurum.md`), the TUI architecture, conventions and testing guides, and the board columns editor (reference TUI, config, team setup) in French and English.

### B10
- *Internal:* the team-state is used through the small interfaces of `teamstate` everywhere. The commands of `cmd/` ask for the interface (or a minimal composition) they need, not `*teamstate.Repo`, which only stays where a command builds or clones the repository; the temporary `TeamStateReader`/`TeamStateWriter` compositions are gone. The reads of the clone (claims, members, events, config, policies, wiki, patterns, briefs) take a `context.Context`, so that they stop with the command or the TUI that asked (Ctrl+C, closing the shell). No change in behaviour otherwise. A new guard (`TestCmdDoesNotNameTheWholeRepo`) keeps `*teamstate.Repo` out of `cmd/` except in a frozen list.

### C13a
- *Internal:* plan codes (`N17`, `R6`, `L3`, `P3-T30`, `p6-debts`, `v5 critical`…) are gone from the comments of the packages outside `cmd/`, the TUI and the team-state packages: each comment now says why the code has its shape (audit ids `S-xx`, `CP-Rn`, ADR numbers are kept). `beads/clean_n17_test.go` and `diag_bd13_test.go` are `clean_safety_test.go` and `clean_impact_test.go`; `TestSessionRulesR3` is `TestSessionRulesVerificationConditions`. Dead code removed (nothing called it): nine unused Beads helpers (`ListEpics`, `ReadyChildren`, `ClaimTicket`…), the credential-proxy upstream constructors for Anthropic/OpenRouter/OpenAI, `termlaunch.OpenInNewTerminal` and its AppleScript helpers, four `worktree` helpers, `remote.ReleaseURL`, `hubcontent.IsInstalled` and others. No change in behaviour.

### C13b
- *Changed:* in a multiple-choice list (the checklists of the forms and wizards), only Space ticks the highlighted item; `x` no longer does (it is the archive key of the lists of objects). Declared in the notice of the new keys (`keys.Changes()`, group « Forms »: `x→Space tick in a multiple choice`; the notice is shown again once) and in the key reference (`docs/reference/tui.*`).
- *Internal:* plan codes are gone from the comments, test names and assertion messages of `cmd/`, the TUI, `archtest`, `i18n`, `teamstate`, `tracker`, `mcp`, `mcpresolve`, `notify` and the services (754 lines → 0 in these packages; the whole repository goes from 1 601 lines to 20 false positives: keys `F1`, priorities `P0`–`P3`, the claim format of oh 5.0). The 37 `cmd/v5_*` files, `tui_v5_lifecycle.go` and `config_fields_{v5,rob}.go` are named after their subject (`run_start.go`, `session_claims.go`, `tool_registry.go`, `workflow_inputs*.go`…); `c2_test.go`, `doctor_c1_test.go` and `workflow_c3_test.go` are split next to the code they test; `requireV2` becomes `requireSupportedTool`; `checkpoint.CardR3` becomes `CardVerification`. Dead code removed: `Client.Synthetic`, `Client.SwitchAgent`, `Client.BaseURL` (opencodev2), `limits.ModelAllowed`, `cmd.ReloadApp`, `cmd.RootCmd`, `mcpregistry.ExecOrRun`, `checkpoint.GitFingerprint`, `progress.Spinner.SetMessage`.
