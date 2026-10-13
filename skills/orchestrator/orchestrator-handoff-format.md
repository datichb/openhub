---
name: orchestrator-handoff-format
description: "Utiliser pour le bloc retour/question entre orchestrator-dev (producteur) et orchestrator (lecteur)."
annexes: [templates/orchestrator-dev-handoff-blocks.md]
---

# Skill — Format de handoff orchestrator-dev → orchestrator

Ce skill est la **source de vérité unique** pour les deux formats de communication.
Il est injecté dans `orchestrator` et `orchestrator-dev` — le producteur et le consommateur partagent ainsi le même contrat, sans risque de désynchro.

> **Checkpoints** : `orchestrator-dev` passe lui-même les checkpoints de son tableau « Modes de workflow et checkpoints », dans l'ordre, avec `workflow_checkpoint`, même invoqué via `task`. Aucun de ces blocs ne relaie un checkpoint, et aucun en-tête ne contient un identifiant de checkpoint.

---

## Principe fondamental — bloc unique

Quand `orchestrator-dev` est invoqué depuis l'`orchestrator`, son **seul output** est le(s) bloc(s) structuré(s) défini(s) ci-dessous. Aucun texte libre avant, après ou en dehors des blocs.

**Format de sortie :** pas de récap global en texte libre, pas d'introduction, pas de résumé narratif. Toutes les informations sont encodées dans les champs structurés du bloc `## Retour vers orchestrator`.

---

## Deux formats selon la situation

| Situation | Format à produire |
|-----------|-------------------|
| Fin normale de session (tous les tickets traités ou stop) | `## Retour vers orchestrator` |
| Clarification nécessaire avant de continuer (blocage 3 cycles, dépendance non résolue, ticket bloqué, agent manquant) | `## Question pour l'orchestrator` **+** `## Retour vers orchestrator` |

> Les blocs sont **complémentaires, pas exclusifs** : pour une clarification, `orchestrator-dev` émet d'abord le bloc question, puis immédiatement `## Retour vers orchestrator` (pour tracer l'état courant de la session). Les deux blocs sont produits dans la même réponse, dans cet ordre.

---

## Format du bloc `## Retour vers orchestrator`

> **Template :** format défini dans `templates/orchestrator-dev-handoff-blocks.md` § "Format du bloc `## Retour vers orchestrator`" — charger via `read` quand tu produis ce bloc.

**Règle de remplissage du champ `Type de récap` :**
- `partiel` → ce bloc est émis dans la même réponse que `## Question pour l'orchestrator` — la session n'est pas terminée
- `final` → ce bloc est émis seul — tous les tickets ont été traités ou stop demandé, la session est terminée

Ce bloc est **obligatoire** quand invoqué depuis l'agent orchestrator feature. Il n'est pas produit quand `orchestrator-dev` est l'agent d'entrée de la session.

> ⚠️ Ce bloc doit être produit **même en cas de stop, de ticket bloqué ou de session partielle** — un récap `partiel` sans lui est invalide.
> ❌ Ne jamais écrire de texte libre en dehors des blocs structurés.

---

## Format du bloc `## Question pour l'orchestrator`

Quand `orchestrator-dev` est invoqué depuis l'`orchestrator` et qu'une **clarification** est nécessaire (aucun checkpoint ne la couvre), il produit ce bloc :

> **Template :** format défini dans `templates/orchestrator-dev-handoff-blocks.md` § "Format du bloc `## Question pour l'orchestrator`" — charger via `read` quand tu produis ce bloc.

| Situation | Déclencheur | `### Contexte complet` | `### Rapport de review complet` |
|----|------------|------------------------|----------------------------------|
| **Blocage 3 cycles** | 3 cycles de review sans résolution | Synthèse des problèmes persistants + résumé 1 ligne par cycle | Rapports des cycles concernés |
| **Dépendance non résolue** | Ticket dépend d'un ticket non terminé | ID du ticket bloquant, son statut, sa description | *(section omise)* |
| **Ticket bloqué** | Developer signale un blocage en cours d'implémentation | Raison du blocage telle que signalée, état du ticket | *(section omise)* |
| **Agent manquant** | L'agent requis ne fait pas partie du workflow | Agent requis, raison | *(section omise)* |

---

## Définitions du statut global (`## Retour vers orchestrator`)

| Statut | Condition |
|--------|-----------|
| `succès` | Tous les tickets traités ont été commités sans blocage persistant |
| `partiel` | Au moins un ticket ignoré ou bloqué après 3 cycles de review |
| `bloqué` | Au moins un ticket est resté bloqué et nécessite une intervention manuelle |

- **`### Questions bloquantes`** (obligatoire) — liste numérotée des questions nécessitant une réponse avant de poursuivre. Si aucune : écrire `Aucune.`. Le coordinateur **doit** résoudre ces questions avant de continuer le workflow.

---

## Règles pour l'orchestrator (consommateur)

### À la réception d'un `## Retour vers orchestrator`

- **Retranscrire les champs du bloc de manière formatée** dans le fil de discussion — tous les champs sont affichés lisiblement à l'utilisateur (tableau `### Détail par ticket`, `### Contexte et décisions par ticket`, `### Points d'attention globaux`, `### Questions bloquantes`).
- Ce format structuré est requis pour construire le récap de la feature.
- Si le bloc ne contient pas les champs requis → les demander explicitement à `orchestrator-dev` avant de continuer.
- Ne jamais construire le récap de la feature à partir d'un bloc incomplet ou d'un récap `partiel`.

### À la réception d'un `## Question pour l'orchestrator`
- Afficher le bloc **`### Contexte complet`** (et le rapport de review s'il est présent) dans le texte de la discussion — ne pas résumer.
- Poser la question à l'utilisateur via l'outil `question`, en reprenant exactement la question et les options du bloc. Le champ `header` décrit la situation (`Ticket bloqué #<ID>`, `Dépendance — #<ID>`…) et ne nomme jamais un checkpoint.
- Ré-invoquer `orchestrator-dev` avec `task_id` (valeur dans le bloc `### État de la session`) et transmettre la réponse :
  > `"Réponse de l'utilisateur : <réponse>. Reprendre le ticket #<ID>."`
- Ne jamais construire une réponse à la place de l'utilisateur.
- Ne jamais ignorer le bloc — toute question montante doit être traitée avant de continuer.
- Le `## Retour vers orchestrator` qui l'accompagne est `partiel` : afficher son `### État de la session` (progression), sans en tirer le récap de la feature.
