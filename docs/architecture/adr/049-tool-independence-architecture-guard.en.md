> [Lire en français](049-tool-independence-architecture-guard.fr.md)

# ADR-049 — Tool Independence and Architecture Guard

## Status

Accepted

## Date

2026-10-07

## Context

[ADR-038](038-sessionspec-tool-adapters.en.md) set a neutral model (`SessionSpec`, `BundleSpec`) and an adapter interface (`adapters.ToolAdapter`), with a single real adapter, `internal/adapters/opencodev2`. The rest of oh was meant to depend on it only through the interface.

An inventory on 07/10/2026 found 81 Go files, outside the adapter, still naming the tool:

- **direct imports** of the adapter in six `cmd` files and in the remote runner service, which handled the concrete `*opencodev2.Adapter` type;
- **tool-specific logic** outside the adapter: provider ids (`bricks.OpencodeProviderID`), installation of the Linux binary of remote jobs, the `OH_OPENCODE_VERSION` variable, the `+opencode:` image identity, data folders removed by `oh purge`, the format of the former deployment (`internal/deploycleanup`: `.opencode/`, `opencode.json`), native agent names;
- **configuration keys** named after the tool (`[opencode] default_provider`, `[execution] opencode_version`);
- **displayed texts**: 58 messages of the locale files, including the `--include-opencode` flag of `oh purge`;
- comments.

Nothing prevented adding more. Decision D19 (07/10/2026) requires every tool specificity to live only in `internal/adapters/<tool>`.

## Decision

1. **Adapter registry** (`adapters.Registry`): name → constructor, order of preference, `Detect` (the first installed and supported tool) and `Get` (the adapter of a recorded server group, former names included). It is built **explicitly** in a single **composition root**, `cmd/v5_adapters.go`, the only oh file outside the adapters that names a tool. The rest of `cmd` only sees `adapters.ToolAdapter` and `adapters.ToolInfo` (displayed name, binary, version, supported range).
2. **Neutral capabilities** (optional interfaces of `internal/adapters`), implemented by the adapter:
   - `ProviderMapper` (provider id of the tool);
   - `LinuxInstaller` (tool of the job images);
   - `DataLocator` (tool data, files of the former deployment);
   - `LegacyCleaner` (`oh migrate deploy-cleanup`, with a neutral `LegacyPlan`);
   - `Pairer` (opening in the browser);
   - `OutcomeReader` (outcome of the last turn).
   Detection errors are neutral: `adapters.ErrToolNotInstalled` and `*adapters.UnsupportedVersionError`. The format of the former deployment moves to `internal/adapters/opencodev2/deploycleanup`.
3. **Neutral keys and variables**, with the former ones still read and no migration:
   - `[llm] default_provider` and `[execution] tool_version`; the former keys are read in `internal/config/legacy_keys.go`, and `oh config set` still accepts them;
   - `OH_TOOL_VERSION`, with the former variable still read by the adapter during v5.0; the remote pipeline schema becomes 2;
   - `oh purge --include-tool-data`;
   - `oh status --json` gives `tool` and `tool_version`.
   Values already written stay readable (`servers.adapter = "opencode-v2"`, session platform).
4. **Displayed texts**: the tool name comes from `ToolInfo.DisplayName`, passed as a parameter (`%s`) to the messages. Command helps, which take no parameter, use neutral wording ("the session tool"). Comments are neutral, except a dated finding about a release of the tool.
5. **Guard** (`internal/archtest`). These tests cite D19 in their failure message:
   - (1) no file or package imports an adapter, except the adapter itself and the composition root (import analysis and `go list`);
   - (2) no Go identifier or literal containing `opencode` / `Opencode` / `OPENCODE` outside the **allow-list**: the adapter, the composition root, `internal/config/legacy_keys*.go`, and the contract tests against a real server (tags `integration`, `e2e`, `container`);
   - (3) no literal list of the tool's native agents;
   - (4) no locale message naming the tool.

## Consequences

### Positive

- A second tool comes with a package under `internal/adapters/`, one line in the composition root and its capabilities; nothing else changes.
- Any new leak fails in CI, with the reference to D19.
- The optional capabilities have an explicit fallback when a tool cannot do it (no browser, no export, no former deployment).

### Negative / Trade-offs

- Breaking for scripts: `oh purge --include-opencode` becomes `--include-tool-data`, the JSON keys of `oh status --json` change, and the remote pipeline must be generated again (`oh remote setup`).
- The identity of job images changes (`+<adapter>:<version>`): the first remote run after the update rebuilds the image.
- Command helps no longer name the tool: they are vaguer, and the help points to `oh doctor` to know which one is installed.
- The guard relies on a list of tool names (`toolWord`, `toolAdapters`) to extend when an adapter is added.

## Alternatives Considered

| Alternative | Rejected because |
|---|---|
| Adapters registering themselves in `init()` with a blank import in `main.go` (`database/sql` drivers) | Implicit registration; `main.go` would have become the composition root without saying so. |
| Guard limited to Go code, texts and comments cleaned once | Leaks come back through the locale files (58 messages at the time of the inventory). |
| SQLite migration of the `opencode-v2` values and of the `hub.toml` keys | Nothing to gain: the values are read by the registry (aliases) and by `legacy_keys.go`, and written back at the next save. |
| `--include-opencode` kept as a hidden alias | It would have needed a literal naming the tool in `cmd`, and the flag only serves to test a reinstallation. |
