---
name: context-mode-usage
description: Règles d'usage des outils context-mode — choix du bon outil selon la nature de la commande, timeout obligatoire, gestion des processus non-terminants (serveurs dev, watchers).
plugin: context-mode
---

## Règle fondamentale — deux catégories de commandes

Avant d'exécuter une commande, déterminer si elle **se termine d'elle-même** ou non.

| Type de commande | Exemples | Outil correct |
|---|---|---|
| **Se termine seule** | `tsc`, `jest`, `git diff`, `ls`, `bd show`, `curl` | `ctx_batch_execute` avec `timeout` obligatoire |
| **Ne se termine pas** | `yarn dev`, `npm run dev`, `vite`, `nodemon`, `tail -f`, `webpack --watch` | `ctx_execute` avec `background: true` |

**Ne jamais passer une commande non-terminante dans `ctx_batch_execute`.**

---

## Pourquoi `ctx_batch_execute` sans timeout est dangereux

Sans `timeout`, aucun timer n'est installé. Avec `concurrency ≥ 2`, si une commande bloque, **tout le batch est suspendu indéfiniment**.

### Règle : `timeout` est obligatoire sur tout appel `ctx_batch_execute`

---

## Commandes non-terminantes — utiliser `ctx_execute` avec `background: true`

Pour lancer un serveur de dev, un watcher, ou tout process qui doit rester actif, utiliser `ctx_execute(language: "shell", code: "<cmd> 2>&1", background: true)`.

`background: true` détache le process après le timeout : il continue de tourner sans bloquer l'agent. L'output partiel (démarrage, port, erreurs initiales) est retourné avant le détachement.

Exemples de commandes : `yarn dev`, `npm run start:dev`, `tsc --watch`, `vite build --watch`.

---

## Tableau de décision rapide

```
La commande se termine toute seule ?
├── OUI → ctx_batch_execute  (+ timeout obligatoire)
│         Plusieurs commandes indépendantes ? → concurrency: 2-4
│         Commandes séquentielles ou dépendantes ? → concurrency: 1
│
└── NON → ctx_execute avec background: true
          (yarn dev, vite, nodemon, watchers, tail -f, etc.)
          ⚠️  Arrêter le process en fin de tâche : Bash("pkill -f '...'")
```

---

## Valeurs de `timeout` recommandées

> **Important — `timeout` est un paramètre de l'outil MCP (millisecondes), pas une commande shell.**
> Ne jamais utiliser `timeout yarn dev` ou `gtimeout yarn dev` dans le champ `code` :
> la commande `timeout` n'existe pas nativement sur macOS (`gtimeout` nécessite GNU coreutils)
> et est de toute façon le mauvais outil — c'est le paramètre de l'outil qui gère l'interruption.

| Type de commande | Timeout suggéré |
|---|---|
| Lecture/listing (`ls`, `bd show`, `git status`) | `10000` (10s) |
| Compilation, lint, typecheck | `60000` (60s) |
| Tests unitaires | `120000` (2min) |
| Tests d'intégration / E2E | `300000` (5min) |
| Build complet | `300000` (5min) |
| Commande réseau / curl / fetch | `30000` (30s) |

---

## Anti-patterns — règles compactes

| Anti-pattern | Pourquoi c'est faux | Correct |
|---|---|---|
| `ctx_batch_execute` sans `timeout` | Aucun timer → hang indéfini | Toujours passer `timeout` |
| Commande non-terminante dans `ctx_batch_execute` | Process tué au timeout, jamais démarré | `ctx_execute` avec `background: true` |
| Watcher dans un batch parallèle | Bloque le worker indéfiniment | Séparer : batch pour les terminantes, `ctx_execute` pour le watcher |
| `timeout`/`gtimeout` dans le champ `code` | Indisponible sur macOS, mauvaise couche | Utiliser le paramètre `timeout` de l'outil MCP |
| Fin de tâche sans `pkill` du process background | Port occupé, process zombie | `Bash("pkill -f '<cmd>'")` avant de clore |

---

## Capturer les erreurs de démarrage d'un serveur

**Toujours rediriger stderr vers stdout** (`2>&1`) — sans cela, les erreurs de crash ne sont pas retournées.

`ctx_execute` avec `background: true` retourne l'output partiel accumulé pendant le démarrage :
- Crash immédiat → l'output contient l'erreur complète
- Démarrage OK → l'output contient les premières lignes (port, mode, URL)

**Pattern :** lancer avec `2>&1` + `background: true`, lire l'output retourné pour détecter un crash. Pas besoin de `sleep`, `wait` ou polling.

---

## Arrêter un process background — obligatoire

**Tout process lancé en background doit être arrêté avant la fin de la tâche, sans exception.**
Un process oublié occupe le port pour la session suivante et consomme des ressources.

**Arrêt par nom :** `Bash("pkill -f 'yarn dev'")`
**Arrêt par PID (plus précis) :** capturer le PID au démarrage avec `& echo "PID:$!"`, puis `Bash("kill <pid>")`.

**Séquence obligatoire :** démarrer → utiliser → `pkill` AVANT de clore la tâche.
