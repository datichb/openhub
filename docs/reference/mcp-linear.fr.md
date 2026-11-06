> [Read in English](mcp-linear.en.md)

# Référence du serveur MCP Linear

## Activation

Le serveur MCP Linear est placé automatiquement dans le paquet de session au lancement lorsque `[mcp.linear]` a `enabled = true` **et** `token_key` dans `hub.toml` (voir le [guide](../guides/linear-integration.fr.md)). `oh mcp setup linear` écrit les deux. Un service activé sans `token_key` (ni `LINEAR_API_KEY`) n'est pas placé dans le paquet : oh l'écrit au lancement de la session, dans `oh mcp status linear` et dans `oh doctor`.

```json
// Déclaré dans le paquet de session au lancement
{
  "mcpServers": {
    "linear": {
      "command": "oh",
      "args": ["mcp", "serve", "linear"]
    }
  }
}
```

## Authentification

Une clé API personnelle Linear, rangée dans le trousseau de `oh` (`oh mcp setup linear`, ou `oh secrets set --global openhub.mcp.linear.token`) et désignée par `token_key` dans `[mcp.linear]`. Le serveur la lit dans la variable `LINEAR_API_KEY`, que `oh mcp serve linear --token-key …` remplit depuis le trousseau ; exportée dans votre shell, elle n'atteint pas les sessions (l'environnement des agents est filtré).

Pour générer une clé : Paramètres Linear → API → Clés API personnelles → Créer une clé

Le serveur MCP Linear communique avec l'API GraphQL de Linear à l'adresse :
```
https://api.linear.app/graphql
```

Toutes les opérations sont exécutées en tant que requêtes et mutations GraphQL sur cet endpoint. Cette adresse est fixe : le serveur ne lit aucune variable d'URL (oh ne pose plus `LINEAR_URL`, que le serveur ne lisait pas) et `url` et `trusted_url` de `[mcp.linear]` sont sans effet.

## Outils disponibles

| Outil | Description | Accès |
|-------|-------------|-------|
| `linear_list_issues` | Lister les issues avec filtres GraphQL | Tous les agents |
| `linear_get_issue` | Récupérer une issue par identifiant (ex. ENG-123) | Tous les agents |

---

### `linear_list_issues`

Lister les issues Linear avec des filtres GraphQL.

**Paramètres obligatoires :** aucun (au moins un filtre recommandé)

**Paramètres optionnels :**
- `team_key` (string) — clé de l'équipe (ex. `ENG`, `OPS`)
- `state` (string) — nom de l'état de workflow (ex. `In Progress`, `Todo`, `Done`)
- `assignee` (string) — nom d'affichage ou email de l'assigné
- `first` (integer) — nombre de résultats à retourner (défaut : 50)

**Exemple d'appel :**
```json
{
  "team_key": "ENG",
  "state": "In Progress",
  "assignee": "John Doe",
  "first": 10
}
```

**Exemple de réponse :**
```json
{
  "issues": [
    {
      "id": "abc-123-def",
      "identifier": "ENG-142",
      "title": "Implémenter le rate limiting pour la gateway API",
      "state": {"name": "In Progress", "type": "started"},
      "priority": 2,
      "priorityLabel": "High",
      "assignee": {"name": "John Doe", "email": "jdoe@entreprise.com"},
      "team": {"key": "ENG", "name": "Engineering"},
      "labels": [{"name": "backend"}, {"name": "infrastructure"}],
      "createdAt": "2026-07-15T10:00:00.000Z",
      "updatedAt": "2026-07-20T16:00:00.000Z",
      "url": "https://linear.app/company/issue/ENG-142"
    }
  ],
  "pageInfo": {
    "hasNextPage": false,
    "endCursor": "cursor_xyz"
  }
}
```

**Valeurs de priorité :**
| Valeur | Libellé |
|--------|---------|
| `0` | Aucune priorité |
| `1` | Urgente |
| `2` | Haute |
| `3` | Moyenne |
| `4` | Basse |

---

### `linear_get_issue`

Récupérer une issue Linear par son identifiant.

**Paramètres obligatoires :**
- `issue_id` (string) — identifiant de l'issue au format `ÉQUIPE-NUMÉRO` (ex. `ENG-123`)

**Paramètres optionnels :** aucun

**Exemple d'appel :**
```json
{
  "issue_id": "ENG-142"
}
```

