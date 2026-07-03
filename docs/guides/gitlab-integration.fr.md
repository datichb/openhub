# Intégration GitLab - Guide de démarrage

> 🇬🇧 [Read in English](gitlab-integration.en.md)

## Vue d'ensemble

L'intégration GitLab enrichit les workflows de planification (Orchestrator, Pathfinder, Planner et Onboarder) avec le contexte projet en interrogeant automatiquement l'API GitLab pour lire les tickets, merge requests, labels et milestones.

### Fonctionnalités

- **Lecture des tickets** : description complète, labels, milestone, commentaires humains
- **Lecture des MRs** : titre, branches, état, nombre de fichiers modifiés
- **Taxonomie des labels** : compréhension automatique de la classification du projet
- **Milestones actifs** : contexte de sprint et dates de livraison
- **Recherche de tickets** : filtrage par état, labels, mots-clés

---

## Configuration rapide

### 1. Configurer via `oh service`

La méthode recommandée est d'utiliser la commande `oh service setup` qui vous guide interactivement :

```bash
oh service setup gitlab
# ou via l'alias :
oh gitlab setup
```

Cette commande va :
1. Vous demander votre **Personal Access Token** GitLab
2. Vous demander l'**URL de votre instance** (laisser vide pour gitlab.com)
3. Valider la connexion à l'API GitLab
4. Sauvegarder la configuration dans `~/.config/opencode/config.json`
5. Builder automatiquement le serveur MCP si nécessaire

Vérifier l'état à tout moment :
```bash
oh service status gitlab
# ou :
oh gitlab status
```

### 2. Obtenir votre Personal Access Token

1. Aller sur `<votre-gitlab>/-/profile/personal_access_tokens`
2. Cliquer sur **"Add new token"**
3. Choisir un nom (ex: `openhub`)
4. Sélectionner les scopes requis :
   - `api` — accès complet aux issues, MRs, labels, milestones
   - `read_user` — validation d'identité
5. Définir une date d'expiration
6. Copier le token généré (format : `glpat-xxxxxxxxxxxxxxxxxxxx`)

> Pour une instance **self-hosted**, remplacer l'URL par celle de votre instance GitLab lors de la configuration.

### 3. Configuration manuelle (alternative)

Créer ou éditer `~/.config/opencode/config.json` :

```json
{
  "env": {
    "GITLAB_PERSONAL_ACCESS_TOKEN": "glpat-xxxxxxxxxxxxxxxxxxxx",
    "GITLAB_BASE_URL": "https://gitlab.monentreprise.com"
  }
}
```

> `GITLAB_BASE_URL` est optionnel. Laisser vide ou omettre pour utiliser `gitlab.com`.

### 4. Déployer sur un projet

```bash
oh deploy opencode MON-PROJET
# ou uniquement le MCP GitLab :
oh service deploy gitlab --project MON-PROJET
# ou via l'alias :
oh gitlab deploy --project MON-PROJET
```

---

## Utilisation

### Avec l'Orchestrator

L'Orchestrator ne lit pas les tickets GitLab directement. Quand l'utilisateur fournit un ID de ticket, il le transmet tel quel au `pathfinder` ou au `planner` qui effectuent la lecture dans leur propre session :

```
"Implémente le ticket #42 du projet mon-groupe/mon-projet"
"Prends en charge l'issue #42"
"Travaille sur la MR !15"
```

L'Orchestrator transmet l'ID brut (`#42`, `!15`) au `pathfinder` ou `planner` — c'est ces agents qui lisent le ticket via leurs propres accès MCP GitLab et routent en conséquence.

### Avec le Pathfinder

Le Pathfinder enrichit son estimation avec le contexte GitLab :

```
"Pathfinder le ticket #42"
"Estime la complexité de l'issue #42 du projet mon-groupe/mon-projet"
```

Le skill `gitlab-pathfinder-protocol` ajuste l'estimation selon :
- La richesse de la description et des critères d'acceptation
- Les labels de type et priorité
- Le milestone et son échéance
- Les commentaires avec blockers ou questions ouvertes

### Avec le Planner

Le Planner utilise le ticket comme source de vérité pour la décomposition :

```
"Planifie l'issue #42 du projet mon-groupe/mon-projet"
"Décompose le ticket #42 en sous-tickets"
```

