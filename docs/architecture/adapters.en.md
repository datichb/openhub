# Adapters Architecture

An **adapter** translates canonical hub agents to the native format
of a target AI tool (opencode, claude-code, etc.).

---

## Mandatory Contract

Every adapter (`scripts/adapters/<target>.adapter.sh`) must export **8 functions**.
Loading is performed by `load_adapter()` in `scripts/lib/adapter-manager.sh`,
which verifies via `declare -F` that the 8 functions exist after the `source`.

| Function | Role | Signature |
|----------|------|-----------|
| `adapter_validate` | Checks that the target tool is installed and accessible | `adapter_validate()` — returns 0/1 |
| `adapter_needs_node` | Indicates whether Node.js is required for the tool | `adapter_needs_node()` — `return 0` (yes) or `return 1` (no) |
| `adapter_deploy_files` | **Phase 1** — Copies canonical agents to the target project | `adapter_deploy_files deploy_dir project_id [provider_override]` |
| `adapter_deploy_config` | **Phase 2** — Applies provider/model configuration (e.g. `opencode.json`) | `adapter_deploy_config deploy_dir project_id [provider_override]` |
| `adapter_deploy` | Compatibility wrapper — chains Phase 1 + Phase 2 | `adapter_deploy deploy_dir project_id [provider_override]` |
| `adapter_install` | Installs the target tool (called by `oc install`) | `adapter_install()` |
| `adapter_update` | Updates the target tool (called by `oc update`) | `adapter_update()` |
| `adapter_start` | Launches the tool in the project (called by `oc start`) | `adapter_start project_path prompt project_id` |

### Phase separation

`oc deploy` runs both phases sequentially with a distinct visual section for each:

```
▶  Phase 1 — Copy agents
◆  12 agent(s) deployed

▶  Phase 2 — Provider / model configuration
◆  opencode.json  (model: amazon-bedrock/..., provider: bedrock)
```

`oc start --provider <provider>` only runs **Phase 2** when agents are already in place —
Phase 1 is unnecessary in that case.

### Parameter Details

#### `adapter_deploy_files deploy_dir project_id [provider_override]`

- `deploy_dir`: path of the project directory to deploy into (e.g. `/home/user/my-project`)
- `project_id`: project identifier in `projects.md` (e.g. `MY-PROJECT`). Used to
  read the language (`get_project_language`) and agent filters (`should_deploy_agent`).
- `provider_override`: ignored in Phase 1 — present for signature consistency.

Responsibilities:
1. Create the output directory structure (e.g. `.opencode/agents/`, `.claude/agents/`)
2. Load agent metadata via `_load_agent_metadata` (scan without writing)
3. For each retained agent: call `build_agent_content` and write the `.md` file
4. Populate global variables `_DEPLOY_FILES_AGENT_KEYS/VALS/FILES/COUNT`

#### `adapter_deploy_config deploy_dir project_id [provider_override]`

- `deploy_dir`: path of the project directory
- `project_id`: project identifier (to resolve provider and API key)
- `provider_override`: provider override (e.g. `bedrock`, `anthropic`)

Responsibilities:
1. Load agent metadata if `_DEPLOY_FILES_AGENT_KEYS` is empty (direct call without Phase 1)
2. Resolve the effective model and provider
3. Build and write the configuration file (e.g. `opencode.json`)
4. For adapters without configuration (e.g. `claude-code`): explicit no-op

**Autonomy:** this function can be called standalone without having run `adapter_deploy_files`
first — it loads the necessary metadata itself.

#### `adapter_deploy deploy_dir project_id [provider_override]`

Compatibility wrapper that chains `adapter_deploy_files` then `adapter_deploy_config`.
Used by `cmd-deploy.sh --diff`, `cmd-sync.sh`, `cmd-provider.sh` and tests.

#### `adapter_start project_path prompt project_id`

- `project_path`: absolute path of the project directory
- `prompt`: initial prompt (may be empty)
- `project_id`: project identifier (for specific configuration)

---

## Available Utility Functions

An adapter has access to functions from `common.sh` and `prompt-builder.sh`:

| Function | Usage |
|----------|-------|
| `extract_frontmatter_value file key` | Reads a value from YAML frontmatter |
| `extract_frontmatter_list file key` | Parses an inline YAML list → one value per line |
| `strip_frontmatter file` | Returns the body without the frontmatter |
| `agent_supports_target file target` | Checks if an agent supports the target |
| `get_agent_id file` | Returns the `id` from the frontmatter |
| `get_agent_mode file` | Returns the `mode` from the frontmatter (`primary` by default) |
| `get_effective_agent_mode file project_id` | Effective mode: project override > frontmatter > `primary` |
| `build_agent_content file [target] [lang]` | Assembles complete content (header + skills + body) |
| `get_project_language project_id` | Returns the project language (or empty string) |
| `get_project_api_provider project_id` | Returns the API provider (anthropic, litellm, etc.) |
| `get_project_api_key project_id` | Returns the API key |
| `get_project_api_base_url project_id` | Returns the base URL (or empty string) |

