---
name: workflow-map
description: "Carte du workflow ticket : agents, délégations, checkpoints, modes et sorties. GÉNÉRÉ AUTOMATIQUEMENT depuis le YAML du workflow."
---

# Carte du workflow `ticket`

Implémenter un ticket Beads

- Agent d'entrée : `orchestrator-dev`
- Risque : `write`
- Commandes Beads autorisées : `show`, `update`, `close`

Cette carte est la seule référence sur l'enchaînement : n'invente ni agent, ni étape, ni checkpoint qui n'y figure pas.

## Agents du workflow

| Agent | Rôle | Mode | Après | Peut déléguer à |
|---|---|---|---|---|
| `orchestrator-dev` | workflow (entrée) | primary | — | `developer`, `reviewer` |
| `developer` | workflow | subagent | `cp-1` | — |
| `reviewer` | workflow | subagent | `developer` | — |
| `documentarian` | independent | primary | — | — |

Règles de délégation :
- Tu ne lances (outil `task`) que les agents de la colonne « Peut déléguer à » de ta ligne. Tout autre appel est refusé par l'outil.
- Un agent avec une valeur « Après » ne se lance qu'une fois ce checkpoint passé (ou cet agent terminé).
- Les agents `independent` sont disponibles à la demande, hors de la chaîne principale.

## Checkpoints (dans l'ordre)

| Checkpoint | manuel | semi-auto | auto | distant |
|---|---|---|---|---|
| **cp-1** — Démarrer | pause | auto | auto | auto |
| **cp-2** — Commit ou correction (obligatoire) | pause | pause | pause | defer |
| **cp-3** — Revue | pause | conditional | auto | defer |

- **cp-2** : Montrer le diff avant de committer.
- **cp-3** (conditional) : le diff dépasse 300 lignes

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
- Disjoncteur : après 12 lancements `task` consécutifs sans interaction de l'utilisateur, fais une pause et demande s'il faut continuer, faire une revue ou arrêter. Le compteur repart de zéro à chaque échange avec l'utilisateur.

## Sorties à déclarer

En fin de travail, annonce chaque sortie produite sur une ligne `Sortie <id> : <valeur>` :
- `branch` (branch) : Branche de travail
- `mr` (merge_request)
