---
name: workflow-map
description: "Carte du workflow feature : agents, délégations, checkpoints, modes et sorties. GÉNÉRÉ AUTOMATIQUEMENT depuis le YAML du workflow."
---

# Carte du workflow `feature`

Réaliser une feature de bout en bout (exploration ou planification, design, implémentation, review)

- Agent d'entrée : `orchestrator`
- Risque : `write`

Cette carte est la seule référence sur l'enchaînement : n'invente ni agent, ni étape, ni checkpoint qui n'y figure pas.

## Agents du workflow

| Agent | Rôle | Mode | Après | Peut déléguer à |
|---|---|---|---|---|
| `orchestrator` | workflow (entrée) | primary | — | `designer`, `documentarian`, `orchestrator-dev`, `pathfinder`, `planner` |
| `pathfinder` | workflow | subagent | — | `designer`, `documentarian` |
| `planner` | workflow | subagent | — | `designer`, `documentarian` |
| `designer` | workflow | subagent | — | — |
| `orchestrator-dev` | workflow | subagent | `cp-0` | `developer`, `developer-migrator`, `developer-refactor`, `documentarian`, `reviewer` |
| `developer` | workflow | subagent | `cp-1` | `documentarian` |
| `developer-refactor` | workflow | subagent | `cp-1` | `documentarian` |
| `developer-migrator` | workflow | subagent | `cp-1` | `documentarian` |
| `reviewer` | workflow | subagent | — | `documentarian` |
| `documentarian` | independent | primary | — | — |

Règles de délégation :
- Tu ne lances (outil `task`) que les agents de la colonne « Peut déléguer à » de ta ligne. Tout autre appel est refusé par l'outil.
- Un agent avec une valeur « Après » ne se lance qu'une fois ce checkpoint passé (ou cet agent terminé).
- Les agents `independent` sont disponibles à la demande, hors de la chaîne principale.

## Checkpoints (dans l'ordre)

| Checkpoint | manuel | semi-auto | auto |
|---|---|---|---|
| **cp-0** — Valider le plan (obligatoire) | pause | pause | pause |
| **cp-spec** — Valider la spec UX/UI | conditional | conditional | conditional |
| **cp-1** — Démarrer le ticket | pause | auto | auto |
| **cp-2** — Commit ou correction (obligatoire) | pause | pause | pause |
| **cp-3** — Ticket suivant | pause | auto | auto |
| **cp-feature** — Récap de la feature | pause | pause | auto |

- **cp-0** : Afficher le tableau des tickets (ordre de traitement, agent prévu) et attendre la confirmation avant toute implémentation.
- **cp-spec** (conditional) : le designer a produit une spécification UX ou UI pour la feature
- **cp-feature** : Récap global en fin de feature (tickets traités, commits, points ouverts).

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
- `tickets` (beads-ids) : Tickets de la feature
- `branch` (branch) : Branche de travail
