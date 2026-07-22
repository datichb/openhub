> [Read in English](mcp-github.en.md)

# Référence du serveur MCP GitHub

## Activation

Le serveur MCP GitHub est déployé automatiquement lorsque `[github].enabled = true` dans `hub.toml`.

```json
// Injecté dans opencode.json par oh deploy
{
  "mcpServers": {
    "github": {
      "command": "oh",
      "args": ["mcp", "serve", "github"]
    }
  }
}
```

## Authentification

Définir la variable d'environnement `GITHUB_TOKEN` avec un personal access token (PAT) ou un token GitHub App.

```bash
export GITHUB_TOKEN=ghp_xxxxxxxxxxxxxxxxxxxx
```

Scopes requis :
- `repo` — accès en lecture aux dépôts, issues, pull requests
- `workflow` — accès en lecture aux workflows et runs GitHub Actions

Le mode écriture nécessite le scope supplémentaire : `repo` (écriture)

## Outils disponibles

| Outil | Description | Accès |
|-------|-------------|-------|
| `github_get_repo` | Récupérer les métadonnées d'un dépôt | Tous les agents |
| `github_list_issues` | Lister les issues avec filtres | Tous les agents |
| `github_get_issue` | Récupérer une issue par son numéro | Tous les agents |
| `github_list_prs` | Lister les pull requests avec filtres | Tous les agents |
| `github_get_pr` | Récupérer une PR avec ses reviews et check runs | Tous les agents |
| `github_list_workflows` | Lister les workflows d'un dépôt | Tous les agents |
| `github_get_workflow_run` | Récupérer un run de workflow spécifique | Tous les agents |

---

### `github_get_repo`

Récupérer les métadonnées d'un dépôt GitHub.

**Paramètres obligatoires :**
- `repo` (string) — dépôt au format `owner/name`

**Paramètres optionnels :** aucun

**Exemple d'appel :**
```json
{
  "repo": "acme/backend"
}
```

**Exemple de réponse :**
```json
{
  "id": 123456,
  "full_name": "acme/backend",
  "description": "Service backend principal",
  "default_branch": "main",
  "private": true,
  "open_issues_count": 14,
  "stargazers_count": 0,
  "language": "Go",
  "topics": ["backend", "api"],
  "updated_at": "2026-07-20T10:00:00Z"
}
```

---

### `github_list_issues`

Lister les issues d'un dépôt avec des filtres optionnels.

**Paramètres obligatoires :**
- `repo` (string) — dépôt au format `owner/name`

**Paramètres optionnels :**
- `state` (string) — `open` | `closed` | `all` (défaut : `open`)
- `labels` (string) — noms de labels séparés par des virgules
- `assignee` (string) — nom d'utilisateur GitHub, ou `none`, ou `*`

**Exemple d'appel :**
```json
{
  "repo": "acme/backend",
  "state": "open",
  "labels": "bug,high-priority",
  "assignee": "jdoe"
}
```

**Exemple de réponse :**
```json
[
  {
    "number": 42,
    "title": "Nil pointer dans le middleware auth",
    "state": "open",
    "labels": [{"name": "bug"}, {"name": "high-priority"}],
    "assignees": [{"login": "jdoe"}],
    "created_at": "2026-07-15T08:30:00Z",
    "updated_at": "2026-07-20T11:00:00Z",
    "html_url": "https://github.com/acme/backend/issues/42"
  }
]
```

---

### `github_get_issue`

Récupérer une issue par son numéro.

**Paramètres obligatoires :**
- `repo` (string) — dépôt au format `owner/name`
- `issue_number` (integer) — numéro de l'issue

**Paramètres optionnels :** aucun

**Exemple d'appel :**
```json
{
  "repo": "acme/backend",
  "issue_number": 42
}
```

**Exemple de réponse :**
```json
{
  "number": 42,
  "title": "Nil pointer dans le middleware auth",
  "state": "open",
  "body": "Reproductible avec la commande curl suivante...",
  "labels": [{"name": "bug"}],
  "assignees": [{"login": "jdoe"}],
  "comments": 3,
  "created_at": "2026-07-15T08:30:00Z",
  "html_url": "https://github.com/acme/backend/issues/42"
}
```

---

### `github_list_prs`

Lister les pull requests d'un dépôt.

**Paramètres obligatoires :**
- `repo` (string) — dépôt au format `owner/name`

**Paramètres optionnels :**
- `state` (string) — `open` | `closed` | `all` (défaut : `open`)
- `base` (string) — filtrer par nom de branche cible

**Exemple d'appel :**
```json
{
  "repo": "acme/backend",
  "state": "open",
  "base": "main"
}
```

