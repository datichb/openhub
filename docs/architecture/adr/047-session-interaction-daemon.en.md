> [Lire en français](047-session-interaction-daemon.fr.md)

# ADR-047 — Session Interaction, Multi-Session and the `ohd` Daemon

## Status

Accepted

## Date

2026-10-05

## Context

Up to v4, `oh` launched opencode in the terminal: the session was a process. Closing the window ended it, and oh knew nothing more about it.

Parallel mode relied on a separate coordinator that polled the API every 5 seconds ([ADR-036](./036-platform-abstraction-layer.en.md)), with a dedicated monitor. Worktrees ([ADR-012](./012-git-worktree.en.md)) were created by the orchestrator or by `--parallel`. Nothing allowed following several sessions, answering them without opening them, or freeing the memory of a forgotten server.

opencode V2 changes the picture:

- a **long-lived server** one attaches to;
- **several clients** on a session, the first answer winning (F20);
- **several directories per server** (F19);
- an **event stream** without replay (F18).

A server uses about 340 to 440 MB and starts in 1.4 s (F22).

Decisions I1 to I7 (interaction, multi-session, lifecycle) and S2 (real-time tracking). The daemon was decided on 2026-10-05 during phase 0.

## Decision

### 1. oh is the control tower, opencode the cockpit (I1)

Closing the opencode window **does not stop** the session: only "Stop" interrupts it. Structured decisions (checkpoints, questions, permissions) can be taken from any client ([ADR-042](./042-checkpoints-headless-decisions.en.md)); free conversation happens in opencode.

### 2. Server groups (I5)

- One `opencode serve` per **group**. The group key has the form `<project>-<bundle12>-<config10>-<runtime>`, with an `-sN` suffix for a neighbor group in a container.
- It contains:
  - the bundle hash, hence the workflow version;
  - a fingerprint of the configuration: project, provider, region, source and secret fingerprint, model list when active, strict isolation when active;
  - the runtime.
- The sessions of a group each have their own directory (base or worktree), session environment and rules.
- Server registry: `servers` table (v28) and `~/.oh/servers/<group>/`. On resume, a session stays in its original group, where its data is.

### 3. The `ohd` daemon

**Operation.**

- One daemon per user, on the Unix socket `~/.oh/run/ohd.sock` (0600, peer UID checked).
- It is started on demand, with a spawn lock. It stops after 10 minutes without a live server, except during an image build or remote tracking.
- It is only replaced by a new version when no session is active.

**What it hosts.**

- the credential proxy ([ADR-044](./044-credential-proxy-session-limits.en.md));
- the gateways ([ADR-046](./046-beads-gateways.en.md));
- the plugin hooks and the CheckpointService;
- system notifications;
- the periodic task (remote pipeline tracking, Beads lease).

**Session tracking.**

- One subscription to the event stream per ready server. At each (re)connection, a resync reads the active sessions and the pending permissions and forms.
- Derived states: `waiting` (pending decision) > `active` (loop running) > `idle`. Cost and tokens are read at the end of each turn.
- Sub-sessions are attached to their root session: their activity and decisions are reported on it.
- Live stream `GET /v1/stream` (NDJSON, preceded by the last 100 entries) for the TUI and `oh session follow`.

**Safety.** A PID is never signaled unless an authenticated call to the server has succeeded: a PID reused by another application is left alone.

### 4. Opening a session (I2)

- In `auto` mode, oh tries in order:
  1. the current terminal (iTerm2: tab, split or window; Terminal.app: new window);
  2. the other terminal;
  3. tmux, if `$TMUX`;
  4. the browser (`pair`);
  5. as a last resort, suspending the TUI.
- Settings `[session] attach` and `iterm_style`.
- The tab runs `oh session attach <id>`, without any secret on the command line. The attached client sends heartbeats to the daemon.

### 5. Lifecycle and sleep (I7)

- States: `preparing`, `queued`, `active`, `waiting`, `idle`, `sleeping`, `completed`, `failed`, `stopped`.
- **Sleep**, decided by the daemon for each group:
  - never if a client is attached or a loop is running;
  - a pending decision keeps the server awake while a TUI is open;
  - otherwise, after `[session] idle_sleep_minutes` (5 by default): server stopped, tokens revoked, container removed, sessions `sleeping`.
- A sleeping session can be resumed (same data directory, bundle reloaded by hash); `oh session attach` resumes it automatically.
- **When quitting oh**:
  - idle or waiting sessions are put to sleep;
  - for each working session, the user chooses: "finish the step then sleep" (default), "continue in the background" or "stop now".
- At the next start, a "while you were away" summary lists the sessions, the cost and the decisions still pending.
- Results (diff, cost) are saved in `~/.oh/sessions/<id>/results/` before sleep or stop. A `session.complete` team event is emitted when a session ends.

### 6. Multi-session

- `oh run <wf> --tickets a,b` (or multi-selection in the launch form) opens N sessions in a single server.
- **Automatic worktree (O10)**: a session that writes gets a worktree if another writing session already holds the directory (a sleeping session does not hold it). Several tickets that write get one worktree each.
- A lock per project and per directory prevents a double launch.
- This replaces the `--parallel` coordinator and monitor.

### 7. Surfaces

- **TUI**:
  - the **Sessions** view replaces the parallel view, with the To handle, Running, Sleeping, Finished and To fetch sections;
  - session detail, live stream, instruction, interrupt, model change, stop, resume, browser, "Chain with…";
  - `● N ⏸ M` badge and Sessions sections on the hub, project and team home screens.
- **CLI**: `oh session list|inbox|attach|follow|approve|answer|dismiss|send|interrupt|compact|model|fork|results|stop|resume|open|fetch|resolve`. A unique ID prefix is enough.

### 8. Windows

The daemon runs inside the oh process. Sessions are put to sleep when oh exits: "continue in the background" is not offered.

## Consequences

### Positive

- A session survives closing its window and oh exiting, and it can be resumed even after a machine restart.
- Several sessions are followed and driven from a single place, with notifications, without polling.
- Memory and cost are kept in check: an unused server stops on its own.
- Parallel mode is no longer a separate path: it is N sessions in one group.

### Negative / Trade-offs

- Each group costs about 340 to 440 MB, and a different workflow means another server.
- The daemon is a central point: if it is stopped, nothing is tracked. The `oh session` commands restart it when a server is still alive.
- No event replay: what is missed during an outage is only recovered by the resync.
- A workflow session only becomes "finished" when it is stopped: a finished loop stays `idle`, then `sleeping`.
- Opening:
  - the macOS "Automation" permission cannot be detected in advance;
  - Terminal.app tabs would require the Accessibility permission, hence one window per session;
  - iTerm2 and tmux were not tried for real;
  - with `attach = "iterm"` and no iTerm2, the client is started inside the `oh run` process (issue Q3-3).
- Sessions of other team members (claims) are not shown yet.

## Alternatives Considered

| Alternative | Rejected because |
|---|---|
| Launch opencode in the current terminal (as in v4) | One window = one session; nothing to track or resume, no decision from oh. |
| One global server for every workflow | Session permissions do not hide sub-agents (F21): the closed world would be lost. |
| One server per session | Memory and startup multiplied, without benefiting from a server's multiple directories (F19). |
| Poll the API at regular intervals | 180 requests per minute for 5 sessions in the former coordinator; the event stream with resync is enough. |
| Supervise from the TUI or the CLI | Nothing is tracked, put to sleep or notified anymore when oh is closed. |
| Stop everything, or leave everything running, when quitting oh | Loss of work in progress on one side, endless cost and memory on the other; hence "finish the step then sleep" by default. |
