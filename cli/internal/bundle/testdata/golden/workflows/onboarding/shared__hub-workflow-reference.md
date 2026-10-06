---
name: hub-workflow-reference
description: "Agents et délégations du workflow onboarding. GÉNÉRÉ AUTOMATIQUEMENT depuis le YAML du workflow."
---

# Référence du workflow `onboarding`

Agent d'entrée : `onboarder`. Seuls les agents ci-dessous existent dans cette session.

## Agents du workflow

| Agent | Rôle | Mode | Après | Peut déléguer à |
|---|---|---|---|---|
| `onboarder` | workflow (entrée) | primary | — | — |

Règles de délégation :
- Tu ne lances (outil `task`) que les agents de la colonne « Peut déléguer à » de ta ligne. Tout autre appel est refusé par l'outil.
- Un agent avec une valeur « Après » ne se lance qu'une fois ce checkpoint passé (ou cet agent terminé).
- Les agents `independent` sont disponibles à la demande, hors de la chaîne principale.
