> [Read in English](linear-integration.en.md)

# Intégration Linear - Guide de démarrage

## Vue d'ensemble

L'intégration Linear connecte les agents à ton workspace Linear via l'**API GraphQL Linear**, permettant des workflows modernes de suivi d'issues. Linear est conçu pour les équipes d'ingénierie agiles avec un modèle de données épuré et des filtres puissants.

### Fonctionnalités

- **Lecture des issues** : description complète, état, priorité, assignee, commentaires
- **Création d'issues** (mode écriture) : créer des issues avec titre, description, équipe, priorité
- **Mise à jour d'issues** (mode écriture) : changer l'état ou l'assignee
- **Filtrage flexible** : par équipe, état ou assignee

---

## Prérequis

Tu as besoin d'une **Personal API Key Linear** :

1. Va dans Linear Paramètres > API > Personal API Keys
2. Clique sur **"Create new API key"**
3. Donne-lui un label (ex. `openhub`)
4. Copie la clé générée (format : `lin_api_xxxxxxxxxxxxxxxxxxxx`)

Tu la ranges dans le trousseau de `oh` (voir ci-dessous) : c'est la seule voie qui marche dans une session (l'environnement des agents est filtré, `LINEAR_API_KEY` n'y passe jamais, même avec `env_allow`).

---

## Setup

> **Le plus court : `oh mcp setup linear`.** L'assistant demande la clé (champ masqué, jamais affichée), la range dans le trousseau sous `openhub.mcp.linear.token`, puis écrit `[mcp.linear]` dans `hub.toml` (`enabled = true`, `token_key`, et `write_enabled` selon votre réponse). Avec `-p <projet>`, la clé est propre au projet. Les gestes ci-dessous font la même chose à la main, sans terminal (`oh config set` ne connaît pas les clés de `[mcp.linear]`).

### 1. Ranger la clé dans le trousseau

```bash
oh secrets set --global openhub.mcp.linear.token
```

La valeur est saisie au terminal (masquée). `openhub.mcp.linear.token` est la clé conventionnelle du trousseau.

### 2. Déclarer le serveur dans `hub.toml`

```toml
[mcp.linear]
enabled = true
token_key = "openhub.mcp.linear.token"   # nom de la clé du trousseau, PAS la clé API
write_enabled = false                    # true : création et mise à jour d'issues
```

- `token_key` est **obligatoire** : sans lui (et sans variable `LINEAR_API_KEY`), le serveur n'est pas placé dans le paquet de session. Ce n'est pas silencieux : un avertissement est écrit au lancement de la session, `oh mcp status linear` et `oh doctor` le signalent, avec la commande `oh mcp setup linear`.
- `url` et `trusted_url` sont sans effet pour Linear : le serveur appelle toujours `https://api.linear.app/graphql`.
- `oh mcp enable linear` et `oh mcp disable linear` changent `enabled` (avec `-p <projet>`, pour un projet seulement) ; le reste s'écrit dans le fichier.

### 3. Lancer une session

Aucun redéploiement nécessaire (`oh deploy` supprimé en v5) : le changement est pris en compte au prochain lancement de session, quand le paquet de session est reconstruit :

```bash
oh run <workflow>
```

`oh mcp list` montre que le serveur `linear` existe. `oh mcp status` ne l'affiche pas encore : vérifie en regardant les outils `linear_*` dans la session. Si la clé du trousseau est introuvable, `oh mcp serve linear` écrit un avertissement et le serveur répond « `LINEAR_API_KEY` environment variable not set ».

---

## Outils disponibles

| Outil | Description | Utilisé par |
|-------|-------------|-------------|
| `linear_list_issues` | Lister les issues avec filtres (équipe, état, assignee) | Planner, Pathfinder |
| `linear_get_issue` | Détails complets d'une issue (description, commentaires) | Planner, Pathfinder |
| `linear_create_issue` | Créer une nouvelle issue (mode écriture uniquement) | Planner |
| `linear_update_issue` | Mettre à jour l'état ou l'assignee (mode écriture uniquement) | Planner |

