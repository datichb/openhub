# ADR-020 — Fusion des agents ux-designer et ui-designer en agent designer unifié

## Statut

Accepté

## Date

2026-06-30

## Contexte

Avant cette décision, cinq agents du hub disposaient d'un accès direct au MCP Figma :
`planner`, `pathfinder`, `onboarder`, `ux-designer` et `ui-designer`. Chacun embarquait
un skill adaptateur Figma dédié (`figma-planner-protocol`, `figma-pathfinder-protocol`,
`figma-onboarder-protocol`, `figma-ux-designer-protocol`, `figma-ui-designer-protocol`),
représentant 1 285 lignes de contenu Figma dupliqué ou décliné pour chaque contexte.

Par ailleurs, les agents `ux-designer` et `ui-designer` étaient rarement invoqués de
façon indépendante : dans plus de 90 % des cas, une feature nécessitait les deux
perspectives (UX et UI) ou au moins une exploration préalable des maquettes existantes.
Cette séparation en deux agents distincts forçait les coordinateurs à orchestrer deux
invocations `task` séquentielles pour un travail conceptuellement unitaire.

La dispersion de l'accès Figma sur cinq agents rendait la gouvernance de l'intégration
difficile : toute mise à jour du MCP Figma nécessitait des modifications coordonnées
dans cinq fichiers, avec un risque de dérive entre les versions.

## Décision

Nous avons décidé de fusionner `ux-designer` et `ui-designer` en un agent unique
`designer`, capable d'opérer en quatre modes distincts :

- **Mode `recon`** — exploration Figma : recherche de maquettes, extraction des tokens,
  détection du design system. Remplace les 3 skills adaptateurs Figma des agents de
  planification (planner, pathfinder, onboarder).
- **Mode `ux`** — spécifications UX : flows utilisateurs, heuristiques Nielsen, critères
  d'acceptance. Reprend le workflow de l'ancien `ux-designer`.
- **Mode `ui`** — spécifications UI : design tokens, composants, variants, guidelines.
  Reprend le workflow de l'ancien `ui-designer`.
- **Mode `ux+ui`** — traitement complet UX puis UI en une seule session.

L'agent `designer` est désormais le **seul agent du hub avec accès MCP Figma**. Les
agents `planner`, `pathfinder` et `onboarder` délèguent tous leurs besoins Figma au
`designer` via `task` (mode `recon`), au lieu d'appeler le MCP directement.

Les cinq skills adaptateurs Figma dédiés sont supprimés. Deux nouveaux skills de
protocole Figma sont créés dans `designer/` : `figma-recon-protocol` (exploration
légère) et `figma-deep-protocol` (analyse approfondie des structures de composants).

## Conséquences

### Positives

- **−678 lignes** de prompt sur les agents de planification (suppression des 3 skills
  adaptateurs Figma et de la logique MCP inline).
- **−9 fichiers** au total : 2 agents supprimés (`ux-designer`, `ui-designer`),
  5 skills adaptateurs Figma supprimés, 2 skills d'exécution fusionnés en
  `designer-execution-modes`.
- Architecture Figma centralisée : une seule source de vérité pour l'accès MCP,
  les mises à jour du MCP Figma n'impactent qu'un seul agent.
- Expérience utilisateur simplifiée : un seul agent à connaître pour tous les besoins
  design, avec un paramètre `Mode:` explicite dans le prompt.
- Délégation cohérente : `planner`, `pathfinder`, `onboarder` utilisent le même
  pattern `task: designer` + `Mode: recon` que pour les autres délégations.

### Négatives / Compromis

- **+30 s de latence** pour les explorations Figma depuis `planner`/`pathfinder`/
  `onboarder` : l'ajout d'une invocation `task` intermédiaire introduit un saut
  de session supplémentaire par rapport à l'appel MCP direct.
- **Breaking change** : toute configuration `opencode.json` référençant `ux-designer`
  ou `ui-designer` doit être mise à jour. Aucun alias automatique n'est fourni.
- Les permissions `task` des agents `orchestrator`, `planner`, `pathfinder` et
  `onboarder` doivent être mises à jour pour autoriser `designer` à la place de
  `ux-designer` et `ui-designer`.

## Migration

Voir le guide de migration complet :
[docs/guides/migration-designer-fusion.md](../../guides/migration-designer-fusion.md)
