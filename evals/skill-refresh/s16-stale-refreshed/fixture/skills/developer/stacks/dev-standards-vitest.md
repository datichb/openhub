---
name: dev-standards-vitest
description: "Utiliser quand le projet utilise Vitest (configuration, mocks, couverture) et que le ticket touche ce code."
versions:
  vitest: "3.0"
reviewed: 2025-03-03
---

# Skill — Standards Vitest

> Versions de référence (mars 2025) : Vitest 3.0.

## Règles absolues

❌ Jamais de test qui dépend de l'ordre d'exécution
✅ Chaque mock est remis à zéro entre deux tests

## Configuration

```ts
// vitest.config.ts
import { defineConfig } from 'vitest/config'

export default defineConfig({
  test: {
    coverage: {
      provider: 'v8',
      all: true, // compter aussi les fichiers jamais importés par un test
    },
  },
})
```

Plusieurs paquets : déclarer chaque paquet dans un fichier `vitest.workspace.ts`.
