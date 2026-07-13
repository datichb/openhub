---
id: parallel-coordination
bucket: B
agent: orchestrator-dev
condition: parallel_mode
---

# Parallel Coordination Protocol

Ce skill est injecte quand l'orchestrator-dev tourne dans un contexte parallele
(plusieurs sessions sur differents tickets en meme temps).

## Contexte

En mode parallele, plusieurs agents travaillent simultanement sur des tickets
differents, chacun dans son propre worktree. Un coordinateur externe surveille
les fichiers touches par chaque session pour detecter les conflits potentiels.

## Regles en mode parallele

### 1. Independance

- Travailler UNIQUEMENT sur le ticket assigne
- Ne PAS toucher aux fichiers qui ne sont pas lies au ticket
- Minimiser les modifications sur les fichiers partages (config, index, etc.)

### 2. Fichiers partages

Si tu dois modifier un fichier susceptible d'etre touche par d'autres sessions
(fichiers de config, index, barrel exports, etc.) :

- Etre le plus chirurgical possible (modifier le minimum)
- Privilegier l'ajout en fin de fichier plutot que l'insertion au milieu
- Si c'est un fichier d'export (index.ts, mod.rs, etc.) : ajouter ta ligne en fin

### 3. Commits atomiques

- Faire des commits petits et frequents
- Chaque commit doit etre autonome et complet
- Cela facilite le merge ulterieur en cas de conflit

### 4. Nommage coherent

- Respecter les conventions de nommage du projet
- Les branch names suivent le pattern standard (policies)
- Les commits suivent le format defini (policies)

## Ce que tu ne controles PAS

- Le coordinateur gere le merge des branches apres completion
- Tu n'as PAS besoin de merger toi-meme
- Tu n'as PAS besoin de rebase sur main
- Tu travailles sur ta branche isolee dans ton worktree

## Fin de session

Quand ton ticket est termine :
1. Tous les tests passent dans ton worktree
2. Le code est propre (pas de TODO temporaires)
3. Les fichiers non pertinents ne sont pas modifies
4. Tu peux terminer normalement — le coordinateur gere la suite
