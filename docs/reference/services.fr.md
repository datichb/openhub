# Référence CLI — Serveurs MCP (`oh mcp`)

> [Read in English](services.en.md)

Gestion des services MCP (Model Context Protocol) intégrés au binaire `oh`.

---

## Architecture

Les serveurs MCP sont **intégrés au binaire Go** — pas de répertoire `servers/` séparé ni d'étape de build Node.js. Chaque serveur est implémenté nativement dans `cli/internal/mcp/` et servi via stdio JSON-RPC.

Serveurs disponibles :
- **figma** — Intégration API Figma (fichiers, composants, signaux UI)
- **gitlab** — Intégration API GitLab (issues, MRs, labels, milestones)
- **gslides** — Intégration Google Slides
- **team** — Données équipe (membres, wiki, événements) — sans token
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

Démarre un serveur MCP via stdio JSON-RPC. C'est la commande injectée dans `opencode.json` lors du déploiement.

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
token_key = "figma-token"

[mcp.gitlab]
enabled = true
token_key = "gitlab-token"
write_enabled = true

[mcp.gslides]
enabled = false
token_key = "gslides-token"
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

## Déploiement — `mcp` dans `opencode.json`

Lors de `oh deploy`, le CLI injecte un bloc `mcp` dans le `opencode.json` du projet pour chaque service **effectivement activé** (après résolution de la cascade) :

```json
{
  "mcp": {
    "figma": {
      "command": "oh",
      "args": ["mcp", "serve", "figma"]
    },
    "gitlab": {
      "command": "oh",
      "args": ["mcp", "serve", "gitlab"]
    }
  }
}
```

Seuls les serveurs avec un token valide (env, keychain, ou sans token pour `team`) sont déployés.

---

## Variables d'environnement au runtime

| Service | Variable | Description |
|---------|----------|-------------|
| figma | `FIGMA_TOKEN` | Token d'accès Figma |
| gitlab | `GITLAB_TOKEN` | Token d'accès GitLab |
| gitlab | `GITLAB_URL` | URL de l'instance GitLab |
| gslides | `GOOGLE_ACCESS_TOKEN` | Token OAuth Google |
| github | `GITHUB_TOKEN` | Token d'accès GitHub (alias : `GH_TOKEN`) |
| github | `GITHUB_WRITE_ENABLED` | Mettre a `true` pour activer les outils d'ecriture |
| jira | `JIRA_URL` | URL de l'instance Jira (ex. `https://masociete.atlassian.net`) |
| jira | `JIRA_TOKEN` | Token API Jira (ou `JIRA_USER` + `JIRA_API_TOKEN`) |
| jira | `JIRA_WRITE_ENABLED` | Mettre a `true` pour activer les outils d'ecriture |
| linear | `LINEAR_API_KEY` | Cle API Linear |
| linear | `LINEAR_WRITE_ENABLED` | Mettre a `true` pour activer les outils d'ecriture |

---

## Serveur GitHub

**Requis :** `GITHUB_TOKEN` ou `GH_TOKEN`

**Limites de taux :** 60 req/h sans authentification, 5 000 req/h authentifié.

**Mode ecriture :** Definir `GITHUB_WRITE_ENABLED=true` pour activer `github_create_issue`.

**Outils disponibles :**

| Outil | Description |
|-------|-------------|
| `github_get_repo` | Obtenir les metadonnees d'un depot |
| `github_list_issues` | Lister les issues avec filtres |
| `github_get_issue` | Obtenir une issue specifique |
| `github_list_prs` | Lister les pull requests |
| `github_get_pr` | Obtenir une pull request specifique |
| `github_list_workflows` | Lister les workflows GitHub Actions |
| `github_get_workflow_run` | Obtenir une execution de workflow specifique |

---

## Serveur Jira

**Requis :** `JIRA_URL` (ex. `https://masociete.atlassian.net`) et `JIRA_TOKEN` (ou `JIRA_USER` + `JIRA_API_TOKEN`)

**Compatible :** Jira Cloud (API v3) et Jira Server/Data Center.

**Mode ecriture :** Definir `JIRA_WRITE_ENABLED=true` pour activer `jira_transition_issue`.

**Outils disponibles :**

| Outil | Description |
|-------|-------------|
| `jira_list_issues` | Lister les issues avec filtre JQL |
| `jira_get_issue` | Obtenir une issue specifique |
| `jira_get_project` | Obtenir les metadonnees d'un projet |
| `jira_transition_issue` | Changer le statut d'une issue *(mode ecriture uniquement)* |

---

## Serveur Linear

**Requis :** `LINEAR_API_KEY`

**API :** GraphQL.

**Mode ecriture :** Definir `LINEAR_WRITE_ENABLED=true` pour activer les outils de creation/mise a jour.

**Outils disponibles :**

| Outil | Description |
|-------|-------------|
| `linear_list_issues` | Lister les issues avec filtres |
| `linear_get_issue` | Obtenir une issue specifique |
| `linear_create_issue` | Creer une issue *(mode ecriture uniquement)* |
| `linear_update_issue` | Mettre a jour une issue *(mode ecriture uniquement)* |

---

## Migration depuis `oh service`

Les commandes `oh service` sont **dépréciées**. Utilisez les équivalents `oh mcp` :

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
- [Référence CLI complète](cli.fr.md)