**Exemple de réponse :**
```json
{
  "id": "abc-123-def",
  "identifier": "ENG-142",
  "title": "Implémenter le rate limiting pour la gateway API",
  "description": "La gateway API n'a actuellement aucun rate limiting en place...",
  "state": {"name": "In Progress", "type": "started"},
  "priority": 2,
  "priorityLabel": "High",
  "assignee": {"name": "John Doe", "email": "jdoe@entreprise.com"},
  "team": {"id": "team-uuid", "key": "ENG", "name": "Engineering"},
  "labels": [{"name": "backend"}, {"name": "infrastructure"}],
  "parent": null,
  "children": [],
  "comments": [
    {
      "body": "Démarré l'implémentation du token bucket",
      "user": {"name": "John Doe"},
      "createdAt": "2026-07-18T09:00:00.000Z"
    }
  ],
  "createdAt": "2026-07-15T10:00:00.000Z",
  "updatedAt": "2026-07-20T16:00:00.000Z",
  "url": "https://linear.app/company/issue/ENG-142"
}
```

---

## Mode écriture

Le mode écriture est désactivé par défaut. Pour l'activer, répondez oui à la question du mode écriture de `oh mcp setup linear` (ou écrivez `write_enabled = true` dans `[mcp.linear]`) : oh pose alors `LINEAR_WRITE_ENABLED=true` pour le serveur de la session. Une variable exportée dans votre shell n'atteint pas les sessions.

### `linear_create_issue`

Créer une nouvelle issue dans une équipe Linear.

**Paramètres obligatoires :**
- `team_id` (string) — UUID de l'équipe (récupérer via `linear_list_issues` ou les paramètres Linear)
- `title` (string) — titre de l'issue

**Paramètres optionnels :**
- `description` (string) — description de l'issue (Markdown supporté)
- `priority` (integer) — niveau de priorité : `0` (aucune) | `1` (urgente) | `2` (haute) | `3` (moyenne) | `4` (basse)

**Accès :** Agents avec mode écriture activé uniquement

**Exemple d'appel :**
```json
{
  "team_id": "team-uuid-ici",
  "title": "Ajouter un circuit breaker pour les appels vers les services tiers",
  "description": "Quand les services tiers tombent, les requêtes s'accumulent...",
  "priority": 2
}
```

**Exemple de réponse :**
```json
{
  "id": "new-uuid",
  "identifier": "ENG-143",
  "title": "Ajouter un circuit breaker pour les appels vers les services tiers",
  "url": "https://linear.app/company/issue/ENG-143",
  "createdAt": "2026-07-22T10:00:00.000Z"
}
```

---

### `linear_update_issue`

Mettre à jour l'état ou l'assigné d'une issue existante.

**Paramètres obligatoires :**
- `issue_id` (string) — identifiant de l'issue au format `ÉQUIPE-NUMÉRO` (ex. `ENG-142`)

**Paramètres optionnels :**
- `state_id` (string) — UUID de l'état de workflow cible
- `assignee_id` (string) — UUID de l'utilisateur à assigner

**Accès :** Agents avec mode écriture activé uniquement

**Note :** Pour trouver les UUIDs d'états, utiliser `linear_list_issues` et inspecter le champ `state.id`. Pour les UUIDs d'utilisateurs, inspecter le champ `assignee.id`.

**Exemple d'appel :**
```json
{
  "issue_id": "ENG-142",
  "state_id": "state-uuid-in-review",
  "assignee_id": "user-uuid-alice"
}
```

**Exemple de réponse :**
```json
{
  "id": "abc-123-def",
  "identifier": "ENG-142",
  "state": {"name": "In Review", "type": "started"},
  "assignee": {"name": "Alice", "email": "alice@entreprise.com"},
  "updatedAt": "2026-07-22T10:05:00.000Z"
}
```

## Limites de taux

L'API GraphQL de Linear impose les limites suivantes :

| Plan | Requêtes/minute | Complexité/minute |
|------|-----------------|-------------------|
| Free | 60 | 50 000 |
| Plus / Pro | 120 | 150 000 |

Si l'API Linear retourne `429` (rate limit), l'agent appelant doit attendre avant de réessayer. Les requêtes GraphQL complexes (avec des champs imbriqués) consomment davantage de budget de complexité.

## Voir aussi

- [Référence MCP Team Server](mcp-team.fr.md)
- [Référence MCP GitHub Server](mcp-github.fr.md)
- [Référence MCP Jira Server](mcp-jira.fr.md)
- [Documentation API GraphQL Linear](https://developers.linear.app/docs/graphql/working-with-the-graphql-api)
