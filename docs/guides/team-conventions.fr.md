# Guide des Conventions d'Équipe

## Principe

Les conventions d'équipe sont des règles partagées qui s'appliquent au code, aux branches, et aux commits. Elles sont vérifiées automatiquement par les agents AI et par la commande `oh conventions check`.

## Où documenter les conventions

### Conventions spécifiques au projet

Emplacement : `docs/wiki/technical/conventions.md` dans le repo du projet.

Ce fichier est lu directement par les agents pendant leur session (accès filesystem).

### Conventions cross-projet (équipe)

Emplacement : page `conventions` dans le wiki team-state.

Accessible aux agents via `team_wiki_read("conventions")`.

## Format attendu

Les conventions doivent inclure des patterns machine-lisibles pour être vérifiées automatiquement.

### Pattern de branches

```markdown
## Branches

branch_pattern = `^(feat|fix|chore|refactor|docs)/[A-Z]+-\d+-.+`

Convention : `<type>/<TICKET-ID>-<description-slug>`

Exemples :
- `feat/SRU-142-user-authentication`
- `fix/SRU-99-token-refresh`
- `chore/SRU-200-cleanup-deps`
```

### Format des commits

```markdown
## Commits

Nous utilisons Conventional Commits.

commit_pattern = `^(feat|fix|docs|style|refactor|perf|test|chore|ci|build|revert)(\(.+\))?!?:\s.+`

Exemples :
- `feat(auth): implement JWT middleware`
- `fix(api): handle null response body`
- `docs: update API reference`
```

Si le fichier mentionne simplement "Conventional Commits" sans pattern explicite, le vérificateur applique automatiquement le pattern standard.

## Vérification

### Commande standalone

```bash
oh conventions check
```

Affiche :
- Status de la branche courante vs le pattern
- Status des derniers commits vs le format
- Status du claim (si team activé)

### Vérification par les agents

L'orchestrator-dev vérifie automatiquement les conventions :
- **Avant de créer une branche** : applique le pattern de nommage
- **Avant chaque commit** : respecte le format documenté
- **Warnings non bloquants** : l'agent informe mais ne bloque pas

## Enforcement

Le niveau d'enforcement est **medium** :
- Warnings dans le terminal et dans la session agent
- Pas de blocage dur (le dev peut ignorer un warning)
- Pas de hook pre-commit imposé (optionnel)

## Exemples de fichier conventions

```markdown
# Conventions Projet T-SRU

## Branches

branch_pattern = `^(feat|fix|chore|hotfix)/SRU-\d+-.+`

- Feature branches : `feat/SRU-<id>-<slug>`
- Fix branches : `fix/SRU-<id>-<slug>`
- Hotfix (production) : `hotfix/SRU-<id>-<slug>`

## Commits

Conventional Commits obligatoire.
Le scope doit correspondre au module (api, ui, auth, db).
La référence ticket est recommandée dans le body.

## Review

- Minimum 1 reviewer
- L'auteur ne peut pas approuver sa propre MR
- Les findings critiques doivent être corrigés avant merge

## Tests

- Coverage minimum : 80%
- Tests E2E requis pour les features UI
- Tests unitaires requis pour la logique métier
```
