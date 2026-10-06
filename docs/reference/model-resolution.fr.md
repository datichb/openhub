> [Read in English](model-resolution.en.md)

# Résolution du modèle et du provider

---

## Vue d'ensemble

Le CLI Go (`oh`) résout le modèle IA pour chaque agent via une cascade à 7 niveaux. Opencode ne gère pas cette logique — c'est le CLI qui résout à la construction du paquet de session au lancement et écrit le modèle final dans les définitions d'agents du paquet (`agent.<id>.model` dans la config opencode produite).

Le provider est résolu séparément et utilisé pour normaliser le format du nom de modèle (préfixage provider).

---

## Résolution du provider

Le provider est résolu via une cascade à 3 niveaux (premier match gagne) :

| Priorité | Source | Exemple |
|----------|--------|---------|
| 1 | Flag CLI `--provider` | `oh run feature --provider anthropic` |
| 2 | Config hub | `hub.toml` → `[opencode] default_provider = "bedrock"` |
| 3 | Fallback hardcodé | `bedrock` |

---

## Cascade de résolution du modèle par agent

La résolution s'effectue pour chaque agent du paquet de session. Premier match gagne (priorité décroissante) :

| Priorité | Niveau | Source | Commande |
|----------|--------|--------|----------|
| 1 | Project agent | Override modèle pour un agent spécifique dans un projet | `oh config model agent <id> <model> --project <p>` |
| 2 | Project family | Override modèle pour une famille d'agents dans un projet | `oh config model family <name> <model> --project <p>` |
| 3 | Project global | Modèle global du projet | `oh config model default <model> --project <p>` |
| 4 | Hub agent | Override modèle pour un agent spécifique au hub | `oh config model agent <id> <model>` |
| 5 | Hub family | Override modèle pour une famille d'agents au hub | `oh config model family <name> <model>` |
| 6 | Hub global | Modèle global du hub | `oh config model default <model>` |
| 7 | **Team agent** | Recommandation de modèle pour un agent, depuis team-state | `config.toml` d'équipe `[models.agents]` |
| 8 | **Team family** | Recommandation de modèle pour une famille, depuis team-state | `config.toml` d'équipe `[models.families]` |
| 9 | **Team global** | Recommandation de modèle global de l'équipe | `config.toml` d'équipe `[models] default` |
| 10 | Frontmatter floor | Champ `model:` dans le `.md` de l'agent | Édition directe du fichier agent |

> **Les modèles d'équipe (7-9) sont toujours des recommandations** : les overrides du hub et du projet passent avant.

### Niveau workflow (paquets de session v5)

Pour les sessions lancées depuis un workflow (`oh/v1`), le bloc `models:` du workflow s'ajoute **au-dessus** de la cascade (décision O9) :

| Priorité | Niveau | Source |
|----------|--------|--------|
| 0a | Workflow agent | `models.agents.<id>` du workflow |
| 0b | Workflow global | `models.default` du workflow |

Puis les niveaux 1 à 10 ci-dessus. Le niveau workflow n'a pas de familles. Les identifiants complets (`amazon-bedrock/eu.anthropic.claude-sonnet-4-6`, suffixe `#variante`) sont acceptés : le préfixe régional Bedrock est retiré puis remis par l'adaptateur selon la région de la session, la variante est conservée.

### Familles

La famille d'un agent est déduite de son répertoire parent dans `agents/` :

| Répertoire | Famille | Agents |
|------------|---------|--------|
| `agents/planning/` | `planning` | orchestrator, orchestrator-dev, planner, pathfinder, onboarder |
| `agents/developer/` | `developer` | developer, developer-refactor, developer-migrator |
| `agents/quality/` | `quality` | reviewer, debugger |
| `agents/auditor/` | `auditor` | auditor, auditor-subagent |
| `agents/design/` | `design` | designer |
| `agents/documentation/` | `documentation` | documentarian |

---

## Stockage de la configuration

### Hub-level (`~/.oh/hub.toml`)

```toml
[opencode]
default_provider = "bedrock"

[models]
default = "claude-sonnet-4-5"

[models.families]
quality = "claude-opus-4"
planning = "claude-sonnet-4-6"

[models.agents]
reviewer = "claude-opus-4"
```

### Project-level (SQLite DB)

Les overrides projet sont stockés dans la base de données du hub (`~/.oh/oh.db`) :
- `projects.model` → modèle global du projet (niveau 3)
- `projects.model_overrides` → JSON sérialisé pour per-agent et per-family (niveaux 1 et 2)

