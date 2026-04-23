---
name: orchestrator-handoff-format
description: Source de vérité unique pour les deux formats de communication entre orchestrator-dev et orchestrator — le bloc de retour en fin de session (succès/partiel/bloqué) et le bloc de question montante pour les CPs à enjeu fort (CP-2, blocage 3 cycles, dépendance, ticket bloqué). Injecté dans orchestrator et orchestrator-dev pour garantir que le producteur et le consommateur partagent le même contrat de communication.
---

# Skill — Format de handoff orchestrator-dev → orchestrator

Ce skill est la **source de vérité unique** pour les deux formats de communication.
Il est injecté dans `orchestrator` et `orchestrator-dev` — le producteur et le consommateur partagent ainsi le même contrat, sans risque de désynchro.

---

## Deux formats selon la situation

| Situation | Format à produire |
|-----------|-------------------|
| Fin normale de session (tous les tickets traités ou stop) | `## Retour vers orchestrator` |
| Pause sur un CP à enjeu fort — décision requise avant de continuer | `## Question pour l'orchestrator` |

---

## Format du bloc `## Retour vers orchestrator`

Quand `orchestrator-dev` est invoqué depuis l'`orchestrator`, il **doit** produire ce bloc à la fin de son récap global :

```
---

## Retour vers orchestrator

**Tickets traités :** [bd-XX ✅, bd-YY ✅, ...]
**Tickets ignorés :** [bd-ZZ ⏭️, ...]
**Points d'attention :**
- <point 1>
- <point 2>
**Statut global :** succès | partiel | bloqué
```

Ce bloc est **obligatoire** quand invoqué depuis l'orchestrateur feature. Il n'est pas produit quand invoqué standalone.

---

## Format du bloc `## Question pour l'orchestrator`

Quand `orchestrator-dev` est invoqué depuis l'`orchestrator` et atteint un **CP à enjeu fort**,
il **ne pose pas la question lui-même** — il arrête sa session en produisant ce bloc :

```
---

## Question pour l'orchestrator

**Agent :** orchestrator-dev
**Ticket :** #<ID> — <titre>
**Phase :** <CP-2 | Blocage 3 cycles | Dépendance non résolue | Ticket bloqué>

### Contexte complet
<contenu intégral — rapport de review, historique des cycles, raison du blocage, etc.>
<Ne jamais résumer ni abréger — tout le contenu doit être présent>

### Question en attente
<texte exact de la question à poser à l'utilisateur>

### Options disponibles
- `<label-option-1>` : <description de ce que ce choix implique>
- `<label-option-2>` : <description>

### État de la session
**Tickets traités :** [bd-XX ✅, ...]
**En cours :** bd-<ID>
**Tickets restants :** [bd-YY, bd-ZZ, ...]
**task_id :** <task_id de la session en cours>
```

Ce bloc est **obligatoire** pour les CPs à enjeu fort quand invoqué depuis l'orchestrateur feature.
En mode **standalone**, `orchestrator-dev` pose les questions lui-même via l'outil `question` — comportement inchangé.

### CPs à enjeu fort qui déclenchent ce bloc

| CP | Déclencheur | Contexte à inclure |
|----|------------|-------------------|
| **CP-2** | Rapport de review reçu — commit ou corriger ? | Rapport de review intégral (toutes sections) |
| **Blocage 3 cycles** | 3 cycles de review sans résolution | Historique des 3 rapports de review, problèmes persistants |
| **Dépendance non résolue** | Ticket dépend d'un ticket non terminé | ID du ticket bloquant, son statut, sa description |
| **Ticket bloqué** | Developer signale un blocage en cours d'implémentation | Raison du blocage telle que signalée, état du ticket |

---

## Définitions du statut global (`## Retour vers orchestrator`)

| Statut | Condition |
|--------|-----------|
| `succès` | Tous les tickets traités ont été commités sans blocage persistant |
| `partiel` | Au moins un ticket ignoré ou bloqué après 3 cycles de review |
| `bloqué` | Au moins un ticket est resté bloqué et nécessite une intervention manuelle |

---

## Règles pour l'orchestrator (consommateur)

### À la réception d'un `## Retour vers orchestrator`
- Ce format structuré est requis pour construire le CP-feature.
- Si le récap reçu ne contient pas ces champs, les demander explicitement à `orchestrator-dev` avant de continuer.
- Ne jamais construire le CP-feature à partir d'un récap incomplet ou ambigu.

### À la réception d'un `## Question pour l'orchestrator`
- Afficher le bloc **Contexte complet** tel quel dans la discussion — ne pas résumer.
- Poser la question à l'utilisateur via l'outil `question`, en reprenant exactement la question et les options du bloc.
- Le champ `question` doit commencer par : `[OrchestratorDev — <Phase> | Ticket #<ID> — <titre>]\n<question>`
- Ré-invoquer `orchestrator-dev` avec `task_id` (valeur dans le bloc `### État de la session`) et transmettre la réponse :
  > `"Réponse de l'utilisateur au CP <phase> pour le ticket #<ID> : <réponse>. Reprendre depuis l'étape correspondante."`
- Ne jamais construire une réponse à la place de l'utilisateur.
- Ne jamais ignorer le bloc — toute question montante doit être traitée avant de continuer.
