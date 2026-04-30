---
name: dev-standards-github-actions
description: Standards GitHub Actions — structure des workflows, permissions, cache, concurrency, sécurité des pipelines et bonnes pratiques CI/CD.
---

# Skill — Standards GitHub Actions

## Rôle

Ce skill définit les bonnes pratiques pour les pipelines CI/CD avec GitHub Actions.
Il complète `dev-standards-devops.md`.

---

## 🔒 Règles absolues

❌ Jamais de secrets en dur dans les fichiers de workflow
❌ Ne jamais utiliser `pull_request_target` sans analyse de sécurité approfondie
❌ Jamais de `--force` sur des opérations git dans un pipeline sans confirmation explicite
✅ Les permissions sont définies au niveau minimal requis par chaque job
✅ Les déploiements en production ont une validation manuelle ou une approbation

---

## Structure des workflows

```
.github/
└── workflows/
    ├── ci.yml       ← lint, tests, build — déclenché sur chaque PR
    ├── cd.yml       ← déploiement — déclenché sur merge dans main
    └── release.yml  ← publication de release / package
```

- Un workflow par responsabilité — pas de fichier monolithique
- Nommer les jobs et les steps explicitement (`name:`)
- Utiliser `on.pull_request.branches` pour cibler les branches protégées

---

## Permissions

- Définir `permissions` au niveau **minimal requis** par le workflow ou le job
- Partir du principe `permissions: {}` (tout refusé) et n'accorder que ce qui est nécessaire

```yaml
# ✅ Permissions minimales déclarées explicitement
permissions:
  contents: read
  pull-requests: write   # uniquement si le job commente sur la PR
```

| Permission | Cas d'usage |
|---|---|
| `contents: read` | Checkout du code |
| `contents: write` | Créer des tags, releases |
| `packages: write` | Publier vers GitHub Packages |
| `pull-requests: write` | Commenter sur les PRs |
| `id-token: write` | OIDC (authentification sans secret) |

---

## Actions épinglées

- Actions officielles GitHub : `@v4` acceptable (maintenues activement)
- Actions tierces : épingler au **SHA de commit** — jamais `@main` ni `@latest`

```yaml
# ✅ Action tierce épinglée au SHA
- uses: un-tiers/action@abc123def456  # v1.2.3

# ❌ Dangereux — le contenu peut changer sans avertissement
- uses: un-tiers/action@main
```

---

## Cache et performance

- Cacher les dépendances avec `actions/cache` ou l'option `cache` de `setup-*`
- Les jobs indépendants tournent en parallèle — `needs` uniquement si dépendance réelle
- Utiliser `concurrency` pour annuler les runs obsolètes sur une même branche

```yaml
name: CI

on:
  pull_request:
    branches: [main]

concurrency:
  group: ci-${{ github.ref }}
  cancel-in-progress: true

permissions:
  contents: read

jobs:
  lint:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-node@v4
        with:
          node-version: "20"
          cache: "npm"
      - run: npm ci
      - run: npm run lint

  test:
    runs-on: ubuntu-latest
    needs: lint
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-node@v4
        with:
          node-version: "20"
          cache: "npm"
      - run: npm ci
      - run: npm test -- --coverage
```

---

## Secrets et variables

- Les secrets sont injectés via `${{ secrets.NOM_SECRET }}` — jamais en dur
- Les variables non sensibles utilisent `${{ vars.NOM_VARIABLE }}`
- Ne jamais afficher de secrets dans les logs (`echo ${{ secrets.TOKEN }}` interdit)
- Utiliser OIDC (OpenID Connect) pour les authentifications cloud sans secret statique :

```yaml
# ✅ Authentification AWS sans secret statique (OIDC)
- uses: aws-actions/configure-aws-credentials@v4
  with:
    role-to-assume: arn:aws:iam::123456789:role/github-actions
    aws-region: eu-west-1
```

---

## Sécurité des pipelines

- `pull_request_target` uniquement si absolument nécessaire — risque d'injection de code malveillant depuis un fork
- Les workflows déclenchés par des forks n'ont pas accès aux secrets par défaut — ne pas contourner
- Scanner les images Docker dans le pipeline (Trivy, Grype)
- Utiliser `GITHUB_TOKEN` avec le scope minimal — ne pas réutiliser des PATs personnels

---

## Déploiement

- Les déploiements en production nécessitent une **approbation manuelle** (Environments + required reviewers)
- Utiliser les `environments` GitHub pour séparer staging et production
- Un déploiement raté déclenche une alerte — pas de rollback silencieux

```yaml
# ✅ Déploiement avec approbation manuelle
deploy:
  environment:
    name: production
    url: https://app.exemple.com
  runs-on: ubuntu-latest
  needs: build
  steps:
    - name: Deploy
      run: ./scripts/deploy.sh production
```

---

## Ce que tu ne fais PAS

- Stocker des secrets dans les fichiers de workflow
- Utiliser `pull_request_target` sans analyse de sécurité
- Épingler des actions tierces à un tag mutable (`@main`, `@latest`)
- Créer des permissions plus larges que nécessaire
- Ignorer les échecs de pipeline "pour aller plus vite"
