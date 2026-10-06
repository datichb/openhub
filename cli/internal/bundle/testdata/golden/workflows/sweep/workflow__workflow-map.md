---
name: workflow-map
description: "Carte du workflow sweep : agents, délégations, checkpoints, modes et sorties. GÉNÉRÉ AUTOMATIQUEMENT depuis le YAML du workflow."
---

# Carte du workflow `sweep`

Atteindre un objectif transverse en le découpant en sous-tâches indépendantes, puis vérifier le résultat

- Agent d'entrée : `conductor`
- Risque : `write`

Cette carte est la seule référence sur l'enchaînement : n'invente ni agent, ni étape, ni checkpoint qui n'y figure pas.

## Agents du workflow

| Agent | Rôle | Mode | Après | Peut déléguer à |
|---|---|---|---|---|
| `conductor` | workflow (entrée) | primary | — | `developer`, `developer-migrator`, `developer-refactor` |
| `developer` | workflow | subagent | `cp-plan` | — |
| `developer-refactor` | workflow | subagent | `cp-plan` | — |
| `developer-migrator` | workflow | subagent | `cp-plan` | — |

Règles de délégation :
- Tu ne lances (outil `task`) que les agents de la colonne « Peut déléguer à » de ta ligne. Tout autre appel est refusé par l'outil.
- Un agent avec une valeur « Après » ne se lance qu'une fois ce checkpoint passé (ou cet agent terminé).
- Les agents `independent` sont disponibles à la demande, hors de la chaîne principale.

## Checkpoints (dans l'ordre)

| Checkpoint | manuel | semi-auto | auto |
|---|---|---|---|
| **cp-plan** — Valider le découpage | pause | pause | auto |
| **cp-recap** — Récap du sweep | pause | pause | auto |

- **cp-plan** : Présenter la liste des sous-tâches (description, périmètre de fichiers) avant toute exécution.
- **cp-recap** : Résultat par sous-tâche et résultat de la vérification.

Comportements :
- `pause` : arrête-toi, présente l'état et demande la validation de l'utilisateur avec l'outil `question` ; ne continue jamais sans réponse.
- `auto` : annonce le passage du checkpoint en une ligne, puis continue.
- `skip` : le checkpoint est ignoré dans ce mode.
- `conditional` : évalue la condition ; si elle est remplie, comporte-toi comme `pause`, sinon comme `auto`.
- Un checkpoint obligatoire n'est jamais sauté, quel que soit le mode.

## Mode de workflow

- Modes autorisés : `manuel`, `semi-auto`, `auto` · mode par défaut : `semi-auto`.
- Le mode de la session est indiqué dans le premier message (`Mode de workflow : <mode>`) ; sans indication, applique le mode par défaut. Ne le redemande pas.
- Quand tu lances un agent avec l'outil `task`, inclus la ligne `Mode de workflow : <mode>` dans le prompt.
- Disjoncteur : après 20 lancements `task` consécutifs sans interaction de l'utilisateur, fais une pause et demande s'il faut continuer, faire une revue ou arrêter. Le compteur repart de zéro à chaque échange avec l'utilisateur.

## Sorties à déclarer

En fin de travail, annonce chaque sortie produite sur une ligne `Sortie <id> : <valeur>` :
- `branch` (branch) : Branche de travail
