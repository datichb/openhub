> [Read in English](services.en.md)

# Référence CLI — Serveurs MCP (`oh mcp`)

Gestion des services MCP (Model Context Protocol) intégrés au binaire `oh`.

---

## Architecture

Les serveurs MCP sont **intégrés au binaire Go** — pas de répertoire `servers/` séparé ni d'étape de build Node.js. Chaque serveur est implémenté nativement dans `cli/internal/mcp/` et servi via stdio JSON-RPC.

Serveurs disponibles :
- **figma** — Intégration API Figma (fichiers, nœuds, styles)
- **gitlab** — Intégration API GitLab (issues, MR, discussions, labels, reviewers)
- **gslides** — Intégration Google Slides
- **team** — Données équipe (membres, wiki, événements, claims, politiques, patterns, briefs de reprise) — sans token ; expose le cycle de vie des claims (5 statuts : `planned`, `in_progress`, `review`, `blocked`, `done`), les labels de claim (`agent-reviewed`, `needs-human-review`), le champ `ExternalIID` pour le lien avec les trackers externes, et les événements émis lors des opérations claim/release/transfer
- **github** — Intégration API GitHub (dépôts, issues, PRs, workflows)
- **jira** — Intégration Jira (Cloud et Server/Data Center)
- **linear** — Intégration Linear (issues, projets)

> **Registre dynamique :** Des serveurs personnalisés peuvent être chargés depuis `~/.oh/mcp/<name>/manifest.json`. Champs obligatoires : `name`, `description`, `binary`, `required_tokens`.

---

## Commandes

### `oh mcp enable <service> [--project <name>]`

Active un service MCP au niveau hub ou pour un projet spécifique.

```bash
# Activer au niveau hub
oh mcp enable figma

# Activer pour un projet spécifique
oh mcp enable figma --project mon-projet
```

**Comportement avec `--project` :**
- Si aucun token n'est trouvé (ni projet, ni hub, ni env), un prompt propose :
  - Hériter de la configuration du hub (utiliser le token hub existant)
  - Configurer un token spécifique au projet

---

### `oh mcp disable <service> [--project <name>]`

Désactive un service MCP.

```bash
# Désactiver au niveau hub
oh mcp disable gitlab

# Désactiver pour un projet (override : désactivé même si le hub l'active)
oh mcp disable gitlab --project mon-projet
```

Avec `--project`, le service est **explicitement désactivé** pour ce projet, indépendamment de la configuration hub.

---

### `oh mcp reset <service> --project <name>`

Supprime l'override projet pour un service MCP, revenant à la configuration hub.

```bash
oh mcp reset figma --project mon-projet
```

> **Note :** `--project` est obligatoire. Cette commande n'a pas de sens au niveau hub.

Après un reset, le projet hérite à nouveau de l'état hub pour ce service (enabled/disabled, token, options).

---

### `oh mcp setup [--project <name>]`

Lance un wizard interactif pour configurer un service MCP (token, options).

```bash
# Configuration hub
oh mcp setup

# Configuration pour un projet
oh mcp setup --project mon-projet
```

**Le wizard :**
1. Sélection du service (Figma, GitLab, Google Slides)
2. Saisie du token (masquée)
3. Pour GitLab : activation optionnelle du mode écriture
4. Stockage sécurisé dans le keychain

---

### `oh mcp status [--project <name>]`

Affiche le statut de tous les services MCP.

```bash
# Statut hub
oh mcp status

# Statut effectif pour un projet (inclut les overrides)
oh mcp status --project mon-projet
```

**Colonnes affichées :**

| Colonne | Description |
|---------|-------------|
| SERVICE | Nom du service (Figma, GitLab, etc.) |
| STATUS  | enabled / disabled |
| SOURCE  | hub / project (origine de la configuration effective) |
| TOKEN   | env:VAR / keychain / missing / — |

---

### `oh mcp serve <name>`

Démarre un serveur MCP via stdio JSON-RPC. C'est la commande déclarée dans le paquet de session au lancement.

```bash
oh mcp serve figma
oh mcp serve gitlab
oh mcp serve gslides
oh mcp serve team
oh mcp serve github
oh mcp serve jira
oh mcp serve linear
```

---

### `oh mcp list [--json]`

Liste tous les serveurs MCP disponibles.

```bash
oh mcp list
oh mcp list --json
```

---

## Configuration

### Hub-level (`~/.oh/hub.toml`)

L'activation globale des services est stockée dans `hub.toml` :

```toml
[mcp.figma]
enabled = true
token_key = "openhub.mcp.figma.token"

[mcp.gitlab]
enabled = true
token_key = "openhub.mcp.gitlab.token"
write_enabled = true

[mcp.jira]
enabled = false
token_key = "openhub.mcp.jira.token"
url = "https://masociete.atlassian.net"   # facultatif (GitLab, Jira) ; vide = URL du team-state ou défaut

[mcp.gslides]
enabled = false
token_key = "openhub.mcp.gslides.token"
```

