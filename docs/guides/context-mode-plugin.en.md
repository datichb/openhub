> 🇫🇷 [Lire en français](context-mode-plugin.fr.md)

# Context-mode Plugin Installation Guide

This guide explains how to install the context-mode plugin for OpenCode from opencode-hub.

## Prerequisites

1. **OpenCode** >= 1.15.0 installed
   ```bash
   opencode --version
   ```

2. **opencode-hub** cloned and configured
   ```bash
   cd ~/.opencode-hub
   git pull
   ```

---

## Automatic Installation (Recommended)

```bash
oc plugin install context-mode
```

The script will:
1. Verify that OpenCode is installed
2. Add `"context-mode"` to the `"plugin"` array in `.opencode/opencode.json`

OpenCode will automatically install the `context-mode` npm package from its cache on the next startup, using its native Bun runtime.

---

## How It Works

context-mode is an **OpenCode native npm plugin** (declared via `"plugin": ["context-mode"]` in `opencode.json`). OpenCode handles installation, updates, and loading automatically — no `.ts` wrapper or manual `node_modules` management required.

The package is cached in `~/.cache/opencode/node_modules/` and loaded by OpenCode's integrated Bun runtime.

---

## Verifying the Installation

### 1. Restart OpenCode

If OpenCode is running, close it and relaunch it from the hub directory:

```bash
cd ~/.opencode-hub
opencode
```

### 2. Check the Logs

```bash
tail -f ~/.cache/opencode/logs/opencode.log | grep context-mode
```

At session start, you should see the plugin loaded.

### 3. Test the Plugin

In OpenCode, open a large file or perform a webfetch:

```
> Read the file src/services/auth.service.ts
```

If the file is more than ~4,000 tokens, a toast appears:
```
🗜️ context-mode sandboxed ~12.3K tokens (read)
```

### 4. Session Statistics

At end of session (closing OpenCode), a summary toast appears:
```
🗜️ context-mode: 8 tools sandboxed, ~45.2K tokens saved
```

---

## What the Plugin Does

The plugin works on three axes complementary to RTK:

| Axis | What RTK covers | What context-mode adds |
|------|----------------|------------------------|
| Bash outputs | ✅ `git diff`, `find`, `cat`, logs... | — |
| `read` / `webfetch` outputs | ❌ | ✅ Indexed out-of-context (SQLite + BM25) |
| MCP outputs | ❌ | ✅ Same |
| Session continuity | ❌ | ✅ Resume via BM25 after compaction |

### Sandbox tools

When the agent reads a large file or makes a webfetch call, context-mode intercepts the result and indexes it outside the LLM context. The agent can then query the index by semantic similarity — only the relevant passage enters the context.

**Measured impact:** 80-98% reduction on large outputs (files > 1K tokens, full web pages).

### Session continuity

Each session event is stored in SQLite. If OpenCode automatically compacts the context, the agent retrieves the session state via BM25 without re-exploring the codebase.

**Measured impact:** 0 tokens wasted after compaction (vs. full codebase re-exploration).

### Think in Code

The plugin instructs the agent to write a targeted analysis script rather than chaining 10 `read`/`glob`/`grep` calls. One script replaces multi-file exploration.

---

## OpenCode Hooks Used

| Hook | Stability | Role |
|------|-----------|------|
| `tool.execute.before` | Stable | Intercepts `read`, `webfetch` calls before execution |
| `tool.execute.after` | Stable | Estimates tokens saved on large outputs |
| `dispose` | Stable | Session summary (toast + log) |
| `experimental.chat.system.transform` | **Experimental** | Injects context-mode instructions into the system prompt — no AGENTS.md needed |
| `experimental.session.compacting` | **Experimental** | Session continuity after automatic compaction |

> **Note on experimental hooks:** The `experimental.*` hooks may change with OpenCode updates. The plugin operates in degraded mode (stable hooks only) if these hooks are absent or modified — the basic sandbox remains active. Update the plugin after each major OpenCode update.

---

## Complementarity with RTK

RTK and context-mode are **orthogonal** — they cover different layers:

```
Bash command         → RTK intercepts         → compressed output before injection
read/webfetch call   → context-mode intercepts → output indexed out-of-context
```

Both plugins can coexist without conflict. Installation order doesn't matter.

**Recommended full stack:**
1. `oc plugin install rtk` — bash outputs (-60-90%)
2. `oc plugin install context-mode` — read/webfetch/MCP outputs (-80-98%) + session continuity

---

## Troubleshooting

### OpenCode doesn't load context-mode

Verify that the hub's `.opencode/opencode.json` contains `"context-mode"` in the `"plugin"` array:

```bash
cat .opencode/opencode.json
# Expected: { "$schema": "...", "plugin": ["context-mode"] }
```

If absent, re-run the installation:
```bash
oc plugin install context-mode
```

### The `experimental.*` hooks are not active

If `experimental.chat.system.transform` is absent in your OpenCode version, the plugin operates in degraded mode: the basic sandbox (tracking + token estimation) remains active, but context-mode instructions are not injected into the system prompt.

Check your OpenCode version:
```bash
opencode --version
```

If < 1.15.0, update: `npm install -g opencode-ai`

### Conflict with a `context-mode` MCP server

If you already have a `context-mode` MCP server installed, the OpenCode plugin takes precedence on system prompt injection but both coexist without functional conflict.

---

## Expected Metrics

| Output type | Estimated token reduction |
|-------------|---------------------------|
| Source file (>1K tokens) | 80-95% |
| Full webfetch page | 85-98% |
| Large MCP output | 70-90% |
| After compaction (session continuity) | 100% (0 tokens lost) |

---

## Updates

```bash
cd ~/.opencode-hub && git pull
# OpenCode updates the package automatically on next startup
```

## Uninstallation

```bash
oc plugin remove context-mode
# Then restart OpenCode
```

---

**Version:** 2.0.0 (2026-06-12)
**Compatible with:** context-mode npm ^1.0.0, OpenCode >= 1.15.0
