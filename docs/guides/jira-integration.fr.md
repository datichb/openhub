# Intégration Jira - Guide de démarrage

> 🇬🇧 [Read in English](jira-integration.en.md)

## Vue d'ensemble

L'intégration Jira connecte les agents à tes données de gestion de projet Jira — issues, projets et workflows — avec support de **Jira Cloud** et **Jira Server / Data Center**.

### Fonctionnalités

- **Lecture des issues** : description complète, sous-tâches, commentaires, métadonnées des pièces jointes, champs personnalisés
- **Métadonnées de projet** : boards, sprints, composants, schémas de types d'issues
- **Requêtes JQL** : support complet du Jira Query Language pour filtrer les issues avec précision
- **Transitions d'issues** (mode écriture) : faire avancer les issues dans les états du workflow
- **Jira Cloud et Server/Data Center** : API unifiée avec les différences d'authentification gérées de façon transparente

---

## Prérequis

### Jira Cloud

- `JIRA_URL` — l'URL de ton instance Jira Cloud (ex. `https://macompagnie.atlassian.net`)
- `JIRA_TOKEN` — Personal Access Token (PAT) généré sur `https://id.atlassian.com/manage-profile/security/api-tokens`

### Jira Server / Data Center

- `JIRA_URL` — l'URL de ton serveur Jira (ex. `https://jira.macompagnie.com`)
- `JIRA_USER` — ton nom d'utilisateur Jira
- `JIRA_API_TOKEN` — Personal Access Token généré dans ton profil Jira Server (Jira 8.14+) ou ton mot de passe pour les versions antérieures

---

## Setup

### 1. Configurer via `oh mcp setup`

```bash
oh mcp setup jira
```

Le wizard interactif va :
1. Demander si tu utilises Jira Cloud ou Server
2. Demander ton `JIRA_URL`
3. Demander ton token (PAT pour Cloud, PAT ou mot de passe pour Server)
4. Valider la connexion à l'API Jira
5. Stocker les credentials de façon sécurisée dans le keychain système
6. Mettre à jour `hub.toml` avec le bloc `[mcp.jira]`

### 2. Configuration manuelle

Définis les variables d'environnement avant de lancer `oh` :

```bash
# Jira Cloud
export JIRA_URL=https://macompagnie.atlassian.net
export JIRA_TOKEN=ton-api-token

# Jira Server / Data Center
export JIRA_URL=https://jira.macompagnie.com
export JIRA_USER=monnomdutilisateur
export JIRA_API_TOKEN=ton-pat-ou-mot-de-passe
```

---

## Configuration dans hub.toml

```toml
[mcp.jira]
enabled = true
# Credentials définis via variables d'env (recommandé) ou keychain
# jira_url = "https://macompagnie.atlassian.net"
write_enabled = false  # Mettre à true pour activer les transitions d'issues
```

Déploie après les modifications :

```bash
oh deploy
```

---

## Outils disponibles

| Outil | Description | Utilisé par |
|-------|-------------|-------------|
| `jira_list_issues` | Lister les issues via une requête JQL avec pagination | Planner, Pathfinder |
| `jira_get_issue` | Détails complets d'une issue (description, sous-tâches, commentaires, transitions) | Planner, Pathfinder |
| `jira_get_project` | Métadonnées du projet (composants, types d'issues, versions) | Onboarder |
| `jira_transition_issue` | Faire avancer une issue vers un nouvel état workflow (mode écriture uniquement) | Planner |

---

## Exemples de JQL

Le JQL (Jira Query Language) permet aux agents de filtrer les issues avec précision :

```jql
# Issues en cours assignées à l'utilisateur courant
project = MONPROJ AND status = "In Progress" AND assignee = currentUser()

# Bugs non résolus dans le sprint courant
project = MONPROJ AND issuetype = Bug AND sprint in openSprints() AND resolution = Unresolved

# Issues mises à jour ces 7 derniers jours
project = MONPROJ AND updated >= -7d ORDER BY updated DESC

# Issues bloquant d'autres issues
issueFunction in linkedIssuesOf("project = MONPROJ", "is blocked by")
```

Passe le JQL directement dans les prompts agents :
```
"Liste tous les tickets In Progress dans le projet MONPROJ assignés à moi"
"Trouve tous les bugs mis à jour cette semaine dans le projet FRONTEND"
```

---

## Mode écriture

Par défaut, le serveur MCP Jira est en lecture seule. Pour activer les transitions d'issues :

```bash
export JIRA_WRITE_ENABLED=true
```

Ou dans `hub.toml` :

```toml
[mcp.jira]
write_enabled = true
```

Cela déverrouille `jira_transition_issue`, permettant à l'agent planner de faire avancer les issues dans ton workflow Jira (ex. "To Do" → "In Progress" → "Done").

---

## Exemples d'utilisation

### Planner lisant un ticket Jira

```
"Planifie le ticket Jira MONPROJ-42"
"Décompose FRONTEND-15 en sous-tickets"
```

Le planner lit la description de l'issue, les critères d'acceptation et les issues liées pour décomposer le travail en tickets Beads en respectant la priorité Jira et le contexte de sprint.

### Dev mode avec des tickets Jira

```
"Travaille sur MONPROJ-42"
"Implémente le ticket BACKEND-8"
```

Le pathfinder estime la complexité d'après la richesse de la description, le nombre de sous-tâches, la taxonomie des labels et la proximité de la deadline du sprint.

---

## Jira Cloud vs Server

| Fonctionnalité | Jira Cloud | Jira Server / Data Center |
|---------------|-----------|--------------------------|
| Méthode d'auth | API Token (email + token) | PAT (Jira 8.14+) ou authentification basique |
| URL du token | `id.atlassian.com/manage-profile/security/api-tokens` | Profil Jira > Personal Access Tokens |
| Base de l'API REST | `/rest/api/3/` | `/rest/api/2/` |
| Champs personnalisés | Supportés | Supportés (nommage peut différer) |

Le serveur MCP détecte automatiquement la version de l'API depuis le format de `JIRA_URL`.

---

## Dépannage

### 401 Unauthorized

Les credentials sont manquants ou incorrects :
```bash
oh mcp setup jira  # reconfigurer
```
Pour Jira Cloud : assure-toi d'utiliser l'**API token**, pas le mot de passe de ton compte.

### 403 Forbidden

Le token n'a pas les permissions pour ce projet. Vérifie :
- L'utilisateur a au moins la permission **Parcourir les projets** sur le projet cible
- Pour les transitions : l'utilisateur a la permission **Transition Issues**

### JIRA_URL manquant

```
Error: JIRA_URL is required
```

Définis la variable d'env ou reconfigure avec `oh mcp setup jira`.

### Erreur de syntaxe JQL

Teste ton JQL directement dans l'interface de recherche d'issues Jira avant de l'utiliser dans un prompt, car le serveur MCP transmet les erreurs JQL telles quelles.

---

## Ressources

- [Documentation API REST Jira](https://developer.atlassian.com/cloud/jira/platform/rest/v3/)
- [Référence JQL](https://support.atlassian.com/jira-service-management-cloud/docs/use-advanced-search-with-jira-query-language-jql/)
- [Tokens API Atlassian](https://support.atlassian.com/atlassian-account/docs/manage-api-tokens-for-your-atlassian-account/)

---

## Support

- `oh mcp status jira` — vérifier la configuration
- `oh mcp setup jira` — reconfigurer le service
- Problème persistant → signaler sur [GitHub Issues](https://github.com/anomalyco/opencode)
