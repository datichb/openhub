> 🇫🇷 [Lire en français](figma-integration.fr.md)

# Figma Integration - Getting Started Guide

## Overview

The Figma integration gives the `designer` agent (the only agent with Figma MCP access) **read-only** access to your Figma files: file structure, nodes, styles. The other agents (pathfinder, planner, onboarder…) delegate Figma reconnaissance to it (`Mode: recon`) to link a feature to its mockups, spot components and UX/UI signals, and adjust their estimates.

The Figma MCP server is **built into the `oh` binary** (`oh mcp serve figma`): nothing to install or compile.

---

## Setup

### 1. Get a Figma token

**Personal Access Token:**
1. Go to https://www.figma.com/developers/api#authentication
2. Section "Personal access tokens"
3. Create a token with the scopes: `current_user:read`, `file_content:read`, `file_metadata:read`, `projects:read`, `library_assets:read`

### 2. Configure via `oh mcp setup`

```bash
oh mcp setup                 # pick Figma, then enter the token
oh mcp setup -p my-project   # token specific to a project
```

The wizard:
1. Asks for your Figma **Personal Access Token** (masked input)
2. Stores it in the OS keychain (key `figma-token`, or `figma-token-<project-id>` for a project)
3. Enables the service: `[mcp.figma]` block of `hub.toml`, or project override

Without a token, the server reads the `FIGMA_TOKEN` environment variable.

Check and manage the service:

```bash
oh mcp status                       # MCP services status (with -p: project overrides)
oh mcp enable figma                 # enable (hub, or -p for a project)
oh mcp disable figma -p my-project  # disable for a project
oh mcp reset figma -p my-project    # back to the hub configuration
oh doctor                           # "API keys" line
```

### 3. Organize your Figma files

Recommended conventions:

- **Naming**: `[Project] - [Feature] - [Type]`
- **Tags**: `#feature-xxx`, `#ready-dev`, `#wip`
- **Pages**: Cover, Flows, UI Design, States, Dev Notes

### 4. Launch a session

```bash
oh run <workflow> -p MY-PROJECT
```

No deploy step (`oh deploy` removed in v5): once enabled, the Figma server is declared in the session bundle at launch. A workflow without an `mcp:` field receives the project's MCP servers; with `mcp:`, only the listed ones (see [Workflows: CLI](../reference/cli-workflows.en.md)). To check: `oh bundle show <workflow>`.

---

## Usage

### With Pathfinder

```bash
> Pathfinder this feature: user dashboard
```

The Pathfinder will:
1. Explore the codebase (normal workflow)
2. Delegate to the `designer` (`Mode: recon`) the search and analysis of related mockups (components, UX/UI signals)
3. Include the Figma data in its report

**Enriched report:**
```markdown
## 🎨 Figma Context Detected
- Files: Dashboard - UI (Figma URL)
- Components: 7 detected
- Signals: UX ⚠️ | UI ⚠️
- Adjusted complexity: S → M
```

### With Planner

```bash
> Plan this feature: signup process
```

The Planner will:
1. **Phase 1.2**: Explore the codebase
2. **Phase 1.3**: Explore Figma (delegated to the `designer`, `Mode: recon`)
   - Search for related mockups
   - Detect UX/UI signals
3. **Phase 1.5**: Offer delegation to the designer if signals are detected
4. **Phase 5**: Pre-fill the tickets' `--design` with Figma data

---

## Available MCP Tools

| Tool | Purpose | Input |
|------|---------|-------|
| `figma_get_file` | Gets a Figma file (structure, frames, components) | `file_key` |
| `figma_get_node` | Gets a specific node of a file | `file_key`, `node_id` |
| `figma_get_styles` | Gets the styles of a file | `file_key` |

The file key (`file_key`) is in the Figma URL: `https://www.figma.com/file/<file_key>/...`.

---

## Architecture

The implementation lives in `cli/internal/mcp/figma/` (stdio JSON-RPC MCP server, Figma API client). The agents' protocols are in `skills/designer/figma-recon-protocol.md` and `skills/designer/figma-deep-protocol.md`.

At runtime, opencode starts the server with the command declared in the session bundle:

```bash
oh mcp serve figma --token-key figma-token
```

The token is read from the keychain by `oh mcp serve`: it is never written to the bundle or the project.

---

## Testing

### Test 1: Simple Pathfinder

```bash
# In a project with Figma mockups
> Pathfinder this feature: settings page

# Check in the report:
- "🎨 Figma Context" section present
- Valid Figma URLs
- Components listed
- Estimate adjusted if > 3 components
```

### Test 2: Planner with signals

```bash
> Plan this feature: signup flow

# Check:
- Phase 1.3 executed (Figma exploration)
- Phase 1 recap contains Figma data
- Phase 1.5 offered if signals detected
- Tickets created with pre-filled --design
```

---

## Troubleshooting

### No Figma files found

The onboarder runs a progressive search before concluding there are no results:
1. Root folder name or `package.json "name"`
2. Project ID (e.g. `t-sru`)
3. `Name` field in `projects.md` (e.g. `SRU`)

If the 3 attempts fail, the onboarder asks you for the name or URL of the Figma file.

**If the search still fails:**
- Give the Figma file URL to the agent directly
- Rename Figma files following the conventions (`[Project] - [Feature] - [Type]`)
- Check the token scopes: `current_user:read`, `file_content:read`, `file_metadata:read`, `projects:read`, `library_assets:read`

### Token not recognized

**Error:** `FIGMA_TOKEN environment variable not set`

**Solutions:**
- Run `oh mcp setup` (Figma) again to store the token in the keychain
- Check that the service is enabled: `oh mcp status`
- Launch the session again: the bundle is rebuilt at launch

### MCP server issues

The Figma MCP server is built into the `oh` binary, so there is no separate build step. If the server does not start:

```bash
oh mcp status                 # check the service configuration
oh bundle show <workflow>     # check that figma is in the bundle
oh mcp serve figma            # test the server by hand (stdio)
```

### Figma API timeout

**Symptom:** the agent mentions `⚠️ Figma unavailable (timeout)` in its report.

**Possible causes:** slow connection, large Figma file, overloaded Figma API. Requests time out after 30 s.

---

## Current Limitations

- ❌ No webhooks (real-time notifications)
- ❌ No Figma comment creation (read-only)
- ❌ No ticket → Figma links (Dev Resources)
- ❌ No design tokens extraction (Figma Variables)
- ❌ No cache (each call = API request)

---

## Future Enhancements

**Bidirectional traceability**
- `create_figma_comment(fileId, message)`
- `link_ticket_to_figma(fileId, ticketId)`

**Design tokens**
- `get_design_tokens(fileId)`
- `get_component_specs(componentId)`

**Webhooks**
- Real-time notifications on Figma changes
- Automatic synchronization

---

## Resources

- **Figma API**: https://www.figma.com/developers/api
- **MCP Protocol**: https://modelcontextprotocol.io/
- **MCP services reference**: [Services](../reference/services.en.md)

---

## Support

If you run into a problem:
1. Check this troubleshooting guide
2. Run `oh doctor`
3. Test the MCP manually: `oh mcp serve figma`
4. Check the configuration: `oh mcp status`
