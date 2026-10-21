---
name: workflow-map
description: "Carte du workflow merge-conflicts : agents, délégations, checkpoints, modes et sorties. GÉNÉRÉ AUTOMATIQUEMENT depuis le YAML du workflow."
---

# Carte du workflow `merge-conflicts`

Terminer un merge arrêté sur des conflits (lancé par oh dans un fan-out, ou à la main) en gardant les deux côtés, vérifier, puis le conclure par un commit ; jamais de push

- Agent d'entrée : `merger`
- Risque : `write`
- Commandes Beads autorisées : `show`

Cette carte est la seule référence sur l'enchaînement : n'invente ni agent, ni étape, ni checkpoint qui n'y figure pas.

## Agents du workflow

| Agent | Rôle | Mode | Après | Peut déléguer à |
|---|---|---|---|---|
| `merger` | workflow (entrée) | primary | — | — |

Règles de délégation :
- Tu ne lances (outil `task`) que les agents de la colonne « Peut déléguer à » de ta ligne. Tout autre appel est refusé par l'outil.
- Un agent avec une valeur « Après » ne se lance qu'une fois ce checkpoint passé (ou cet agent terminé).
- Les agents `independent` sont disponibles à la demande, hors de la chaîne principale.

## Checkpoints

Aucun checkpoint dans ce workflow : avance sans pause imposée.
<!-- oh:when role=entry -->
Pour toute confirmation (et avant toute action irréversible), utilise l'outil `question`, sans nommer de checkpoint.
<!-- /oh:when -->

## Mode de workflow

- Modes autorisés : `auto` · mode par défaut : `auto`.
- Le mode de la session est indiqué dans le premier message (`Mode de workflow : <mode>`) ; sans indication, applique le mode par défaut. Ne le redemande pas.
- Quand tu lances un agent avec l'outil `task`, inclus la ligne `Mode de workflow : <mode>` dans le prompt.