**Exemple de réponse :**
```json
[
  {
    "number": 87,
    "title": "feat: add JWT refresh endpoint",
    "state": "open",
    "base": {"ref": "main"},
    "head": {"ref": "feat/SRU-99-jwt-refresh"},
    "draft": false,
    "created_at": "2026-07-19T14:00:00Z",
    "html_url": "https://github.com/acme/backend/pull/87"
  }
]
```

---

### `github_get_pr`

Récupérer une pull request avec ses reviews et statuts de check runs.

**Paramètres obligatoires :**
- `repo` (string) — dépôt au format `owner/name`
- `pr_number` (integer) — numéro de la pull request

**Paramètres optionnels :** aucun

**Exemple d'appel :**
```json
{
  "repo": "acme/backend",
  "pr_number": 87
}
```

**Exemple de réponse :**
```json
{
  "number": 87,
  "title": "feat: add JWT refresh endpoint",
  "state": "open",
  "mergeable": true,
  "reviews": [
    {
      "user": {"login": "alice"},
      "state": "APPROVED",
      "submitted_at": "2026-07-20T09:00:00Z"
    }
  ],
  "check_runs": [
    {
      "name": "ci/build",
      "status": "completed",
      "conclusion": "success",
      "started_at": "2026-07-20T08:50:00Z",
      "completed_at": "2026-07-20T08:55:00Z"
    }
  ]
}
```

---

### `github_list_workflows`

Lister tous les workflows GitHub Actions d'un dépôt.

**Paramètres obligatoires :**
- `repo` (string) — dépôt au format `owner/name`

**Paramètres optionnels :** aucun

**Exemple d'appel :**
```json
{
  "repo": "acme/backend"
}
```

**Exemple de réponse :**
```json
[
  {
    "id": 1001,
    "name": "CI",
    "path": ".github/workflows/ci.yml",
    "state": "active"
  },
  {
    "id": 1002,
    "name": "Release",
    "path": ".github/workflows/release.yml",
    "state": "active"
  }
]
```

---

### `github_get_workflow_run`

Récupérer les détails d'un run de workflow spécifique.

**Paramètres obligatoires :**
- `repo` (string) — dépôt au format `owner/name`
- `workflow_id` (integer) — ID du run de workflow (pas l'ID de la définition du workflow)

**Paramètres optionnels :** aucun

**Exemple d'appel :**
```json
{
  "repo": "acme/backend",
  "workflow_id": 9876543
}
```

**Exemple de réponse :**
```json
{
  "id": 9876543,
  "name": "CI",
  "status": "completed",
  "conclusion": "failure",
  "head_branch": "feat/SRU-99-jwt-refresh",
  "head_sha": "abc123def456",
  "event": "push",
  "run_started_at": "2026-07-20T08:50:00Z",
  "updated_at": "2026-07-20T08:58:00Z",
  "html_url": "https://github.com/acme/backend/actions/runs/9876543",
  "jobs_url": "https://api.github.com/repos/acme/backend/actions/runs/9876543/jobs"
}
```

---

## Mode écriture

Le mode écriture est désactivé par défaut. Pour l'activer :

```bash
export GITHUB_WRITE_ENABLED=true
```

### `github_create_issue`

Créer une nouvelle issue dans un dépôt.

**Paramètres obligatoires :**
- `repo` (string) — dépôt au format `owner/name`
- `title` (string) — titre de l'issue

**Paramètres optionnels :**
- `body` (string) — corps de l'issue (Markdown supporté)
- `labels` ([]string) — liste de noms de labels à appliquer

**Accès :** Agents avec mode écriture activé uniquement

**Exemple d'appel :**
```json
{
  "repo": "acme/backend",
  "title": "Ajouter le rate limiting sur /api/v2/auth",
  "body": "Actuellement l'endpoint n'a pas de rate limiting. Devrait être 100 req/min par IP.",
  "labels": ["enhancement", "security"]
}
```

**Exemple de réponse :**
```json
{
  "number": 91,
  "html_url": "https://github.com/acme/backend/issues/91",
  "state": "open",
  "created_at": "2026-07-22T10:00:00Z"
}
```

## Limites de taux

L'API REST GitHub impose les limites suivantes :

| Type de token | Requêtes/heure |
|---------------|----------------|
| Personal Access Token (PAT) | 5 000 |
| Token d'installation GitHub App | 15 000 |
| Non authentifié | 60 |

Le serveur MCP retourne des réponses HTTP 429 lorsque la limite est dépassée. Consulter l'en-tête `X-RateLimit-Reset` pour le timestamp de réinitialisation.

## Voir aussi

- [Référence MCP Team Server](mcp-team.md)
- [Référence MCP Jira Server](mcp-jira.fr.md)
- [Référence MCP Linear Server](mcp-linear.fr.md)
- [Documentation API REST GitHub](https://docs.github.com/en/rest)
