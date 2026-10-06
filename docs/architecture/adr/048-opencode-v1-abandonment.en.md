> [Lire en français](048-opencode-v1-abandonment.fr.md)

# ADR-048 — Dropping opencode V1

## Status

Accepted

## Date

2026-10-06

## Context

Decision D3 of the v5 study (2026-10-05) made opencode V2 the target while keeping V1 compatible: a V1 adapter was planned (rendering to the V1 schema, `--agent` launch, reduced capabilities, with the interface hiding what V1 cannot do).

In practice, phase 0 kept the **legacy path** for V1 instead of writing that adapter:

- `launcher.Launch` and `internal/opencode` (interactive and headless launch);
- the V1 server client of `internal/parallel`, plus `internal/sweep` and `internal/llm`;
- the global V1 plugins (`internal/plugin`, RTK, context-mode) and `oh upgrade opencode`;
- statistics read from opencode's database;
- an alias fallback (`oh start`, `audit`, `review`, `debug`, `takeover-brief`) when V2 or the workflow was missing, and the `OH_V5=0` variable to force the legacy path.

The cost was twofold:

- **two products**: nothing that makes v5 existed under V1 (bundle per workflow, verified closed world, controlled checkpoints, decisions from oh, container, remote);
- **two paths to maintain and test** in every track: fallbacks in the CLI and the TUI, V1 statistics, deployment code kept for V1.

D3 was revised on 2026-10-06: **V1 is no longer supported in v5**.

## Decision

1. **opencode V2 is required**. Without opencode, or with a version outside the range, oh refuses to launch with a clear message (`cmd.v1.unsupported.*`) pointing to `oh doctor` and the migration guide. The range is declared in `opencodev2/compatibility.json` (oh 5.0: opencode 2.0.0 → 2.99.99). The "opencode V2" Doctor check runs first.
2. **The legacy launch is removed** (track 3.E, P3-T30):
   - `internal/opencode`, `internal/parallel`, `internal/sweep`, `internal/llm`, `internal/headlesstrack`, `internal/plugin`;
   - `launcher.Launch`, `platform.SessionPlatform` and its types;
   - `oh upgrade opencode`, `oh plugin`, the Plugins view;
   - the parallel monitor and the merge view;
   - the `[opencode] version|channel|auto_update|install_dir` keys;
   - `OH_V5`.
3. **The neutral model and the adapter interface stay** ([ADR-038](./038-sessionspec-tool-adapters.en.md)) for other tools (BL-6). Only the `opencodev2` adapter exists.
4. **The former commands always go through `oh run`**, without fallback:
   - `oh start --agent X` and the TUI `coder` command launch the shipped `libre` workflow (entry agent chosen at launch, additive `entry.selectable` field in the `oh/v1` schema);
   - `oh start --resume <id>` becomes `oh session attach <id>`;
   - `--parallel` without `--tickets` is refused;
   - a missing workflow gives an explicit error.
5. **Metrics** (`oh metrics`, dashboard, Metrics view) are read from `oh.db` (`internal/sessionstats`).
6. opencode **plugins** are declared per workflow (`plugins:`); a plugin written for V1 (`server()` export) is not loaded by V2. [ADR-014](./014-context-mode-plugin.en.md) (context-mode installed globally) is therefore deprecated.
7. Migration is documented: `docs/guides/migration-v5.{fr,en}.md`, `MIGRATION.md`.

## Consequences

### Positive

- A single launch path: every v5 guarantee (closed world, checkpoints, secrets kept out of the tool) holds for every session.
- A lot of code removed (six packages, the CLI and TUI fallbacks, 125 orphan i18n keys), and tests without a V1/V2 matrix.
- oh no longer reads opencode's database nor changes the user's global configuration (V1 plugins).

### Negative / Trade-offs

- A user still on V1 must move to V2 before oh 5; there is no transition release.
- Plugins that only exist for V1 are lost: `context-mode` does not load under V2, and RTK support is removed.
- `oh upgrade opencode` is gone: opencode is installed or updated outside oh. V2 is published through Homebrew, not through the `opencode-ai` npm package.
- oh depends on a single version family of a single tool; an incompatible V3 would block every launch until oh is updated.

## Alternatives Considered

| Alternative | Rejected because |
|---|---|
| Write the planned V1 adapter (reduced capabilities) | Development and test cost for a replaced tool; none of the v5 features would have been available with it. |
| Keep the legacy V1 path during v5.x | Two products to maintain, fallbacks in every command, and deployment code kept for V1 only ([ADR-043](./043-session-bundle-deploy-removal.en.md)). |
| First ship a transition release (4.x) compatible with V1 and V2 | Delays v5 without avoiding the migration; V2 was already the target and is available. |
