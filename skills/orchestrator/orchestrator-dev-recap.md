---
name: orchestrator-dev-recap
description: Récap global et retour vers orchestrator — format du récap d'implémentation, bloc retour structuré, métriques de vélocité, état de session.
---

## Récap global — Fin de session

**Deux étapes obligatoires dans cet ordre — ne jamais les inverser, ne jamais en omettre une.**

### Étape 1 — Récap global complet (texte)

Afficher en fin de workflow (tous les tickets traités ou suite à un **stop**).
Construire ce récap en agrégeant les données structurées collectées à chaque étape 6 :

```
## Récap implémentation — <nom de la feature ou session>

### Synthèse par ticket

<Pour chaque ticket traité, inclure la synthèse structurée issue du bloc handoff developer-* et du compte rendu d'étape (étape 6).>
<"Aucun ticket traité" si la session est interrompue avant toute implémentation>

#### Ticket #bd-XX — <titre>
**Statut :** `implémenté` | `partiellement-implémenté` | `bloqué`
**Agent :** developer (<domaine>)
**Fichiers clés :** <1-3 fichiers les plus significatifs modifiés>
**Critères couverts :** tous | partielle — <critères non couverts>
**Points d'attention :** <liste issue du ### Points d'attention pour la review du developer — ou "Aucun">

#### Ticket #bd-YY — <titre>
**Statut :** ...
**Agent :** ...
**Fichiers clés :** ...
**Critères couverts :** ...
**Points d'attention :** ...

### Points d'attention globaux
<Agrégation des points d'attention techniques collectés à chaque étape 6 :
 - Points signalés par les developer-* (décisions techniques, compromis, dette)
 - Points récurrents signalés par le reviewer sur plusieurs tickets>
<"Aucun point d'attention" si aucun point n'a été signalé en cours de session>
```

> ❌ Ne jamais omettre ce récap — il contient la synthèse par ticket et les points d'attention. Le tableau de synthèse des tickets et les statistiques sont dans le bloc structuré `## Retour vers orchestrator` qui suit.

### Étape 2 — Bloc de retour structuré (obligatoire si invoqué depuis l'agent orchestrator feature)

> ⚠️ Ce bloc est **requis sans exception** — y compris en cas de stop, de ticket bloqué ou de session incomplète.
> **Champ `Type de récap` obligatoire :** renseigner `**Type de récap :** final` — ce bloc est émis seul en fin de session, tous les tickets ont été traités ou stop demandé.
> Il vient **après** le récap global complet — il en est le résumé structuré, il ne le remplace pas.
> Ne jamais clore la session sans avoir produit les deux.

Ajouter immédiatement après le récap global le bloc `## Retour vers orchestrator` :

```
---

## Retour vers orchestrator

**Tickets traités :** [bd-XX ✅, bd-YY ✅, ...]
**Tickets ignorés :** [bd-ZZ ⏭️, ...]

### Détail par ticket
| ID | Agent (domaine) | Cycles review | Critères couverts | Statut |
|----|----------------|---------------|-------------------|--------|
| bd-XX | developer (frontend) | 1 | tous | ✅ Terminé |
| bd-YY | developer (backend)  | 2 | partielle | ✅ Terminé |
| bd-ZZ | developer (api)      | — | — | ⏭️ Ignoré  |

**Points d'attention :**
- <agrégation des points d'attention techniques collectés à chaque étape 6>
**Statut global :** succès | partiel | bloqué
```

Le format exact, les champs obligatoires et les définitions des statuts (`succès`, `partiel`, `bloqué`) sont définis dans le skill `orchestrator-handoff-format` — s'y référer comme source de vérité unique.

> Les `### Points d'attention` doivent reprendre l'agrégation ci-dessus — jamais une liste vide si des points ont été signalés en cours de session.

**Vérification obligatoire avant de clore la session :**
> « Suis-je invoqué depuis l'agent orchestrator feature ? Si oui, ai-je produit le bloc `## Retour vers orchestrator` avec tous les champs requis (dont `### Contexte et décisions par ticket` et `### Points d'attention globaux`) ? Si non, le produire maintenant. Aucun texte libre avant ou après le bloc. »

---

## Métriques de vélocité — Points d'intégration

Les événements du workflow sont loggés dans `.opencode/metrics.jsonl` pour permettre l'analyse de la vélocité.

Les fonctions de logging sont définies dans `scripts/lib/metrics.sh` :

| Fonction | Usage | Quand l'appeler |
|----------|-------|-----------------|
| `metrics_start_timer <ticket_id>` | Démarre le chrono d'un ticket | CP-1 — après validation du démarrage |
| `metrics_ticket_start <ticket_id> [agent]` | Log l'événement de démarrage | CP-1 — après validation du démarrage |
| `metrics_review_cycle <ticket_id> [cycle_number]` | Log un cycle de review | Étape 4 — à chaque soumission au reviewer |
| `metrics_correction <ticket_id> [reason]` | Log une correction demandée | CP-2 — si l'option "Corriger" est choisie |
| `metrics_get_duration <ticket_id>` | Récupère la durée depuis le start | Étape 6 — pour calculer la durée totale |
| `metrics_ticket_complete <ticket_id> [agent] [duration]` | Log la complétion du ticket | Étape 6 — après clôture du ticket Beads |
| `metrics_clear_timer <ticket_id>` | Nettoie le timer (optionnel) | Étape 6 — après ticket_complete |

### Séquence d'appels typique

```
# CP-1 — Démarrage du ticket
metrics_start_timer "bd-42"
metrics_ticket_start "bd-42" "developer"

