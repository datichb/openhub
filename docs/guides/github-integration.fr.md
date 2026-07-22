# Intégration GitHub - Guide de démarrage

> 🇬🇧 [Read in English](github-integration.en.md)

## Vue d'ensemble

L'intégration GitHub donne aux agents un accès en lecture à tes dépôts GitHub — issues, pull requests et workflows Actions — permettant aux workflows de planning et d'estimation d'utiliser les données réelles du projet comme source de vérité.

### Fonctionnalités

- **Lecture des issues** : description complète, labels, milestone, assignees, commentaires
- **Lecture des pull requests** : titre, branches, état, statut de review, nombre de fichiers modifiés
- **Workflows Actions** : définitions de workflows et historique des exécutions
- **Création d'issues** (mode écriture) : créer des issues directement depuis l'agent planner
- **Requêtes multi-repo** : accès à tout repo que le token permet de lire

---

## Prérequis

Tu as besoin d'un Personal Access Token GitHub (classique ou fine-grained) :

- **PAT classique** — scope `repo` (accès lecture aux repos privés) ou `public_repo` (repos publics uniquement)
- **PAT fine-grained** — permissions dépôt : Issues (Lecture), Pull requests (Lecture), Actions (Lecture)

Stocke le token dans `GITHUB_TOKEN` (variable d'environnement ou keychain — voir Setup ci-dessous).

---

## Setup

### 1. Configurer via `oh mcp setup`

```bash
oh mcp setup github
```

Le wizard interactif va :
1. Demander ton **Personal Access Token** GitHub
2. Demander optionnellement une **URL de base GitHub Enterprise** (laisser vide pour `github.com`)
3. Valider la connexion à l'API GitHub
4. Stocker le token de façon sécurisée dans le keychain système
5. Mettre à jour `hub.toml` avec le bloc `[mcp.github]`

Vérifier le statut à tout moment :
```bash
oh mcp status github
```

### 2. Créer un Personal Access Token

1. Va sur `https://github.com/settings/tokens`
2. Clique sur **"Generate new token (classic)"** ou **"Fine-grained tokens"**
3. Pour classique : sélectionne le scope `repo`
4. Pour fine-grained : sélectionne Issues (Lecture), Pull requests (Lecture), Actions (Lecture)
5. Copie le token généré (format : `ghp_xxxxxxxxxxxxxxxxxxxx`)

### 3. Configuration manuelle (alternative)

Définis la variable d'environnement avant de lancer `oh` :

```bash
export GITHUB_TOKEN=ghp_xxxxxxxxxxxxxxxxxxxx
```

Ou ajoute-la dans `hub.toml` :

```toml
[mcp.github]
enabled = true
token = "ghp_xxxxxxxxxxxxxxxxxxxx"
# base_url = "https://github.macompany.com/api/v3"  # pour GitHub Enterprise
```

---

## Configuration dans hub.toml

```toml
[mcp.github]
enabled = true
# Le token peut aussi être défini via la variable GITHUB_TOKEN (recommandé)
# base_url = "https://github.macompany.com/api/v3"   # GitHub Enterprise uniquement
write_enabled = false  # Mettre à true pour activer la création d'issues
```

Déploie vers un projet après la mise à jour de hub.toml :

```bash
oh deploy
```

---

## Outils disponibles

| Outil | Description | Utilisé par |
|-------|-------------|-------------|
| `github_get_repo` | Métadonnées du dépôt (description, langage, topics, stars) | Onboarder |
| `github_list_issues` | Lister les issues avec filtres (état, labels, assignee, milestone) | Planner, Pathfinder |
| `github_get_issue` | Détails complets d'une issue (body, labels, commentaires, PRs liées) | Planner, Pathfinder |
| `github_list_prs` | Lister les pull requests (état, branche base/head, auteur) | Pathfinder |
| `github_get_pr` | Détails complets d'une PR (titre, body, reviewers, nombre de fichiers) | Pathfinder |
| `github_list_workflows` | Lister les définitions de workflows Actions | Onboarder |
| `github_get_workflow_run` | Détails d'une exécution de workflow (statut, conclusion, URL des logs) | Onboarder |

---

## Mode écriture

Par défaut, le serveur MCP GitHub est en lecture seule. Pour activer la création d'issues :

```bash
export GITHUB_WRITE_ENABLED=true
```

Ou dans `hub.toml` :

```toml
[mcp.github]
write_enabled = true
```

Cela déverrouille l'outil `github_create_issue`, que l'agent planner peut utiliser pour pousser directement les sous-tickets décomposés vers GitHub Issues.

> **Note :** Le mode écriture requiert un token avec le scope `repo` (PAT classique) ou la permission Issues (Écriture) (PAT fine-grained).

---

## Skills d'agents

Trois skills adaptateurs intègrent le contexte GitHub dans les protocoles d'agents existants :

| Skill | Agent | Ce qu'elle fait |
|-------|-------|----------------|
| `github-planner-protocol` | Planner | Utilise le body de l'issue comme document de requirements, les labels pour la priorité, le milestone pour la deadline |
| `github-pathfinder-protocol` | Pathfinder | Enrichit l'estimation avec la richesse de l'issue, les PRs liées, les questions ouvertes dans les commentaires |
| `github-onboarder-protocol` | Onboarder | Mappe la structure du repo, la taxonomie des labels, les patterns de workflows CI/CD |

---

## Exemples d'utilisation

### Planner avec une GitHub Issue

```
"Planifie l'issue #42 du repo owner/my-repo"
"Décompose l'issue GitHub #42 en sous-tickets"
```

La skill `github-planner-protocol` :
- Lit le body complet de l'issue comme document de requirements
- Utilise les critères d'acceptation (checkboxes) pour pré-remplir les tickets Beads
- Lit le milestone pour calibrer la priorité de livraison
- Détecte les issues liées comme dépendances

### Pathfinder avec une Pull Request

```
"Pathfinder PR #15 du repo owner/my-repo"
"Estime la complexité de la pull request #15"
```

Le pathfinder lit le titre, la description, le nombre de fichiers modifiés et les commentaires de review pour produire une estimation de complexité et des signaux de risque.

### Onboarder détectant les workflows CI

```
"Onboard sur owner/my-repo (GitHub)"
```

La skill `github-onboarder-protocol` mappe :
- La taxonomie des labels (type, priorité, labels de domaine)
- Le milestone ouvert et les dates de livraison du sprint
- Les workflows GitHub Actions actifs (build, test, deploy)
- Le volume du backlog et sa distribution par état

---

## Limites de débit

| Authentification | Limite de débit |
|-----------------|----------------|
| Non authentifié | 60 requêtes/heure |
| Authentifié (PAT) | 5 000 requêtes/heure |
| GitHub Enterprise Cloud | 15 000 requêtes/heure |

Le serveur MCP gère automatiquement les réponses `429` avec un backoff exponentiel.

---

## Dépannage

### 401 Unauthorized

```
Error: GitHub API returned 401 Unauthorized
```

Le token est manquant ou invalide. Reconfigure :
```bash
oh mcp setup github
```

### 404 Not Found

Le chemin du dépôt est incorrect ou le token n'a pas accès. Vérifie :
- Le format : `owner/repo` (sensible à la casse)
- Le token a au moins un accès lecture à ce dépôt

### Limite de débit atteinte (403 / 429)

Tu as épuisé ton quota horaire. Solutions :
- Attendre la réinitialisation (indiquée dans le header `X-RateLimit-Reset`)
- Utiliser un token authentifié (5 000 req/h au lieu de 60)
- Mettre en cache les requêtes lourdes avec `oh mcp cache enable github`

### Serveur MCP ne démarre pas

```bash
# Vérifier la configuration
oh mcp status github

# Reconfigurer
oh mcp setup github

# Lancer le serveur manuellement pour voir les erreurs brutes
oh mcp serve github
```

---

## Ressources

- [Documentation API REST GitHub](https://docs.github.com/fr/rest)
- [Gérer les Personal Access Tokens](https://docs.github.com/fr/authentication/keeping-your-account-and-data-secure/managing-your-personal-access-tokens)
- [Référence CLI `oh mcp`](../reference/mcp.fr.md)
- [Protocole MCP](https://modelcontextprotocol.io/)

---

## Support

- `oh mcp status github` — vérifier la configuration
- `oh mcp setup github` — reconfigurer le service
- Problème persistant → signaler sur [GitHub Issues](https://github.com/anomalyco/opencode)
