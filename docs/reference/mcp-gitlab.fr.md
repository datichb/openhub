> [Read in English](mcp-gitlab.en.md)

# Référence du serveur MCP GitLab

Nom du serveur : `gitlab-mcp` | Version : `2.0.0`

---

## Activation

```toml
[mcp.gitlab]
enabled = true
env = { GITLAB_TOKEN = "glpat-...", GITLAB_URL = "https://gitlab.example.com" }
```

Après modification de `hub.toml`, exécutez `oh deploy` pour appliquer les changements.

---

## Authentification

| Variable | Requis | Description |
|----------|--------|-------------|
| `GITLAB_TOKEN` | Oui | Jeton d'accès personnel GitLab (en-tête : `PRIVATE-TOKEN`) |
| `GITLAB_URL` | Non | URL de l'instance GitLab (par défaut : `https://gitlab.com`) |

> **Sécurité :** Stockez les jetons dans le trousseau du système via `oh secrets set GITLAB_TOKEN` au lieu de les écrire dans `hub.toml`.

---

## Sécurité des URL

Toutes les requêtes API sont validées avant exécution :
- Le schéma doit être `https` (le HTTP en clair est rejeté)
- Le nom d'hôte ne doit PAS résoudre vers une IP privée (127.0.0.1, 10.x, 172.16.x, 192.168.x, 169.254.x)
- La résolution DNS est vérifiée pour les noms d'hôte non-IP

---

## Client HTTP

| Propriété | Valeur |
|-----------|--------|
| Timeout | 30 secondes |
| Taille max. de réponse | 50 Mo |
| Retry | Aucun |
| Pagination | Aucune |
| Journalisation | Journalisation HTTP structurée (`mcp.gitlab`), URLs de webhook masquées |

---

## Outils en lecture seule

Ces outils sont toujours disponibles lorsque le serveur MCP GitLab est activé.

### `gitlab_get_project`

Récupère un projet GitLab par ID ou chemin.

| Paramètre | Type | Requis | Description |
|-----------|------|--------|-------------|
| `project_id` | string | Oui | ID du projet ou chemin encodé en URL |

**API :** `GET /api/v4/projects/{project_id}`

---

### `gitlab_list_issues`

Liste les issues d'un projet.

| Paramètre | Type | Requis | Description |
|-----------|------|--------|-------------|
| `project_id` | string | Oui | ID du projet |
| `state` | string | Non | Filtrer par état : `opened`, `closed`, `all` |

**API :** `GET /api/v4/projects/{project_id}/issues`

---

### `gitlab_list_mrs`

Liste les merge requests d'un projet.

| Paramètre | Type | Requis | Description |
|-----------|------|--------|-------------|
| `project_id` | string | Oui | ID du projet |
| `state` | string | Non | Filtrer par état : `opened`, `merged`, `closed`, `all` |

**API :** `GET /api/v4/projects/{project_id}/merge_requests`

---

### `gitlab_list_mr_discussions`

Liste les fils de discussion d'une merge request. Retourne les commentaires de code en ligne et les discussions générales avec auteur, contenu, statut de résolution et position dans le fichier. Les notes système sont exclues.

| Paramètre | Type | Requis | Description |
|-----------|------|--------|-------------|
| `project_id` | string | Oui | ID du projet ou chemin encodé en URL |
| `mr_iid` | integer | Oui | IID de la merge request (ID interne) |
| `unresolved_only` | boolean | Non | Ne retourner que les discussions non résolues (par défaut : `true`) |

**API :** `GET /api/v4/projects/{project_id}/merge_requests/{mr_iid}/discussions`

**Comportement :**
1. Récupère toutes les discussions depuis l'API
2. Filtre les discussions uniquement système (ne conserve que celles avec des notes humaines)
3. Si `unresolved_only` est vrai (par défaut), ne conserve que les discussions ayant au moins une note non résolue

---

### `gitlab_get_mr_approvals`

Récupère le statut d'approbation d'une merge request. Retourne qui a approuvé, combien d'approbations sont requises et combien il en reste.

| Paramètre | Type | Requis | Description |
|-----------|------|--------|-------------|
| `project_id` | string | Oui | ID du projet ou chemin encodé en URL |
| `mr_iid` | integer | Oui | IID de la merge request (ID interne) |

**API :** `GET /api/v4/projects/{project_id}/merge_requests/{mr_iid}/approvals`

> **Note :** Nécessite GitLab Premium ou Ultimate. Sur GitLab Free, retourne un message explicite au lieu d'une erreur.

---

## Outils en écriture

Ces outils ne sont disponibles que lorsque `GITLAB_WRITE_ENABLED=true` est défini. Ils sont invisibles pour les agents dans le cas contraire.

```toml
[mcp.gitlab]
enabled = true
env = { GITLAB_TOKEN = "glpat-...", GITLAB_WRITE_ENABLED = "true" }
```

### `gitlab_create_mr`

Crée une merge request. Vérifie automatiquement si une MR existe déjà pour la branche source avant de la créer.

