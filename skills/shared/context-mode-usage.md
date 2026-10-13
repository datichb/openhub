---
name: context-mode-usage
description: "Utiliser quand les outils ctx_* (plugin context-mode) sont présents : choix de l'outil, timeout obligatoire, commandes non terminantes."
plugin: context-mode
---

## Deux catégories de commandes

Avant d'exécuter une commande, détermine si elle **se termine d'elle-même**.

| Type | Exemples | Outil |
|---|---|---|
| **Se termine seule** | `tsc`, `jest`, `git diff`, `ls`, `bd show`, `curl` | `ctx_batch_execute`, `timeout` obligatoire ; `concurrency` 2 à 4 si indépendantes, 1 si dépendantes |
| **Ne se termine pas** | `yarn dev`, `npm run dev`, `vite`, `nodemon`, `tail -f`, `webpack --watch`, `tsc --watch` | `ctx_execute(language: "shell", code: "<cmd> 2>&1", background: true)` |

- **Ne jamais passer une commande non terminante dans `ctx_batch_execute`** : tuée au timeout sans avoir démarré, ou, dans un batch parallèle, elle bloque le worker indéfiniment.
- **`timeout` est obligatoire sur tout appel `ctx_batch_execute`** : sans lui, aucun timer, et avec `concurrency ≥ 2` une commande bloquée suspend tout le batch. C'est un paramètre de l'outil (millisecondes), jamais une commande shell : ni `timeout yarn dev` ni `gtimeout` dans `code` (absents de macOS).
- Valeurs : lecture ou listing 10000 ; compilation, lint, typecheck 60000 ; tests unitaires 120000 ; tests d'intégration, E2E ou build complet 300000 ; réseau 30000.

## Process en arrière-plan

`background: true` détache le process après le timeout et rend l'output partiel du démarrage. **Toujours rediriger stderr** (`2>&1`) : un crash immédiat donne alors l'erreur complète, un démarrage réussi les premières lignes (port, URL) ; pas besoin de `sleep` ni de polling.

**Arrêt obligatoire** avant de clore la tâche (port occupé, process zombie sinon) : note le PID au lancement (`& echo "PID:$!"`), puis `Bash("kill <pid>")`. Pas de `pkill -f` : il demande une confirmation et peut tuer le serveur de la session.
