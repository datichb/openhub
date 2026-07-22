> [Read in English](mcp-jira.en.md)

# Référence du serveur MCP Jira

## Activation

Le serveur MCP Jira est déployé automatiquement lorsque `[jira].enabled = true` dans `hub.toml`.

```json
// Injecté dans opencode.json par oh deploy
{
  "mcpServers": {
    "jira": {
      "command": "oh",
      "args": ["mcp", "serve", "jira"]
    }
  }
}
```

## Authentification

Deux méthodes d'authentification sont supportées :

**Token Bearer (recommandé) :**
```bash
export JIRA_TOKEN=votre_personal_access_token
```

**Authentification Basic (Atlassian Cloud) :**
```bash
export JIRA_USER=utilisateur@entreprise.com
export JIRA_API_TOKEN=votre_api_token
```

**Configuration obligatoire :**
```bash
export JIRA_URL=https://entreprise.atlassian.net
```

La variable `JIRA_URL` est obligatoire. Elle doit pointer vers la racine de votre instance Jira (sans slash final).

## Outils disponibles

| Outil | Description | Accès |
|-------|-------------|-------|
| `jira_list_issues` | Recherche d'issues par JQL | Tous les agents |
| `jira_get_issue` | Récupérer une issue par clé (ex. PROJ-123) | Tous les agents |
| `jira_get_project` | Récupérer les métadonnées d'un projet | Tous les agents |

---

### `jira_list_issues`

Rechercher des issues Jira à l'aide d'une requête JQL.

**Paramètres obligatoires :**
- `jql` (string) — requête JQL

**Paramètres optionnels :**
- `max_results` (integer) — nombre maximum de résultats à retourner (défaut : 50, max : 100)

**Exemple d'appel :**
```json
{
  "jql": "project = PROJ AND status = 'In Progress' AND assignee = currentUser()",
  "max_results": 20
}
```

**Exemple de réponse :**
```json
{
  "total": 3,
  "issues": [
    {
      "id": "10042",
      "key": "PROJ-123",
      "summary": "Implémenter le flux de connexion OAuth2",
      "status": "In Progress",
      "priority": "High",
      "assignee": {"displayName": "John Doe", "emailAddress": "jdoe@entreprise.com"},
      "reporter": {"displayName": "Jane Smith"},
      "labels": ["auth", "backend"],
      "created": "2026-07-10T09:00:00.000+0000",
      "updated": "2026-07-20T14:30:00.000+0000"
    }
  ]
}
```

**Exemples de requêtes JQL courantes :**
```
project = PROJ AND sprint in openSprints()
status != Done AND priority = High
labels = "needs-review" ORDER BY updated DESC
assignee = "jdoe" AND created >= -7d
```

---

### `jira_get_issue`

Récupérer une issue Jira par sa clé.

**Paramètres obligatoires :**
- `issue_key` (string) — clé de l'issue au format `PROJET-NUMÉRO` (ex. `PROJ-123`)

**Paramètres optionnels :** aucun

**Exemple d'appel :**
```json
{
  "issue_key": "PROJ-123"
}
```

**Exemple de réponse :**
```json
{
  "id": "10042",
  "key": "PROJ-123",
  "summary": "Implémenter le flux de connexion OAuth2",
  "description": "Nous devons implémenter le flux complet OAuth2 authorization code...",
  "status": "In Progress",
  "priority": "High",
  "issuetype": "Story",
  "assignee": {"displayName": "John Doe", "emailAddress": "jdoe@entreprise.com"},
  "reporter": {"displayName": "Jane Smith"},
  "labels": ["auth", "backend"],
  "components": [{"name": "Authentication"}],
  "sprint": {"name": "Sprint 14", "state": "active"},
  "story_points": 5,
  "created": "2026-07-10T09:00:00.000+0000",
  "updated": "2026-07-20T14:30:00.000+0000",
  "comments": [
    {
      "author": {"displayName": "Alice"},
      "body": "La PR est prête pour la review",
      "created": "2026-07-20T12:00:00.000+0000"
    }
  ]
}
```

---

### `jira_get_project`

Récupérer les métadonnées d'un projet Jira.

**Paramètres obligatoires :**
- `project_key` (string) — clé du projet (ex. `PROJ`)

**Paramètres optionnels :** aucun

**Exemple d'appel :**
```json
{
  "project_key": "PROJ"
}
```

**Exemple de réponse :**
```json
{
  "id": "10001",
  "key": "PROJ",
  "name": "Produit Principal",
  "description": "Projet de développement du produit principal",
  "projectTypeKey": "software",
  "lead": {"displayName": "Alice Manager", "emailAddress": "alice@entreprise.com"},
  "issueTypes": [
    {"name": "Epic", "subtask": false},
    {"name": "Story", "subtask": false},
    {"name": "Bug", "subtask": false},
    {"name": "Task", "subtask": false},
    {"name": "Sub-task", "subtask": true}
  ],
  "statuses": ["Backlog", "To Do", "In Progress", "In Review", "Done"]
}
```

---

## Mode écriture

Le mode écriture est désactivé par défaut. Pour l'activer :

```bash
export JIRA_WRITE_ENABLED=true
```

### `jira_transition_issue`

Faire passer une issue vers un nouveau statut via un ID de transition.

**Paramètres obligatoires :**
- `issue_key` (string) — clé de l'issue au format `PROJET-NUMÉRO`
- `transition_id` (string) — ID de la transition à appliquer

**Accès :** Agents avec mode écriture activé uniquement

**Note :** Pour connaître les IDs de transition disponibles pour une issue, utiliser `jira_get_issue` et inspecter le champ `transitions`, ou consulter l'administrateur du projet Jira.

**Exemple d'appel :**
```json
{
  "issue_key": "PROJ-123",
  "transition_id": "31"
}
```

**Exemple de réponse :**
```json
{
  "success": true,
  "issue_key": "PROJ-123",
  "new_status": "In Review"
}
```

**IDs de transition courants** (varient selon le workflow du projet) :
| Transition | ID typique |
|------------|------------|
| Démarrer le travail | `11` |
| Envoyer en review | `31` |
| Terminé | `41` |
| Rouvrir | `51` |

## Limites de taux

Jira Cloud impose des limites d'API par utilisateur et par instance :

- **Niveau Standard** : 100 requêtes/10 secondes
- **Niveau Premium** : 1 000 requêtes/10 secondes

Le serveur MCP gère les réponses 429 avec une relance automatique en backoff exponentiel (max 3 tentatives).

## Voir aussi

- [Référence MCP Team Server](mcp-team.md)
- [Référence MCP GitHub Server](mcp-github.fr.md)
- [Référence MCP Linear Server](mcp-linear.fr.md)
- [Documentation API REST Jira](https://developer.atlassian.com/cloud/jira/platform/rest/v3/)
