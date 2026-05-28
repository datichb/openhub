# Matrice de différenciation des agents developer-*

Ce document clarifie les frontières et responsabilités de chaque agent developer pour éviter les ambiguïtés de routing.

---

## Matrice de responsabilités

| Agent | Scope principal | Exclusions | Routing depuis |
|-------|----------------|------------|----------------|
| **developer-backend** | Services, repositories, logique métier interne, schémas BDD | APIs publiques, webhooks, intégrations tierces, spec OpenAPI | orchestrator-dev |
| **developer-api** | APIs REST/GraphQL, webhooks, intégrations tierces, spec OpenAPI (contrat technique) | Logique métier interne, services non exposés, documentation narrative | orchestrator-dev |
| **developer-frontend** | Composants UI, pages, state client, accessibilité front, CSS | Logique serveur, schémas BDD, APIs backend | orchestrator-dev |
| **developer-fullstack** | Features bout-en-bout traversant front + back de façon couplée | Spécialisation pure (déléguer à frontend/backend si trop complexe) | orchestrator-dev |
| **developer-devops** | Dockerfiles, pipelines CI/CD applicatifs, scripts shell de build/deploy | Infrastructure as code (Terraform, K8s, Helm) | orchestrator-dev |
| **developer-platform** | Terraform, K8s, Helm, GitOps, infra as code | Dockerfiles applicatifs, pipelines CI/CD | orchestrator-dev |
| **developer-refactor** | Extraction, renommage, simplification, reorganisation, patterns, dette technique | Changement de versions/frameworks, nouvelle feature, migration | orchestrator-dev |
| **developer-migrator** | Upgrade frameworks, versions majeures, build tools, dépendances EOL | Refactoring non lié à un breaking change de migration, nouvelle feature | orchestrator-dev |
| **developer-security** | Hardening suite à audit (CORS, headers HTTP, JWT, rate limiting, secrets) | Audit sécurité (c'est auditor-security), nouvelles features | orchestrator-dev |
| **developer-data** | Pipelines ETL, dbt, Airflow, jobs Spark, ML, BI | APIs web, composants UI, backend applicatif | orchestrator-dev |
| **developer-mobile** | Apps iOS/Android (React Native, Flutter, Swift, Kotlin) | Backend, web frontend | orchestrator-dev |

---

## Cas limites et arbitrages

### 1. **developer-backend vs developer-api**

**Frontière :** Une API REST est-elle backend ou api ?

**Règle :**
- Si l'API est **publique** (exposée à des clients externes, partenaires, webhooks) → **developer-api**
- Si l'API est **interne** (entre services du même projet, non documentée pour l'externe) → **developer-backend**
- La **spec OpenAPI** (contrat technique) est maintenue par **developer-api**, la **documentation narrative** (guides d'utilisation) par le **documentarian**

**Exemple :**
- `POST /api/v1/users` (API publique) → **developer-api**
- `POST /internal/sync-users` (API interne entre services) → **developer-backend**

---

### 2. **developer-devops vs developer-platform**

**Frontière :** Qui gère le Dockerfile ? Qui gère Kubernetes ?

**Règle :**
- **developer-devops** : Dockerfiles, docker-compose, pipelines CI/CD (GitHub Actions, GitLab CI), scripts shell, déploiements applicatifs
- **developer-platform** : Infrastructure as code (Terraform, Pulumi), Kubernetes manifests/Helm charts, ArgoCD, GitOps, gestion des clusters

**Exemple :**
- `Dockerfile` d'une app Node.js → **developer-devops**
- `.github/workflows/deploy.yml` → **developer-devops**
- `terraform/main.tf` (provision AWS infra) → **developer-platform**
- `k8s/deployment.yaml` (manifests K8s) → **developer-platform**
- `helm/myapp/values.yaml` → **developer-platform**

---

### 3. **developer-refactor vs developer-migrator**

**Frontière :** Un upgrade qui nécessite du refactoring → qui prend le lead ?

**Règle :**
- **developer-refactor** : Ne **jamais** modifier les versions de dépendances ou frameworks. Scope : amélioration du code existant sans changer les dépendances.
- **developer-migrator** : Peut refactorer **uniquement si requis par un breaking change** de la migration (ex : méthode dépréciée supprimée, API changée).

**Exemple :**
- Extraire une fonction en plusieurs petites fonctions → **developer-refactor**
- Renommer des variables pour cohérence → **developer-refactor**
- Upgrade Vue 2 → Vue 3 (nécessite refactoring Composition API) → **developer-migrator**
- Upgrade Node 16 → Node 20 (breaking changes sur crypto) → **developer-migrator**

**Si le refactoring est **indépendant** de la migration :**
- D'abord **developer-migrator** (upgrade minimal pour que ça compile)
- Puis **developer-refactor** (amélioration du code migré)

---

### 4. **developer-fullstack : Quand l'utiliser ?**

**Règle :**
- Utiliser **developer-fullstack** uniquement si la feature traverse **front + back de façon fortement couplée** (ex : nouvelle authentification, système de notifications temps réel)
- Si la feature est **décomposable** en tickets front + back indépendants → router vers **developer-frontend** + **developer-backend** séparément

**Exemple d'usage légitime :**
- Feature "Authentification JWT avec refresh tokens" → **developer-fullstack** (login form + backend endpoints + middleware + state management couplés)

**Exemple d'usage illégitime :**
- Feature "Ajout d'un champ email dans le formulaire d'inscription" → **developer-frontend** (le backend existe déjà)
- Feature "Nouvel endpoint GET /users" → **developer-backend** (pas de changement front)

---

### 5. **developer-security : Quand l'utiliser ?**

**Règle :**
- **developer-security** intervient **après** un audit de `auditor-security` pour **implémenter les corrections**
- Il **ne fait PAS** l'audit lui-même (c'est le rôle de `auditor-security`)
- Il **ne crée PAS** de nouvelles features (c'est le rôle de `developer-backend` / `developer-api`)

**Exemple :**
- Rapport d'audit : "Absence de rate limiting sur /login" → **developer-security** implémente le rate limiting
- Rapport d'audit : "Headers HTTP manquants (CSP, HSTS)" → **developer-security** configure les headers
- Nouvelle feature "Système d'authentification" → **developer-fullstack** ou **developer-backend** (pas developer-security)

---

### 6. **developer-data : Frontière avec backend**

**Règle :**
- **developer-data** : Pipelines de données, transformations, jobs batch, ML, BI
- **developer-backend** : APIs synchrones, logique métier, CRUD applicatif

**Exemple :**
- Pipeline dbt pour agrégations analytics → **developer-data**
- Job Airflow pour sync quotidien → **developer-data**
- Endpoint API `GET /analytics/dashboard` → **developer-backend** (même s'il lit des données agrégées par developer-data)
- Script pandas pour nettoyage de données → **developer-data**

---

## Workflow de routing (orchestrator-dev)

**Matrice de décision par signaux :**

| Signal dans ticket | Agent prévu | Seconde option si ambiguïté |
|--------------------|-------------|------------------------------|
| frontend, UI, composant, Vue, React, CSS | developer-frontend | developer-fullstack (si couplé back) |
| backend, service, repository, SQL migration, schéma | developer-backend | developer-fullstack (si couplé front) |
| fullstack, feature traversante, front + back liés | developer-fullstack | — |
| API, REST, GraphQL, webhook, intégration tierce | developer-api | developer-backend (si API interne) |
| data, ETL, pipeline, ML, dbt, Airflow | developer-data | — |
| docker, CI/CD, script shell, pipeline de build | developer-devops | developer-platform (si K8s/Helm) |
| mobile, React Native, Flutter, Swift, Kotlin | developer-mobile | — |
| Terraform, K8s, Helm, GitOps | developer-platform | — |
| refactoring, extraction, renommage, dette | developer-refactor | — |
| migration, upgrade, version majeure | developer-migrator | — |
| sécurité, hardening, CORS, headers, audit | developer-security | developer-backend (si pas suite audit) |

**En cas d'ambiguïté :** choisir **developer-fullstack** et l'indiquer dans le compte rendu d'étape.

---

## Référence

**Source de vérité :** `orchestrator-dev-protocol.md` (Matrice de routing lignes 91-111)

**Mise à jour :** 28 mai 2026
