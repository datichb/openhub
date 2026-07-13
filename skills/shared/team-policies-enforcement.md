---
id: team-policies-enforcement
bucket: A
scope: all
condition: team_enabled
---

# Team Policies Enforcement Protocol

Ce skill definit comment les agents doivent respecter les team policies.
Il est complementaire a `team-awareness` et s'applique a tous les agents
pour lesquels au moins une policy est pertinente.

## Chargement des policies

Au debut de chaque session, appeler `team_policies` avec le projet courant
pour obtenir la liste des regles actives. Garder ces regles en memoire
pour la duree de la session.

## Matrice agent / policy

Chaque agent ne verifie que les policies pertinentes a ses actions :

| Agent | Policies a verifier |
|-------|---------------------|
| `orchestrator-dev` | `branch_naming` (avant delegation), `review_required` (avant fermeture ticket) |
| `developer`, `developer-refactor`, `developer-migrator` | `commit_format` (avant commit), `forbidden_patterns` custom (sur leur diff) |
| `reviewer` | `tests_required` (mentionner dans le rapport si non respecte) |
| `orchestrator` | `max_ticket_wip` (avant de proposer un nouveau ticket) |
| `designer` | Aucune policy code applicable |
| `documentarian` | Aucune policy code applicable |
| `pathfinder` | Aucune (read-only) |
| `onboarder` | Aucune (read-only) |

## Comportement selon l'enforcement

### `enforcement = "refuse"`

L'agent DOIT :
1. ARRETER l'action en cours
2. INFORMER l'utilisateur de la violation
3. INDIQUER quelle policy est violee et pourquoi
4. PROPOSER une correction

L'agent NE DOIT PAS :
- Contourner la policy
- Continuer malgre la violation
- Demander a l'utilisateur s'il veut ignorer (la policy est non-contournable)

### `enforcement = "warn"`

L'agent DOIT :
1. INFORMER l'utilisateur de la violation
2. MENTIONNER la policy et la raison
3. CONTINUER le travail si l'utilisateur ne reagit pas

L'agent PEUT :
- Proposer une correction
- Demander confirmation avant de continuer

## Verification par type de policy

### `branch_naming` (type: regex)

**Quand** : avant de creer une branche ou de deleguer a un developer
**Comment** : verifier que le nom de branche genere/propose matche la regex
**Si violation** : proposer un nom corrige qui respecte le pattern

### `commit_format` (type: regex)

**Quand** : avant chaque commit
**Comment** : verifier que le message de commit matche la regex
**Si violation** : reformuler le message pour respecter le format

### `forbidden_patterns` (type: forbidden_pattern)

**Quand** : avant de finaliser un commit, sur les lignes ajoutees/modifiees
**Comment** : scanner le diff pour les patterns interdits
**Si violation** : supprimer ou commenter les patterns interdits

### `review_required` (type: boolean)

**Quand** : avant de considerer un ticket comme "done"
**Comment** : verifier qu'une review a bien eu lieu
**Si violation** : informer que le ticket ne peut pas etre ferme sans review

### `tests_required` (type: boolean)

**Quand** : avant de considerer un ticket comme "done"
**Comment** : verifier que les tests passent
**Si violation** : informer et recommander d'ajouter/fixer les tests

### `max_ticket_wip` (type: limit)

**Quand** : avant de proposer un nouveau ticket a travailler
**Comment** : compter les claims actifs du membre
**Si violation** : ne pas proposer de nouveau ticket, suggerer de terminer un ticket en cours

## Regles custom

Pour les policies de type `forbidden_pattern` avec un nom commencant par `custom_` :
- Appliquer la meme logique que les patterns standard
- Le champ `scope` determine quoi verifier :
  - `diff_only` : uniquement les lignes ajoutees dans le diff
  - `modified_files` : tout le contenu des fichiers modifies
  - `all_files` : tous les fichiers du projet (rarement utilise par les agents)

## Integration dans le workflow

```
1. Debut de session → appeler team_policies
2. Avant chaque action (branch, commit, ticket close) :
   a. Identifier les policies pertinentes pour cette action
   b. Verifier chaque policy
   c. Si refuse-violation : STOP + informer
   d. Si warn-violation : informer + continuer
3. En fin de session : aucune action supplementaire (le CLI gere)
```
