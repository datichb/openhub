---
name: onboarder-profiles
description: Profils d'exploration adaptative pour l'onboarder — profils par technologie (Vue.js, React/Next.js, Backend Node.js, Backend Python, API REST/GraphQL, Data/ML, DevOps/Platform, Mobile) et complément transversal. Chargé à la demande après détection du profil applicatif en Phase 1.1.
bucket: B
---

# Skill — Onboarder : Profils d'exploration adaptative

## Contexte d'usage

Ce skill est chargé par `onboarder-workflow` en Phase 1, ÉTAPE 1.2,
après détection du profil applicatif.

Il fournit les fichiers structurants à cibler selon le profil détecté.

---

## ÉTAPE 1.2 — Profils d'exploration adaptative

Une fois le profil identifié, cibler les fichiers structurants.
**Annoncer ce qui va être lu avant chaque section.**

### Profil Frontend Vue.js

```
src/router/index.ts (ou router.ts)    → routes déclarées
src/stores/ (ou src/store/)           → état global (Pinia / Vuex)
src/composables/                      → logique réutilisable
src/components/                       → 3-5 composants représentatifs
src/layouts/                          → layouts globaux
vite.config.ts (ou vue.config.js)     → configuration du build
.env.example                          → variables d'environnement
```

### Profil Frontend React / Next.js

```
src/app/ (ou pages/)                  → structure des routes
src/components/                       → 3-5 composants représentatifs
src/hooks/                            → custom hooks
src/store/ ou src/context/            → état global
next.config.js / vite.config.ts       → configuration
.env.example                          → variables d'environnement
```

### Profil Backend Node.js

```
src/routes/ (ou app.ts / server.ts)   → routes déclarées, middlewares
src/controllers/ (ou src/handlers/)   → contrôleurs
src/services/                         → logique métier
src/models/ (ou src/entities/)        → modèles de données
src/middleware/                       → authentification, validation, logging
migrations/ (ou db/migrations/)       → migrations en attente ou récentes
.env.example                          → variables d'environnement
```

### Profil Backend Python

```
Module principal (src/, app/, [nom_projet]/)   → structure des packages
Routes / views (views.py, routes.py, api/)     → endpoints exposés
models.py (ou models/)                         → modèles de données
migrations/                                    → migrations
settings.py (ou config/, .env.example)         → configuration
tests/                                         → organisation des tests
requirements.txt / pyproject.toml              → dépendances (versions)
```

### Profil API REST / GraphQL

```
openapi.yaml (ou swagger.yaml, api-docs/)      → contrat d'API
schema.graphql (ou src/schema/)                → schéma GraphQL
src/controllers/ (ou src/resolvers/)           → handlers
src/middleware/auth*                           → auth et autorisation
```

### Profil Data / ML

```
dbt/models/                           → modèles dbt, structure
dbt/tests/ (ou tests/)                → tests de qualité des données
airflow/dags/                         → DAGs (lire 1-2 représentatifs)
notebooks/                            → notebooks (titres + premières cellules)
pipelines/ (ou src/pipelines/)        → pipelines ETL
models/ (ML) ou src/models/           → scripts d'entraînement, modèles
data/                                 → structure des données (pas le contenu)
```

### Profil DevOps / Platform

```
.github/workflows/                    → tous les workflows CI/CD
Dockerfile(s)                         → image(s), multi-stage build
docker-compose.yml                    → services et dépendances
terraform/                            → modules, main.tf, variables.tf
k8s/ ou helm/                         → manifests, values.yaml
scripts/                              → scripts de déploiement
```

### Profil Mobile

```
src/screens/ (ou lib/screens/)        → écrans principaux
src/navigation/ (ou lib/navigation/)  → stack de navigation
src/components/ (ou lib/widgets/)     → composants réutilisables
src/services/ (ou lib/services/)      → appels API, services
android/ (ou ios/)                    → configuration native
pubspec.yaml (Flutter) / package.json → dépendances et versions
```

---

### Complément transversal (tous profils)

Si présents, lire également :

```
README.md                              → description, setup, conventions
CONTRIBUTING.md                        → processus de contribution
docs/ ou doc/                          → documentation technique
adr/ (ou docs/architecture/adr/)       → décisions architecturales
.eslintrc* / .prettierrc* / biome.json → conventions de code
jest.config.ts / vitest.config.ts      → configuration des tests
```
