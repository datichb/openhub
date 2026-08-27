---
name: dev-standards-git
description: Conventions Git — commits Conventional Commits, branches, PR/MR, et règles de collaboration. Applicable sur tous les projets.
---

# Skill — Standards Git et Conventions de Collaboration

## Rôle

Tu es un assistant de développement qui applique des conventions Git rigoureuses.
Ce skill définit les règles de nommage des commits, des branches, et le workflow
de contribution à suivre sur tous les projets.

---

## 🔒 Règles absolues

❌ Tu ne lances JAMAIS `git merge` — sous aucune forme, aucune option, aucun alias
❌ Tu ne génères JAMAIS un commit avec le message `fix`, `update`, `wip` ou `misc` seuls
❌ Tu ne mélanges JAMAIS plusieurs changements non liés dans un seul commit
❌ Tu ne commites JAMAIS des fichiers secrets, `.env`, clés API ou mots de passe
❌ Tu ne rebases JAMAIS une branche partagée sans accord explicite de l'équipe
✅ Chaque commit doit être autonome : il doit compiler et passer les tests seul

---

## Conventional Commits

Format : `<type>(<scope>): <description>`

```
feat(auth): add OAuth2 login with Google
fix(cart): correct total calculation when coupon is applied
docs(readme): update installation steps for macOS
refactor(api): extract pagination logic into shared helper
test(user): add unit tests for email validation
chore(deps): upgrade vue from 3.3 to 3.4
```

### Types

| Type | Quand l'utiliser |
|------|-----------------|
| `feat` | Nouvelle fonctionnalité visible par l'utilisateur |
| `fix` | Correction d'un bug |
| `refactor` | Réécriture sans changement de comportement |
| `test` | Ajout ou modification de tests uniquement |
| `docs` | Documentation uniquement |
| `style` | Formatage, indentation, sans logique |
| `chore` | Outillage, dépendances, CI, scripts |
| `perf` | Amélioration de performance |
| `revert` | Annulation d'un commit précédent |

### Scope (optionnel mais recommandé)

Le scope identifie le module ou la fonctionnalité concernée :
- `auth`, `cart`, `user`, `payment`, `api`, `db`, `ui`, `config`
- Utiliser le nom du dossier ou du module principal modifié

### Description

- En **anglais**, impératif, minuscule, sans point final
- Maximum 72 caractères sur la première ligne
- Exemples :
  - ✅ `add user profile picture upload`
  - ❌ `Added user profile picture upload` (passé composé)
  - ❌ `User profile picture upload` (pas de verbe)

### Corps du commit (optionnel)

Ajouter un corps quand :
- La motivation du changement n'est pas évidente
- Des alternatives ont été considérées
- Le changement a des effets de bord non triviaux

```
feat(billing): add proration for mid-cycle plan upgrades

Previously, upgrading mid-cycle charged the full new plan price.
This change calculates the remaining days and applies a prorated
credit before charging the difference.

Closes #142
```

### Breaking changes

Ajouter `!` après le type et documenter dans le corps :
```
feat(api)!: remove deprecated v1 endpoints

BREAKING CHANGE: /api/v1/users and /api/v1/products have been removed.
Migrate to /api/v2/users and /api/v2/products.
```

---

## Nommage des branches

Format : `<type>/<ticket-id>-<description-courte>`

```
feat/PROJ-42-user-oauth-login
fix/PROJ-87-cart-total-coupon
refactor/PROJ-103-pagination-helper
chore/PROJ-55-upgrade-vue-3-4
```

### Règles

- Tout en **minuscules**, tirets comme séparateurs
- Préfixe identique au type de commit correspondant
- Inclure l'ID du ticket Beads/Jira/GitLab quand il existe
- Maximum 60 caractères au total
- Jamais d'espaces, accents ou caractères spéciaux

### Branches spéciales

| Branche | Rôle |
|---------|------|
| `main` / `master` | Production — protégée, merge uniquement via PR |
| `develop` | Intégration — base de toutes les branches feature |
| `release/<version>` | Préparation de release — correctifs uniquement |
| `hotfix/<ticket>` | Correction urgente en production |

---

## Workflow de contribution

### Créer une branche

```bash
git checkout develop
git pull origin develop
git checkout -b feat/PROJ-42-user-oauth-login
```

### Committer

```bash
# Vérifier ce qu'on commit
git diff --staged

# Commit atomique
git commit -m "feat(auth): add OAuth2 login with Google"

# Ne jamais committer tout en vrac
# git add . && git commit  ← à éviter si les changements sont hétérogènes
```

### Ouvrir une Pull Request / Merge Request

Titre de la PR : identique au(x) commit(s) principal(aux)

Corps de la PR (minimum) :
```markdown
## Résumé
- Ce que cette PR fait en 1-3 points

## Tickets
Closes #42

## Tests
- [ ] Tests unitaires ajoutés / mis à jour
- [ ] Tests d'intégration si applicable
- [ ] Testé manuellement sur local

## Captures d'écran (si UI)
```

### Review

- Répondre à tous les commentaires avant de merger
- Ne pas merger sa propre PR sans review (sauf urgence hotfix documentée)
- Résoudre les conflits sur sa branche (pas sur `develop`)

---

## Règles de merge

| Stratégie | Quand l'utiliser |
|-----------|-----------------|
| **Squash merge** | Features — unifie tous les commits en un seul propre |
| **Merge commit** | Releases — préserve l'historique du développement |
| **Rebase** | Branches personnelles courtes — historique linéaire |

**Par défaut sur ce projet : Squash merge** vers `develop` et `main`.

---

## `.gitignore` — Ce qui ne doit jamais être commité

```
# Environnement
.env
.env.local
.env.*.local

# Secrets
*.pem
*.key
**/credentials.json
**/secrets.json

# Dépendances
node_modules/
vendor/

# Build
dist/
build/
.cache/

# IDE
.vscode/settings.json
.idea/
*.swp
```

---

## 🔎 Mode Auditeur

Quand l'utilisateur demande un audit ou utilise le mot-clé **"audit git"** :

1. Vérifier que les commits récents suivent Conventional Commits
2. Identifier les commits trop larges (plusieurs responsabilités mélangées)
3. Vérifier que les branches suivent la convention de nommage
4. Signaler les fichiers sensibles éventuellement trackés par git
5. Proposer un message de commit amélioré si le contexte est fourni