### Global variables populated by `adapter_deploy_files` / `_load_agent_metadata`

These variables are available to `adapter_deploy_config` after Phase 1:

| Variable | Content |
|----------|---------|
| `_DEPLOY_FILES_AGENT_KEYS` | Array of retained `agent_id` values |
| `_DEPLOY_FILES_AGENT_VALS` | Array of effective modes (`primary`, `subagent`, …) |
| `_DEPLOY_FILES_AGENT_FILES` | Array of canonical source file paths |
| `_DEPLOY_FILES_COUNT` | Number of retained agents |

---

## Creating a New Adapter

1. Create `scripts/adapters/<target>.adapter.sh` with the **8 contract functions**
2. Add the target to `config/hub.json` (`active_targets` and `default_target` if relevant)
3. The file will be loaded automatically by `load_adapter` — no modification of
   `adapter-manager.sh` is needed
4. Test: `oc deploy <target>` then verify the generated files

### Minimal Example

```bash
#!/bin/bash
# scripts/adapters/my-tool.adapter.sh

adapter_validate() {
  command -v my-tool &>/dev/null || { log_error "my-tool not installed"; return 1; }
}

adapter_needs_node() { return 1; }

# Phase 1: copy agent files
adapter_deploy_files() {
  local deploy_dir="${1:-$HUB_DIR}"
  local project_id="${2:-}"
  local out_dir="$deploy_dir/.my-tool/agents"
  mkdir -p "$out_dir"

  local lang=""
  [ -n "$project_id" ] && lang=$(get_project_language "$project_id")

  _DEPLOY_FILES_AGENT_KEYS=()
  _DEPLOY_FILES_AGENT_VALS=()
  _DEPLOY_FILES_AGENT_FILES=()
  _DEPLOY_FILES_COUNT=0

  while IFS= read -r f; do
    [ -f "$f" ] || continue
    agent_supports_target "$f" "my-tool" || continue
    local agent_id; agent_id=$(get_agent_id "$f")
    should_deploy_agent "$project_id" "$agent_id" || continue
    build_agent_content "$f" "my-tool" "$lang" "$deploy_dir" > "$out_dir/${agent_id}.md"
    local eff_mode; eff_mode=$(get_effective_agent_mode "$f" "$project_id")
    _DEPLOY_FILES_AGENT_KEYS+=("$agent_id")
    _DEPLOY_FILES_AGENT_VALS+=("$eff_mode")
    _DEPLOY_FILES_AGENT_FILES+=("$f")
    _DEPLOY_FILES_COUNT=$((_DEPLOY_FILES_COUNT + 1))
  done < <(find "$CANONICAL_AGENTS_DIR" -name "*.md" | sort)
}

# Phase 2: provider/model configuration (no-op if not applicable)
adapter_deploy_config() {
  log_info "  No provider/model configuration to apply (not supported by my-tool)"
}

# Compatibility wrapper
adapter_deploy() {
  adapter_deploy_files "${1:-}" "${2:-}" "${3:-}"
  adapter_deploy_config "${1:-}" "${2:-}" "${3:-}"
}

adapter_install() {
  log_info "Installing my-tool..."
  # ...
}

adapter_update() {
  log_info "Updating my-tool..."
  # ...
}

adapter_start() {
  local project_path="$1" prompt="${2:-}" project_id="${3:-}"
  cd "$project_path" || exit 1
  exec my-tool
}
```

---

## Existing Adapters

| Target | File | Node required | Specifics |
|--------|------|--------------|-----------|
| opencode | `opencode.adapter.sh` | Yes | Phase 1: `.opencode/agents/*.md` — Phase 2: `opencode.json` (provider, model, subagent modes, permissions, disabled agents) |
| claude-code | `claude-code.adapter.sh` | Yes | Phase 1: `.claude/agents/*.md` (prefixed subagents) — Phase 2: no-op (no provider config) |

### Mode Behavior by Target

| Agent mode | opencode | claude-code |
|-----------|----------|-------------|
| `primary` | Deployed normally, absent from the `"agent":` block | Deployed normally |
| `subagent` | Deployed normally, listed in `"agent": { "mode": "subagent" }` | Deployed with description prefixed `"Internal subagent — invoke only via a coordinator agent…"` |
