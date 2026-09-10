---
updated: 2026-09-10
confidence: confirmed
agents: [developer]
---

# Cascade de configuration — OpenHub

## Vue d'ensemble

OpenHub utilise 3 niveaux de configuration empilés. Chaque domaine implémente sa propre
logique de cascade avec des sémantiques spécifiques.

```
┌──────────────────────────────────────────────────────────────┐
│                    TEAM (repo git partagé)                    │
│  Source autoritaire · Peut ENFORCER · Peut RECOMMANDER       │
└─────────────────────────┬────────────────────────────────────┘
                          │
┌─────────────────────────▼────────────────────────────────────┐
│                   HUB (hub.toml local)                        │
│  Surcharges personnelles · Ne peut pas override un enforced   │
└─────────────────────────┬────────────────────────────────────┘
                          │
┌─────────────────────────▼────────────────────────────────────┐
│                  PROJET (SQLite local)                         │
│  Surcharges projet · Bloqué si team enforced                  │
└──────────────────────────────────────────────────────────────┘
```
— `CONFIRMÉ` · developer · 2026-09-10 · config/resolution.go

## Matrice de cascade par domaine

### Hub-only (pas de cascade)

| Champ | Type | Défaut |
|-------|------|--------|
| `cli.language` | string | `"en"` |
| `opencode.version` | string | — |
| `opencode.channel` | string | `"stable"` |
| `opencode.auto_update` | bool | false |
| `opencode.default_provider` | string | `""` |
| `deploy.disable_native_agents` | []string | `[]` |
| `worktree.auto_cleanup` | bool | false |
| `worktree.base_branch` | string | auto-détecté (main/master) |
| `worktree.branch_pattern` | string | auto-détecté ou `"feat/%s"` |

— `CONFIRMÉ` · developer · 2026-09-10 · config/config.go

### MCP Services (5 niveaux, enforceable)

**Cascade** : Team(enforced?) → Projet → Hub → Team(recommandé) → Défaut

| Champ | Hub | Team | Projet | Enforceable ? | Défaut |
|-------|-----|------|--------|---------------|--------|
| enabled | `bool` | `*bool` rec/enf | `*bool` override | **Oui** | false |
| url | `string` | `string` rec/enf | `string` override | **Oui** | `""` |
| token_key | `string` | — | `string` override | Non (personnel) | `""` |
| write_enabled | `bool` | — (WriteRecommended info) | `*bool` override | Non (personnel) | false |

— `CONFIRMÉ` · developer · 2026-09-10 · tracker/resolve_mcp.go

### Tracker local (2 niveaux, pointeur nil-inherit)

**Cascade** : Hub(`*bool`) → Team(`bool` source) → Défaut système

| Champ | Hub (*bool) | Team (bool) | Défaut | TUI hub |
|-------|-------------|-------------|--------|---------|
| enabled | `*bool` | `bool` | `true` | Tri-state si team, bool si solo |
| auto_sync | `*bool` | `bool` | `false` | Tri-state si team, bool si solo |
| push_labels | `*bool` | `bool` | `false` | Tri-state si team, bool si solo |
| auto_plan_assigned | `*bool` | `bool` | `false` | Tri-state si team, bool si solo |
| max_auto_plan | `*int` | `int` | `5` | Int (vide = défaut) |

Sémantique hub `*bool` : `nil` = utiliser la valeur équipe, `true/false` = surcharge explicite.

— `CONFIRMÉ` · developer · 2026-09-10 · tracker/resolve_config.go

### Tracker connexion (2 niveaux)

**Cascade** : Projet → Team

| Champ | Team | Projet | Notes |
|-------|------|--------|-------|
| tracker_url | `string` source | `string` override | |
| tracker_project | `string` source | `string` override | |
| tracker_token_key | `string` source | `string` override | |
| ticket_pattern | `string` source | `string` override | |

— `CONFIRMÉ` · developer · 2026-09-10 · tracker/resolve_config.go

### Models (3 niveaux, toujours recommandation)

**Cascade** : Projet.Agent → Projet.Family → Projet.Default → Hub.Agent → Hub.Family → Hub.Default → Team.Agent(rec) → Team.Family(rec) → Team.Default(rec) → Frontmatter

**Jamais enforceable** — le team recommande, le membre/projet décide.

— `CONFIRMÉ` · developer · 2026-09-10 · deploy/model_resolve.go

### Workflow (overlay additif, enforceable total)

**Cascade** : Base ← Hub overrides ← Team overrides ← Projet overrides

Quand `TeamConfig.Workflow.Enforced = true` :
- Les overrides projet sont **complètement ignorés**
- La vue workflow projet est en **lecture seule** (🔒)

— `CONFIRMÉ` · developer · 2026-09-10 · config/workflow_resolve.go

### Team-only (pas de cascade)

| Domaine | Champs principaux |
|---------|-------------------|
| Notification | type, webhook_url, channel, bot_name, destinations |
| Takeover | stale_days (défaut: 3) |
| Parallel | max_sessions (défaut: 3), port_range_start, auto_merge_beads |
| Claim | done_retention_days (défaut: 7) |

— `CONFIRMÉ` · developer · 2026-09-10 · teamstate/teamconfig.go

## Champs d'enforcement

| Champ | Effet résolution | Effet TUI | Activable par |
|-------|-----------------|-----------|---------------|
| `MCP.EnabledEnforced` | Ignore hub/projet | Lock icon + édition bloquée | Toggle dans team MCP view |
| `MCP.URLEnforced` | Impose URL team | Lock icon + édition bloquée | Toggle dans team MCP view |
| `Workflow.Enforced` | Ignore overrides projet | Vue lecture seule 🔒 | Toggle dans team config |
| `Tracker.TypeEnforced` | Bloque édition TUI | Grayed + toast | Toggle dans team detail view |
| `Tracker.PushLabelsEnforced` | Impose valeur team dans résolution + bloque TUI | Grayed + toast | Toggle dans team detail view |

— `CONFIRMÉ` · developer · 2026-09-10 · ADR-033

## Référence ADR

- [ADR-030 — Enforced/Recommended config](../architecture/adr/030-enforced-recommended-config.fr.md)
- [ADR-033 — Matrice de cascade et d'enforcement](../architecture/adr/033-config-cascade-enforcement.fr.md)
