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

> **Note — préfixes provider :** les préfixes provider (ex. `anthropic/`) sont optionnels dans la cascade de résolution — les noms courts sont recommandés. L'adaptateur opencode applique automatiquement le préfixe du provider configuré en sortie (voir [Préfixage provider dans opencode.json](#préfixage-provider-dans-opencodejson)). Les valeurs frontmatter ou de configuration peuvent inclure un préfixe (ex. `anthropic/claude-opus-4`) — il sera strippé puis réappliqué selon le provider effectif.

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

> **Contrainte d'ordre frontmatter :** le champ `model:` **doit apparaître avant** `skills:` dans le frontmatter. Le parser utilise un early exit après lecture de `id` et `skills` — si `model:` est placé après `skills:`, il ne sera pas lu.

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

> **Note :** les flags `--family-model` et `--agent-model` nécessitent l'amélioration `oh config set` (ticket .6). Si elle n'est pas encore implémentée, configurer `hub.json` et `api-keys.local.md` manuellement comme montré dans les exemples ci-dessus.

```bash
# Niveau hub
oh config set --family-model planning=claude-opus-4
oh config set --agent-model debugger=claude-sonnet-4-5

# Niveau projet
oh config set MY-APP --family-model planning=claude-opus-4
oh config set MY-APP --agent-model reviewer=claude-sonnet-4-5
```

---

## Règle d'injection dans opencode.json

- Si le modèle résolu (après clamp) == modèle global du projet → **pas d'injection** (l'agent utilise le modèle par défaut, ce qui évite le bruit dans la configuration)
- Si le modèle résolu ≠ modèle global → injection de `"model": "<valeur>"` dans l'entrée de l'agent

---

## Préfixage provider dans opencode.json

Opencode exige que les noms de modèles soient préfixés avec le provider au format `provider/model`.
L'adaptateur opencode applique ce préfixage **automatiquement** après la résolution et le clamp,
en utilisant les champs `opencode_prefix` et `model_aliases` de `config/providers.json`.

### Flux complet

```
Cascade (7 niveaux) → Clamp (plancher) → Strip préfixe existant → Alias provider → Préfixe provider → opencode.json
                       ↑ noms courts                                                 ↑ noms préfixés
```

> **Important :** la cascade de résolution et le clamp travaillent exclusivement avec des noms courts
> (ex. `claude-sonnet-4-5`). Le préfixage est une transformation en sortie de l'adaptateur opencode —
> il n'affecte pas la résolution interne.

### Champs de `config/providers.json`

| Champ | Type | Description |
|-------|------|-------------|
| `opencode_prefix` | `string \| null` | Préfixe opencode du provider. `null` = pas de préfixe (providers litellm). |
| `model_aliases` | `object \| null` | Mapping nom interne → nom spécifique au provider. `null` si le provider n'utilise pas d'alias. |

### Exemples par provider

| Provider | `opencode_prefix` | Modèle interne | Alias | Résultat dans opencode.json |
|----------|-------------------|----------------|-------|-----------------------------|
| `anthropic` | `"anthropic"` | `claude-sonnet-4-5` | — | `anthropic/claude-sonnet-4-5` |
| `bedrock` | `"amazon-bedrock"` | `claude-sonnet-4-5` | `anthropic.claude-sonnet-4-5-20250929-v1:0` | `amazon-bedrock/anthropic.claude-sonnet-4-5-20250929-v1:0` |
| `github-copilot` | `"github-copilot"` | `claude-sonnet-4-5` | `claude-sonnet-4.5` | `github-copilot/claude-sonnet-4.5` |
| `mammouth` | `null` | `claude-sonnet-4-5` | — | `claude-sonnet-4-5` |
| `ollama` | `null` | `llama3.2` | — | `llama3.2` |

### Rétrocompatibilité

- La cascade de résolution (niveaux 1-7) n'est pas affectée — elle continue d'accepter les noms courts et les noms préfixés
- Le clamp continue de fonctionner avec les noms courts (le préfixe est strippé avant comparaison)
- Les configurations existantes (`hub.json`, `api-keys.local.md`) n'ont pas besoin d'être modifiées
