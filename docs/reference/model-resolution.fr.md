# Résolution du modèle et du provider

---

## Vue d'ensemble

Le CLI Go (`oh`) résout le modèle IA pour chaque agent via une cascade à 7 niveaux. Opencode ne gère pas cette logique — c'est le CLI qui résout au moment du deploy et écrit le modèle final dans `opencode.json` sous `agent.<id>.model`.

Le provider est résolu séparément et utilisé pour normaliser le format du nom de modèle (préfixage provider).

---

## Résolution du provider

Le provider est résolu via une cascade à 3 niveaux (premier match gagne) :

| Priorité | Source | Exemple |
|----------|--------|---------|
| 1 | Flag CLI `--provider` | `oh deploy --provider anthropic` |
| 2 | Config hub | `hub.toml` → `[opencode] default_provider = "bedrock"` |
| 3 | Fallback hardcodé | `bedrock` |

---

## Cascade de résolution du modèle par agent

La résolution s'effectue pour chaque agent déployé. Premier match gagne (priorité décroissante) :

| Priorité | Niveau | Source | Commande |
|----------|--------|--------|----------|
| 1 | Project agent | Override modèle pour un agent spécifique dans un projet | `oh config model agent <id> <model> --project <p>` |
| 2 | Project family | Override modèle pour une famille d'agents dans un projet | `oh config model family <name> <model> --project <p>` |
| 3 | Project global | Modèle global du projet | `oh config model default <model> --project <p>` |
| 4 | Hub agent | Override modèle pour un agent spécifique au hub | `oh config model agent <id> <model>` |
| 5 | Hub family | Override modèle pour une famille d'agents au hub | `oh config model family <name> <model>` |
| 6 | Hub global | Modèle global du hub | `oh config model default <model>` |
| 7 | Frontmatter floor | Champ `model:` dans le `.md` de l'agent | Édition directe du fichier agent |

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

Opencode exige que les noms de modèles soient préfixés avec le provider au format `provider/model`. Le CLI applique ce préfixage **automatiquement** lors du déploiement.

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

## Résultat dans opencode.json

Après un `oh deploy`, chaque agent sélectionné obtient un bloc dans `opencode.json` :

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

### Ce qui est écrit par le deploy

| Champ | Condition |
|-------|-----------|
| `agent.<id>.mode` | Écrit uniquement si `mode: subagent` (primary est le défaut) |
| `agent.<id>.model` | Écrit si un modèle est résolu (cascade non vide) |
| `agent.<id>.permission` | Écrit si des permissions sont déclarées dans le frontmatter |

---

## Phases du deploy

Le deploy s'exécute en 5 phases transactionnelles (rollback automatique en cas d'erreur) :

| # | Phase | Rôle |
|---|-------|------|
| 1 | **Agents** | Copie les `.md` des agents sélectionnés dans `.opencode/agents/` |
| 2 | **Skills** | Copie les skills dans `.opencode/skills/` |
| 3 | **Configuration** | Écrit provider/model global + désactive les agents natifs |
| 4 | **Agent Configuration** | Parse le frontmatter, résout le modèle via cascade, écrit per-agent dans opencode.json |
| 5 | **MCP** | Injecte les serveurs MCP configurés |
