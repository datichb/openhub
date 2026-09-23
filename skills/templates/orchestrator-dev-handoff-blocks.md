# Templates — Blocs de handoff orchestrator-dev → orchestrator

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

<!-- Obligatoire — voir shared/handoff-bloc-unique-rule -->
### Questions bloquantes

Aucune.

**Statut global :** `succès` | `partiel` | `bloqué`
```

---

## Format du bloc `## Question pour l'orchestrator`

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

---

## Format du bloc `## Question batch pour l'orchestrator`

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

---

## Format du bloc `## Question CP pour l'orchestrator` (mode `manuel` parallèle — FIFO)

```
---

## Question CP pour l'orchestrator

**Agent :** orchestrator-dev
**Phase :** <CP-1 | CP-3>
**Ticket :** #<ID> — <titre>
**Domaine :** developer (<domaine>)

### Contexte

<Résumé court de l'état du ticket à ce CP — ce que le developer a fait ou va faire>

### Question en attente

<La question du CP — ex: "Lancer l'implémentation ?" pour CP-1, "Clore le ticket ?" pour CP-3>

### Options disponibles
- `Continuer` : Valider ce CP et poursuivre
- `Détails` : Afficher plus d'informations avant de décider
- `Arrêter ce ticket` : Suspendre le traitement de ce ticket

### File d'attente CP
<NB_EN_ATTENTE> CP en file : <liste — ex: #bd-43 CP-3, #bd-44 CP-1>

### État de la session
**task_id :** <task_id de la session en cours>
```
