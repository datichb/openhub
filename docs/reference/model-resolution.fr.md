# Résolution du modèle et du provider

---

## Vue d'ensemble

Le CLI Go (`oh`) utilise un système de résolution simple pour déterminer le provider et le modèle IA utilisés par chaque projet. Il n'y a pas d'assignation de modèle par agent ou par famille — opencode gère le routage de modèle en interne.

---

## Résolution du provider

Le provider est résolu via une cascade à 3 niveaux (premier match gagne) :

| Priorité | Source | Exemple |
|----------|--------|---------|
| 1 | Flag CLI `--provider` | `oh deploy --provider anthropic` |
| 2 | Config hub | `hub.toml` → `[opencode] default_provider = "bedrock"` |
| 3 | Fallback hardcodé | `bedrock` |

---

## Configuration du modèle

Le modèle est configuré au niveau projet, pas par agent. Il est défini dans le `opencode.json` du projet (déployé par le CLI) :

```json
{
  "model": "amazon-bedrock/anthropic.claude-sonnet-4-5-20250929-v1:0"
}
```

### Définir le modèle

**Lors du déploiement :**

```bash
oh deploy --model claude-sonnet-4-5
```

**Override par projet :**

```bash
oh project configure --provider anthropic --model claude-opus-4
```

Le CLI applique automatiquement le bon préfixe provider en fonction du provider (ex. `anthropic/claude-opus-4`, `amazon-bedrock/anthropic.claude-opus-4-20250514-v1:0`).

---

## Préfixage provider

Opencode exige que les noms de modèles soient préfixés avec le provider au format `provider/model`. Le CLI applique ce préfixage **automatiquement** lors du déploiement, en utilisant la configuration interne des providers.

### Exemples par provider

| Provider | Modèle interne | Résultat dans opencode.json |
|----------|----------------|----------------------------|
| `anthropic` | `claude-sonnet-4-5` | `anthropic/claude-sonnet-4-5` |
| `bedrock` | `claude-sonnet-4-5` | `amazon-bedrock/anthropic.claude-sonnet-4-5-20250929-v1:0` |
| `github-copilot` | `claude-sonnet-4-5` | `github-copilot/claude-sonnet-4.5` |

---

## Configuration hub (`hub.toml`)

Le provider et le modèle par défaut sont stockés dans `~/.oh/hub.toml` :

```toml
[opencode]
default_provider = "bedrock"
```

Ce fichier est géré par le CLI :

```bash
# Voir la config actuelle
oh config show

# Changer le provider par défaut
oh config set opencode.default_provider anthropic
```

---

## Plancher de modèle agent (frontmatter)

Les agents peuvent déclarer un modèle minimum via le champ `model:` dans leur frontmatter. Ce champ définit un **plancher** — pas un override. Opencode respecte ce plancher en interne lors du routage des requêtes vers les agents.

```yaml
---
id: orchestrator
model: anthropic/claude-opus-4
skills: [skill-a, skill-b]
---
```

### Agents avec plancher par défaut

| Agent | Plancher |
|-------|----------|
| `orchestrator` | `anthropic/claude-opus-4` |
| `orchestrator-dev` | `anthropic/claude-opus-4` |
| `reviewer` | `anthropic/claude-opus-4` |
| `planner` | `anthropic/claude-opus-4` |

---

## Différences avec l'ancien système

| Ancien (ère bash) | Actuel (CLI Go) |
|---|---|
| Cascade à 7 niveaux par agent | Simple provider + modèle au niveau projet |
| `api-keys.local.md` par projet | `hub.toml` + `opencode.json` |
| `config/hub.json` | `~/.oh/hub.toml` |
| Modèle par agent et par famille | Pas de modèle par agent — opencode route en interne |
| Fallback `prompt-builder.sh` | Fallback hardcodé dans le binaire Go |
| `config/providers.json` | Logique provider intégrée au CLI Go |
