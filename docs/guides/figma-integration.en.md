# Figma Integration - Getting Started Guide

> 🇫🇷 [Lire en français](figma-integration.fr.md)

## Overview

The Figma integration enriches planning workflows (Scout and Planner) with design context by automatically querying the Figma API to detect mockups, components, and UX/UI signals.

### Features

- **Automatic search** for Figma files by feature name
- **UX/UI signal detection**: multi-step flows, visual components, states
- **Estimation adjustment** based on number of detected components
- **Automatic enrichment** of Scout reports and Planner plans

---

## Quick Setup

### 1. Get your Figma tokens

**Personal Access Token:**
1. Go to https://www.figma.com/developers/api#authentication
2. "Personal access tokens" section
3. Create a token with scopes: `file:read`, `projects:read`

**Team ID:**
1. Open your Figma team
2. The ID is in the URL: `https://www.figma.com/files/team/123456/...`
3. Copy `123456`

### 2. Configure OpenCode

Create `~/.config/opencode/config.json`:

```json
{
  "$schema": "https://opencode.ai/config.json",
  "env": {
    "FIGMA_PERSONAL_ACCESS_TOKEN": "figd_xxx",
    "FIGMA_TEAM_ID": "123456"
  }
}
```

### 3. Organize your Figma files

Follow conventions in [`config/figma.conventions.md`](../../config/figma.conventions.md):

- **Naming**: `[Project] - [Feature] - [Type]`
- **Tags**: `#feature-xxx`, `#ready-dev`, `#wip`
- **Pages**: Cover, Flows, UI Design, States, Dev Notes

### 4. Deploy

```bash
oh deploy opencode MY-PROJECT
```

The Figma MCP Server will be deployed automatically with the agents.

---

## Usage

### With Scout

```bash
> Scout this feature: user dashboard
```

Scout will:
1. Explore the codebase (normal workflow)
2. Search in Figma: `search_figma_files("dashboard")`
3. Analyze found files: `detect_ui_signals(fileId)`
4. Include Figma data in its report

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
> Plan this feature: registration process
```

Planner will:
1. **Phase 1.2**: Explore codebase
2. **Phase 1.3**: Explore Figma (new)
   - Search for related mockups
   - Automatically detect UX/UI signals
3. **Phase 1.5**: Suggest designer delegation if signals detected
4. **Phase 5**: Pre-fill `--design` fields in tickets with Figma data

---

## Available MCP Tools

### `search_figma_files`

Search for Figma files by name.

```typescript
Input: { query: "dashboard" }
Output: [
  { id: "abc123", name: "MyApp - Dashboard - UI", url: "...", lastModified: "..." }
]
```

### `get_file_structure`

Get file structure (frames, components).

```typescript
Input: { fileId: "abc123" }
Output: {
  frames: [...],
  componentsCount: 7
}
```

### `detect_ui_signals`

Automatically detect UX/UI signals and estimate complexity.

```typescript
Input: { fileId: "abc123" }
Output: {
  hasUXSignal: true,
  hasUISignal: true,
  componentsCount: 7,
  complexity: "M",
  reasoning: [...],
  recommendations: [...]
}
```

---

## Architecture

```
openhub/
├── servers/figma-mcp/        ← TypeScript MCP Server
│   ├── src/
│   │   ├── index.ts          ← Entry point
│   │   ├── client.ts         ← Figma API wrapper
│   │   ├── config.ts         ← Token configuration
│   │   └── tools/            ← 3 MCP tools
│   └── dist/                 ← Compiled
├── skills/adapters/
│   ├── figma-scout-protocol.md
│   └── figma-planner-protocol.md
└── scripts/
    ├── build-mcp.sh          ← Build MCP
    ├── check-mcp.sh          ← Check build
    └── lib/mcp-deploy.sh     ← Deployment
```

---

## Testing

### Test 1: Simple Scout

```bash
# In a project with Figma mockups
> Scout this feature: settings page

# Check in the report:
- "🎨 Figma Context" section present
- Valid Figma URLs
- Components listed
- Adjusted estimation if > 3 components
```

### Test 2: Planner with signals

```bash
> Plan this feature: registration flow

# Check:
- Phase 1.3 executed (Figma exploration)
- Phase 1 summary contains Figma data
- Phase 1.5 suggested if signals detected
- Tickets created with pre-filled --design
```

---

## Troubleshooting

### No Figma files found

**Error:** `No Figma files found for search: "xxx"`

The onboarder runs a progressive search automatically before concluding that no files exist:
1. Root folder name or `package.json "name"`
2. Project ID (e.g. `t-sru`)
3. `Nom` field in `projects.md` (e.g. `SRU`)

If all 3 attempts fail, the onboarder asks you to provide the exact Figma file name or URL.

**If the search still returns nothing:**
- Verify Team ID is correct
- Rename Figma files according to conventions (`[Project] - [Feature] - [Type]`)
- Check token scopes: `file:read`, `projects:read`

### Token not recognized

**Error:** `FIGMA_PERSONAL_ACCESS_TOKEN environment variable is required`

**Solutions:**
- Verify `~/.config/opencode/config.json` exists
- Check JSON syntax (commas, quotes)
- Restart OpenCode after modification

### MCP build fails

```bash
cd servers/figma-mcp
rm -rf node_modules package-lock.json
npm install
npm run build
```

---

## Current Limitations (v1)

- ❌ No webhooks (real-time notifications)
- ❌ No Figma comment creation (read-only)
- ❌ No ticket → Figma links (Dev Resources)
- ❌ No design token extraction (Figma Variables)
- ❌ No cache (each call = API request)

These features can be added in v2+ based on needs.

---

## Future Enhancements

**v2: Bidirectional traceability**
- `create_figma_comment(fileId, message)`
- `link_ticket_to_figma(fileId, ticketId)`

**v3: Design tokens**
- `get_design_tokens(fileId)`
- `get_component_specs(componentId)`

**v4: Webhooks**
- Real-time notifications on Figma changes
- Automatic synchronization

---

## Resources

- **Figma API**: https://www.figma.com/developers/api
- **Figma Conventions**: [`config/figma.conventions.md`](../../config/figma.conventions.md)
- **MCP Infrastructure**: [`servers/README.md`](../../servers/README.md)
- **MCP Protocol**: https://modelcontextprotocol.io/

---

## Support

If you encounter issues:
1. Consult this troubleshooting guide
2. Check OpenCode logs
3. Test MCP manually: `cd servers/figma-mcp && npm start`
4. Verify Figma token configuration

**The Figma integration is ready to enrich your planning workflows!** 🎨
