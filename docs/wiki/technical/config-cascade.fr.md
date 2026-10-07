---
page: config-cascade
title: Cascade de configuration
confidence: CONFIRMED
agents: [developer]
sources:
  - cli/internal/config/resolution.go
  - cli/internal/config/config.go
  - cli/internal/mcpresolve/resolve.go
  - cli/internal/tracker/resolve_config.go
  - cli/internal/bricks/model_resolve.go
  - cli/internal/limits/limits.go
  - cli/internal/workflow/layers.go
  - docs/architecture/adr/033-config-cascade-enforcement.fr.md
last_updated: 2026-10-06
---

> [Read in English](config-cascade.en.md)

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

## Principe d'affichage : nil = "non configuré"

Toute valeur nil affiche **"non configuré"** — jamais "désactivé" ou "(vide)".
Au niveau projet, une parenthèse précise la source d'héritage :
`"non configuré (hérite du hub)"` ou `"non configuré (hérite de l'équipe)"`.
Au niveau hub (plus haut niveau), pas de parenthèse.

— `CONFIRMÉ` · developer · 2026-09-10 · tui/v2/views/config_field.go

## Matrice de cascade par domaine

### Hub-only (pas de cascade)

| Champ | Type | Défaut |
|-------|------|--------|
| `cli.language` | string | `"en"` |
| `opencode.default_provider` | string | `""` |
| `deploy.instruction_files` | []string | `[]` |
| `worktree.auto_cleanup` | bool | false |
| `worktree.base_branch` | string | auto-détecté (main/master) |
| `worktree.branch_pattern` | string | auto-détecté ou `"feat/%s"` |

