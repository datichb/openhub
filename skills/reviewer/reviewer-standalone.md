---
name: reviewer-standalone
description: Parcours d'exécution du reviewer en mode standalone (invoqué directement par l'utilisateur) — rapport de review complet, enrichissement des documents vivants proposé en fin de session, sans bloc handoff orchestrator-dev.
---

# Skill — Parcours Reviewer Standalone

> Ce skill est chargé automatiquement quand le reviewer est invoqué directement par l'utilisateur (aucun `[SKILL:...]` injecté dans le prompt).

## Principe fondamental

En mode standalone, le rapport de review est **directement affiché** dans la discussion et l'utilisateur prend la décision finale lui-même.

---

## Comportement standalone

1. Exécuter le workflow de review complet (voir skill `review-protocol`)
2. Produire le rapport structuré complet au format défini dans `review-protocol`
3. Appliquer le skill `living-docs-enrichment` : proposer l'enrichissement des documents vivants via l'outil `question`
4. **Ne pas** produire le bloc `## Retour vers orchestrator-dev`

---

## Format final (standalone)

Produire uniquement le rapport de review (voir skill `review-protocol` pour le format exact), **sans** le bloc `## Retour vers orchestrator-dev`.

L'utilisateur consulte le rapport et décide lui-même de l'action à prendre (commit, corriger, ignorer).
