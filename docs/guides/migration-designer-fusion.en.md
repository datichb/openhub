> [Lire en francais](migration-designer-fusion.fr.md)

# Migration: ux-designer + ui-designer fusion → designer

> **Historical (v5).** This migration concerns versions before v5. Since v5, oh no longer writes an `opencode.json` into projects (`oh deploy` removed): agents and their permissions come from the session bundle built at launch from the workflow (see [Built-in workflows](../reference/workflows.en.md)), which already uses the `designer` agent. Leftovers of former deployments are removed with `oh migrate deploy-cleanup` (see [Migrating to oh v5](migration-v5.en.md)). The rest of this page is kept for history.

## Summary of the change

The `ux-designer` and `ui-designer` agents have been merged into a single `designer` agent
operating in 4 modes: `recon`, `ux`, `ui`, `ux+ui`. MCP Figma access has been centralized
on this single agent. The `planner`, `pathfinder`, and `onboarder` agents no longer have
direct Figma access — they delegate to `designer` via `task` (mode `recon`).

See [ADR-020](../architecture/adr/020-designer-fusion.fr.md) for the complete decision.

---

## Breaking changes

### Removed agents

| Removed agent | Replacement |
|---------------|-------------|
| `ux-designer` | `designer` + `Mode: ux` in the prompt |
| `ui-designer` | `designer` + `Mode: ui` in the prompt |

### Removed skills (9 files)

| Removed file | Reason |
|-------------|--------|
| `adapters/figma-ux-designer-protocol.md` | Integrated into `designer/figma-deep-protocol.md` |
| `adapters/figma-ui-designer-protocol.md` | Integrated into `designer/figma-deep-protocol.md` |
| `adapters/figma-planner-protocol.md` | Delegation via `task: designer` mode `recon` |
| `adapters/figma-pathfinder-protocol.md` | Delegation via `task: designer` mode `recon` |
| `adapters/figma-onboarder-protocol.md` | Delegation via `task: designer` mode `recon` |
| `designer/ux-subagent.md` | Merged into `designer/designer-execution-modes.md` |
| `designer/ui-subagent.md` | Merged into `designer/designer-execution-modes.md` |
| `designer/ux-standalone.md` | Merged into `designer/designer-execution-modes.md` |
| `designer/ui-standalone.md` | Merged into `designer/designer-execution-modes.md` |

### Added skills (8 files)

| Added file | Content |
|-----------|---------|
| `designer/designer-protocol.md` | Unified protocol — mode detection, routing, Figma gate |
| `designer/figma-recon-protocol.md` | Lightweight Figma exploration (former planner/pathfinder/onboarder adapters) |
| `designer/figma-deep-protocol.md` | In-depth Figma analysis (former ux/ui-designer adapters) |
| `designer/designer-execution-modes.md` | Merged standalone + sub-agent workflows |
| `planning/planner-design-templates.md` | `task: designer` delegation templates |
| `planning/planner-beads-templates.md` | Beads ticket templates by feature type |
| `planning/onboarder-profiles.md` | 7 adaptive exploration profiles |
| `quality/debugger-forensic.md` | Detailed forensic protocol |
| `quality/debugger-report-templates.md` | Diagnostic report templates |

---

## Invocation migration

| Before | After |
|--------|-------|
| `task: ux-designer` | `task: designer` + `Mode: ux` in the prompt |
| `task: ui-designer` + UI prompt | `task: designer` + `Mode: ui` in the prompt |
| Direct MCP Figma in planner/pathfinder/onboarder | `task: designer` + `Mode: recon` |
| `task: ux-designer` + `task: ui-designer` (two invocations) | `task: designer` + `Mode: ux+ui` (single invocation) |

### Example — before (planner with direct Figma)

```
[SKILL:adapters/figma-planner-protocol]
→ planner directly calls figma_search_files("login flow")
```

### Example — after (planner delegates to designer)

```
task({
  subagent_type: "designer",
  prompt: "Mode: recon\nSearch for Figma mockups for the login flow.\n[CONTEXTE] Invoked from the planner.",
  description: "Recon Figma login flow"
})
```

---

## Permission migration (opencode.json)

### Before

```json
{
  "agent": {
    "orchestrator": {
      "permission": {
        "task": {
          "*": "deny",
          "planner": "allow",
          "onboarder": "allow",
          "ux-designer": "allow",
          "ui-designer": "allow",
          "auditor-subagent": "allow",
          "orchestrator-dev": "allow",
          "debugger": "allow"
        }
      }
    },
    "planner": {
      "mcpServers": ["figma", "gitlab"]
    },
    "pathfinder": {
      "mcpServers": ["figma", "gitlab"]
    },
    "onboarder": {
      "mcpServers": ["figma", "gitlab"]
    }
  }
}
```

### After

```json
{
  "agent": {
    "orchestrator": {
      "permission": {
        "task": {
          "*": "deny",
          "planner": "allow",
          "onboarder": "allow",
          "designer": "allow",
          "auditor-subagent": "allow",
          "orchestrator-dev": "allow",
          "debugger": "allow"
        }
      }
    },
    "designer": {
      "mcpServers": ["figma"]
    },
    "planner": {
      "mcpServers": ["gitlab"],
      "permission": {
        "task": {
          "designer": "allow"
        }
      }
    },
    "pathfinder": {
      "mcpServers": ["gitlab"],
      "permission": {
        "task": {
          "designer": "allow"
        }
      }
    },
    "onboarder": {
      "mcpServers": ["gitlab"],
      "permission": {
        "task": {
          "designer": "allow"
        }
      }
    }
  }
}
```

---

## Compatibility note

**No automatic alias is provided.** References to `ux-designer` and `ui-designer`
in `opencode.json`, invocation prompts, deployment scripts, and documented
workflows must be updated explicitly.

Projects that did not use the design agents or the MCP Figma are not impacted.
