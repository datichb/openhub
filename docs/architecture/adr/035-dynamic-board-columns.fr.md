# ADR-035 — Colonnes de board dynamiques et discovery tracker

## Statut

accepted

## Date

2026-09-15

## Contexte

Le board d'équipe comportait 6 colonnes en dur (TODO, IN PROGRESS, REVIEW, VALIDATION,
DONE, BLOCKED). Cette rigidité posait plusieurs problèmes :

- **Workflows non adaptables** — les équipes avec des étapes spécifiques (TESTING au lieu
  de VALIDATION, PREPROD, STAGING) ne pouvaient pas personnaliser le board. Chaque équipe
  devait se conformer aux 6 statuts imposés, même si leur workflow réel était différent.

- **Configuration tracker manuelle** — les mappings label→statut et la configuration des
  labels « pool » (tickets non-assignés) nécessitaient une édition manuelle du TOML.
  Aucun guidage n'existait pour cette étape critique du setup.

- **Pas d'onboarding tracker** — les nouveaux membres d'équipe n'avaient aucun wizard
  guidé pour configurer la connexion au tracker (GitLab/Jira) et ses mappings.

- **Couverture partielle des tickets** — le sync tracker ne récupérait que les tickets
  non-assignés, ignorant les tickets assignés à des non-membres ou les tickets
  correspondant aux labels de workflow de l'équipe.

## Décision

Nous avons décidé d'introduire un système de colonnes dynamiques et un wizard de
discovery tracker en 5 étapes.

### 1. `BoardConfig` avec colonnes personnalisables

Chaque colonne porte un rôle sémantique (`initial`, `active`, `terminal`, `blocked`)
qui permet au board de conserver son comportement (colonne de départ, colonnes terminales,
colonne bloquée) tout en acceptant des colonnes arbitraires.

### 2. Wizard de discovery en 5 étapes (TUI + CLI)

1. **Connexion tracker** — connexion au tracker configuré (GitLab/Jira)
2. **Découverte labels/statuts** — interrogation de l'API tracker pour lister les labels
   et statuts disponibles
3. **Suggestion de colonnes** — proposition de colonnes board basées sur le workflow
   détecté dans le tracker
4. **Mapping label→colonne** — proposition de mappings via un moteur heuristique bilingue
   (FR/EN) qui matche les labels aux colonnes
5. **Configuration pool** — configuration des labels « pool » pour l'import des tickets
   non-assignés

### 3. Column ID = claim status

L'identifiant de colonne devient directement le statut du claim. La couche de bridge
entre colonnes et statuts n'est plus nécessaire pour les colonnes personnalisées.

### 4. Rétrocompatibilité

Les 6 statuts existants restent comme valeurs par défaut. `IsValidBoardStatus` étend
la validation pour accepter tout column ID personnalisé en plus des constantes.

### 5. Transitions relaxées

Les statuts personnalisés ne sont pas contraints par `ValidTransitions` — toute
transition est autorisée (any→any) pour ne pas imposer de workflow rigide aux équipes
qui définissent leurs propres colonnes.

### 6. Sync tracker complet sur 'r'

La touche `r` sur le board déclenche un sync complet via l'API tracker avant le
git pull, récupérant tous les tickets correspondant aux labels de workflow (pas
uniquement les tickets non-assignés).

## Conséquences

### Positives

- Les équipes peuvent personnaliser leur workflow sans modification de code
- Le wizard de discovery réduit le setup de « édition TOML manuelle » à un flow guidé de 2 minutes
- Tous les tickets correspondant aux labels de workflow apparaissent sur le board (plus seulement les non-assignés)
- Le composant `InlineWizardView` (ADR-034) est réutilisé pour le wizard TUI

### Négatives / Compromis

- `ValidTransitions` est contourné pour les statuts personnalisés — moins de garde-fous sur les changements de statut manuels
- Augmentation des appels API sur la touche `r` (5-15s vs 1-2s pour un git pull seul)
- Les valeurs de claim status ne sont plus limitées aux 6 constantes — tout column ID est valide
- L'API serve utilise toujours `DefaultBoardConfig` — les colonnes personnalisées ne sont pas exposées via HTTP

## Alternatives rejetées

| Alternative | Raison du rejet |
|-------------|----------------|
| Visibilité/ordre uniquement — permettre de masquer/réordonner les 6 colonnes par défaut sans en créer de nouvelles | Trop limité — ne supporte pas les workflows avec des étapes custom (PREPROD, STAGING, QA) |
| Statuts entièrement dynamiques dans `claims.go` — remplacer toutes les constantes de statut par un registre dynamique | Trop invasif, risque de régression élevé. L'approche choisie conserve les constantes comme fallback et étend avec `IsValidBoardStatus` |
| API GitLab Boards pour la discovery — utiliser `GET /api/v4/projects/:id/boards` pour inférer les colonnes depuis les board lists GitLab | Toutes les équipes n'utilisent pas les boards GitLab, et le moteur heuristique couvre plus de patterns (labels, statuts Jira, matching bilingue) |