### Projet-level (`ProjectMCPConfig`)

Chaque projet peut surcharger la configuration hub. La config projet est stockée en base de données via `oh mcp enable/disable/setup --project`.

Champs par service :

| Champ | Type | Description |
|-------|------|-------------|
| `name` | string | Nom du service (figma, gitlab, gslides, team) |
| `enabled` | *bool | `nil` = hériter du hub, `true` = forcer l'activation, `false` = forcer la désactivation |
| `token_key` | string | Clé keychain override (vide = hériter du hub) |
| `write_enabled` | *bool | Mode écriture GitLab (nil = hériter du hub) |
| `url` | string | URL de l'instance (vide = hériter du hub ou de l'équipe) |

### Cascade et héritage

```
Hub (hub.toml)
  └── Projet (MCPConfig)
        └── Environnement (variables env)
```

**Règles de résolution :**

1. Si le projet n'a pas de `MCPConfig` → hérite intégralement du hub
2. Si le projet a une entrée pour un service :
   - `enabled = nil` → hérite de l'état hub
   - `enabled = true/false` → override explicite
   - `token_key` vide → hérite du token hub
   - `token_key` non-vide → utilise le token projet
3. Les variables d'environnement (`FIGMA_TOKEN`, etc.) ont toujours priorité sur le keychain

---

## Paquet de session — bloc `mcp`

