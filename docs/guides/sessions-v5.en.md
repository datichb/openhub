> [Lire en français](sessions-v5.fr.md)

# Sessions on the v5 runtime (opencode V2)

When opencode V2 is installed, `oh start`, the TUI launches, `--parallel`, `--sweep` and headless runs use the v5 runtime. With opencode V1, nothing changes.

## What changes

| | opencode V1 | opencode V2 (v5 runtime) |
|---|---|---|
| Agents and skills | deployed into the project's `.opencode/` | compiled into a session bundle outside the project (`~/.oh/bundles/<hash>`) |
| Visible agents and skills | everything opencode finds | only those of the bundle ("closed world", checked at every start) |
| LLM key | passed to opencode | never passed to opencode: the oh daemon's proxy holds it |
| Session window | oh is suspended | new terminal tab or window; oh stays usable |
| Closing the window | ends the session | the session keeps running; reopen it with `oh session attach` |

## Opening a session

The session client opens in the first method that works, in this order:

1. iTerm2 (tab, split or window: `iterm_style`)
2. Terminal.app
3. tmux (new window when oh runs inside tmux)
4. browser (opencode web interface)

Suspending oh is a last resort only (`attach = "suspend"`).

## Configuration

`~/.oh/config.toml`:

```toml
[session]
attach = "auto"            # auto | iterm | terminal | tmux | browser | suspend
iterm_style = "tab"        # tab | split | window
idle_sleep_minutes = 5     # an idle server goes to sleep after N minutes
```

`OH_SESSION_ATTACH` overrides `attach` for a single command.

## Sleep and resume

- A server goes to sleep when **no** session is executing, no client is attached, and nothing happened for `idle_sleep_minutes`. A decision waiting for you (permission, question) keeps the server awake while oh is open.
- A sleeping session resumes when you reopen it: `oh session attach <id>` restarts its server, then opens the client.
- When you quit the TUI (Ctrl+Q or the `quit` command) while sessions are working, oh asks what to do **for each one**: "finish step, sleep" (default), "background" or "stop now". Esc cancels. Waiting or idle sessions go to sleep right away.
- When you come back, a message sums up what happened while you were away (changed sessions, new decisions, cost).

## Commands

| Command | Purpose |
|---|---|
| `oh session list [--all] [--json]` | sessions, their state (active, waiting, idle, sleeping, stopped) and their waiting decisions |
| `oh session inbox [--json]` | decisions waiting in every session: `⏸` checkpoint, `?` question, `!` permission, `$` budget, `✗` error |
| `oh session approve <id> [--decision once\|always\|reject] [-m "…"]` | answer a permission without opening the session (`always` refused under strict isolation) |
| `oh session answer <id> --field key=value…` | answer an agent question; without `--field`, shows the expected fields |
| `oh session dismiss <id>` | dismiss an alert (error, budget) |
| `oh session send <id> "…" [--queue] [--synthetic]` | send a short instruction (taken at the next step, or after the step with `--queue`) |
| `oh session follow <id>` | follow a session live, read-only (Ctrl+C to quit) |
| `oh session interrupt\|compact <id>` | interrupt the current step, compact the history |
| `oh session model <id> <provider/model>` | switch the model of the next steps |
| `oh session fork <id>` | create a variant (copy of the history, same server) |
| `oh session results <id> [--mr] [--patch] [--json]` | changed files, branch, cost; merge request description; diff |
| `oh session attach <id> [--how auto\|iterm\|terminal\|tmux\|browser\|suspend]` | open (or resume) a session |
| `oh session open <id> --browser [--print]` | open a session in the browser (one-time code, 5 min) |
| `oh session fetch <id> [--no-import]` | fetch a finished remote session (artifacts, import to resume it) — [remote execution](remote-runners.en.md) |
| `oh session resolve <id> [--dry-run] [--yes]` | replay its Beads journal (conflicts, confirmation) |
| `oh session resume <id>` | resume a sleeping session without opening a client |
| `oh session stop <id>` | stop a session, and its server if no other session uses it |
| `oh daemon status` | daemon state (servers, proxy tokens) |
| `oh daemon stop [--force]` | stop the daemon (refused while sessions run, unless `--force`) |
| `oh doctor` | v5 checks: runtime, daemon, git, terminal |

