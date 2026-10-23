> [Read in English](mcp-gitlab.en.md)

# Référence du serveur MCP GitLab

Nom du serveur : `gitlab-mcp` | Version : `2.1.0`

---

## Activation

```toml
[mcp.gitlab]
enabled = true
env = { GITLAB_TOKEN = "glpat-...", GITLAB_URL = "https://gitlab.example.com" }
```

Les modifications de `hub.toml` sont prises en compte au prochain lancement de session (paquet reconstruit) — aucun redéploiement nécessaire.

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
| Pagination | `page` / `per_page` sur les outils de liste (20 par page par défaut, 100 au plus) ; le résultat donne `total`, `total_pages` et `next_page` |
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

### `gitlab_get_issue`

Lit une issue par son IID (le numéro affiché `#623`) : l'outil à utiliser quand un ticket est donné en `#<iid>` ou par l'URL d'une issue.

| Paramètre | Type | Requis | Description |
|-----------|------|--------|-------------|
| `project_id` | string | Oui | ID du projet ou chemin (`groupe/projet`) |
| `iid` | integer | Oui | IID de l'issue : le numéro après `#` (`"623"` et `"#623"` sont acceptés aussi) |
| `include_notes` | boolean | Non | Inclure les commentaires des utilisateurs (par défaut : `true`) |
| `notes_limit` | integer | Non | Nombre maximal de commentaires, du plus ancien au plus récent (par défaut : 20, max : 100) |

**API :** `GET /api/v4/projects/{project_id}/issues/{iid}`, puis `…/notes`, `…/links` et `…/related_merge_requests`.

**Résultat** (JSON) : `iid`, `title`, `state`, `type`, `description`, `labels`, `assignees` (`username`, `name`), `author`, `milestone` (`title`, `due_date`, `state`), `reference`, `web_url`, `confidential`, `created_at`, `updated_at`, `closed_at`, `due_date`, `notes` (auteur, contenu, date ; notes système exclues) avec `notes_truncated`, `related_issues` et `related_merge_requests`.

**Un échec est dit, jamais silencieux :** les commentaires et les liens sont lus à côté de l'issue ; si l'un de ces appels échoue, l'issue est quand même renvoyée, avec `notes_error` ou `links_error`. Un IID inconnu, ou un projet que le jeton ne voit pas, donne une erreur qui nomme l'issue et le projet.

---

### `gitlab_list_issues`

Liste les issues d'un projet, une page à la fois. Chaque issue est un résumé (`iid`, `title`, `state`, `labels`, `assignees`, `author`, `milestone`, `web_url`, `updated_at`) : la description se lit avec `gitlab_get_issue`.

| Paramètre | Type | Requis | Description |
|-----------|------|--------|-------------|
| `project_id` | string | Oui | ID du projet ou chemin (`groupe/projet`) |
| `iids` | array of integer | Non | Seulement ces IID d'issues (les numéros après `#`) |
| `search` | string | Non | Recherche dans le titre et la description |
| `assignee_username` | string | Non | Seulement les issues assignées à cet utilisateur |
| `labels` | string | Non | Labels séparés par des virgules ; l'issue doit les avoir tous |
| `state` | string | Non | Filtrer par état : `opened`, `closed`, `all` |
| `page` | integer | Non | Numéro de page, à partir de 1 (par défaut : 1) |
| `per_page` | integer | Non | Issues par page (par défaut : 20, max : 100) |

**API :** `GET /api/v4/projects/{project_id}/issues`

**Résultat :** `{ "issues": [...], "page", "per_page", "total", "total_pages", "next_page" }`. `next_page` vaut `null` sur la dernière page ; `total` et `total_pages` valent `null` au-delà de 10 000 résultats (GitLab ne les compte pas).

---

### `gitlab_get_mr`

Lit une merge request par son IID (le numéro affiché `!12`).

| Paramètre | Type | Requis | Description |
|-----------|------|--------|-------------|
| `project_id` | string | Oui | ID du projet ou chemin (`groupe/projet`) |
| `mr_iid` | integer | Oui | IID de la merge request : le numéro après `!` |

**API :** `GET /api/v4/projects/{project_id}/merge_requests/{mr_iid}`, puis `…/closes_issues`.

**Résultat** (JSON) : `iid`, `title`, `description`, `state`, `draft`, `source_branch`, `target_branch`, `sha`, `author`, `assignees`, `reviewers`, `labels`, `milestone`, `merge_status`, `has_conflicts`, `web_url`, dates, `closes_issues` (et `closes_issues_error` si cet appel a échoué). Les fils de revue se lisent avec `gitlab_list_mr_discussions`.

---

### `gitlab_list_mrs`

Liste les merge requests d'un projet, une page à la fois (résumés : on en lit une avec `gitlab_get_mr`).

| Paramètre | Type | Requis | Description |
|-----------|------|--------|-------------|
| `project_id` | string | Oui | ID du projet ou chemin (`groupe/projet`) |
| `iids` | array of integer | Non | Seulement ces IID de merge requests (les numéros après `!`) |
| `search` | string | Non | Recherche dans le titre et la description |
| `source_branch` | string | Non | Seulement les merge requests de cette branche |
| `state` | string | Non | Filtrer par état : `opened`, `merged`, `closed`, `all` |
| `page` | integer | Non | Numéro de page, à partir de 1 (par défaut : 1) |
| `per_page` | integer | Non | Merge requests par page (par défaut : 20, max : 100) |

**API :** `GET /api/v4/projects/{project_id}/merge_requests`

**Résultat :** `{ "merge_requests": [...], "page", "per_page", "total", "total_pages", "next_page" }` (mêmes règles que `gitlab_list_issues`).

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
| 2 | `gitlab_get_issue` | Lecture | GET | project_id, iid, include_notes, notes_limit |
| 3 | `gitlab_list_issues` | Lecture | GET | project_id, iids, search, assignee_username, labels, state, page, per_page |
| 4 | `gitlab_get_mr` | Lecture | GET | project_id, mr_iid |
| 5 | `gitlab_list_mrs` | Lecture | GET | project_id, iids, search, source_branch, state, page, per_page |
| 6 | `gitlab_list_mr_discussions` | Lecture | GET | project_id, mr_iid, unresolved_only |
| 7 | `gitlab_get_mr_approvals` | Lecture | GET | project_id, mr_iid |
| 8 | `gitlab_create_mr` | Écriture | POST | project_id, source_branch, title |
| 9 | `gitlab_add_mr_note` | Écriture | POST | project_id, mr_iid, body |
| 10 | `gitlab_update_issue` | Écriture | PUT | project_id, issue_iid |
| 11 | `gitlab_assign_reviewer` | Écriture | PUT | project_id, mr_iid, reviewer_ids |
| 12 | `gitlab_add_label` | Écriture | PUT | project_id, issue_iid, labels |
| 13 | `gitlab_reply_to_mr_discussion` | Écriture | POST | project_id, mr_iid, discussion_id, body |

---

## Voir aussi

- [Guide d'intégration GitLab](../guides/gitlab-integration.fr.md)
- [Référence du serveur MCP Team](mcp-team.fr.md)
- [Référence du serveur MCP GitHub](mcp-github.fr.md)
- [Guide de revue et feedback](../guides/review-feedback.fr.md)