À chaque lancement (`oh run <workflow>`), oh place dans le paquet de session une entrée pour chaque service **effectivement activé** (après résolution de la cascade) et dont le token est disponible (variable d'environnement, trousseau, ou sans token pour `team`). L'adaptateur la reporte dans la configuration opencode de la session ; rien n'est écrit dans le `opencode.json` du projet (`oh deploy` supprimé en v5) :

```json
{
  "mcp": [
    { "name": "gitlab", "type": "local",
      "command": ["oh", "mcp", "serve", "gitlab", "--token-key", "openhub.mcp.gitlab.token"],
      "environment": { "GITLAB_WRITE_ENABLED": "true", "GITLAB_URL": "https://gitlab.example.com" } },
    { "name": "team", "type": "local",
      "command": ["oh", "mcp", "serve", "team"],
      "environment": { "OH_TEAM_ID": "acme", "OH_PROJECT_ID": "web-app" } }
  ]
}
```

- **Sélection par le workflow** : si le workflow déclare `mcp:` ([schéma](workflow-schema.fr.md#ressources)), seuls les serveurs de la liste sont gardés ; absent, la session garde les serveurs du projet. Le serveur `workflow` (checkpoints, sorties) est toujours ajouté.
- **Serveur `team`** : il lit l'équipe et le projet de la session dans `OH_TEAM_ID` et `OH_PROJECT_ID` (`.opencode/team.json` n'est plus lu).
- **Conteneur et distant** : les serveurs MCP d'oh ne tournent pas hors de la machine. Le démon `ohd` les sert par la **passerelle MCP** (HTTP) ; les tokens restent dans le trousseau de la machine ([ADR-046](../architecture/adr/046-beads-gateways.fr.md)).
- Inspecter : `oh bundle show <workflow>`.

---

## Variables d'environnement au runtime

Lues par `oh mcp serve <nom>` ; oh les renseigne dans le paquet à partir de la configuration (les tokens viennent du trousseau, `--token-key`).

| Service | Variable | Description |
|---------|----------|-------------|
| figma | `FIGMA_TOKEN` | Token d'accès Figma |
| gitlab | `GITLAB_TOKEN` | Token d'accès GitLab |
| gitlab | `GITLAB_URL` | URL de l'instance GitLab (défaut : `https://gitlab.com`) |
| gitlab | `GITLAB_WRITE_ENABLED` | `true` pour activer les outils d'écriture (posée par oh quand `write_enabled = true`) |
| gslides | `GOOGLE_ACCESS_TOKEN` | Token OAuth Google |
| github | `GITHUB_TOKEN` | Token d'accès GitHub (alias : `GH_TOKEN`) |
| github | `GITHUB_WRITE_ENABLED` | `true` pour activer les outils d'écriture |
| jira | `JIRA_URL` | URL de l'instance Jira (ex. `https://masociete.atlassian.net`) |
| jira | `JIRA_TOKEN` | Token API Jira (ou `JIRA_USER` + `JIRA_API_TOKEN`) |
| jira | `JIRA_WRITE_ENABLED` | `true` pour activer les outils d'écriture |
| linear | `LINEAR_API_KEY` | Clé API Linear |
| linear | `LINEAR_WRITE_ENABLED` | `true` pour activer les outils d'écriture |
| team | `OH_TEAM_ID`, `OH_PROJECT_ID` | Équipe et projet de la session (posées par oh) |

---

## Serveur GitLab

**Requis :** `GITLAB_TOKEN` (Personal Access Token avec le scope `api`)

**Optionnel :** `GITLAB_URL` pour une instance auto-hébergée (défaut : `https://gitlab.com`)

**Mode écriture :** `write_enabled = true` dans `hub.toml` (ou pour le projet) ; oh pose alors `GITLAB_WRITE_ENABLED=true`.

**Outils de lecture :**

| Outil | Description |
|-------|-------------|
| `gitlab_get_project` | Métadonnées d'un projet |
| `gitlab_list_issues` | Lister les issues avec filtres |
| `gitlab_list_mrs` | Lister les merge requests |
| `gitlab_list_mr_discussions` | Lister les discussions d'une MR |
| `gitlab_get_mr_approvals` | Approbations d'une MR |

**Outils d'écriture** (mode écriture) :

| Outil | Description |
|-------|-------------|
| `gitlab_create_mr` | Créer une merge request |
| `gitlab_add_mr_note` | Ajouter une note à une MR |
| `gitlab_update_issue` | Mettre à jour une issue (labels, assignee, statut) |
| `gitlab_assign_reviewer` | Assigner un reviewer à une MR |
| `gitlab_add_label` | Ajouter un label à une issue ou une MR |
| `gitlab_reply_to_mr_discussion` | Répondre à une discussion de MR |

---

## Serveur GitHub

**Requis :** `GITHUB_TOKEN` ou `GH_TOKEN`

**Limites de taux :** 60 req/h sans authentification, 5 000 req/h authentifié.

**Mode écriture :** `GITHUB_WRITE_ENABLED=true` active `github_create_issue`.

**Outils disponibles :**

| Outil | Description |
|-------|-------------|
| `github_get_repo` | Métadonnées d'un dépôt |
| `github_list_issues` | Lister les issues avec filtres |
| `github_get_issue` | Obtenir une issue |
| `github_list_prs` | Lister les pull requests |
| `github_get_pr` | Obtenir une pull request |
| `github_list_workflows` | Lister les workflows GitHub Actions |
| `github_get_workflow_run` | Exécutions d'un workflow (par nom de fichier ou ID) |
| `github_create_issue` | Créer une issue *(mode écriture uniquement)* |

---

## Serveur Jira

**Requis :** `JIRA_URL` (ex. `https://masociete.atlassian.net`) et `JIRA_TOKEN` (ou `JIRA_USER` + `JIRA_API_TOKEN`)

**Compatible :** Jira Cloud (API v3) et Jira Server/Data Center.

**Mode écriture :** `JIRA_WRITE_ENABLED=true` active les outils d'écriture.

**Outils disponibles :**

| Outil | Description |
|-------|-------------|
| `jira_list_issues` | Lister les issues avec un filtre JQL |
| `jira_get_issue` | Obtenir une issue |
| `jira_get_project` | Métadonnées d'un projet |
| `jira_list_comments` | Commentaires d'une issue |
| `jira_transition_issue` | Changer le statut d'une issue *(mode écriture uniquement)* |
| `jira_create_issue` | Créer une issue *(mode écriture uniquement)* |

---

## Serveur Linear

**Requis :** `LINEAR_API_KEY`

**API :** GraphQL.

**Mode écriture :** `LINEAR_WRITE_ENABLED=true` active les outils de création et de mise à jour.

**Outils disponibles :**

| Outil | Description |
|-------|-------------|
| `linear_list_issues` | Lister les issues avec filtres |
| `linear_get_issue` | Obtenir une issue |
| `linear_create_issue` | Créer une issue *(mode écriture uniquement)* |
| `linear_update_issue` | Mettre à jour une issue *(mode écriture uniquement)* |

---

## Migration depuis `oh service`

Les commandes `oh service` sont **dépréciées** (masquées de l'aide). Utilisez les équivalents `oh mcp` :

| Ancienne commande | Nouvelle commande |
|---|---|
| `oh service` | `oh mcp status` |
| `oh service setup` | `oh mcp setup` |
| `oh service setup -p <projet>` | `oh mcp setup --project <projet>` |
| `oh service remove <service>` | `oh mcp disable <service>` |

Les commandes `oh service` restent fonctionnelles mais affichent un message de dépréciation.

---

## Voir aussi

- [Guide d'intégration Figma](../guides/figma-integration.fr.md)
- [Guide d'intégration GitLab](../guides/gitlab-integration.fr.md)
- [Schéma des workflows](workflow-schema.fr.md) (champ `mcp`)
- [Référence CLI complète](cli.fr.md)
