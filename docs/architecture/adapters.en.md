# Adapters Architecture

An **adapter** translates canonical hub agents to the native format
of a target AI tool (e.g., opencode).

---

## Mandatory Contract

Every adapter (`scripts/adapters/<target>.adapter.sh`) must export **9 functions**.
Loading is performed by `load_adapter()` in `scripts/lib/adapter-manager.sh`,
which verifies via `declare -F` that the 9 functions exist after the `source`.

| Function | Role | Signature |
|----------|------|-----------|
| `adapter_validate` | Checks that the target tool is installed and accessible | `adapter_validate()` — returns 0/1 |
| `adapter_needs_node` | Indicates whether Node.js is required for the tool | `adapter_needs_node()` — `return 0` (yes) or `return 1` (no) |
| `adapter_deploy_files` | **Phase 1** — Copies canonical agents to the target project | `adapter_deploy_files deploy_dir project_id [provider_override]` |
| `adapter_deploy_skills` | **Phase 2** — Deploys native skills to `.opencode/skills/` | `adapter_deploy_skills deploy_dir project_id` |
| `adapter_deploy_config` | **Phase 3** — Applies provider/model configuration (e.g. `opencode.json`) | `adapter_deploy_config deploy_dir project_id [provider_override]` |
| `adapter_deploy` | Compatibility wrapper — chains Phase 1 + Phase 2 + Phase 3 | `adapter_deploy deploy_dir project_id [provider_override]` |
| `adapter_install` | Installs the target tool (called by `oh install`) | `adapter_install()` |
| `adapter_update` | Updates the target tool (called by `oh update`) | `adapter_update()` |
| `adapter_start` | Launches the tool in the project (called by `oh start`) | `adapter_start project_path prompt project_id` |

### Phase separation

`oh deploy` runs all three phases sequentially with a distinct visual section for each:

```
▶  Phase 1 — Copy agents
◆  12 agent(s) deployed

▶  Phase 2 — Deploy skills
◆  8 skills deployed

▶  Phase 3 — Provider / model configuration
◆  opencode.json  (model: amazon-bedrock/..., provider: bedrock)
```

`oh start --provider <provider>` only runs **Phase 3** when agents are already in place —
Phases 1 and 2 are unnecessary in that case.

### Parameter Details

#### `adapter_deploy_files deploy_dir project_id [provider_override]`

- `deploy_dir`: path of the project directory to deploy into (e.g. `/home/user/my-project`)
- `project_id`: project identifier in `projects.md` (e.g. `MY-PROJECT`). Used to
  read the language (`get_project_language`) and agent filters (`should_deploy_agent`).
- `provider_override`: ignored in Phase 1 — present for signature consistency.

Responsibilities:
1. Create the output directory structure (e.g. `.opencode/agents/`)
2. Load agent metadata via `_load_agent_metadata` (scan without writing)
3. For each retained agent: call `build_agent_content` and write the `.md` file
4. Populate global variables `_DEPLOY_FILES_AGENT_KEYS/VALS/FILES/COUNT`
5. Expose `_DEPLOY_PRECOMPUTED_STACKS` for reuse by Phase 2 (avoids double computation)

#### `adapter_deploy_skills deploy_dir project_id`

- `deploy_dir`: path of the project directory
- `project_id`: project identifier (for stack detection if Phase 1 was not run)

Responsibilities:
1. Reuse `_DEPLOY_PRECOMPUTED_STACKS` exposed by Phase 1 — or recompute if called standalone
2. Collect all unique native skills: `native_skills` from agent frontmatters + resolved stack skills
3. Deploy each skill to `.opencode/skills/<name>/SKILL.md`
4. Expose `_DEPLOY_NATIVE_SKILLS_COUNT` and `_DEPLOY_NATIVE_SKILLS_SKIPPED`

**Autonomy:** this function can be called standalone without having run `adapter_deploy_files`
first — it loads metadata and recomputes stacks as needed.

#### `adapter_deploy_config deploy_dir project_id [provider_override]`

- `deploy_dir`: path of the project directory
- `project_id`: project identifier (to resolve provider and API key)
- `provider_override`: provider override (e.g. `bedrock`, `anthropic`)

Responsibilities:
1. Load agent metadata if `_DEPLOY_FILES_AGENT_KEYS` is empty (direct call without Phase 1)
2. Resolve the effective model and provider
3. Build and write the configuration file (e.g. `opencode.json`)
4. Expose `_DEPLOY_CONFIG_CLAMPS`: number of agents whose model floor was applied
5. For adapters without configuration: explicit no-op

