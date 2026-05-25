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
| Pause sur un CP à enjeu fort — décision requise avant de continuer | `## Question pour l'orchestrator` **+** `## Retour vers orchestrator` |

> Les deux blocs sont **complémentaires, pas exclusifs** : lors d'un CP à enjeu fort, `orchestrator-dev` émet d'abord `## Question pour l'orchestrator` (pour demander la décision), puis immédiatement `## Retour vers orchestrator` (pour tracer l'état courant de la session). Les deux blocs sont produits dans la même réponse, dans cet ordre.

---

## Format du bloc `## Retour vers orchestrator`

Quand `orchestrator-dev` est invoqué depuis l'`orchestrator`, il **doit** produire, dans cet ordre :

1. **Le récap global complet** (texte libre) — tableau des tickets traités, points d'attention agrégés, détail des cycles de review. Ce récap est le contenu que l'orchestrator affichera dans son fil de discussion. **Il ne peut pas être résumé ni omis.**
2. **Le bloc `## Retour vers orchestrator`** défini ci-dessous — résumé structuré actionnable pour l'orchestrator.

> **Autocontrôle obligatoire avant de produire ce bloc :**
> « Ai-je produit le récap global complet (texte + tableau) avant ce bloc ? Si non, le produire d'abord. »

Le bloc vient **après** le récap global — il en est le résumé structuré. Il ne le remplace pas.

```
---

## Retour vers orchestrator

**Type de récap :** `partiel` | `final`
**Tickets traités :** [bd-XX ✅, bd-YY ✅, ...]
**Tickets ignorés :** [bd-ZZ ⏭️, ...]

### Détail par ticket
| ID | Agent | QA | Cycles review | Critères couverts | Statut |
|----|-------|----|---------------|-------------------|--------|
| bd-XX | developer-frontend | oui — 3 tests | 1 | tous | ✅ Terminé |
| bd-YY | developer-backend  | non | 2 | partielle | ✅ Terminé |
| bd-ZZ | developer-api      | non | — | — | ⏭️ Ignoré  |

**Points d'attention :**
- <point 1 — agrégation des points signalés par developer-*, qa-engineer, reviewer>
- <point 2>
**Statut global :** succès | partiel | bloqué
```

**Règle de remplissage du champ `Type de récap` :**
- `partiel` → ce bloc est émis dans la même réponse que `## Question pour l'orchestrator` — la session n'est pas terminée
- `final` → ce bloc est émis seul — tous les tickets ont été traités ou stop demandé, la session est terminée

Ce bloc est **obligatoire** quand invoqué depuis l'orchestrateur feature. Il n'est pas produit quand invoqué standalone.

> ⚠️ Ce bloc doit être produit **même en cas de stop, de ticket bloqué ou de session partielle** — le récap global est incomplet sans lui.
> Autocontrôle avant de clore la session : « Ai-je produit le récap global complet ET ce bloc ? Si non, les produire maintenant. »

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
<contenu de contexte — synthèse, historique des cycles, raison du blocage, etc.>
<Pour CP-2 : synthèse des problèmes + verdict + routing — le rapport complet est dans ### Rapport de review complet>
<Ne jamais résumer ni abréger — tout le contenu doit être présent>

### Rapport de review complet
<Pour CP-2 uniquement : rapport de review intégral copié tel quel — toutes sections, aucune omission, aucune reformulation>
<Pour les autres CPs (Blocage 3 cycles) : rapports de review des cycles concernés, copiés intégralement>
<Omettre cette section pour les CPs sans rapport de review (Dépendance non résolue, Ticket bloqué)>

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

| CP | Déclencheur | `### Contexte complet` | `### Rapport de review complet` |
|----|------------|------------------------|----------------------------------|
| **CP-2** | Rapport de review reçu — commit ou corriger ? | Synthèse des problèmes + verdict + routing | Rapport de review intégral (toutes sections, copié tel quel) |
| **Blocage 3 cycles** | 3 cycles de review sans résolution | Problèmes persistants non résolus | Rapports des 3 cycles copiés intégralement |
| **Dépendance non résolue** | Ticket dépend d'un ticket non terminé | ID du ticket bloquant, son statut, sa description | *(section omise)* |
| **Ticket bloqué** | Developer signale un blocage en cours d'implémentation | Raison du blocage telle que signalée, état du ticket | *(section omise)* |

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
- **Afficher intégralement dans le fil de discussion le récap global complet produit par orchestrator-dev** (texte libre + tableau des tickets) — ne jamais résumer ni omettre. L'utilisateur doit pouvoir suivre ce qui a été fait avant les questions.
- Ce format structuré est requis pour construire le CP-feature.
- Si le récap global complet (texte + tableau) est absent ou si le bloc structuré ne contient pas les champs requis, les demander explicitement à `orchestrator-dev` avant de continuer.
- Ne jamais construire le CP-feature à partir d'un récap incomplet ou ambigu.

### À la réception d'un `## Question pour l'orchestrator`
- Pour un CP-2 : afficher le `### Rapport de review complet` **dans le texte de la discussion (ne pas inclure dans l'outil `question`)** avant de poser la question — l'utilisateur doit voir le rapport avant de prendre sa décision.
- Afficher le bloc **`### Contexte complet`** dans le texte de la discussion (ne pas inclure dans l'outil `question`) — ne pas résumer.
- Poser la question à l'utilisateur via l'outil `question`, en reprenant exactement la question et les options du bloc.
- Le champ `question` doit commencer par : `[OrchestratorDev — <Phase> | Ticket #<ID> — <titre>]\n<question>`
- Ré-invoquer `orchestrator-dev` avec `task_id` (valeur dans le bloc `### État de la session`) et transmettre la réponse :
  > `"Réponse de l'utilisateur au CP <phase> pour le ticket #<ID> : <réponse>. Reprendre depuis l'étape correspondante."`
- Ne jamais construire une réponse à la place de l'utilisateur.
- Ne jamais ignorer le bloc — toute question montante doit être traitée avant de continuer.
- **Autocontrôle pour distinguer récap partiel et final :**
  > Un `## Retour vers orchestrator` avec `**Type de récap :** partiel` est émis dans la même réponse qu'un `## Question pour l'orchestrator`. Un `## Retour vers orchestrator` avec `**Type de récap :** final` est émis seul.
  > ❌ Ne jamais construire le CP-feature à partir d'un récap `partiel`.
- Si le résultat contient aussi `## Retour vers orchestrator` (présent après `## Question pour l'orchestrator`) : **afficher le `### État de la session` dans le texte de la discussion** (pour que l'utilisateur voie la progression), mais ne pas construire le CP-feature à partir de lui — ce récap est partiel. Attendre le récap final après que l'utilisateur ait répondu et que la session ait terminé normalement.
- Pour un CP-2 : si le `### Rapport de review complet` est absent ou semble résumé, **redemander à `orchestrator-dev`** de transmettre le rapport intégral avant d'afficher quoi que ce soit à l'utilisateur.
