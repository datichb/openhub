---
updated: 2026-10-01
confidence: mixed
agents: [onboarder]
---

# Architecture — todo

## Structure globale
- `main.go` : la commande `todo` (sous-commandes `add` et `list`), qui lit `os.Args` et appelle le stockage.
  — `CONFIRMÉ` · onboarder · 2026-10-01 · main.go:12
- `internal/store/store.go` : le paquet `store`, qui garde les tâches dans `todo.txt`, une par ligne (`Open`, `Add`, `List`).
  — `CONFIRMÉ` · onboarder · 2026-10-01 · internal/store/store.go:2

## Découpage en couches
- Deux couches : la commande (entrées et sorties) et le stockage (fichier) ; le stockage ne sait rien de la ligne de commande.
  — `CONFIRMÉ` · onboarder · 2026-10-01 · main.go:9

## Communication entre modules
- `main.go` importe `example.com/todo/internal/store` et appelle `store.Open("todo.txt")`.
  — `CONFIRMÉ` · onboarder · 2026-10-01 · main.go:17

## Décisions architecturales notables
- Fichier texte plutôt qu'une base de données : outil personnel, une tâche par ligne.
  — `DÉDUIT` · onboarder · 2026-10-01 · README.md

## Points de fragilité connus
- Une tâche qui contient un retour à la ligne est lue comme deux tâches.
  — `DÉDUIT` · onboarder · 2026-10-01 · internal/store/store.go