```json
{
  "families": {"quality": "claude-opus-4"},
  "agents": {"reviewer": "claude-opus-4"}
}
```

---

## Commandes de configuration

```bash
# --- Hub-level ---
oh config model default claude-sonnet-4-5
oh config model family quality claude-opus-4
oh config model agent reviewer claude-opus-4

# --- Project-level ---
oh config model default claude-opus-4 --project my-app
oh config model family planning claude-sonnet-4-6 --project my-app
oh config model agent reviewer claude-opus-4 --project my-app

# --- Voir la configuration ---
oh config model show
oh config model show --project my-app

# --- Supprimer un override ---
oh config model unset default
oh config model unset family quality
oh config model unset agent reviewer --project my-app
```

---

## Préfixage provider (normalisation)

Opencode exige que les noms de modèles soient préfixés avec le provider au format `provider/model`. Le CLI applique ce préfixage **automatiquement** à la construction du paquet de session.

Le modèle résolu par la cascade (quel que soit son format d'entrée) est normalisé vers le provider du projet :

| Provider | Entrée (cascade) | Résultat dans opencode.json |
|----------|-------------------|----------------------------|
| `anthropic` | `claude-sonnet-4-5` | `anthropic/claude-sonnet-4-5` |
| `bedrock` | `claude-sonnet-4-5` | `amazon-bedrock/anthropic.claude-sonnet-4-5-20250929-v1:0` |
| `bedrock` | `anthropic/claude-opus-4` | `amazon-bedrock/anthropic.claude-opus-4-20250514-v1:0` |
| `github-copilot` | `claude-sonnet-4-5` | `github-copilot/claude-sonnet-4.5` |
| `openrouter` | `claude-opus-4` | `anthropic/claude-opus-4` |

La normalisation extrait le "short name" (ex: `claude-opus-4`) depuis n'importe quel format d'entrée, puis le re-formate pour le provider cible.

---

## Plancher de modèle agent (frontmatter)

Les agents peuvent déclarer un modèle minimum via le champ `model:` dans leur frontmatter :

```yaml
---
id: orchestrator
model: anthropic/claude-sonnet-4-6
---
```

Ce champ est le **niveau 7** de la cascade — il s'applique uniquement si aucun override n'est défini aux niveaux supérieurs.

### Agents avec plancher déclaré

| Agent | Plancher |
|-------|----------|
| `orchestrator` | `anthropic/claude-sonnet-4-6` |
| `orchestrator-dev` | `anthropic/claude-sonnet-4-6` |
| `planner` | `anthropic/claude-sonnet-4-6` |
| `pathfinder` | `anthropic/claude-sonnet-4-6` |
| `reviewer` | `anthropic/claude-opus-4` |

---

## Résultat dans la config de session

À chaque lancement, chaque agent du paquet de session obtient un bloc dans la config opencode produite (à inspecter avec `oh bundle show <workflow>`) ; les changements de modèle (`oh config model ...`) sont pris en compte au prochain lancement, sans redéploiement :

```json
{
  "agent": {
    "orchestrator": {
      "model": "amazon-bedrock/anthropic.claude-sonnet-4-6-20250715-v1:0",
      "permission": {
        "question": "allow",
        "bash": "deny",
        "task": { "*": "deny", "planner": "allow" }
      }
    },
    "developer": {
      "mode": "subagent",
      "model": "amazon-bedrock/anthropic.claude-sonnet-4-5-20250929-v1:0",
      "permission": {
        "bash": { "*": "deny", "git *": "allow", "npm *": "allow" },
        "read": "allow",
        "edit": "allow"
      }
    }
  }
}
```

### Ce qui est écrit dans le paquet

| Champ | Condition |
|-------|-----------|
| `agent.<id>.mode` | Écrit uniquement si `mode: subagent` (primary est le défaut) |
| `agent.<id>.model` | Écrit si un modèle est résolu (cascade non vide) |
| `agent.<id>.permission` | Écrit si des permissions sont déclarées dans le frontmatter |

---

## Construction du paquet de session

Les 5 anciennes phases du deploy (`oh deploy`, supprimé en v5) sont remplacées par la construction du paquet de session : à chaque lancement, `internal/bundle` construit `~/.oh/bundles/<hash>/` à partir du workflow — agents avec leurs skills Bucket A intégrées et leur modèle résolu via la cascade, skills à la demande, permissions, serveurs MCP et plugin. L'adaptateur en produit la config opencode ; rien n'est écrit dans le projet.
