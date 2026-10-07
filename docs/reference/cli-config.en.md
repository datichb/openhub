> [Lire en francais](cli-config.fr.md)

# CLI Reference — Configuration

## Configuration

`oh config` reads and changes the hub configuration (`hub.toml`, in `~/.oh/` or `$OH_HOME`). Keys use the dotted notation (`section.key`). See the [configuration reference](config.en.md).

### oh config list

Shows the whole configuration (key, value).

**Alias:** `oh config ls`

```
oh config list [options]
```

| Flag | Type | Description |
|------|------|-------------|
| `--json` | bool | Output in JSON format |

**Example:**

```bash
oh config list
oh config ls --json
```

---

### oh config get

Shows the value of a key (any key listed by `oh config list`).

```
oh config get <key>
```

**Example:**

```bash
oh config get opencode.default_provider
oh config get cli.language
```

---

### oh config set

Changes a value. Only known keys are accepted (otherwise: "unknown config key" error); booleans accept `true`/`false`, `1`/`0`, `yes`/`no`.

```
oh config set <key> <value>
```

| Group | Settable keys |
|-------|---------------|
| CLI | `cli.language`, `cli.setup_done` |
| Provider | `opencode.default_provider`, `provider.bedrock.aws_profile`, `provider.bedrock.aws_region`, `provider.bedrock.auth_mode`, `provider.anthropic.auth_mode`, `provider.openrouter.auth_mode` |
| MCP | `mcp.figma.enabled`, `mcp.figma.token_key`, `mcp.gitlab.enabled`, `mcp.gitlab.token_key`, `mcp.gitlab.write_enabled`, `mcp.gitlab.url`, `mcp.jira.enabled`, `mcp.jira.token_key`, `mcp.jira.url`, `mcp.gslides.enabled`, `mcp.gslides.token_key` |
| Worktrees | `worktree.auto_cleanup`, `worktree.base_branch`, `worktree.branch_pattern` |
| Models | `models.default` |
| Web search | `websearch.enabled` |
| Tracker | `tracker.enabled`, `tracker.auto_sync`, `tracker.push_labels`, `tracker.auto_plan_assigned`, `tracker.max_auto_plan_per_member`, `tracker.tracker_url`, `tracker.tracker_token_key`, `tracker.write_enabled` |

The other sections (`[execution]`, `[limits]`, `[[teams]]`…) are set in the TUI (Settings) or with their own command (`oh budget set`, `oh team …`).

**Example:**

```bash
oh config set opencode.default_provider anthropic
oh config set mcp.gitlab.url https://gitlab.example.com
oh config set worktree.auto_cleanup true
```

---

### oh config unset

Removes a known key from `hub.toml`; the default value then applies, when there is one.

```
oh config unset <key>
```

**Example:**

```bash
oh config unset provider.bedrock.aws_profile
```

---

### oh config path

Prints the path of the configuration file.

```
oh config path
```

**Example:**

```bash
oh config path
# /Users/alice/.oh/hub.toml
```

---

### oh config language

Without argument, shows the interface language; with `fr` or `en`, changes it.

```
oh config language [fr|en]
```

**Example:**

```bash
oh config language        # Show the current language
oh config language fr     # Switch to French
oh config language en     # Switch to English
```

---

### oh config websearch

Enables or disables the `websearch` and `webfetch` permissions of the agents (web search through Exa AI). Applied at the next session launch (the session bundle is rebuilt). An argument is required.

```
oh config websearch enable|disable|status
```

**Example:**

```bash
oh config websearch status
oh config websearch enable
oh config websearch disable
```

---

## Provider & Model Configuration

### oh provider setup

Wizard configuring the LLM provider credentials (API key, AWS profile, bearer token), at hub or project level. Without argument, offers a picker.

```
oh provider setup [provider-name] [options]
```

| Flag | Short | Type | Description |
|------|-------|------|-------------|
| `--project` | `-p` | string | Configure the provider of a project |

Providers: `bedrock`, `anthropic`, `openrouter`, `github-copilot`.

```bash
oh provider setup
oh provider setup anthropic
oh provider setup bedrock -p my-app
```

---

### oh config model

Models per agent, per family or global, at hub level (`hub.toml [models]`) or project level (hub database). The resolved model is normalized to the project provider when the session bundle is built.

```
oh config model default <model> [-j <project>]
oh config model family <family> <model> [-j <project>]
oh config model agent <agent-id> <model> [-j <project>]
oh config model show [-j <project>] [--json]
oh config model unset default|family <family>|agent <agent-id> [-j <project>]
```

| Flag | Short | Type | Description |
|------|-------|------|-------------|
| `--project` | `-j` | string | Project (without: hub level). Note: the short form is `-j`, not `-p` |
| `--json` | | bool | `show` only: JSON output |

Families: `planning`, `developer`, `quality`, `auditor`, `design`, `documentation`.

Resolution order (highest first): workflow·agent > workflow (workflow `models`) > project·agent > project·family > project > hub·agent > hub·family > hub > team recommendations (agent, family, global, team-state `config.toml`) > agent frontmatter `model:`. Both workflow levels are set in the workflow, not with this command. See [Model resolution](model-resolution.en.md).

```bash
oh config model default anthropic/claude-sonnet-4-5
oh config model family quality anthropic/claude-haiku-4-5
oh config model agent reviewer anthropic/claude-opus-4-1 -j my-app
oh config model show -j my-app --json
oh config model unset family quality
```
