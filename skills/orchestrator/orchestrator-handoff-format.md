---
name: orchestrator-handoff-format
description: Source de vérité unique pour les formats de communication entre orchestrator-dev et orchestrator — le bloc de retour en fin de session (succès/partiel/bloqué), le bloc de question montante pour les CPs à enjeu fort (CP-2, blocage 3 cycles, dépendance, ticket bloqué), et le bloc de question batch pour les CP-2 groupés (N tickets avec verdict commit). Injecté dans orchestrator et orchestrator-dev pour garantir que le producteur et le consommateur partagent le même contrat de communication.
---

# Skill — Format de handoff orchestrator-dev → orchestrator

Ce skill est la **source de vérité unique** pour les trois formats de communication.
Il est injecté dans `orchestrator` et `orchestrator-dev` — le producteur et le consommateur partagent ainsi le même contrat, sans risque de désynchro.

---

## Principe fondamental — bloc unique

Quand `orchestrator-dev` est invoqué depuis l'`orchestrator`, son **seul output** est le(s) bloc(s) structuré(s) défini(s) ci-dessous. Aucun texte libre avant, après ou en dehors des blocs.

**Règle absolue :** pas de récap global en texte libre, pas d'introduction, pas de résumé narratif. Toutes les informations sont encodées dans les champs structurés du bloc `## Retour vers orchestrator`.

---

## Trois formats selon la situation

| Situation | Format à produire |
|-----------|-------------------|
| Fin normale de session (tous les tickets traités ou stop) | `## Retour vers orchestrator` |
| Pause sur un CP à enjeu fort — décision requise avant de continuer (1 ticket) | `## Question pour l'orchestrator` **+** `## Retour vers orchestrator` |
| Pause sur un CP-2 batch — N tickets avec verdict `commit` (mode parallèle) | `## Question batch pour l'orchestrator` **+** `## Retour vers orchestrator` |

