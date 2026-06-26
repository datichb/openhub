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
```
