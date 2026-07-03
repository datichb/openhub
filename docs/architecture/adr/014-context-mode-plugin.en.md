> 🇫🇷 [Lire en français](014-context-mode-plugin.fr.md)

# ADR-014 — context-mode plugin as a token optimization layer complementary to RTK

## Status

Accepted

## Context

The hub has had RTK since v1.3.0 to compress bash outputs (60-90% reduction). Three gaps were identified that RTK does not cover:

**Gap 1 — read/webfetch/MCP outputs enter the context in full.** When an agent reads a 2,000-line file or fetches documentation via webfetch, the entire output is injected into the LLM context. RTK only intercepts bash commands — native OpenCode tools (`read`, `webfetch`) and MCP calls do not go through this hook.

**Gap 2 — No session continuity after compaction.** When OpenCode automatically compacts the context (configurable threshold via `compaction.auto`), the agent loses session state. It often has to re-explore the codebase to recover state, wasting tokens identical to those already consumed.

**Gap 3 — Expensive multi-file exploration.** An agent searching for specific information in a codebase will typically chain 10-15 `read`/`glob`/`grep` calls. A targeted analysis script achieves the same task in 1-2 calls with structured output, but this pattern is not naturally adopted without explicit instruction.

## Decision

Integrate the `context-mode` OpenCode plugin as a global installation (`~/.config/opencode/plugins/context-mode.ts`).

**Reasons for the choice:**
- npm plugin with native OpenCode plugin — no HTTP proxy required, no Python/Rust dependency
- Orthogonal coverage to RTK: RTK handles bash, context-mode handles native tools
- Global installation via `oh plugin install context-mode` — no per-project AGENTS.md required thanks to the `experimental.chat.system.transform` hook which injects instructions into each session
- Complementarity with the existing compaction mechanism (`compaction.auto`, `compaction.prune`)

**Architecture:**

The `plugins/context-mode/context-mode.ts` plugin is a thin wrapper that:
1. Verifies the availability of the `context-mode` npm package
2. Imports and delegates to the upstream npm plugin (stable + experimental hooks)
3. Adds hub-specific session tracking (toasts, logs, metrics)

AGENTS.md is not required for global installation because the `experimental.chat.system.transform` hook injects instructions directly into the system prompt of each session.

**Documented configuration** in `config/hub.json` under `token_optimization.plugins.context-mode`.

## Consequences

### Positive

- **-80-98% on large tool outputs**: files > 1K tokens, full webfetch pages, large MCP outputs — indexed out-of-context, only the relevant passage enters the LLM.
- **Session continuity**: 0 tokens wasted after automatic compaction. The agent recovers session state via BM25 without re-exploration.
- **Think in Code**: reduction in `read`/`glob`/`grep` call count for broad explorations.
- **Zero friction**: one-command installation, no per-project configuration.
- **RTK complementarity**: both plugins coexist without conflict. Full stack = RTK (bash) + context-mode (read/webfetch/MCP).

### Negative / trade-offs

- **Node.js >= 22.5.0 dependency**: non-negotiable prerequisite of the `context-mode` npm package. Environments with Node < 22.5 cannot use this plugin. RTK remains functional without context-mode.
- **Experimental hooks**: `experimental.chat.system.transform` and `experimental.session.compacting` may change with OpenCode updates. The plugin operates in degraded mode (stable hooks only) if these hooks are absent — the base sandbox remains active but context-mode instructions are not injected into the system prompt.
- **Dynamic npm package import**: if the `context-mode` npm package changes its export API, the wrapper must be updated. The plugin is designed to be resilient (fallback to degraded mode if import fails).

## Rejected Alternatives

**headroom (chopratejas/headroom)**: Python + Rust + HuggingFace model compression layer. Too heavy for a hub already well optimized. In MCP mode (the only lightweight option), headroom is opt-in — the agent must explicitly choose to call `headroom_compress` tools. Automatic interception requires HTTP proxy mode, which introduces a network dependency in the infrastructure. headroom cites RTK as the right layer for shell outputs — the two tools have overlapping scopes without complementing each other as cleanly as RTK + context-mode.

**MCP server context-mode without OpenCode plugin**: the MCP server alone exposes sandboxing tools but cannot automatically intercept native OpenCode `read`/`webfetch` calls. The agent must explicitly choose to call `ctx_fetch_and_index` — reduced efficiency, no session continuity.

**Do nothing**: the 3 identified gaps (read/webfetch outputs, compaction, multi-file exploration) have a real impact on long sessions with extensive codebase exploration. The integration cost is low (thin wrapper, global installation). The benefit/risk ratio justifies adoption.

## Impact

| File | Action |
|------|--------|
| `plugins/context-mode/context-mode.ts` | Created — thin wrapper plugin |
| `plugins/context-mode/package.json` | Created — npm metadata |
| `scripts/cmd-plugin.sh` | Modified — added context-mode verification block |
| `config/hub.json` | Modified — added `token_optimization.plugins.context-mode` |
| `docs/guides/context-mode-plugin.fr.md` | Created — installation guide |
| `docs/guides/context-mode-plugin.en.md` | Created — installation guide (EN) |
