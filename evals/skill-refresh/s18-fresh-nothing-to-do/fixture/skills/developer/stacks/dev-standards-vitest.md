---
name: dev-standards-vitest
description: "Utiliser quand le projet utilise Vitest (configuration, mocks, couverture) et que le ticket touche ce code."
versions:
  vitest: "4.0"
reviewed: TODAY
---

# Skill — Standards Vitest

> Versions de référence : Vitest 4.0.

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
      include: ['src/**'], // compter aussi les fichiers jamais importés par un test
    },
  },
})
```

Plusieurs paquets : déclarer chaque paquet dans `test.projects`.