Le skill `gitlab-planner-protocol` exploite :
- La **description** comme cahier des charges
- Les **critères d'acceptation** pour pré-remplir les tickets Beads
- Le **milestone** pour calibrer la priorité
- Les **tickets liés** pour détecter les dépendances

### Avec l'Onboarder

L'Onboarder cartographie le projet GitLab lors de la découverte :

```
"Onboarde-toi sur le projet mon-groupe/mon-projet (GitLab)"
```

Le skill `gitlab-onboarder-protocol` produit dans `ONBOARDING.md` :
- La taxonomie des labels (types, priorités, domaines)
- La cadence de livraison (sprints, milestones)
- L'état du backlog (volume et répartition)

Et dans `CONVENTIONS.md` :
- Les conventions de labelling du projet
- Le workflow des tickets (triage → in-progress → review → done)

---

## Tools MCP disponibles

| Tool | Description | Utilisé par |
|---|---|---|
| `get_gitlab_issue` | Lit un ticket complet (titre, description, labels, milestone, commentaires) | Pathfinder, Planner |
| `list_gitlab_issues` | Liste les tickets avec filtres (état, labels, recherche) | Planner, Pathfinder, Onboarder |
| `get_gitlab_merge_request` | Lit une MR (titre, branches, état, changements) | Pathfinder |
| `list_gitlab_labels` | Liste tous les labels du projet | Onboarder, Planner |
| `list_gitlab_milestones` | Liste les milestones actifs/fermés | Onboarder, Planner |

---

## Architecture

```
servers/gitlab-mcp/
├── src/
│   ├── index.ts              ← Entrée MCP (5 tools)
│   ├── config.ts             ← Variables d'environnement
│   ├── client.ts             ← GitLabClient (axios + retry)
│   └── tools/
│       ├── get-issue.ts
│       ├── list-issues.ts
│       ├── get-merge-request.ts
│       ├── list-labels.ts
│       └── list-milestones.ts
├── dist/                     ← Compilé (gitignored)
└── package.json

skills/adapters/
├── gitlab-planner-protocol.md
├── gitlab-pathfinder-protocol.md
└── gitlab-onboarder-protocol.md
```

---

## Dépannage

### Token non reconnu

```
Error: GITLAB_PERSONAL_ACCESS_TOKEN is required
```

**Solution :** Vérifier que le token est bien configuré :
```bash
oh gitlab status
```

### Accès refusé (403)

Le token n'a pas les scopes requis. Recréer un token avec les scopes `api` et `read_user`.

### Projet non trouvé (404)

Le chemin de projet est incorrect ou le token n'a pas accès à ce projet. Vérifier :
- Le format : `mon-groupe/mon-sous-groupe/mon-projet`
- Les permissions : le token doit avoir au moins le rôle **Reporter** sur le projet

### Instance self-hosted inaccessible

Vérifier que `GITLAB_BASE_URL` est bien défini :
```bash
oh gitlab status
# Si absent :
oh gitlab setup
```

### Timeout sur les requêtes

Augmenter le timeout pour les instances lentes :
```bash
# Lors du setup
GITLAB_TIMEOUT=60000 oh gitlab setup
```

### Build du serveur MCP échoue

```bash
cd servers/gitlab-mcp
npm install
npm run build
```

---

## Limitations actuelles (v1)

- ❌ Lecture seule — aucune écriture de ticket, commentaire ou MR
- ❌ Diffs de MR non inclus (contenu de code trop volumineux pour le contexte agent)
- ❌ Pagination non exposée pour `list_gitlab_issues` au-delà de 100 tickets
- ❌ Webhooks GitLab non supportés
- ❌ GraphQL GitLab non utilisé (REST uniquement)

---

## Évolutions futures

- **v2** : Création de tickets via l'agent planner (`create_gitlab_issue`)
- **v3** : Lecture des diffs de MR pour le reviewer agent
- **v4** : Liens bidirectionnels tickets Beads ↔ tickets GitLab

---

## Ressources

- [Documentation API GitLab](https://docs.gitlab.com/ee/api/)
- [Personal Access Tokens GitLab](https://docs.gitlab.com/ee/user/profile/personal_access_tokens.html)
- [Référence CLI `oh service`](../reference/services.fr.md)
- [Architecture MCP servers](../../servers/README.md)

---

## Support

- `oh gitlab status` — vérifier la configuration
- `oh gitlab setup` — reconfigurer le service
- Problème persistant → reporter sur [GitHub Issues](https://github.com/anomalyco/opencode)
