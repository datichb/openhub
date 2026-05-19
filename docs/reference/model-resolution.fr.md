# Résolution du modèle par agent

---

## Vue d'ensemble

Chaque agent peut recevoir un modèle IA spécifique via une cascade de résolution à 7 niveaux.
Le premier niveau qui retourne une valeur gagne. Un mécanisme de plancher (clamp) garantit
qu'un agent critique ne reçoit jamais un modèle inférieur à son minimum déclaré.

---

## Cascade de résolution (7 niveaux)

Pour un agent `X` de famille `F` dans un projet `P` :

| Priorité | Source | Clé |
|----------|--------|-----|
| 1 | Projet — agent spécifique | `api-keys.local.md` → `agent_models.agents.X=...` |
| 2 | Projet — famille | `api-keys.local.md` → `agent_models.families.F=...` |
| 3 | Projet — modèle global | `api-keys.local.md` → `model=...` |
| 4 | Hub — agent spécifique | `config/hub.json` → `.agent_models.agents.X` |
| 5 | Hub — famille | `config/hub.json` → `.agent_models.families.F` |
| 6 | Hub — modèle global | `config/hub.json` → `.opencode.model` |
| 7 | Fallback hardcodé | `claude-sonnet-4-5` (valeur actuelle — voir `DEFAULT_MODEL` dans `prompt-builder.sh`) |

**Exemple :** si le projet définit un modèle pour la famille `planning` (niveau 2) et que le hub définit un modèle pour l'agent `orchestrator` (niveau 4), c'est le niveau 2 qui l'emporte car il est prioritaire.

> **Note — préfixes provider :** les préfixes provider (ex. `anthropic/`) sont optionnels dans la cascade de résolution. Le fallback hardcodé (niveau 7) n'en inclut pas (`claude-sonnet-4-5`), tandis que les valeurs frontmatter ou de configuration peuvent en inclure (ex. `anthropic/claude-opus-4`). Les deux formes sont acceptées.

> **Note — `default_provider.model` :** le champ `default_provider.model` de `hub.json` n'est PAS utilisé dans cette cascade. Il sert uniquement à la configuration du provider OpenCode, pas à la résolution de modèle par agent.

---

## Plancher (clamp) via frontmatter

Les agents peuvent déclarer un modèle minimum via le champ `model:` dans leur frontmatter.
Ce champ définit un **plancher** — pas un override. Le modèle résolu par la cascade est conservé
s'il est supérieur ou égal au plancher ; sinon le plancher est appliqué.

```yaml
---
id: orchestrator
model: anthropic/claude-opus-4
skills: [skill-a, skill-b]
---
```

> **Contrainte d'ordre frontmatter :** le champ `model:` **doit apparaître avant** `skills:` dans le frontmatter. Le parser utilise un early exit après lecture de `id`, `targets` et `skills` — si `model:` est placé après `skills:`, il ne sera pas lu.

Après résolution de la cascade, si le modèle résolu est **inférieur** au plancher déclaré,
le plancher est appliqué et un warning est émis :

```
WARN  Modèle résolu 'claude-haiku-4-5' inférieur au plancher 'anthropic/claude-opus-4' pour l'agent 'orchestrator' — plancher appliqué
```

### Hiérarchie des modèles (rangs)

Chaque modèle est associé à un rang numérique pour la comparaison :

| Modèle | Rang |
|--------|------|
| `claude-opus-4` | 3 |
| `claude-sonnet-4-5` | 2 |
| `claude-haiku-4-5` | 1 |
| Tout autre modèle | 0 |

Un modèle inconnu (rang 0) est **toujours inférieur** à haiku, ce qui force systématiquement
le clamp au plancher déclaré. Cela évite qu'un modèle non reconnu contourne silencieusement
un plancher opus ou sonnet.

### Agents avec plancher par défaut

| Agent | Plancher | Rang |
|-------|----------|------|
| `orchestrator` | `anthropic/claude-opus-4` | 3 |
| `orchestrator-dev` | `anthropic/claude-opus-4` | 3 |
| `reviewer` | `anthropic/claude-opus-4` | 3 |
| `planner` | `anthropic/claude-opus-4` | 3 |

---

## Famille d'un agent

La famille est déduite du sous-dossier parent dans `agents/` :

- `agents/planning/orchestrator.md` → famille `planning`
- `agents/developer/developer-frontend.md` → famille `developer`
- `agents/quality/reviewer.md` → famille `quality`

---

## Exemples de configuration

### hub.json — modèle par famille et par agent

```json
{
  "opencode": {
    "model": "claude-sonnet-4-5"
  },
  "agent_models": {
    "families": {
      "planning": "claude-opus-4",
      "developer": "claude-sonnet-4-5"
    },
    "agents": {
      "debugger": "claude-opus-4",
      "documentarian": "claude-haiku-4-5"
    }
  }
}
```

Dans cet exemple :
- Tous les agents de la famille `planning` (orchestrator, planner…) reçoivent `claude-opus-4` (niveau 5)
- L'agent `debugger` reçoit `claude-opus-4` (niveau 4 — prioritaire sur la famille `developer`)
- L'agent `documentarian` reçoit `claude-haiku-4-5` (niveau 4)
- Les agents sans override utilisent le modèle global `claude-sonnet-4-5` (niveau 6)

### api-keys.local.md — override projet

Dans le fichier `api-keys.local.md` d'un projet spécifique :

```markdown
# API Keys

model=claude-opus-4
agent_models.families.quality=claude-opus-4
agent_models.agents.developer-frontend=claude-haiku-4-5
```

Dans cet exemple :
- L'agent `developer-frontend` reçoit `claude-haiku-4-5` (niveau 1 — override agent projet)
- Tous les agents de la famille `quality` reçoivent `claude-opus-4` (niveau 2)
- Tous les autres agents du projet reçoivent `claude-opus-4` (niveau 3 — modèle global projet)
- Les niveaux 4-7 (hub) ne sont jamais consultés car le niveau 3 répond pour tous

---

## Configuration via CLI

> **Note :** les flags `--family-model` et `--agent-model` nécessitent l'amélioration `oc config set` (ticket .6). Si elle n'est pas encore implémentée, configurer `hub.json` et `api-keys.local.md` manuellement comme montré dans les exemples ci-dessus.

```bash
# Niveau hub
oc config set --family-model planning=claude-opus-4
oc config set --agent-model debugger=claude-sonnet-4-5

# Niveau projet
oc config set MY-APP --family-model planning=claude-opus-4
oc config set MY-APP --agent-model reviewer=claude-sonnet-4-5
```

---

## Règle d'injection dans opencode.json

- Si le modèle résolu (après clamp) == modèle global du projet → **pas d'injection** (l'agent utilise le modèle par défaut, ce qui évite le bruit dans la configuration)
- Si le modèle résolu ≠ modèle global → injection de `"model": "<valeur>"` dans l'entrée de l'agent
