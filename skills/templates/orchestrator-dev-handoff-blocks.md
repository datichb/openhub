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
- <point 1 — agrégation des points signalés par developer et reviewer>
- <point 2>
<"Aucun point d'attention global" si tous les tickets sont propres>

### Données techniques brutes
<Diffs significatifs, résultats de tests globaux, informations nécessaires à l'orchestrator pour le récap de la feature — uniquement si pertinent>
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
**Phase :** <Ping-pong reviewer | Dépendance non résolue | Ticket bloqué | Agent manquant>

### Contexte complet
<contenu de contexte — synthèse, historique des cycles, raison du blocage, etc.>
<Ne jamais résumer ni abréger — tout le contenu doit être présent>

### Rapport de review complet
<Ping-pong reviewer uniquement : rapports de review des cycles concernés, copiés intégralement>
<Omettre cette section dans les autres cas (Dépendance non résolue, Ticket bloqué, Agent manquant)>

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