# Étape 4 — Premier passage en review
metrics_review_cycle "bd-42" 1

# CP-2 — Correction demandée
metrics_correction "bd-42" "lint errors"

# Étape 4 — Deuxième passage en review
metrics_review_cycle "bd-42" 2

# Étape 6 — Ticket terminé
duration=$(metrics_get_duration "bd-42")
metrics_ticket_complete "bd-42" "developer" "$duration"
metrics_clear_timer "bd-42"
```

### Format des événements loggés

Chaque événement est une ligne JSON dans `.opencode/metrics.jsonl` :

```json
{"timestamp":"2024-01-15T10:30:00Z","event":"ticket_start","ticket_id":"bd-42","agent":"developer","domain":"backend"}
{"timestamp":"2024-01-15T10:35:00Z","event":"review_cycle","ticket_id":"bd-42","cycle":1}
{"timestamp":"2024-01-15T10:40:00Z","event":"correction","ticket_id":"bd-42","reason":"lint errors"}
{"timestamp":"2024-01-15T10:42:00Z","event":"review_cycle","ticket_id":"bd-42","cycle":2}
{"timestamp":"2024-01-15T10:45:00Z","event":"ticket_complete","ticket_id":"bd-42","agent":"developer","domain":"backend","duration_seconds":900}
```

> **Note :** Ces fonctions sont destinées à être appelées par les agents orchestrateurs qui pilotent le workflow. Les agents `developer-*` n'appellent pas directement les fonctions de métriques — c'est l'agent orchestrator qui trace les événements.

---

## État de session — Points d'intégration (Dashboard TUI)

L'état de session permet au dashboard TUI (`oc dashboard`) d'afficher l'avancement en temps réel.
L'état est stocké dans `.opencode/session-state.json`.

Les fonctions de gestion sont définies dans `scripts/lib/session-state.sh` :

| Fonction | Usage | Quand l'appeler |
|----------|-------|-----------------|
| `session_state_init <session_id> <mode>` | Initialise l'état de session | CP-0 — après choix du mode |
| `session_state_add_ticket <id> <title>` | Ajoute un ticket à la session | CP-0 — pour chaque ticket à traiter |
| `session_state_update_ticket <id> <status>` | Met à jour le statut d'un ticket | CP-1, Étape 6 — transitions de statut |
| `session_state_set_current <id> <agent> <action>` | Définit le ticket en cours | CP-1, Étapes 3/4/5 — changement d'action |
| `session_state_clear_current` | Efface le ticket en cours | Étape 6 — entre deux tickets |
| `session_state_end` | Termine la session | Fin de session — supprime l'état |
| `session_state_read` | Lit l'état JSON | Dashboard — pour afficher l'état |
| `session_state_is_active` | Vérifie si une session est active | Dashboard — pour décider de l'affichage |

### Séquence d'appels typique

```
# CP-0 — Initialisation
session_state_init "ses_$(date +%s)" "semi-auto"
session_state_add_ticket "bd-42" "Fix null guard"
session_state_add_ticket "bd-43" "Add tests"

# CP-1 — Démarrage d'un ticket
session_state_update_ticket "bd-42" "in_progress"
session_state_set_current "bd-42" "developer" "implementing"

# Étape 4 — Passage en review
session_state_set_current "bd-42" "developer" "reviewing"

# Étape 5 — CP-2
session_state_set_current "bd-42" "developer" "waiting_cp2"

# Étape 6 — Ticket terminé
session_state_update_ticket "bd-42" "completed"
session_state_clear_current

# Fin de session
session_state_end
```

### Valeurs de statut

| Statut | Description | Emoji dashboard |
|--------|-------------|-----------------|
| `pending` | En attente de traitement | ⏳ |
| `in_progress` | En cours d'implémentation | 🔄 |
| `review` | En attente de review | 👁️ |
| `completed` | Terminé et clos | ✅ |
| `blocked` | Bloqué | 🚫 |

### Valeurs d'action

| Action | Description |
|--------|-------------|
| `implementing` | Implémentation en cours par le developer |
| `reviewing` | Review en cours par le reviewer |
| `waiting_cp2` | En attente de décision CP-2 |
| `idle` | Pas d'action en cours |

> **Note :** Le format complet de l'état JSON est défini dans `skills/orchestrator/session-state-protocol.md`.
