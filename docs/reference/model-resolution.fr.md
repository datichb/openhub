> [Read in English](model-resolution.en.md)

# Résolution du modèle et du provider

---

## Vue d'ensemble

Le CLI Go (`oh`) résout le modèle IA de chaque agent du paquet de session par une **cascade à 12 niveaux** (2 niveaux workflow, 3 niveaux projet, 3 niveaux hub, 3 niveaux de recommandation d'équipe, puis le frontmatter de l'agent). Opencode ne gère pas cette logique : oh résout à la construction du paquet de session, au lancement, et écrit le modèle final dans les définitions d'agents du paquet (`agent.<id>.model` dans la config opencode produite).

Le provider est résolu séparément et sert à normaliser le nom du modèle (préfixe provider). Les restrictions I6 peuvent en plus limiter les modèles utilisables (voir [liste blanche](#liste-blanche-des-modèles-limitsmodels)).

---

## Résolution du provider

Le provider est résolu par une cascade à 4 niveaux (premier trouvé gagne) :

| Priorité | Source | Exemple |
|----------|--------|---------|
| 1 | Option CLI `--provider` / `-P` | `oh run feature --provider anthropic` |
| 2 | Provider du projet | base `oh.db` (`oh project configure`) |
| 3 | Config hub | `hub.toml` → `[opencode] default_provider = "bedrock"` |
| 4 | Repli en dur | `bedrock` |

---

## Cascade de résolution du modèle par agent

La résolution se fait pour chaque agent du paquet de session. Premier trouvé gagne (priorité décroissante) :

| Priorité | Niveau | Source | Commande |
|----------|--------|--------|----------|
| 1 | Workflow · agent | `models.agents.<id>` du workflow | YAML du workflow ([schéma](workflow-schema.fr.md#ressources)) |
| 2 | Workflow | `models.default` du workflow | YAML du workflow |
| 3 | Projet · agent | Modèle d'un agent dans un projet | `oh config model agent <id> <model> --project <p>` |
| 4 | Projet · famille | Modèle d'une famille d'agents dans un projet | `oh config model family <name> <model> --project <p>` |
| 5 | Projet | Modèle global du projet | `oh config model default <model> --project <p>` |
| 6 | Hub · agent | Modèle d'un agent au niveau hub | `oh config model agent <id> <model>` |
| 7 | Hub · famille | Modèle d'une famille d'agents au niveau hub | `oh config model family <name> <model>` |
| 8 | Hub | Modèle global du hub | `oh config model default <model>` |
| 9 | Équipe · agent | Recommandation d'équipe pour un agent | `[models.agents]` du `config.toml` du team-state (TUI : Équipe › Modèles) |
| 10 | Équipe · famille | Recommandation d'équipe pour une famille | `[models.families]` |
| 11 | Équipe | Modèle recommandé par l'équipe | `[models] default` |
| 12 | Frontmatter | Champ `model:` dans le `.md` de l'agent | Édition du fichier agent |

### Niveau workflow (décision O9)

- Le bloc `models:` d'un workflow `oh/v1` passe **avant** toute configuration du projet et du hub. Il n'a pas de familles.
- Les identifiants complets (`amazon-bedrock/eu.anthropic.claude-sonnet-4-6`, suffixe `#variante`) sont acceptés : le préfixe régional Bedrock est retiré puis remis par l'adaptateur selon la région de la session, la variante est conservée.
- Un patch (`extends`) remplace `models.default` et fusionne `models.agents` par agent.

### Modèles d'équipe

Le `config.toml` du team-state peut contenir des recommandations `[models]` (`default`, `families`, `agents`, éditées dans la TUI, Équipe › Modèles). Elles s'appliquent au lancement des sessions des projets de l'équipe, après le hub (équipe · agent > équipe · famille > équipe) : ce sont des recommandations, que le workflow, le projet et le hub remplacent.

### Familles

La famille d'un agent est déduite de son dossier dans `agents/` :

| Dossier | Famille | Agents |
|------------|---------|--------|
| `agents/planning/` | `planning` | conductor, orchestrator, orchestrator-dev, planner, pathfinder, onboarder |
| `agents/developer/` | `developer` | developer, developer-refactor, developer-migrator, database, infra |
| `agents/quality/` | `quality` | reviewer, debugger, benchmarker, test-generator |
| `agents/auditor/` | `auditor` | auditor, auditor-subagent |
| `agents/design/` | `design` | designer |
| `agents/documentation/` | `documentation` | documentarian |
| `agents/utility/` | `utility` | brief-enricher |

### Liste blanche des modèles (`limits.models`)

Les restrictions I6 peuvent limiter les modèles d'une session (motifs avec `*`, ex. `eu.anthropic.claude-*`) :

- niveaux : hub (`[limits] models`, `oh budget set models …`), équipe (`[limits.recommended]` ou `[limits.enforced]`), projet (`oh budget set models … -p <projet>`), workflow (`limits.models`) ;
- la liste la plus précise l'emporte (workflow > projet > hub > recommandation d'équipe) ; une liste **imposée** par l'équipe est un plafond : seuls les motifs qu'elle couvre sont gardés ;
- la liste ne change pas le modèle choisi par la cascade : elle est appliquée par le **proxy d'identifiants**, qui refuse les appels vers un autre modèle (identifiant envoyé au fournisseur, ex. `eu.anthropic.claude-sonnet-4-6`).

Choisissez donc des modèles de cascade compatibles avec la liste. Voir `oh budget show` et [ADR-044](../architecture/adr/044-credential-proxy-session-limits.fr.md).

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
- `projects.model` → modèle global du projet (niveau 5)
- `projects.model_overrides` → JSON sérialisé pour per-agent et per-family (niveaux 3 et 4)

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

| Provider | Entrée (cascade) | Résultat dans la config de session |
|----------|-------------------|----------------------------|
| `anthropic` | `claude-sonnet-4-5` | `anthropic/claude-sonnet-4-5` |
| `bedrock` | `claude-sonnet-4-5` | `amazon-bedrock/anthropic.claude-sonnet-4-5-20250929-v1:0` |
| `bedrock` | `anthropic/claude-opus-4` | `amazon-bedrock/anthropic.claude-opus-4-20250514-v1:0` |
| `github-copilot` | `claude-sonnet-4-5` | `github-copilot/claude-sonnet-4.5` |
| `openrouter` | `claude-opus-4` | `anthropic/claude-opus-4` |

La normalisation extrait le "short name" (ex: `claude-opus-4`) depuis n'importe quel format d'entrée, puis le re-formate pour le provider cible.

---

## Plancher de modèle agent (frontmatter)

Les agents peuvent déclarer un modèle via le champ `model:` de leur frontmatter :

```yaml
---
id: orchestrator
model: claude-sonnet-4-6
---
```

Ce champ est le **niveau 9** de la cascade : il ne s'applique que si aucun niveau au-dessus ne définit de modèle.

### Agents avec plancher déclaré

| Agent | Plancher |
|-------|----------|
| `conductor`, `orchestrator`, `orchestrator-dev`, `planner`, `pathfinder`, `onboarder` | `claude-sonnet-4-6` |
| `auditor`, `auditor-subagent`, `designer`, `debugger`, `documentarian` | `claude-sonnet-4-6` |
| `reviewer`, `benchmarker`, `test-generator`, `database`, `infra` | `claude-opus-4-6` |
| `brief-enricher` | `anthropic/claude-sonnet-4-5` |

Les agents `developer*` n'en déclarent pas : sans configuration, opencode utilise son modèle par défaut.

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
