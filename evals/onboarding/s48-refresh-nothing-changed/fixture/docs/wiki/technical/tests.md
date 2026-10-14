---
updated: 2026-10-01
confidence: mixed
agents: [onboarder]
---

# Stratégie de tests — todo

## Frameworks
- `testing` de la bibliothèque standard.
  — `CONFIRMÉ` · onboarder · 2026-10-01 · go.mod

## Organisation
- Un fichier `_test.go` à côté du code testé, dans le même paquet.
  — `CONFIRMÉ` · onboarder · 2026-10-01 · README.md

## Seuil de couverture
- Aucun seuil.
  — `CONFIRMÉ` · onboarder · 2026-10-01 · README.md

## Philosophie
- Tests sur un fichier temporaire (`t.TempDir()`), jamais sur le `todo.txt` réel.
  — `DÉDUIT` · onboarder · 2026-10-01

## Commandes

```
go test ./...
```

## Conventions de nommage des tests
- `Test<Fonction>` ou `Test<Fonction1><Fonction2>` pour un scénario qui en enchaîne plusieurs.
  — `DÉDUIT` · onboarder · 2026-10-01
