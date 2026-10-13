---
name: dev-standards-jest
description: "Utiliser quand le projet utilise Jest (configuration, mocks, matchers) et que le ticket touche ce code."
versions:
  jest: "30"
reviewed: TODAY
---

# Skill — Standards Jest

> Versions de référence : Jest 30.

## Règles absolues

❌ Jamais d'alias de matcher retiré (`toBeCalled`) : utiliser `toHaveBeenCalled`
✅ Chaque mock est remis à zéro entre deux tests (`clearMocks: true`)
