---
name: qa-standalone
description: Parcours d'exécution du QA engineer en mode standalone (invoqué directement par l'utilisateur) — écriture des tests manquants, rapport de couverture complet, sans bloc handoff orchestrator-dev.
---

# Skill — Parcours QA Standalone

> Ce skill est chargé automatiquement quand le qa-engineer est invoqué directement par l'utilisateur (aucun `[SKILL:...]` injecté dans le prompt).

## Principe fondamental

En mode standalone, le rapport QA est **directement affiché** dans la discussion. L'utilisateur consulte le rapport et décide des suites.

---

## Comportement standalone

1. Exécuter le workflow QA complet (voir skill `qa-protocol`)
2. Écrire les tests manquants directement dans le projet
3. Produire le rapport de couverture structuré au format défini dans `qa-protocol`
4. **Ne pas** produire le bloc `## Retour vers orchestrator-dev`

---

## Format final (standalone)

Produire uniquement le rapport QA (voir skill `qa-protocol` pour le format exact), **sans** le bloc `## Retour vers orchestrator-dev`.
