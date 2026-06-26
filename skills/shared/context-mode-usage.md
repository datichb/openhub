---
name: context-mode-usage
description: Règles d'usage des outils context-mode — choix du bon outil selon la nature de la commande, timeout obligatoire, gestion des processus non-terminants (serveurs dev, watchers).
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

Sans paramètre `timeout`, le spawner de processus n'installe **aucun timer** :

```js
// internals context-mode — si timeout === undefined, aucun kill n'est planifié
let timer = timeout === undefined ? undefined : setTimeout(() => kill(process), timeout);
```

Avec `concurrency ≥ 2`, trois workers parallèles attendent tous un `Promise.allSettled` —
si une commande bloque, **tout le batch est suspendu indéfiniment**.

### Règle : `timeout` est obligatoire sur tout appel `ctx_batch_execute`

```
// ✅ correct
ctx_batch_execute(commands: [...], timeout: 30000, concurrency: 3)

// ❌ interdit — aucun timer, hang possible si une commande bloque
ctx_batch_execute(commands: [...], concurrency: 3)
```

---

## Commandes non-terminantes — utiliser `ctx_execute` avec `background: true`

Pour lancer un serveur de dev, un watcher, ou tout process qui doit rester actif :

```
ctx_execute(
  language: "shell",
  code: "yarn dev",
  background: true
)
```

`background: true` détache le process après le timeout : il continue de tourner sans bloquer
l'agent. L'output partiel (démarrage, port, erreurs initiales) est retourné avant le détachement.

### Cas d'usage typiques

```
// Serveur de dev frontend
ctx_execute(language: "shell", code: "yarn dev", background: true)

// Serveur de dev backend
ctx_execute(language: "shell", code: "npm run start:dev", background: true)

// Watcher de compilation
ctx_execute(language: "shell", code: "tsc --watch", background: true)

// Build en watch mode
ctx_execute(language: "shell", code: "vite build --watch", background: true)
```

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

## Anti-patterns à éviter

```
// ❌ timeout absent
ctx_batch_execute(commands: [{ label: "dev", command: "yarn dev" }])

// ❌ commande non-terminante dans ctx_batch_execute
ctx_batch_execute(commands: [{ label: "server", command: "npm run dev" }], timeout: 5000)
// → le process est tué après 5s, serveur jamais démarré correctement

// ❌ watcher dans un batch parallèle — bloque les autres workers
ctx_batch_execute(
  commands: [
    { label: "typecheck", command: "tsc" },
    { label: "watch", command: "tsc --watch" },  // bloque le worker
  ],
  timeout: 30000,
  concurrency: 2
)

// ❌ commande shell timeout/gtimeout — indisponible sur macOS, mauvaise approche
ctx_execute(language: "shell", code: "timeout 10 yarn dev")
ctx_execute(language: "shell", code: "gtimeout 10 yarn dev")
```

---

## Capturer les erreurs de démarrage d'un serveur

Quand un serveur dev crashe au boot (port occupé, mauvaise config, dépendance manquante...),
`ctx_execute` avec `background: true` capture automatiquement les logs de démarrage
**avant** le détachement — l'erreur est dans l'output retourné.

### Toujours rediriger stderr vers stdout

```
// ✅ capture stdout + stderr (erreurs de boot incluses)
ctx_execute(
  language: "shell",
  code: "yarn dev 2>&1",
  background: true
)
```

Sans `2>&1`, les erreurs écrites sur stderr (la majorité des crashs) ne sont **pas** retournées.

### Lire l'output retourné

`ctx_execute` avec `background: true` retourne l'output partiel accumulé pendant le démarrage.
Si le serveur crashe immédiatement → l'output contient l'erreur complète.
Si le serveur démarre correctement → l'output contient les premières lignes (port, mode, URL).

```
// Exemple de retour en cas de crash :
// Error: listen EADDRINUSE: address already in use :::3000
//   at Server.setupListenHandle [as _listen2] (node:net:1738:16)

// Exemple de retour en cas de succès :
// vite v5.0.0  ready in 312 ms
// ➜  Local:   http://localhost:3000/
```

### Pattern complet — démarrer et vérifier

```
// 1. Lancer le serveur en background (stderr capturé)
ctx_execute(language: "shell", code: "yarn dev 2>&1", background: true)

// 2. Lire l'output retourné pour détecter un crash
//    → erreur présente : diagnostiquer sans relancer
//    → démarrage OK    : continuer avec l'URL affichée
```

Il n'est **pas nécessaire** d'utiliser `sleep`, `wait`, ou des boucles de polling —
le détachement intervient après que le process a produit ses premiers outputs.