In these commands, `<id>` may be the beginning of an ID (`oh session follow dRcJ`); `approve`, `answer` and `dismiss` also accept a decision ID (shown by `inbox`) when a session has several. **First answer wins**: when the decision was already made in the opencode UI or the browser, oh says so and sends nothing.

## Sessions view (TUI)

Open it from the omnibar (`sessions`) or from the "Sessions" sections of the home, project and team landings. The bottom bar shows `● N ⏸ M` on every screen (live sessions, waiting decisions).

- **To handle**: decisions of every session. `Enter` opens the card (permission: once / always / reject + message; question: generated form; alert: dismiss or attach), `y`/`n` approve or reject a permission, `x` dismisses an alert.
- **Running, Sleeping, Finished (7 days)**; detail of the selected session at the bottom.
- `t` (or `Enter` on a session): live feed on the right (agent, tools, messages, cost). `a` attach, `A` choose how to open (iTerm2, Terminal.app, tmux, browser, here), `m` instruction, `i` interrupt, `M` model, `s` stop, `c` resume, `o` results and merge request description, `w` browser, `f` active project / every project, `r` refresh.

With tmux, the session opens in a new window; with `[session] iterm_style = "split"`, in a pane next to it.

## Checkpoints

A session started from a workflow passes its checkpoints through the `workflow_checkpoint` tool (oh's `workflow` MCP server, added to every bundle). oh applies the mode:

- a checkpoint that is **automatic** in the mode goes through without asking;
- a **paused** checkpoint shows up in "To handle" (⏸) and the agent waits. `Enter` opens the card: the agent's summary, the changes (`Full diff`), the last messages, the timeline; then **Decide**: *Validate*, *Fix first* or *Other instruction*, with a message to the agent (required for the last two). `y` validates directly, `n` opens the card on "Fix first". You can still validate in the opencode UI: the first answer wins;
- an agent locked by `after:` in the workflow is refused until its checkpoint is passed;
- **circuit breaker** (`circuit_breaker`): after N delegations in a row without you, delegations are held and a ✗ alert shows up; `x` (or `oh session dismiss`) dismisses it and releases them.

The session detail shows the timeline: `✔ cp-1 10:03 → developer (3) → ⏸ cp-2 → ○ cp-3`.

From the command line: `oh session approve <id>` validates (`--decision once`), `--decision fix -m "…"` or `--decision other -m "…"` refuses with an instruction, `--decision reject` refuses without one.

## Notifications

The oh daemon shows a system notification when a decision waits for you and when a session finishes its step while nobody is attached. Close notifications are grouped and never contain session content. With [`terminal-notifier`](https://github.com/julienXX/terminal-notifier) installed, a click brings oh's terminal back; otherwise oh uses `osascript` (or `notify-send` on Linux). To turn them off: `[session] notify = "off"` in `hub.toml` (applied when the daemon next starts).

## LLM keys

The key is looked up in this order: project, team, hub, then (Bedrock only) the AWS profile with SigV4 signing. A team key is set in **Team detail**, key `K`, and stored in the keychain as `openhub.team.<team>.provider.<provider>.token`.

The Bedrock region comes from the oh config, then `AWS_REGION` / `AWS_DEFAULT_REGION`, then the AWS profile. When none is set, `us-east-1` is used and oh logs a warning.

Changing the provider, region or key of a project starts a new server on the next launch. Running sessions keep their settings until their server sleeps.

## Environment variables

| Variable | Effect |
|---|---|
| `OH_HOME` | relocates `~/.oh` (test environments) |
| `OH_SESSION_ATTACH` | overrides `[session] attach` |
| `OH_V5=0` | forces the legacy pipeline. **With opencode V2 installed, this pipeline does not work**: use it only for diagnosis. |

## Limitations

- **Windows**: v5 sessions are not supported yet, because the daemon is missing. Use opencode V1 or WSL.
- **Proxy port**: the daemon keeps its proxy port across restarts. If that port was taken by another program meanwhile, the daemon picks another one and puts to sleep the servers that still use the old one (they can no longer reach the provider); working sessions show an error in the inbox. Resume them (`oh session resume <id>` or attach): their server restarts with the new port.
- **Local security**: the agent runs as your user and can reach the daemon socket and `oh.db`, but never the LLM key. See [SECURITY.md](../../SECURITY.md).
