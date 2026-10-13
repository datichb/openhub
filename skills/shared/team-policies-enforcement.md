---
name: team-policies-enforcement
description: "Utiliser quand les outils team_* sont présents : respecter les politiques d'équipe."
mcp: team
---

# Team Policies Enforcement Protocol

Ce skill définit comment les agents doivent respecter les team policies.
Il est complémentaire à `team-awareness` et s'applique à tous les agents
pour lesquels au moins une policy est pertinente.

## Chargement des policies

Au début de chaque session, appeler `team_policies` avec le projet courant
pour obtenir la liste des règles actives. Garder ces règles en mémoire
pour la durée de la session.

## Matrice agent / policy

Chaque agent ne vérifie que les policies pertinentes à ses actions :

| Agent | Policies à vérifier |
|-------|---------------------|
| `orchestrator-dev` | `branch_naming` (avant délégation), `review_required` (avant fermeture du ticket) |
| `developer` | `commit_format` (avant commit), `forbidden_patterns` custom (sur leur diff) |
| `reviewer` | `tests_required` (mentionner dans le rapport si non respecté) |
| `orchestrator` | `max_ticket_wip` (avant de proposer un nouveau ticket) |
| `conductor` | `branch_naming` (avant de déléguer à un developer), `review_required` (avant le checkpoint qui déverrouille le commit : une étape de review de la carte doit avoir rendu son retour) |
| `designer` | Aucune policy code applicable |
| `documentarian` | Aucune policy code applicable |
| `pathfinder` | Aucune (read-only) |
| `onboarder` | Aucune (read-only) |
| `auditor` | `branch_naming` (dans le rapport d'audit), `commit_format` (dans le rapport d'audit) |
| `auditor-subagent` | Aucune (lecture seule ; le rapport de l'`auditor` couvre `branch_naming` et `commit_format`) |
| `debugger` | Aucune policy applicable |
| `planner` | Aucune (read-only) |
| `brief-enricher` | Aucune policy applicable |
| `operator` | Aucune policy code applicable (aucune modification de fichier, aucun commit) |
| `benchmarker` | Aucune policy code applicable (écrit seulement dans `reports/benchmarks/`, aucun commit) |

## Comportement selon l'enforcement

### `enforcement = "refuse"`

L'agent DOIT :
1. ARRÊTER l'action en cours
2. INFORMER l'utilisateur de la violation
3. INDIQUER quelle policy est violée et pourquoi
4. PROPOSER une correction

L'agent NE DOIT PAS :
- Contourner la policy
- Continuer malgré la violation
- Demander à l'utilisateur s'il veut ignorer (la policy est non-contournable)

### `enforcement = "warn"`

L'agent DOIT :
1. INFORMER l'utilisateur de la violation
2. MENTIONNER la policy et la raison
3. CONTINUER le travail si l'utilisateur ne réagit pas

L'agent PEUT :
- Proposer une correction
- Demander confirmation avant de continuer

## Vérification par type de policy

### `branch_naming` (type: regex)

**Quand** : avant de créer une branche ou de déléguer à un developer
**Comment** : vérifier que le nom de branche généré ou proposé correspond à la regex
**Si violation** : proposer un nom corrigé qui respecte le pattern

### `commit_format` (type: regex)

**Quand** : avant chaque commit
**Comment** : vérifier que le message de commit correspond à la regex
**Si violation** : reformuler le message pour respecter le format

### `forbidden_patterns` (type: forbidden_pattern)

**Quand** : avant de finaliser un commit, sur les lignes ajoutées ou modifiées
**Comment** : scanner le diff pour les patterns interdits
**Si violation** : supprimer ou commenter les patterns interdits

### `review_required` (type: boolean)

**Quand** : avant de considérer un ticket comme "done"
**Comment** : vérifier qu'une review a bien eu lieu
**Si violation** : informer que le ticket ne peut pas être fermé sans review

### `tests_required` (type: boolean)

**Quand** : avant de considérer un ticket comme "done"
**Comment** : vérifier que les tests passent
**Si violation** : informer et recommander d'ajouter ou de corriger les tests

### `max_ticket_wip` (type: limit)

**Quand** : avant de proposer un nouveau ticket à travailler
**Comment** : compter les claims actifs du membre
**Si violation** : ne pas proposer de nouveau ticket, suggérer de terminer un ticket en cours

## Règles custom

Pour les policies de type `forbidden_pattern` avec un nom commençant par `custom_` :
- Appliquer la même logique que les patterns standard
- Le champ `scope` détermine quoi vérifier :
  - `diff_only` : uniquement les lignes ajoutées dans le diff
  - `modified_files` : tout le contenu des fichiers modifiés
  - `all_files` : tous les fichiers du projet (rarement utilisé par les agents)

## Intégration dans le workflow

```
1. Début de session → appeler team_policies
2. Avant chaque action (branch, commit, ticket close) :
   a. Identifier les policies pertinentes pour cette action
   b. Vérifier chaque policy
   c. Si refuse-violation : STOP + informer
   d. Si warn-violation : informer + continuer
3. En fin de session : aucune action supplémentaire (le CLI gère)
```
