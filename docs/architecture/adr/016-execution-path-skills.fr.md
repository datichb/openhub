> 🇬🇧 [Read in English](016-execution-path-skills.en.md)

# ADR-016 — Skills de parcours d'exécution : extraction du double chemin standalone/sous-agent

## Statut

Accepté

## Contexte

Les agents primaires invocables directement (`planner`, `pathfinder`, `onboarder`, `auditor`, `orchestrator-dev`, `reviewer`, `qa-engineer`) peuvent être invoqués de deux façons :

1. **Standalone** — directement par l'utilisateur : communication via l'outil `question`, récaps texte visibles dans la discussion, pas de blocs handoff orchestrateur
2. **Sous-agent** — via `task` depuis l'orchestrateur : mécanisme d'interruption de session, blocs structurés `## Retour intermédiaire vers orchestrateur` + `## Question pour l'orchestrateur`, `task_id` obligatoire, outil `question` interdit

Avant cette décision, les deux parcours cohabitaient dans les skills workflow des agents (ex. `planner-workflow.md`), séparés par un branchement conditionnel détecté au démarrage :

```
Si le prompt contient `[CONTEXTE] Invoqué depuis l'orchestrateur feature` :
  → Parcours sous-agent
Sinon :
  → Parcours standalone
```

Cette architecture posait plusieurs problèmes :

1. **Charge contextuelle inutile** : les deux parcours (jusqu'à ~300 lignes chacun) voyageaient toujours ensemble dans le context, même si un seul était actif pour la session.
2. **Confusion de parcours** : les règles croisées ("si CONTEXTE = X, sinon…") augmentaient le risque d'erreur LLM en cours de session — les checklists anti-erreur dans les skills en étaient le symptôme.
3. **Couplage fort** : ajouter une règle au parcours sub-agent nécessitait d'éditer un skill complexe partagé avec le parcours standalone, augmentant le risque de régression.
4. **Testabilité dégradée** : il n'était pas possible de tester un parcours indépendamment de l'autre.

## Décision

Extraire chaque parcours d'exécution dans un **skill Bucket B dédié**, chargé au démarrage de la session selon le contexte d'invocation.

### Mécanisme de chargement — Option B1 (injection + fallback)

L'orchestrateur injecte le skill sous-agent dans le prompt `task` via le marqueur `[SKILL:<nom>]` :

```
[CONTEXTE] Invoqué depuis l'orchestrateur feature.
[SKILL:planning/planner-subagent]
```

L'agent applique la règle suivante au démarrage :

> Si le prompt contient `[SKILL:<nom>]` → charger ce skill via l'outil `skill`.
> Sinon (invocation directe) → charger le skill `<agent>-standalone` (défaut implicite).

### Structure des nouveaux skills

Pour chaque agent concerné, deux skills Bucket B sont créés :

| Agent | Skill standalone | Skill sous-agent |
|-------|-----------------|-----------------|
| `planner` | `planning/planner-standalone` | `planning/planner-subagent` |
| `pathfinder` | `planning/pathfinder-standalone` | `planning/pathfinder-subagent` |
| `onboarder` | `planning/onboarder-standalone` | `planning/onboarder-subagent` |
| `auditor` | `auditor/auditor-standalone` | `auditor/auditor-subagent` |
| `orchestrator-dev` | `orchestrator/orchestrator-dev-standalone` | `orchestrator/orchestrator-dev-subagent` |
| `reviewer` | `reviewer/reviewer-standalone` | `reviewer/reviewer-subagent` |
| `qa-engineer` | `qa/qa-standalone` | `qa/qa-subagent` |

### Ce que contient chaque skill

**Skill `-standalone`** :
- Règle absolue : récap texte avant appel `question`
- Autocontrôle avant chaque checkpoint
- Format des questions de validation via l'outil `question` (une par phase)
- Format final (sans bloc handoff orchestrateur)

**Skill `-subagent`** :
- Confirmation du contexte au démarrage
- Mécanisme d'interruption : produire récap + blocs structurés + terminer session
- Autocontrôle avant chaque fin de session
- Format des blocs `## Retour intermédiaire vers orchestrateur` et `## Question pour l'orchestrateur` avec `task_id`
- Format final (avec bloc handoff orchestrateur)
- Liste des erreurs fréquentes à éviter

### Ce qui ne change pas

Les skills sources (`planner-workflow`, `onboarder-workflow`, etc.) conservent les phases détaillées du workflow, les templates de création Beads, et les règles métier. Seule la logique de bifurcation standalone/sous-agent en est retirée et remplacée par un renvoi aux nouveaux skills dédiés.

## Conséquences

### Positives

- **Allègement du context** : en session standalone, le parcours sous-agent n'est jamais chargé (~30-60% de réduction sur les skills de parcours). En sous-agent, inversement.
- **Clarté** : chaque skill a une responsabilité unique, sans branchement conditionnel.
- **Testabilité** : les deux parcours peuvent être testés et validés indépendamment.
- **Extensibilité** : ajouter un comportement à un parcours ne risque plus de perturber l'autre.
- **Observabilité** : l'orchestrateur contrôle explicitement quel parcours est activé via l'injection `[SKILL:...]`.

### À surveiller

- L'orchestrateur doit injecter `[SKILL:...]` dans **tous** les prompts `task` vers les agents concernés, y compris les ré-invocations avec `task_id`.
- Les agents concernés doivent avoir `skill: allow` dans leurs permissions pour pouvoir charger le skill au démarrage.
- Si le `[SKILL:...]` est omis dans un prompt sous-agent, l'agent charge le skill standalone par défaut — comportement dégradé mais pas cassé.

## Alternatives considérées

### Option A — Auto-détection complète par l'agent

L'agent détecte lui-même le marqueur `[CONTEXTE]` et charge le bon skill. Cette option a été rejetée car elle maintient la logique de détection dans l'agent et ne permet pas à l'orchestrateur de contrôler explicitement le parcours activé.

### Option B2 — Standalone en native_skill auto-load

Le skill standalone est toujours chargé automatiquement ; seul le skill sous-agent est injecté dans le prompt. Cette option est moins explicite : il n'est pas clair, à la lecture du prompt, si le parcours standalone est actif par défaut ou non.

L'**Option B1** (implémentée) offre le meilleur équilibre : le fallback standalone est implicite et prévisible, l'injection sous-agent est explicite et observable.
