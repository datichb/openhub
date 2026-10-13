---
name: pathfinder-payload
description: "Utiliser en fin de reconnaissance : recommandation de ton retour (direct, escalade-planner) et sections de ton rapport."
statuses: [direct, escalade-planner, reconnaissance-partielle]
when: role=subagent
---

# Payload du pathfinder

## Statuts

| Statut | Condition |
|---|---|
| `direct` | Reconnaissance complète ; traitement direct recommandé (complexité XS à M) |
| `escalade-planner` | Reconnaissance complète ; planification complète recommandée (L, XL ou signaux forts) |
| `reconnaissance-partielle` | Exploration incomplète : une clarification manque (dans `### Questions bloquantes`) |

## Rapport

Titre : `Pathfinder — <feature>`. En tête : `**Complexité** : XS | S | M | L | XL` et `**Date**`. Sections :

- `### Contexte rapide` : ce qui a été compris de la demande, en 2 ou 3 phrases.
- `### Exploration` : fichiers et modules clés (avec leur raison), tickets Beads liés, patterns réutilisables.
- `### Structure proposée` : epic éventuel, tickets estimés (titre, type, priorité, durée, dépendances), total estimé.
- `### Questions ouvertes` : métier, technique, design (« aucune » si vide).
- `### Risques identifiés` : niveau, description, impact.
- `### Signaux détectés` : tableau UX/UI, sécurité, performance, accessibilité, architecture (❌ aucun, ⚠️ faible, ✅ fort).
- `### Recommandation` : direct ou escalade, et sa justification.
- `### Handoff vers planner` (escalade seulement) : contexte exploré, structure draft, questions restantes, signaux et risques détaillés, pour que le planner ne refasse pas l'exploration.