**Autonomy:** this function can be called standalone without having run `adapter_deploy_files`
first — it loads the necessary metadata itself.

#### `adapter_deploy deploy_dir project_id [provider_override]`

Compatibility wrapper that chains `adapter_deploy_files`, `adapter_deploy_skills`, then `adapter_deploy_config`.
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
| `get_agent_id file` | Returns the `id` from the frontmatter |
| `get_agent_mode file` | Returns the `mode` from the frontmatter (`primary` by default) |
| `get_effective_agent_mode file project_id` | Effective mode: project override > frontmatter > `primary` |
| `build_agent_content file [target] [lang]` | Assembles complete content (header + skills + body) |
| `get_project_language project_id` | Returns the project language (or empty string) |
| `get_project_api_provider project_id` | Returns the API provider (anthropic, litellm, etc.) |
| `get_project_api_key project_id` | Returns the API key |
| `get_project_api_base_url project_id` | Returns the base URL (or empty string) |

### Global variables populated by `adapter_deploy_files` / `_load_agent_metadata`

These variables are available to `adapter_deploy_skills` and `adapter_deploy_config` after Phase 1:

| Variable | Content |
|----------|---------|
| `_DEPLOY_FILES_AGENT_KEYS` | Array of retained `agent_id` values |
| `_DEPLOY_FILES_AGENT_VALS` | Array of effective modes (`primary`, `subagent`, …) |
| `_DEPLOY_FILES_AGENT_FILES` | Array of canonical source file paths |
| `_DEPLOY_FILES_COUNT` | Number of retained agents |
| `_DEPLOY_PRECOMPUTED_STACKS` | Pre-computed stack skills (reused by Phase 2) |

Variables exposed by `adapter_deploy_skills` after Phase 2:

| Variable | Content |
|----------|---------|
| `_DEPLOY_NATIVE_SKILLS_COUNT` | Number of native skills deployed |
| `_DEPLOY_NATIVE_SKILLS_SKIPPED` | Number of skills skipped (source not found) |

Variables exposed by `adapter_deploy_config` after Phase 3:

| Variable | Content |
|----------|---------|
| `_DEPLOY_CONFIG_MODEL` | Resolved global model |
| `_DEPLOY_CONFIG_PROVIDER` | Effective provider |
| `_DEPLOY_CONFIG_SIZE` | Size of the generated `opencode.json` |
| `_DEPLOY_CONFIG_TOTAL` | Total number of configured agents |
| `_DEPLOY_CONFIG_SUBAGENTS` | Number of agents in subagent mode |
| `_DEPLOY_CONFIG_DISABLED` | Number of disabled native agents |
| `_DEPLOY_CONFIG_PERMS` | Number of agents with restricted permissions |
| `_DEPLOY_CONFIG_CLAMPS` | Number of agents whose model floor was applied |
| `_DEPLOY_CONFIG_SKIP` | `true` if `opencode.json` was already up-to-date (no rewrite) |

---

## Creating a New Adapter

1. Create `scripts/adapters/<target>.adapter.sh` with the **9 contract functions**
2. The file will be loaded automatically by `load_adapter` — no modification of
   `adapter-manager.sh` is needed
3. Test: `oh deploy <target>` then verify the generated files

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
  _DEPLOY_PRECOMPUTED_STACKS=""

  while IFS= read -r f; do
    [ -f "$f" ] || continue
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

# Phase 2: native skills deployment (no-op if not applicable)
adapter_deploy_skills() {
  _DEPLOY_NATIVE_SKILLS_COUNT=0
  _DEPLOY_NATIVE_SKILLS_SKIPPED=0
  # Implement deploy_native_skills if the tool supports on-demand skill loading
}

# Phase 3: provider/model configuration (no-op if not applicable)
adapter_deploy_config() {
  log_info "  No provider/model configuration to apply (not supported by my-tool)"
}

# Compatibility wrapper
adapter_deploy() {
  adapter_deploy_files "${1:-}" "${2:-}" "${3:-}"
  adapter_deploy_skills "${1:-}" "${2:-}"
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
| opencode | `opencode.adapter.sh` | Yes | Phase 1: `.opencode/agents/*.md` — Phase 2: `.opencode/skills/<name>/SKILL.md` — Phase 3: `opencode.json` (provider, model, subagent modes, permissions, disabled agents) |

### Mode Behavior by Target

| Agent mode | opencode |
|-----------|----------|
| `primary` | Deployed normally, absent from the `"agent":` block |
| `subagent` | Deployed normally, listed in `"agent": { "mode": "subagent" }` |