| Paramètre | Type | Requis | Description |
|-----------|------|--------|-------------|
| `project_id` | string | Oui | ID du projet ou chemin encodé en URL |
| `source_branch` | string | Oui | Nom de la branche source |
| `target_branch` | string | Non | Branche cible (par défaut : `main`) |
| `title` | string | Oui | Titre de la MR |
| `description` | string | Non | Description de la MR (markdown) |

**API :** `POST /api/v4/projects/{project_id}/merge_requests`

**Détection de doublons :** Avant la création, vérifie l'existence de MR ouvertes sur la même branche source. Si une MR existe déjà, la retourne au lieu de créer un doublon.

---

### `gitlab_add_mr_note`

Ajoute un commentaire/note à une merge request.

| Paramètre | Type | Requis | Description |
|-----------|------|--------|-------------|
| `project_id` | string | Oui | ID du projet |
| `mr_iid` | integer | Oui | ID interne de la MR |
| `body` | string | Oui | Corps du commentaire (markdown) |

**API :** `POST /api/v4/projects/{project_id}/merge_requests/{mr_iid}/notes`

---

### `gitlab_update_issue`

Met à jour une issue (labels, assignés, état).

| Paramètre | Type | Requis | Description |
|-----------|------|--------|-------------|
| `project_id` | string | Oui | ID du projet |
| `issue_iid` | integer | Oui | ID interne de l'issue |
| `state_event` | string | Non | Transition d'état : `reopen` ou `close` |
| `add_labels` | string | Non | Labels à ajouter, séparés par des virgules |
| `assignee_ids` | array of integer | Non | IDs des utilisateurs à assigner |

**API :** `PUT /api/v4/projects/{project_id}/issues/{issue_iid}`

Seuls les champs non vides sont inclus dans la requête. Vous pouvez mettre à jour n'importe quelle combinaison en un seul appel.

---

### `gitlab_assign_reviewer`

Assigne un ou plusieurs relecteurs à une merge request.

| Paramètre | Type | Requis | Description |
|-----------|------|--------|-------------|
| `project_id` | string | Oui | ID du projet |
| `mr_iid` | integer | Oui | ID interne de la MR |
| `reviewer_ids` | array of integer | Oui | IDs des utilisateurs à assigner comme relecteurs |

**API :** `PUT /api/v4/projects/{project_id}/merge_requests/{mr_iid}`

---

### `gitlab_add_label`

Ajoute des labels à une issue (préserve les labels existants).

| Paramètre | Type | Requis | Description |
|-----------|------|--------|-------------|
| `project_id` | string | Oui | ID du projet |
| `issue_iid` | integer | Oui | ID interne de l'issue |
| `labels` | string | Oui | Labels à ajouter, séparés par des virgules |

**API :** `PUT /api/v4/projects/{project_id}/issues/{issue_iid}`

---

### `gitlab_reply_to_mr_discussion`

Répond à un fil de discussion spécifique sur une merge request. Utilisez cet outil pour répondre aux commentaires de relecteurs après avoir appliqué les corrections.

| Paramètre | Type | Requis | Description |
|-----------|------|--------|-------------|
| `project_id` | string | Oui | ID du projet ou chemin encodé en URL |
| `mr_iid` | integer | Oui | ID interne de la MR |
| `discussion_id` | string | Oui | ID du fil de discussion (depuis `gitlab_list_mr_discussions`) |
| `body` | string | Oui | Corps de la réponse (markdown) |

**API :** `POST /api/v4/projects/{project_id}/merge_requests/{mr_iid}/discussions/{discussion_id}/notes`

---

## Résumé des outils

| # | Outil | Mode | HTTP | Paramètres clés |
|---|-------|------|------|-----------------|
| 1 | `gitlab_get_project` | Lecture | GET | project_id |
| 2 | `gitlab_list_issues` | Lecture | GET | project_id, state |
| 3 | `gitlab_list_mrs` | Lecture | GET | project_id, state |
| 4 | `gitlab_list_mr_discussions` | Lecture | GET | project_id, mr_iid, unresolved_only |
| 5 | `gitlab_get_mr_approvals` | Lecture | GET | project_id, mr_iid |
| 6 | `gitlab_create_mr` | Écriture | POST | project_id, source_branch, title |
| 7 | `gitlab_add_mr_note` | Écriture | POST | project_id, mr_iid, body |
| 8 | `gitlab_update_issue` | Écriture | PUT | project_id, issue_iid |
| 9 | `gitlab_assign_reviewer` | Écriture | PUT | project_id, mr_iid, reviewer_ids |
| 10 | `gitlab_add_label` | Écriture | PUT | project_id, issue_iid, labels |
| 11 | `gitlab_reply_to_mr_discussion` | Écriture | POST | project_id, mr_iid, discussion_id, body |

---

## Voir aussi

- [Guide d'intégration GitLab](../guides/gitlab-integration.fr.md)
- [Référence du serveur MCP Team](mcp-team.fr.md)
- [Référence du serveur MCP GitHub](mcp-github.fr.md)
- [Guide de revue et feedback](../guides/review-feedback.fr.md)
