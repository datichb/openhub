---
name: onboarder-payload
description: "Utiliser en fin d'onboarding : statuts de ton retour et sections de ton rapport (stack, conventions, incertitudes)."
statuses: [contexte-établi, contexte-partiel, bloqué]
when: role=subagent
---

# Payload de l'onboarder

## Statuts

| Statut | Condition |
|---|---|
| `contexte-établi` | Exploration complète, fichiers produits, contexte suffisant pour démarrer une feature |
| `contexte-partiel` | Exploration faite, avec des zones d'incertitude significatives : démarrable avec précautions |
| `bloqué` | Exploration impossible ou contexte insuffisant : intervention manuelle requise |

## Rapport

Titre : `Onboarding — <projet>`. Seulement ce qui a été observé dans le projet, jamais inventé ; chaque section vide porte une mention explicite. Sections :

- `### Rapport d'onboarding` : les observations qualitatives, en 3 à 5 phrases au moins.
- `### Stack technique` : langages, frameworks, base de données, infrastructure, outils, versions clés.
- `### Contexte métier` : domaines, utilisateurs cibles, concepts clés, glossaire, pattern d'architecture.
- `### Design et maquettes`, `### Stratégie de test`, `### Conventions identifiées`.
- `### Dette technique détectée` (avec sa criticité), `### Zones d'incertitude`.
- `### Fichiers de contexte produits` : chemins écrits (ou refus de les écrire).