Supprimées en v5 (ignorées si elles restent dans `hub.toml`) : `opencode.version`, `opencode.channel`, `opencode.auto_update`, `opencode.install_dir` (opencode V2 s'installe avec son propre outil) et `deploy.disable_native_agents` (le monde fermé désactive toujours les agents natifs d'opencode). Les sections `[session]` ([Sessions v5](../../guides/sessions-v5.fr.md#configuration)), `[execution]` ([conteneur](../../guides/container.fr.md)), `[remote]` ([runners distants](../../guides/remote-runners.fr.md)) et `[limits]` (`oh budget`) sont décrites dans les guides.

— `CONFIRMÉ` · developer · 2026-10-06 · config/config.go

### MCP Services (5 niveaux, enforceable)

**Cascade** : Team(enforced?) → Projet → Hub → Team(recommandé) → Défaut

| Champ | Hub | Team | Projet | Enforceable ? | Défaut |
|-------|-----|------|--------|---------------|--------|
| enabled | `bool` | `*bool` rec/enf | `*bool` override | **Oui** | false |
| url | `string` | `string` rec/enf | `string` override | **Oui** | `""` |
| token_key | `string` | — | `string` override | Non (personnel) | `""` |
| write_enabled | `bool` | — (WriteRecommended info) | `*bool` override | Non (personnel) | false |

En v5, les serveurs activés à l'issue de cette cascade sont placés dans le paquet de session au lancement (`oh mcp serve <nom> --token-key <clé>` : seul le nom de la clé, le jeton reste dans le trousseau) ; le champ `mcp:` du workflow ne peut que les filtrer.

— `CONFIRMÉ` · developer · 2026-10-06 · mcpresolve/resolve.go

### Tracker local (2 niveaux, pointeur nil-inherit)

**Cascade** : Hub(`*bool`) → Team(`bool` source) → Défaut système

| Champ | Hub (*bool) | Team (bool) | Défaut | TUI hub |
|-------|-------------|-------------|--------|---------|
| enabled | `*bool` | `bool` | `true` | Tri-state (non configuré / activé / désactivé) |
| auto_sync | `*bool` | `bool` | `false` | Tri-state (non configuré / activé / désactivé) |
| push_labels | `*bool` | `bool` | `false` | Tri-state (non configuré / activé / désactivé) |
| auto_plan_assigned | `*bool` | `bool` | `false` | Tri-state (non configuré / activé / désactivé) |
| max_auto_plan | `*int` | `int` | `5` | Int (vide = défaut) |

Sémantique hub `*bool` : `nil` = non configuré, `true/false` = surcharge explicite.

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

**Cascade** : Workflow.Agent → Workflow → Projet.Agent → Projet.Family → Projet.Default → Hub.Agent → Hub.Family → Hub.Default → Team.Agent(rec) → Team.Family(rec) → Team.Default(rec) → Frontmatter

**Jamais enforceable** — le team recommande, le membre/projet décide. Le modèle est résolu à la construction du paquet de session (plus de déploiement).

> **Limite v5 :** les niveaux Team existent dans `bricks.ResolveAgentModel`, mais le lancement (`cmd/v5_launch.go`, `modelOverridesFor`) ne renseigne que le hub et le projet : les recommandations `[models]` du team-state ne sont pas appliquées aux sessions v5.

— `CONFIRMÉ` · developer · 2026-10-06 · bricks/model_resolve.go

### Workflows (couches, sécurité qui ne fait que se durcir)

**Cascade** : Hub ← Équipe ← Projet (← options de session)

- Les workflows d'équipe et de projet sont dans le dépôt team-state (`workflows/published`, `workflows.lock`) ; `extends` hérite d'un workflow d'une couche inférieure.
- Une couche supérieure ne peut que **durcir** la sécurité (permissions, risque), jamais l'assouplir.
- `enforce:` verrouille des champs : les couches suivantes ne peuvent plus les modifier (🔒 dans l'éditeur de la TUI).

L'ancien overlay `TeamConfig.Workflow.Enforced` (vue Workflow, surcharges des checkpoints) n'existe plus. Voir [Workflows livrés](../../reference/workflows.fr.md).

— `CONFIRMÉ` · developer · 2026-10-06 · internal/workflow

### Restrictions des sessions (I6, désactivées par défaut)

**Cascade** : Hub → Équipe (recommandé / imposé) → Projet → Workflow (`limits:`)

Sessions actives max, budget par session et journalier (USD), plafond mémoire, liste de modèles. Commandes : `oh budget show|set|unset|raise`.

— `CONFIRMÉ` · developer · 2026-10-06 · internal/limits

### Environnement d'exécution (runtime)

**Ordre** : `--runtime` (ou la fiche de lancement) → config Exécution du projet → Réglages (`[execution] runtime`) → `runtime.default` du workflow

Un runtime que le workflow n'autorise pas (`runtime.allowed`) est ignoré. `[execution]` de `hub.toml` porte aussi `engine`, `keep_images`, `opencode_version` et `strict_isolation` (hub seulement) ; la config Exécution du projet porte le Dockerfile de dev, les build args, les volumes, le workflow et le runtime par défaut. Voir [Conteneur](../../guides/container.fr.md).

— `CONFIRMÉ` · developer · 2026-10-06 · config/config.go

### Team-only (pas de cascade)

| Domaine | Champs principaux |
|---------|-------------------|
| Notification | type, webhook_url, channel, bot_name, destinations |
| Takeover | stale_days (défaut: 3) |
| Parallel | max_sessions (défaut : 5), port_range_start, auto_merge_beads — réglages de l'ancien mode parallèle, sans effet sur les sessions v5 (limiter les sessions : `oh budget`) |
| Claim | done_retention_days (défaut: 7) |

— `CONFIRMÉ` · developer · 2026-09-10 · teamstate/teamconfig.go

## Champs d'enforcement

| Champ | Effet résolution | Effet TUI | Activable par |
|-------|-----------------|-----------|---------------|
| `MCP.EnabledEnforced` | Ignore hub/projet | Lock icon + édition bloquée | Toggle dans team MCP view |
| `MCP.URLEnforced` | Impose URL team | Lock icon + édition bloquée | Toggle dans team MCP view |
| `enforce:` (YAML du workflow) | Les couches suivantes ne peuvent plus modifier le champ | 🔒 dans l'éditeur de workflow | Déclaré dans le workflow parent |
| `Tracker.TypeEnforced` | Bloque édition TUI | Grayed + toast | Toggle dans team detail view |
| `Tracker.PushLabelsEnforced` | Impose valeur team dans résolution + bloque TUI | Grayed + toast | Toggle dans team detail view |

— `CONFIRMÉ` · developer · 2026-09-10 · ADR-033

## Référence ADR

- [ADR-030 — Enforced/Recommended config](../../architecture/adr/030-enforced-recommended-config.fr.md)
- [ADR-033 — Matrice de cascade et d'enforcement](../../architecture/adr/033-config-cascade-enforcement.fr.md)
