---
name: tool-todowrite
description: Utilisation de l'outil todowrite d'OpenCode — quand et comment suivre la progression des tâches dans une session. Couvre le seuil des 3 étapes, la mise à jour en temps réel et la différence avec Beads.
---

# Skill — Outil `todowrite` (OpenCode)

## Quand utiliser `todowrite`

Utiliser quand la demande implique **3 étapes distinctes ou plus**.

| Situation | `todowrite` ? |
|-----------|:---:|
| Feature multi-étapes (analyse, code, tests, review) | ✅ |
| Refactoring avec 3+ fichiers | ✅ |
| Requête informationnelle | ❌ |
| Tâche triviale (1-2 étapes) | ❌ |
| Exploration ou lecture seule | ❌ |

## Contraintes

- **Exactement 1 tâche `in_progress` à la fois** — on termine ce qu'on a commencé
- **Mise à jour en temps réel** à chaque transition — pas de batch en fin de session
- **Chaque appel envoie la liste complète** (pas de delta)
- **Descriptions concises et actionnables** — pas de micro-étapes ("ouvrir fichier", "sauvegarder")
- Ne pas tracker des réflexions ou hypothèses — uniquement des **actions concrètes**

## Différence avec Beads

| | `todowrite` | Beads |
|--|-------------|-------|
| **Portée** | Session courante uniquement | Persistant entre sessions |
| **Granularité** | Étapes techniques d'une tâche | Tickets métier |
| **Cycle de vie** | Disparaît à la fin de la session | Workflow complet |

Les deux sont complémentaires : un ticket Beads contient le "quoi", `todowrite` découpe le "comment" en étapes techniques.

---

## Isolation des sessions

Chaque agent invoqué via `task` a sa propre session isolée. La todo list est stockée par `session_id` — un sous-agent ne peut **jamais** mettre à jour la liste de son parent.

**Conséquence :** Seul l'agent de plus haut niveau (invoqué directement par l'utilisateur) maintient une liste visible dans l'interface.

| Agent | Invoqué par | Liste visible ? |
|-------|-------------|:---:|
| `orchestrator` | utilisateur | ✅ |
| `orchestrator-dev` | utilisateur directement | ✅ |
| `orchestrator-dev` | orchestrator via `task` | ❌ |
| `developer-*`, `reviewer`… | via `task` | ❌ |

---

## Suffixes de phase — orchestrator-dev standalone

Quand `orchestrator-dev` est invoqué directement, mettre à jour le label de la tâche à chaque phase :

| Phase | Suffixe |
|-------|---------|
| Implémentation | `[dev]` |
| QA | `[QA]` |
| Review | `[review]` |
| En attente CP-2 | `[CP-2]` |
| Terminé | *(pas de suffixe)* |

Exemple : `#bd-12 — Endpoint POST /users [dev]` → `[QA]` → `[review]` → `completed`

> Ces suffixes ne s'appliquent qu'en mode standalone. En mode sous-agent, la liste est invisible.