---

## Exemples de filtrage

Filtre les issues par équipe, état ou assignee dans les prompts agents :

```
"Liste toutes les issues In Progress dans l'équipe BACKEND"
"Montre-moi les issues assignées à alice dans l'équipe FRONTEND"
```

Ou avec des paramètres de filtre explicites passés via l'outil MCP :

```json
{
  "team_key": "BACKEND",
  "state": "In Progress",
  "assignee": "alice@macompagnie.com"
}
```

```json
{
  "team_key": "FRONTEND",
  "state": "Todo",
  "first": 25
}
```

---

## Mode écriture

Par défaut, le serveur MCP Linear est en lecture seule. Pour activer la création et la mise à jour d'issues, dans `hub.toml` :

```toml
[mcp.linear]
write_enabled = true
```

oh passe alors `LINEAR_WRITE_ENABLED=true` au serveur au lancement de la session suivante (inutile de l'exporter soi-même : l'environnement de la session est filtré). Cela déverrouille `linear_create_issue` et `linear_update_issue`.

> **Note :** La clé API doit appartenir à un membre du workspace ayant la permission de créer/modifier des issues dans l'équipe cible.

---

## Exemples d'utilisation

### Lister les tickets en cours

```
"Sur quoi l'équipe BACKEND travaille-t-elle actuellement ?"
"Liste toutes les issues In Progress dans l'équipe PLATFORM assignées à moi"
```

L'agent appelle `linear_list_issues` avec `team_key=BACKEND, state=In Progress` et présente un tableau récapitulatif avec les IDs, titres, assignees et priorités des issues.

### Mettre à jour l'état d'une issue

Avec le mode écriture activé :

```
"Marque LINEAR-42 comme Done"
"Passe FRONTEND-15 en In Review"
```

Le planner appelle `linear_update_issue` pour faire évoluer l'état de l'issue, en ajoutant un commentaire avec un résumé du travail effectué.

---

## Dépannage

### Clé API invalide

```
Error: Authentication failed — invalid Linear API key
```

Régénère la clé API dans Linear Paramètres > API > Personal API Keys, puis range la nouvelle dans le trousseau :
```bash
oh secrets set --global openhub.mcp.linear.token
```

### Les outils `linear_*` n'apparaissent pas

Vérifie dans `hub.toml` que `[mcp.linear]` a `enabled = true` **et** `token_key = "openhub.mcp.linear.token"`, et que la clé existe dans le trousseau (`oh secrets get openhub.mcp.linear.token`). Relance ensuite la session.

### Équipe introuvable

```
Error: Team "XYZ" not found in your workspace
```

Vérifie la clé d'équipe dans les paramètres de ton workspace Linear. Les clés d'équipe sont sensibles à la casse (ex. `BACKEND`, pas `backend`).

### Erreurs GraphQL

L'API GraphQL de Linear retourne des messages d'erreur détaillés. Causes fréquentes :
- Paramètres de filtre malformés (vérifie les types : `first` est un entier)
- Tentative d'utiliser les outils d'écriture sans `write_enabled = true`

---

## Ressources

- [Documentation API Linear](https://developers.linear.app/docs/graphql/working-with-the-graphql-api)
- [Clés API Personal Linear](https://linear.app/settings/api)
- [Référence CLI `oh mcp`](../reference/cli-mcp.fr.md)

---

## Support

- `oh mcp list` — le serveur `linear` est-il connu ?
- `oh mcp enable linear` / `oh mcp disable linear` — activer ou désactiver le service
- `oh mcp setup linear` — configurer ou reconfigurer le service
- `oh mcp status linear` — état, source de l'état et origine du jeton
- Problème persistant → signaler sur [GitHub Issues](https://github.com/anomalyco/opencode)
