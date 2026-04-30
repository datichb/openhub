---
name: dev-standards-gitlab-ci
description: Standards GitLab CI — structure .gitlab-ci.yml, stages, rules, variables masquées, approbations de déploiement et bonnes pratiques CI/CD.
---

# Skill — Standards GitLab CI

## Rôle

Ce skill définit les bonnes pratiques pour les pipelines CI/CD avec GitLab CI/CD.
Il complète `dev-standards-devops.md`.

---

## 🔒 Règles absolues

❌ Jamais de secrets en dur dans `.gitlab-ci.yml` ou les scripts
❌ Jamais de déploiement automatique en production sans `when: manual` ou approbation
❌ Ne jamais utiliser `only`/`except` — utiliser `rules` (syntaxe moderne)
✅ Les variables sensibles sont dans les CI/CD Variables (masked + protected)
✅ Les stages sont définis explicitement

---

## Structure `.gitlab-ci.yml`

```
stages:
  - lint
  - test
  - build
  - deploy
```

- Définir les `stages` explicitement en tête de fichier
- Utiliser des `extends` ou des ancres YAML (`&template`) pour factoriser les jobs communs
- Séparer les jobs par responsabilité — un job = une tâche
- Nommer les jobs de façon descriptive (`lint:eslint`, `test:unit`, `deploy:staging`)

---

## Variables et secrets

- Les credentials (tokens, clés API, DSN) sont dans les **CI/CD Variables** :
  - **Masked** : la valeur est masquée dans les logs
  - **Protected** : disponible uniquement sur les branches protégées
- Les variables non sensibles peuvent être définies dans `.gitlab-ci.yml` sous `variables:`
- Ne jamais afficher de secrets dans les scripts (`echo $SECRET` interdit)

```yaml
# ✅ Variables non sensibles dans le fichier
variables:
  DOCKER_DRIVER: overlay2
  NODE_VERSION: "20"

# ✅ Variables sensibles dans GitLab CI/CD Variables (masked + protected)
# DATABASE_URL, DEPLOY_TOKEN, etc. — jamais dans le fichier
```

---

## Rules (syntaxe moderne)

Utiliser `rules` à la place de `only`/`except` (dépréciés) :

```yaml
# ✅ Rules modernes
deploy:staging:
  stage: deploy
  rules:
    - if: $CI_COMMIT_BRANCH == "main"
      when: on_success
    - when: never

deploy:production:
  stage: deploy
  rules:
    - if: $CI_COMMIT_TAG =~ /^v\d+\.\d+\.\d+$/
      when: manual
    - when: never
```

---

## Templates et factorisation

```yaml
# ✅ Template YAML factorisé avec ancre
.node_template: &node_template
  image: node:20-alpine
  cache:
    key: ${CI_COMMIT_REF_SLUG}
    paths:
      - node_modules/
  before_script:
    - npm ci

lint:
  <<: *node_template
  stage: lint
  script:
    - npm run lint

test:unit:
  <<: *node_template
  stage: test
  script:
    - npm test -- --coverage
  coverage: '/Lines\s*:\s*(\d+\.?\d*)%/'
  artifacts:
    reports:
      coverage_report:
        coverage_format: cobertura
        path: coverage/cobertura-coverage.xml
```

---

## Build et publication d'images

```yaml
build:image:
  stage: build
  image: docker:24
  services:
    - docker:24-dind
  variables:
    DOCKER_TLS_CERTDIR: "/certs"
  rules:
    - if: $CI_COMMIT_BRANCH == "main"
  script:
    - docker login -u $CI_REGISTRY_USER -p $CI_REGISTRY_PASSWORD $CI_REGISTRY
    - docker build -t $CI_REGISTRY_IMAGE:$CI_COMMIT_SHORT_SHA .
    - docker push $CI_REGISTRY_IMAGE:$CI_COMMIT_SHORT_SHA
```

---

## Déploiement

- Les déploiements en **staging** : `when: on_success` sur `main`
- Les déploiements en **production** : `when: manual` — ou approbation via Protected Environments
- Utiliser les **Environments** GitLab pour tracer les déploiements et permettre le rollback

```yaml
deploy:production:
  stage: deploy
  environment:
    name: production
    url: https://app.exemple.com
  rules:
    - if: $CI_COMMIT_TAG =~ /^v\d+\.\d+\.\d+$/
      when: manual
    - when: never
  script:
    - ./scripts/deploy.sh production
```

---

## Artefacts et rapports

- Conserver les artefacts de test (couverture, rapports JUnit) pour les MRs
- Définir `expire_in` sur les artefacts pour éviter l'accumulation

```yaml
test:unit:
  artifacts:
    when: always
    expire_in: 7 days
    reports:
      junit: junit.xml
    paths:
      - coverage/
```

---

## Sécurité des pipelines

- Ne jamais exposer `CI_JOB_TOKEN` au-delà de son scope minimal
- Les runners partagés ne doivent pas avoir accès aux secrets de production
- Utiliser des runners dédiés pour les déploiements en production
- Scanner les images Docker dans le pipeline avant le push

---

## Ce que tu ne fais PAS

- Stocker des secrets dans `.gitlab-ci.yml`
- Utiliser `only`/`except` — utiliser `rules`
- Déployer en production sans `when: manual` ou approbation
- Créer des jobs sans `rules` qui s'exécutent sur toutes les branches
- Ignorer les échecs de pipeline "pour aller plus vite"
