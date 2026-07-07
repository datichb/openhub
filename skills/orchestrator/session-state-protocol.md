---
name: session-state-protocol
description: Protocole de gestion d'état de session pour le dashboard TUI — format JSON, valeurs possibles et points d'intégration avec le workflow orchestrator-dev.
---

# Skill — Protocole d'état de session

## Rôle

Ce skill définit le format et le protocole d'écriture de l'état de session utilisé par le dashboard TUI.
L'état permet de suivre en temps réel l'avancement d'une session orchestrée.

---

## Fichier d'état

**Chemin :** `.opencode/session-state.json`

Le fichier est créé au démarrage d'une session et supprimé (ou vidé) à la fin.
Un fichier absent ou vide signifie qu'aucune session n'est active.

---

## Format JSON

```json
{
  "session_id": "ses_abc123",
  "started_at": "2024-01-15T10:30:00Z",
  "mode": "semi-auto",
  "current_ticket": {
    "id": "bd-42",
    "title": "Fix null guard",
    "status": "in_progress",
    "agent": "developer",
    "domain": "backend",
    "action": "implementing"
  },
  "tickets": [
    { "id": "bd-42", "status": "in_progress", "title": "Fix null guard" },
    { "id": "bd-43", "status": "pending", "title": "Add tests" },
    { "id": "bd-44", "status": "completed", "title": "Update docs" }
  ],
  "last_update": "2024-01-15T10:45:00Z"
}
```

---

## Champs

| Champ | Type | Description |
|-------|------|-------------|
| `session_id` | string | Identifiant unique de la session (format `ses_<random>`) |
| `started_at` | string | Timestamp ISO8601 UTC du démarrage |
| `mode` | string | Mode de workflow : `manuel`, `semi-auto`, `auto` |
| `current_ticket` | object\|null | Ticket actuellement en cours de traitement (null si aucun) |
| `tickets` | array | Liste de tous les tickets de la session avec leur statut |
| `last_update` | string | Timestamp ISO8601 UTC de la dernière mise à jour |

### Objet `current_ticket`

| Champ | Type | Description |
|-------|------|-------------|
| `id` | string | ID du ticket Beads (ex: `bd-42`) |
| `title` | string | Titre court du ticket |
| `status` | string | Statut du ticket (voir valeurs ci-dessous) |
| `agent` | string | Agent délégué (toujours `developer`, `developer-refactor` ou `developer-migrator`) |
| `domain` | string | Domaine du ticket (ex: `backend`, `frontend`, `api`) — présent si agent = `developer` |
| `action` | string | Action en cours (voir valeurs ci-dessous) |

### Objet ticket dans `tickets[]`

| Champ | Type | Description |
|-------|------|-------------|
| `id` | string | ID du ticket Beads |
| `status` | string | Statut du ticket |
| `title` | string | Titre court du ticket |

---

## Valeurs possibles

### `status` (statut d'un ticket)

| Valeur | Description | Emoji dashboard |
|--------|-------------|-----------------|
| `pending` | En attente de traitement | ⏳ |
| `in_progress` | En cours d'implémentation | 🔄 |
| `review` | En attente de review | 👁️ |
| `completed` | Terminé et clos | ✅ |
| `blocked` | Bloqué (dépendance, décision) | 🚫 |

### `action` (action en cours sur `current_ticket`)

| Valeur | Description |
|--------|-------------|
| `implementing` | Implémentation en cours par le developer |
| `reviewing` | Review en cours par le reviewer |
| `waiting_cp2` | En attente de décision CP-2 (commit ou corriger) |
| `idle` | Pas d'action en cours (entre deux tickets) |

> **Note :** Le champ `agent` dans `current_ticket` représente l'agent **assigné au ticket** (celui qui a fait l'implémentation), pas nécessairement l'agent qui exécute l'action en cours. Par exemple, lors d'une action `reviewing`, l'`agent` reste `developer` (domaine `backend`) car c'est lui qui est assigné au ticket, même si c'est le reviewer qui effectue la review.

### `mode` (mode de workflow)

| Valeur | Description |
|--------|-------------|
| `manuel` | Pause à chaque CP — validation explicite |
| `semi-auto` | Pause uniquement à CP-2 |
| `auto` | Enchaînement automatique — CP-2 reste une pause |

---

## Points d'intégration avec orchestrator-dev

Les fonctions de `scripts/lib/session-state.sh` sont appelées aux moments suivants du workflow :

| Moment | Fonction | Usage |
|--------|----------|-------|
| CP-0 — Initialisation | `session_state_init` | Crée l'état avec session_id, mode, liste des tickets en `pending` |
| CP-0 — Après affichage tickets | `session_state_add_ticket` | Ajoute chaque ticket à la liste (si non fait dans init) |
| CP-1 — Démarrage ticket | `session_state_update_ticket` | Passe le ticket en `in_progress` |
| CP-1 — Démarrage ticket | `session_state_set_current` | Définit le ticket courant avec agent et action `implementing` |
| Étape 4 — Review | `session_state_set_current` | Met à jour l'action en `reviewing` |
| Étape 5 — CP-2 | `session_state_set_current` | Met à jour l'action en `waiting_cp2` |
| Étape 6 — Fin ticket | `session_state_update_ticket` | Passe le ticket en `completed` |
| Étape 6 — Fin ticket | `session_state_clear_current` | Efface le ticket courant |
| Fin de session | `session_state_end` | Supprime ou vide l'état |

---

## Exemple de séquence

```bash
# CP-0 — Démarrage de session
session_state_init "ses_$(date +%s)" "semi-auto"
session_state_add_ticket "bd-42" "Fix null guard"
session_state_add_ticket "bd-43" "Add tests"
session_state_add_ticket "bd-44" "Update docs"

# CP-1 — Premier ticket démarre
session_state_update_ticket "bd-42" "in_progress"
session_state_set_current "bd-42" "developer" "implementing"

# Étape 4 — Passage en review
session_state_set_current "bd-42" "developer" "reviewing"

# Étape 5 — CP-2
session_state_set_current "bd-42" "developer" "waiting_cp2"

# Étape 6 — Ticket terminé
session_state_update_ticket "bd-42" "completed"
session_state_clear_current

# CP-1 — Deuxième ticket
session_state_update_ticket "bd-43" "in_progress"
session_state_set_current "bd-43" "developer" "implementing"

# ... etc ...

# Fin de session
session_state_end
```

---

## Lecture de l'état

La commande `oc dashboard` utilise `session_state_read` pour obtenir l'état JSON et l'afficher.
Si le fichier n'existe pas ou est vide, le dashboard affiche "Aucune session active".

```bash
state_json=$(session_state_read)
if [ -z "$state_json" ] || [ "$state_json" = "null" ]; then
  echo "Aucune session active"
else
  # Parser et afficher l'état
fi
```

---

## Règles de cohérence

- **Un seul `current_ticket` à la fois** — sauf en mode parallèle (voir note ci-dessous)
- **L'ordre des tickets dans `tickets[]` est préservé** — ordre d'ajout initial
- **`last_update` est mis à jour à chaque écriture** — permet de détecter les sessions figées
- **Le fichier est atomique** — écriture dans un fichier temporaire puis rename

### Note sur le mode parallèle

En mode `auto` avec parallélisme conditionnel, plusieurs tickets peuvent être `in_progress` simultanément.
Dans ce cas, `current_ticket` reflète le dernier ticket mis à jour (le plus récent).
Le champ `tickets[]` reste la source de vérité pour voir tous les tickets en cours.
