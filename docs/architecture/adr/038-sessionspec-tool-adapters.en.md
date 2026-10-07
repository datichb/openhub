> [Lire en français](038-sessionspec-tool-adapters.fr.md)

# ADR-038 — Neutral `SessionSpec` Model and Tool Adapters

## Status

Accepted — **Evolved by [ADR-049](049-tool-independence-architecture-guard.en.md)**

## Date

2026-10-05

## Context

[ADR-036](./036-platform-abstraction-layer.en.md) introduced `platform.SessionPlatform` to decouple `oh` from opencode. That interface followed the shape of opencode V1: interactive launch through the command line (`RunInteractive`, `ExecReplace`), headless runs, a separate "parallel" server, and `RequiresDeploy()` (agents were deployed into the project).

opencode V2 (2.0.20) broke this model:

- `opencode --agent X` no longer opens the interface (F1): every `oh` launch, from the CLI or the TUI, failed.
- Without a dedicated server, the client talks to a shared service that ignores the client's environment variables (F2).
- On the other hand, V2 provides a full API: sessions created with an ID and permissions chosen by the caller, an event stream, permissions and forms that can be answered headlessly, export and import (F12, F17, F18).

v5 needs one session per workflow with its own bundle, a verified closed world, decisions taken from oh, several sessions per server, and several execution environments. Decision D2 requires all of this logic to stay in `oh`, with one adapter per tool, so that another tool can be plugged in later (BL-6).

## Decision

### 1. Neutral model (`internal/sessionspec`)

`oh` describes a session without knowing anything about the tool:

- **`SessionSpec`**: ID (`ses_` + 26 characters, chosen by oh), group key, workflow (ID, layer, version, hash), bundle, working directory (`Location`), entry agent, mode, rendered prompt, model, provider (`ProviderSpec`: ID, region, proxy URL, authentication type), session-specific environment (`SessionEnv`), session rules (`SessionRules`), runtime, and attach preference.
- **`BundleSpec`**: compiled bundle content (agents, skills, MCP, plugins, neutral permissions, delegation graph `SubagentGraph`, depth `MaxDepth`, Code Mode, isolation level, `StrictIsolation`, description of the workflow checkpoints, default model).
- **Neutral permissions** (`PermissionRule`: action, resource, effect); an MCP tool action is written `mcp:<server>/<tool>` and translated by the adapter.
- Machine-dependent paths are variables (`{{oh.bundle}}`, `{{oh.bin}}`) expanded by the adapter at startup: the bundle hash does not depend on them.

### 2. Adapter interface (`internal/adapters`)

`adapters.ToolAdapter`: `Name`, `Detect` (binary, version, compatibility), `Capabilities` (isolation, events, headless decisions, several directories per server, plugin hooks, attach), `Render`, `StartServer` / `StopServer`, `Attest`, `CreateSession`, `SendPrompt`, `AttachCommand`, `Events`, `ActiveSessions`, `Pending`, `Reply`, `Control` (instruction, interrupt, model, compaction), `Results` (diff, cost, tokens).

Functions a tool may lack are **optional interfaces**, detected at run time: `SessionRulesSetter`, `SessionEnvSetter`, `ChildLister`, `Forker`, `TurnWaiter`, `ActionNamer`, `SessionPorter` (export and import), Linux tool for the container.

Rule: **no opencode agent, tool or event name outside the adapter**. Events are normalized (`EventKind`), the live feed is decoded in the adapter, and actions are neutral.

### 3. Single implementation: `internal/adapters/opencodev2`

Launch model (O3), the same for the CLI and the TUI:

1. A dedicated `opencode serve` per server group: free port, random password, group-specific `XDG_DATA_HOME`, `OPENCODE_DISABLE_PROJECT_CONFIG=1`, rendered config passed through `OPENCODE_CONFIG_CONTENT`, credential proxy token instead of the provider key.
2. Wait until ready: agents and skills listed (loading is asynchronous).
3. `Attest`, which verifies the closed world.
4. Session created through the API: ID chosen by oh, agent, directory, session rules. Then the initial prompt is sent.
5. The interface is opened: `opencode --server <url> -s <id>`, with the password passed in the command's environment.

The oh plugin (embedded TypeScript, `{ id, setup }` object) is installed in the group data directory. It appends the current agent's body to the system prompt (`session.hook("context")`, O2: agents have no `system`, and opencode's base prompt is kept). It also removes agents and skills that are not in the bundle. It contains no business logic.

The accepted version range is declared per oh version (`opencodev2/compatibility.json`: opencode 2.0.0 → 2.99.99 for oh 5.0); `Detect` rejects versions outside the range.

### 4. Services

The RunService (`internal/runsvc`), the SessionService (`internal/services/session`), the CheckpointService and the `ohd` daemon only call the interface. `platform.SessionPlatform`, `SessionServer` and `ParallelRunner` are removed together with opencode V1 ([ADR-048](./048-opencode-v1-abandonment.en.md)). The `internal/platform` package keeps only `Credentials` and the statistics types; `StatsProvider` is now implemented by `internal/sessionstats` on `oh.db`.

## Consequences

### Positive

- The logic (bundle, checkpoints, decisions, budgets, sleep) is written once in oh. Adding a tool means writing an adapter package, without touching the services, the CLI or the TUI.
- The same launch serves the CLI, the TUI, the container and the remote job.
- Decisions (checkpoints, questions, permissions) are taken from oh without attaching an interface, and the first answer wins, whatever the client.
- Three test layers: golden tests of the rendering, contract tests against a real `opencode serve` without an LLM (`-tags integration`, rerun for each opencode version), Haiku e2e tests (`-tags integration e2e`).
- `oh` no longer reads opencode's database: cost, tokens and states come from the API and the event stream.

### Negative / Trade-offs

- The interface has the shape of opencode V2 (HTTP server, permissions and forms that can be driven). A tool without a server mode will only have partial capabilities, and the interface may have to change: neutrality is not proven while only one adapter exists.
- Several V2 API endpoints are experimental (export, import, per-session `instructions`): contract tests must be rerun for each opencode version.
- Some opencode 2.0.20 limitations are worked around in oh, and these workarounds must be reviewed with each version:
  - the session shell only sees the session environment (F30): `PATH`, `HOME` and the useful machine variables are injected again by the RunService;
  - the session environment is not passed to sub-sessions (F31): the daemon applies it to each sub-session it attaches;
  - the message of a permission refusal does not reach the agent: the instruction is sent as a separate message;
  - agents and skills are listed asynchronously: explicit wait before `Attest`.
- Optional interfaces multiply code paths: every caller must handle their absence.

## Alternatives Considered

| Alternative | Rejected because |
|---|---|
| Extend `platform.SessionPlatform` (ADR-036) | Modeled on V1's command-line launch; no session creation through the API, no headless decisions, no multiple sessions per server; assumes deployment into the project. |
| Launch the interface directly (`opencode --agent`, or `--standalone` with the config in the environment) | `--agent` no longer exists for the V2 interface (F1); `--standalone` neither lets oh drive the session nor lets several clients attach to it. |
| Use opencode's shared service | It ignores the client's environment (F2): impossible to isolate a session's config, data and credentials. |
| Put the logic in the opencode plugin | Tied to one tool, hard to test, and the plugin only sees one session at a time; the plugin stays thin and queries oh (O6). |
| Write the config into the project (`.opencode/`, `opencode.json`) | One world per directory, writes into the user's files: see [ADR-043](./043-session-bundle-deploy-removal.en.md). |
