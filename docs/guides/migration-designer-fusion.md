# Migration : fusion ux-designer + ui-designer → designer

## Résumé du changement

Les agents `ux-designer` et `ui-designer` ont été fusionnés en un agent unique `designer`
opérant en 4 modes : `recon`, `ux`, `ui`, `ux+ui`. L'accès MCP Figma a été centralisé
sur cet agent unique. Les agents `planner`, `pathfinder` et `onboarder` ne disposent
plus d'accès Figma direct — ils délèguent au `designer` via `task` (mode `recon`).

Voir [ADR-020](../architecture/adr/020-designer-fusion.fr.md) pour la décision complète.

---

## Breaking changes

### Agents supprimés

| Agent supprimé | Remplacement |
|---------------|--------------|
| `ux-designer` | `designer` + `Mode: ux` dans le prompt |
| `ui-designer` | `designer` + `Mode: ui` dans le prompt |

### Skills supprimés (9 fichiers)

| Fichier supprimé | Raison |
|-----------------|--------|
| `adapters/figma-ux-designer-protocol.md` | Intégré dans `designer/figma-deep-protocol.md` |
| `adapters/figma-ui-designer-protocol.md` | Intégré dans `designer/figma-deep-protocol.md` |
| `adapters/figma-planner-protocol.md` | Délégation via `task: designer` mode `recon` |
| `adapters/figma-pathfinder-protocol.md` | Délégation via `task: designer` mode `recon` |
| `adapters/figma-onboarder-protocol.md` | Délégation via `task: designer` mode `recon` |
| `designer/ux-subagent.md` | Fusionné dans `designer/designer-execution-modes.md` |
| `designer/ui-subagent.md` | Fusionné dans `designer/designer-execution-modes.md` |
| `designer/ux-standalone.md` | Fusionné dans `designer/designer-execution-modes.md` |
| `designer/ui-standalone.md` | Fusionné dans `designer/designer-execution-modes.md` |

### Skills ajoutés (8 fichiers)

| Fichier ajouté | Contenu |
|---------------|---------|
| `designer/designer-protocol.md` | Protocole unifié — détection de mode, routing, gate Figma |
| `designer/figma-recon-protocol.md` | Exploration Figma légère (anciens adapters planner/pathfinder/onboarder) |
| `designer/figma-deep-protocol.md` | Analyse Figma approfondie (anciens adapters ux/ui-designer) |
| `designer/designer-execution-modes.md` | Parcours standalone + sous-agent fusionnés |
| `planning/planner-design-templates.md` | Templates de délégation `task: designer` |
| `planning/planner-beads-templates.md` | Templates de tickets Beads par type de feature |
| `planning/onboarder-profiles.md` | 7 profils d'exploration adaptatifs |
| `quality/debugger-forensic.md` | Protocole forensic détaillé |
| `quality/debugger-report-templates.md` | Templates de rapport de diagnostic |

---

## Migration des invocations

| Avant | Après |
|-------|-------|
| `task: ux-designer` | `task: designer` + `Mode: ux` dans le prompt |
| `task: ui-designer` + prompt UI | `task: designer` + `Mode: ui` dans le prompt |
| MCP Figma direct dans planner/pathfinder/onboarder | `task: designer` + `Mode: recon` |
| `task: ux-designer` + `task: ui-designer` (deux invocations) | `task: designer` + `Mode: ux+ui` (une seule invocation) |

### Exemple — avant (planner avec Figma direct)

```
[SKILL:adapters/figma-planner-protocol]
→ planner appelle directement figma_search_files("login flow")
```

### Exemple — après (planner délègue au designer)

```
task({
  subagent_type: "designer",
  prompt: "Mode: recon\nRecherche les maquettes Figma pour le flow login.\n[CONTEXTE] Invoqué depuis le planner.",
  description: "Recon Figma login flow"
})
```

---

## Migration des permissions (opencode.json)

### Avant

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

### Après

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

## Note sur la compatibilité

**Aucun alias automatique n'est fourni.** Les références à `ux-designer` et `ui-designer`
dans `opencode.json`, les prompts d'invocation, les scripts de déploiement et les
workflows documentés doivent être mis à jour explicitement.

Les projets qui n'utilisaient pas les agents design ou le MCP Figma ne sont pas impactés.
