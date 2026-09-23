---
name: doc-changelog
description: Documentation du changelog — détection du format existant, Keep a Changelog, Conventional Commits, SemVer, génération depuis git log.
---

# Skill — Changelog et Release Notes

## Étape -1 — Détection monorepo

Avant toute action, vérifier si le dépôt est un monorepo :

```bash
ls pnpm-workspace.yaml lerna.json nx.json turbo.json 2>/dev/null
ls packages/*/package.json apps/*/package.json 2>/dev/null
cat Cargo.toml 2>/dev/null | head -5  # [workspace] section
```

**Si monorepo détecté :**
1. Identifier le package cible de la tâche en cours (depuis le contexte, le ticket, ou en demandant à l'utilisateur)
2. Définir `$PROJECT_DIR` = chemin du package (ex: `packages/api-gateway/`)
3. Scoper toutes les recherches et chemins à `$PROJECT_DIR` au lieu de la racine du dépôt

**Si pas de monorepo :** continuer normalement avec la racine comme scope.

---

## Étape 0 — Détecter le format existant

Avant de modifier ou créer un CHANGELOG :

```bash
# Fichier changelog courant
cat CHANGELOG.md 2>/dev/null | head -60
cat HISTORY.md 2>/dev/null | head -30
cat RELEASES.md 2>/dev/null | head -30

# Format des commits (pour la génération)
git log --oneline -20
git log --format="%s" -20
```

Identifier :
- Le fichier utilisé (`CHANGELOG.md`, `HISTORY.md`, autre ?)
- Le format (`Keep a Changelog` ? format libre ? par commit ?)
- La convention de versioning (`v1.2.3` ? `1.2.3` ? date ?)
- La langue (français ? anglais ?)

S'adapter à l'existant.

---

## Format de référence — Keep a Changelog

Format officiel de [keepachangelog.com](https://keepachangelog.com).
Proposé par défaut quand aucun format n'est détecté.

### Structure générale

```markdown
# Changelog

Toutes les modifications notables de ce projet sont documentées dans ce fichier.

Le format est basé sur [Keep a Changelog](https://keepachangelog.com/fr/1.0.0/),
et ce projet respecte le [Semantic Versioning](https://semver.org/lang/fr/).

## [Unreleased]

### Added
- [Fonctionnalité en cours de développement, pas encore releasée]

## [1.2.0] — 2024-03-15

### Added
- Authentification SSO via Azure AD (#142)
- Export CSV des rapports de facturation (#138)

### Changed
- Le champ `user.name` est maintenant obligatoire à l'inscription
- Amélioration des performances de la liste des commandes (×3 plus rapide)

### Deprecated
- `GET /api/v1/users` — sera supprimé en v2.0, utiliser `GET /api/v2/users`

### Fixed
- Correction du calcul du total quand un coupon est appliqué (#155)
- Les emails de confirmation sont maintenant envoyés en UTC (#151)

### Security
- Mise à jour de `lodash` pour corriger CVE-2024-1234

## [1.1.0] — 2024-02-01

### Added
- ...

## [1.0.0] — 2024-01-15

### Added
- Version initiale

[Unreleased]: https://github.com/org/repo/compare/v1.2.0...HEAD
[1.2.0]: https://github.com/org/repo/compare/v1.1.0...v1.2.0
[1.1.0]: https://github.com/org/repo/compare/v1.0.0...v1.1.0
[1.0.0]: https://github.com/org/repo/releases/tag/v1.0.0
```

### Les 6 sections

| Section | Contenu |
|---------|---------|
| `Added` | Nouvelles fonctionnalités |
| `Changed` | Modifications de fonctionnalités existantes |
| `Deprecated` | Fonctionnalités bientôt supprimées |
| `Removed` | Fonctionnalités supprimées |
| `Fixed` | Corrections de bugs |
| `Security` | Corrections de vulnérabilités |

**Règles :**
- Ne documenter que ce qui est pertinent pour l'utilisateur (pas les refactorings internes)
- Chaque entrée = une ligne, avec le numéro de ticket/PR si disponible
- `[Unreleased]` contient les changements non encore releasés
- Mettre à jour `[Unreleased]` à chaque commit significatif

---

## SemVer — Semantic Versioning

Format : `MAJOR.MINOR.PATCH` (exemple : `2.3.1`)

| Composant | Incrémenter quand... | Exemple |
|-----------|---------------------|---------|
| `MAJOR` | Breaking change — incompatible avec la version précédente | `1.5.0` → `2.0.0` |
| `MINOR` | Nouvelle fonctionnalité rétrocompatible | `1.5.0` → `1.6.0` |
| `PATCH` | Correction de bug rétrocompatible | `1.5.0` → `1.5.1` |

### Règles supplémentaires

- `0.x.x` : développement initial — l'API publique n'est pas stable
- `-alpha`, `-beta`, `-rc.1` : pré-releases (exemple : `2.0.0-rc.1`)
- Ne jamais décrémenter un numéro de version
- Une fois publiée, une version est immuable — ne pas modifier son contenu dans le CHANGELOG

### Quand incrementer MAJOR

Un breaking change est tout changement qui force les utilisateurs à modifier leur code :
- Suppression d'une fonctionnalité ou d'un endpoint
- Changement de la signature d'une fonction publique
- Changement de comportement d'une fonctionnalité existante
- Renommage d'un export ou d'une configuration

---

## Conventional Commits — génération du changelog

Si le projet utilise Conventional Commits, le changelog peut être généré automatiquement.

### Correspondance types → sections Keep a Changelog

| Type de commit | Section CHANGELOG |
|---------------|------------------|
| `feat:` | Added |
| `fix:` | Fixed |
| `perf:` | Changed |
| `refactor:` | (ne pas inclure — interne) |
| `docs:` | (ne pas inclure — interne) |
| `chore:` | (ne pas inclure — interne) |
| `feat!:` ou `BREAKING CHANGE:` | Changed (avec mention breaking) |
| `security:` | Security |

### Générer depuis git log

```bash
# Commits depuis la dernière version taguée
git log v1.1.0..HEAD --format="%s %h" --no-merges

# Commits feat et fix uniquement
git log v1.1.0..HEAD --format="%s %h" --no-merges \
  | grep -E "^(feat|fix|perf|security)"

# Avec le numéro de PR (si les merges sont squashés)
git log v1.1.0..HEAD --format="%s" --merges \
  | grep -E "^(feat|fix)"
```

### Workflow de release

```bash
# 1. Vérifier les commits depuis la dernière version
git log $(git describe --tags --abbrev=0)..HEAD --oneline --no-merges

# 2. Déterminer le prochain numéro de version (SemVer)
# 3. Déplacer [Unreleased] → [x.y.z] — date du jour
# 4. Créer une nouvelle section [Unreleased] vide
# 5. Mettre à jour les liens en bas du fichier
# 6. Committer : "chore(release): v1.2.0"
# 7. Taguer : git tag -a v1.2.0 -m "Release v1.2.0"
```

### Variante monorepo

En monorepo, le changelog peut être géré par package ou de façon coordonnée à la racine.

#### Changelogs par package

Chaque package maintient son propre `$PROJECT_DIR/CHANGELOG.md`, scopé à ses changements :

```bash
# Git log scopé au package
git log --oneline -- $PROJECT_DIR

# Commits depuis le dernier tag du package
git log @scope/package@1.2.0..HEAD --oneline -- $PROJECT_DIR
```

#### Changelog coordonné (racine)

Un `CHANGELOG.md` à la racine avec des sections par package :

```markdown
## [2024-03-15]

### @scope/api-gateway (1.3.0)
- Added: Nouveau middleware de rate limiting (#142)

### @scope/shared-utils (0.8.1)
- Fixed: Correction du parser de dates (#155)
```

#### Outils de gestion

| Outil | Usage | Commande type |
|-------|-------|---------------|
| `changesets` | Changelogs par package, versioning indépendant | `npx changeset add` puis `npx changeset version` |
| `lerna version` | Versioning coordonné ou indépendant | `lerna version --conventional-commits` |
| Independent versioning | Chaque package a son propre SemVer | Configuré dans `lerna.json` : `"version": "independent"` |

#### Tags git scopés

En monorepo, les tags suivent le format `@scope/package@version` :

```bash
git tag -a @scope/api-gateway@1.3.0 -m "Release @scope/api-gateway@1.3.0"
```

---

## Ajouter une entrée au changelog

### Cas courant — ajout d'une fonctionnalité

L'agent ajoute l'entrée dans `[Unreleased]`, section `Added` :

```markdown
## [Unreleased]

### Added
- Export PDF des factures avec logo personnalisable (#167)  ← nouvelle entrée
- Authentification SSO via Azure AD (#142)
```

### Cas d'une nouvelle release

L'agent :
1. Lit le git log pour identifier tous les changements depuis la dernière version
2. Classe les commits par section (Added / Changed / Fixed / Security)
3. Déplace `[Unreleased]` vers `[x.y.z] — YYYY-MM-DD`
4. Crée un nouveau `[Unreleased]` vide
5. Met à jour les liens en bas du fichier

```markdown
## [Unreleased]

## [1.2.0] — 2024-03-15   ← anciennement [Unreleased]

### Added
- Export CSV des rapports (#138)

### Fixed
- Correction du calcul du total (#155)
```

---

## Release notes (format étendu)

Pour les releases majeures ou les communications publiques, un format plus narratif :

```markdown
# Release v2.0.0 — 15 mars 2024

## Résumé

Cette version majeure introduit le nouveau système d'authentification SSO
et apporte des améliorations significatives de performance sur les listes.

## Nouvelles fonctionnalités

### Authentification SSO
[Description en 2-3 phrases, orientée valeur utilisateur]

### Export avancé
[...]

## Breaking changes

Cette version contient des breaking changes. Consulter le [guide de migration](docs/migration-v2.md).

| Changement | Action requise |
|-----------|---------------|
| `GET /api/v1/users` supprimé | Migrer vers `GET /api/v2/users` |
| Champ `user.name` obligatoire | Mettre à jour les formulaires d'inscription |

## Corrections de bugs

- Correction du calcul du total avec coupon (#155)
- Emails envoyés en UTC (#151)

## Mise à jour

```bash
npm update @monapp/core
```

Voir le [CHANGELOG complet](CHANGELOG.md) pour tous les détails.
```

---

## Checklist avant de livrer un changelog

- [ ] Le format est cohérent avec le CHANGELOG existant (ou justifié si nouveau)
- [ ] Les entrées `Added` décrivent la valeur utilisateur, pas l'implémentation
- [ ] Les breaking changes sont dans `Changed` avec la mention explicite
- [ ] Les tickets/PR sont référencés quand disponibles (`(#123)`)
- [ ] La section `[Unreleased]` est présente et vide si une release vient d'être faite
- [ ] Les liens de comparaison en bas du fichier sont mis à jour
- [ ] Les refactorings et changements internes ne sont pas inclus
