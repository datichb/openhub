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

> **Note** : Pour un enforcement configurable (bloquant ou warning par règle),
> utilisez les **Team Policies** décrites ci-dessous. Les conventions restent
> la documentation de référence ; les policies en sont l'enforcement automatisé.

## Team Policies — Enforcement configurable

Les Team Policies étendent les conventions avec un mécanisme d'enforcement
configurable : chaque règle peut être un **warning** (informatif) ou un
**refuse** (bloquant). Elles sont stockées dans le repo team-state et
s'appliquent à tous les membres et à tous les projets.

### Fichier `policies.toml`

Créer ce fichier à la racine du repo team-state :

```toml
# Règles structurées — catégories standard

[policies.branch_naming]
type = "regex"
rule = "^(feat|fix|hotfix|chore|refactor)/[a-z0-9-]+"
enforcement = "refuse"
message = "Branch must follow pattern: feat/xxx, fix/xxx, etc."

[policies.commit_format]
type = "regex"
rule = "^(feat|fix|docs|style|refactor|test|chore)(\\(.+\\))?: .+"
enforcement = "refuse"
message = "Commit must follow Conventional Commits"

[policies.review_required]
type = "boolean"
enabled = true
enforcement = "refuse"
message = "Human review required before merge"

[policies.tests_required]
type = "boolean"
enabled = true
enforcement = "warn"
message = "Tests should pass before review"

[policies.max_ticket_wip]
type = "limit"
max = 2
enforcement = "warn"
message = "Limit WIP to 2 tickets per member"

# Règles custom — ajoutables à la demande

[policies.custom_no_console_log]
type = "forbidden_pattern"
patterns = ["console.log", "console.warn"]
scope = "diff_only"
enforcement = "warn"
message = "Remove console.log before commit"
```

### Types de règles

| Type | Usage | Paramètres |
|------|-------|-----------|
| `regex` | Valide une valeur contre un pattern | `rule` (regex) |
| `boolean` | Active/désactive une vérification | `enabled` |
| `limit` | Impose une valeur maximale | `max`, `unit` (optionnel) |
| `forbidden_pattern` | Interdit des motifs dans le code | `patterns` (liste), `scope` |

### Scopes pour `forbidden_pattern`

| Scope | Description |
|-------|-------------|
| `diff_only` | Uniquement les lignes ajoutées dans le diff |
| `modified_files` | Tout le contenu des fichiers modifiés |
| `all_files` | Tous les fichiers du projet |

### Overrides par projet

Pour rendre une policy plus stricte sur un projet spécifique, créer
`projects/<project>/policies-override.toml` :

```toml
# Uniquement le champ enforcement peut être durci (warn → refuse)
[policies.tests_required]
enforcement = "refuse"
message = "Tests MUST pass on T-SRU"
```

> **Important** : les overrides ne peuvent que rendre plus strict. Une policy
> `refuse` au global ne peut pas être adoucie en `warn` par un override.

### Commandes CLI

```bash
# Afficher les policies actives (globales + overrides du projet courant)
oh policies list
oh policies list --project T-SRU

# Vérifier les policies contre l'état courant
oh policies check --branch feat/my-feature --commit "feat: add login"

# Ajouter une policy custom (interactif)
oh policies add
```

### Double enforcement (CLI + Agents)

L'enforcement fonctionne à deux niveaux :

| Niveau | Qui | Quand | Comportement |
|--------|-----|-------|--------------|
| **CLI (hard)** | Le binaire `oh` | `oh claim`, `oh start`, `oh release` | Bloque ou warn selon la policy |
| **Agent (soft)** | Les agents IA via le skill | Pendant la session | Vérifie avant chaque action pertinente |

#### Checks CLI automatiques

| Commande | Policies vérifiées |
|----------|-------------------|
| `oh claim <ticket>` | `max_ticket_wip` |
| `oh start` (création branche) | `branch_naming` |
| `oh release <ticket>` | `review_required`, `tests_required` |

#### Checks agents

Chaque agent vérifie uniquement les policies pertinentes à ses actions.
Voir le skill `team-policies-enforcement` pour la matrice détaillée.

### Relation avec les conventions

Les **conventions** (`docs/wiki/technical/conventions.md` + wiki) restent la
documentation de référence, lisible par les humains et les agents. Les
**policies** en sont la formalisation machine-enforceable.

Recommandation : documentez vos conventions normalement, puis formalisez celles
qui doivent être bloquantes dans `policies.toml`.

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