> Les blocs sont **complémentaires, pas exclusifs** : lors d'un CP à enjeu fort, `orchestrator-dev` émet d'abord le bloc question (unitaire ou batch selon le cas), puis immédiatement `## Retour vers orchestrator` (pour tracer l'état courant de la session). Les deux blocs sont produits dans la même réponse, dans cet ordre.

---

## Format du bloc `## Retour vers orchestrator`

```
---

## Retour vers orchestrator

**Type de récap :** `partiel` | `final`
**Tickets traités :** [bd-XX ✅, bd-YY ✅, ...]
**Tickets ignorés :** [bd-ZZ ⏭️, ...]

### Détail par ticket

| ID | Agent (domaine) | Cycles review | Critères couverts | Statut |
|----|----------------|---------------|-------------------|--------|
| bd-XX | developer (frontend) | 1 | tous | ✅ Terminé |
| bd-YY | developer (backend)  | 2 | partielle | ✅ Terminé |
| bd-ZZ | developer (api)      | — | — | ⏭️ Ignoré  |

### Contexte et décisions par ticket

**bd-XX — <titre>**
- <décision technique notable + justification>
- <compromis fait + raison>
- Points d'attention : <signalés par developer ou reviewer>

**bd-YY — <titre>**
- <décision technique notable + justification>
- Blocage rencontré : <description + résolution>
- Points d'attention : <signalés par developer ou reviewer>

<Répéter pour chaque ticket traité — minimum 1-2 lignes par ticket. Omettre pour les tickets ignorés.>

### Points d'attention globaux
- <point 1 — agrégation des points signalés par developer, developer-refactor, developer-migrator, reviewer>
- <point 2>
<"Aucun point d'attention global" si tous les tickets sont propres>

### Données techniques brutes
<Diffs significatifs, résultats de tests globaux, informations nécessaires à l'orchestrator pour construire le CP-feature — uniquement si pertinent>
<"Aucune" si non applicable>

**Statut global :** `succès` | `partiel` | `bloqué`
```

**Règle de remplissage du champ `Type de récap` :**
- `partiel` → ce bloc est émis dans la même réponse que `## Question pour l'orchestrator` — la session n'est pas terminée
- `final` → ce bloc est émis seul — tous les tickets ont été traités ou stop demandé, la session est terminée

Ce bloc est **obligatoire** quand invoqué depuis l'agent orchestrator feature. Il n'est pas produit quand invoqué standalone.

> ⚠️ Ce bloc doit être produit **même en cas de stop, de ticket bloqué ou de session partielle** — un récap `partiel` sans lui est invalide.
> Autocontrôle avant de clore la session : « Ai-je produit ce bloc ? Si non, le produire maintenant. »
> ❌ Ne jamais écrire de texte libre en dehors des blocs structurés.

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
<Pour CP-2 uniquement : rapport de review intégral copié tel quel depuis le champ ### Rapport complet du bloc reviewer — toutes sections, aucune omission, aucune reformulation>
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

Ce bloc est **obligatoire** pour les CPs à enjeu fort quand invoqué depuis l'agent orchestrator feature.
En mode **standalone**, `orchestrator-dev` pose les questions lui-même via l'outil `question` — comportement inchangé.

### CPs à enjeu fort qui déclenchent ce bloc

| CP | Déclencheur | `### Contexte complet` | `### Rapport de review complet` |
|----|------------|------------------------|----------------------------------|
| **CP-2** | Rapport de review reçu — commit ou corriger ? | Synthèse des problèmes + verdict + routing | Rapport de review intégral (toutes sections, copié tel quel depuis le champ `### Rapport complet` du bloc reviewer) |
| **Blocage 3 cycles** | 3 cycles de review sans résolution | Synthèse des problèmes persistants + résumé 1 ligne par cycle | *(section omise — synthèse dans `### Contexte complet`)* |
| **Dépendance non résolue** | Ticket dépend d'un ticket non terminé | ID du ticket bloquant, son statut, sa description | *(section omise)* |
| **Ticket bloqué** | Developer signale un blocage en cours d'implémentation | Raison du blocage telle que signalée, état du ticket | *(section omise)* |

---

## Format du bloc `## Question batch pour l'orchestrator`

Quand `orchestrator-dev` atteint un **CP-2 batch** (N tickets avec verdict `commit` en mode parallèle),
il produit ce bloc au lieu de N blocs `## Question pour l'orchestrator` unitaires :

```
---

## Question batch pour l'orchestrator

**Agent :** orchestrator-dev
**Phase :** CP-2 Batch
**Nombre de tickets :** <N>

### Récapitulatif du batch

| ID | Titre | Agent (domaine) | Verdict | Cycles |
|-----|-------|----------------|---------|--------|
| bd-XX | <titre court — max 40 car.> | developer (frontend) | commit | 1 |
| bd-YY | <titre court> | developer (backend) | commit | 2 |
| bd-ZZ | <titre court> | developer (api) | commit | 1 |

> Tous les verdicts sont `commit` — aucun problème bloquant détecté sur ces tickets.

### Question en attente
<N> tickets ont reçu un verdict `commit` du reviewer. Quelle action pour ce lot ?

### Options disponibles
- `Commit tous` : Commiter les <N> tickets en séquence avec leurs messages Conventional Commits respectifs
- `Commit sélectif` : Choisir quels tickets commiter parmi les <N> disponibles
- `Voir détails` : Afficher le rapport de review de chaque ticket avant de décider

### Rapports de review complets

<Pour chaque ticket du batch, inclure le rapport complet dans une sous-section dédiée.
 Ces rapports ne sont PAS affichés par défaut — l'orchestrator les affiche uniquement
 si l'utilisateur choisit "Voir détails".>

#### Ticket #bd-XX — <titre>
<rapport de review intégral copié tel quel — toutes sections>

#### Ticket #bd-YY — <titre>
<rapport de review intégral copié tel quel — toutes sections>

#### Ticket #bd-ZZ — <titre>
<rapport de review intégral copié tel quel — toutes sections>

### État de la session
**Tickets traités :** [bd-AA ✅, ...]
**En cours (batch) :** [bd-XX, bd-YY, bd-ZZ]
**Tickets restants :** [bd-WW, ...]
**task_id :** <task_id de la session en cours>
```

### Règles de production (orchestrator-dev)

- **Condition de déclenchement :** N ≥ 2 tickets atteignent CP-2 simultanément, ET tous les verdicts sont `commit`
- **Maximum :** 5 tickets par batch — au-delà (N > 5), ne pas proposer le batch et revenir en mode séquentiel intégral (trop de tickets pour une décision groupée)
- **Titres courts :** tronquer les titres à 40 caractères dans le tableau récapitulatif
- **Rapports complets :** inclure tous les rapports intégralement dans `### Rapports de review complets`
  — ils ne sont pas affichés par défaut mais doivent être disponibles si l'utilisateur demande les détails
- **Si un verdict n'est pas `commit`** : ne pas produire ce bloc — éclater le batch et revenir au mode séquentiel

### Différences avec le format unitaire

| Aspect | Unitaire (`## Question pour l'orchestrator`) | Batch (`## Question batch pour l'orchestrator`) |
|--------|----------------------------------------------|------------------------------------------------|
| Déclencheur | 1 ticket atteint CP-2 | N tickets (2-5) atteignent CP-2 avec verdict `commit` |
| Rapport affiché | Par défaut — dans `### Rapport de review complet` | Sur demande — dans `### Rapports de review complets` via "Voir détails" |
| Options | Commit / Corriger | Commit tous / Commit sélectif / Voir détails |
| Verdict `corriger` possible | Oui | Non — présence d'un `corriger` force l'éclatement |

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

- **Retranscrire les champs du bloc de manière formatée** dans le fil de discussion — tous les champs sont affichés lisiblement à l'utilisateur (tableau `### Détail par ticket`, `### Contexte et décisions par ticket`, `### Points d'attention globaux`).
- Ce format structuré est requis pour construire le CP-feature.
- Si le bloc ne contient pas les champs requis → les demander explicitement à `orchestrator-dev` avant de continuer.
- Ne jamais construire le CP-feature à partir d'un bloc incomplet.

### À la réception d'un `## Question pour l'orchestrator` (unitaire)
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

### À la réception d'un `## Question batch pour l'orchestrator` (CP-2 groupé)

Ce format est produit par `orchestrator-dev` quand **N tickets atteignent CP-2 simultanément** avec tous les verdicts = `commit` (voir section "CP-2 en batch conditionnel" dans `orchestrator-dev-protocol`).

1. **Afficher le tableau récapitulatif** dans le texte de la discussion :
   - Le tableau `### Récapitulatif du batch` résume les N tickets (ID, titre, agent, verdict, cycles de review)
   - Ne pas inclure ce tableau dans l'outil `question` — il doit être visible avant que l'utilisateur réponde

2. **Poser la question via l'outil `question`** en reprenant exactement les options du bloc :
   - Le champ `question` doit commencer par : `[OrchestratorDev — CP-2 Batch | N tickets]\n<question>`

3. **Traiter la réponse selon l'option choisie :**

   | Réponse | Action |
   |---------|--------|
   | `Commit tous` | Ré-invoquer `orchestrator-dev` avec `task_id` et la réponse : `"Réponse de l'utilisateur au CP-2 Batch : Commit tous. Commiter les <N> tickets en séquence."` |
   | `Commit sélectif` | Afficher la liste des tickets du batch, puis poser une question avec `multiple: true` pour sélectionner les tickets à commiter. Ré-invoquer avec la liste des IDs sélectionnés. |
   | `Voir détails` | Afficher les `### Rapports de review complets` un par un (chaque rapport dans sa sous-section `#### Ticket #<ID>`). Après affichage, repasser en mode séquentiel — poser un CP-2 unitaire pour chaque ticket. |

4. **Règle de fallback :**
   - Si la réponse est une saisie libre (hors-liste), interpréter l'intention et redemander clarification si ambiguë.

5. **Ne jamais :**
   - Afficher tous les rapports de review complets par défaut (trop verbeux pour N > 2)
   - Résumer les rapports — les transmettre intégralement via "Voir détails"
   - Construire une réponse "Commit tous" automatique sans validation explicite
