# Guide de routing — domaine à choisir pour l'agent developer

Ce document clarifie quel domaine passer à l'agent `developer` selon le contenu du ticket.
L'agent est toujours `developer` — c'est le **domaine** passé dans le prompt d'invocation qui change.

> Source de vérité pour la matrice complète : `skills/orchestrator/orchestrator-dev-protocol.md`

---

## Matrice de responsabilités par domaine

| Domaine | Scope principal | Exclusions |
|---------|----------------|------------|
| **backend** | Services, repositories, logique métier interne, schémas BDD | APIs publiques, webhooks, intégrations tierces, spec OpenAPI |
| **api** | APIs REST/GraphQL, webhooks, intégrations tierces, spec OpenAPI (contrat technique) | Logique métier interne, services non exposés, documentation narrative |
| **frontend** | Composants UI, pages, state client, accessibilité front, CSS | Logique serveur, schémas BDD, APIs backend |
| **fullstack** | Features bout-en-bout traversant front + back de façon couplée | Spécialisation pure (séparer en tickets frontend/backend si décomposable) |
| **devops** | Dockerfiles, pipelines CI/CD applicatifs, scripts shell de build/deploy | Infrastructure as code (Terraform, K8s, Helm) |
| **platform** | Terraform, K8s, Helm, GitOps, infra as code | Dockerfiles applicatifs, pipelines CI/CD |
| **security** | Hardening suite à audit (CORS, headers HTTP, JWT, rate limiting, secrets) | Audit sécurité (c'est `auditor` domaine security), nouvelles features |
| **data** | Pipelines ETL, dbt, Airflow, jobs Spark, ML, BI | APIs web, composants UI, backend applicatif |
| **mobile** | Apps iOS/Android (React Native, Flutter, Swift, Kotlin) | Backend, web frontend |

**Agents distincts (pas un domaine de `developer`) :**

| Agent | Scope |
|-------|-------|
| `developer-refactor` | Extraction, renommage, simplification, réorganisation, patterns, dette technique |
| `developer-migrator` | Upgrade frameworks, versions majeures, build tools, dépendances EOL |

---

## Cas limites et arbitrages

### 1. backend vs api

**Frontière :** Une API REST est-elle backend ou api ?

**Règle :**
- Si l'API est **publique** (exposée à des clients externes, partenaires, webhooks) → domaine **api**
- Si l'API est **interne** (entre services du même projet, non documentée pour l'externe) → domaine **backend**
- La **spec OpenAPI** (contrat technique) est gérée dans le domaine **api** — la **documentation narrative** (guides d'utilisation) par le `documentarian`

**Exemples :**
- `POST /api/v1/users` (API publique) → **api**
- `POST /internal/sync-users` (API interne entre services) → **backend**

---

### 2. devops vs platform

**Frontière :** Qui gère le Dockerfile ? Qui gère Kubernetes ?

**Règle :**
- Domaine **devops** : Dockerfiles, docker-compose, pipelines CI/CD (GitHub Actions, GitLab CI), scripts shell, déploiements applicatifs
- Domaine **platform** : Infrastructure as code (Terraform, Pulumi), Kubernetes manifests/Helm charts, ArgoCD, GitOps, gestion des clusters

**Exemples :**
- `Dockerfile` d'une app Node.js → **devops**
- `.github/workflows/deploy.yml` → **devops**
- `terraform/main.tf` (provision AWS infra) → **platform**
- `k8s/deployment.yaml` (manifests K8s) → **platform**
- `helm/myapp/values.yaml` → **platform**

---

### 3. developer-refactor vs developer-migrator

**Frontière :** Un upgrade qui nécessite du refactoring → qui prend le lead ?

**Règle :**
- `developer-refactor` : Ne **jamais** modifier les versions de dépendances ou frameworks. Scope : amélioration du code existant sans changer les dépendances.
- `developer-migrator` : Peut refactorer **uniquement si requis par un breaking change** de la migration (ex : méthode dépréciée supprimée, API changée).

**Exemples :**
- Extraire une fonction en plusieurs petites fonctions → **developer-refactor**
- Renommer des variables pour cohérence → **developer-refactor**
- Upgrade Vue 2 → Vue 3 (nécessite refactoring Composition API) → **developer-migrator**
- Upgrade Node 16 → Node 20 (breaking changes sur crypto) → **developer-migrator**

**Si le refactoring est indépendant de la migration :**
- D'abord `developer-migrator` (upgrade minimal pour que ça compile)
- Puis `developer-refactor` (amélioration du code migré)

---

### 4. fullstack : Quand l'utiliser ?

**Règle :**
- Utiliser le domaine **fullstack** uniquement si la feature traverse **front + back de façon fortement couplée** (ex : nouvelle authentification, système de notifications temps réel)
- Si la feature est **décomposable** en tickets front + back indépendants → créer 2 tickets séparés, router l'un en **frontend** et l'autre en **backend**

**Exemple légitime :**
- Feature "Authentification JWT avec refresh tokens" → **fullstack** (login form + backend endpoints + middleware + state management couplés)

**Exemples illégitimes :**
- Feature "Ajout d'un champ email dans le formulaire d'inscription" → **frontend** (le backend existe déjà)
- Feature "Nouvel endpoint GET /users" → **backend** (pas de changement front)

---

### 5. security : Quand l'utiliser ?

**Règle :**
- Domaine **security** intervient **après** un audit de `auditor` (domaine security) pour **implémenter les corrections**
- Il **ne fait PAS** l'audit (c'est `auditor` avec domaine security)
- Il **ne crée PAS** de nouvelles features (domaines **backend** ou **api** selon le cas)

**Exemples :**
- Rapport d'audit : "Absence de rate limiting sur /login" → **security** implémente le rate limiting
- Rapport d'audit : "Headers HTTP manquants (CSP, HSTS)" → **security** configure les headers
- Nouvelle feature "Système d'authentification" → **fullstack** ou **backend** (pas security)

---

### 6. data : Frontière avec backend

**Règle :**
- Domaine **data** : Pipelines de données, transformations, jobs batch, ML, BI
- Domaine **backend** : APIs synchrones, logique métier, CRUD applicatif

**Exemples :**
- Pipeline dbt pour agrégations analytics → **data**
- Job Airflow pour sync quotidien → **data**
- Endpoint API `GET /analytics/dashboard` → **backend** (même s'il lit des données agrégées par data)
- Script pandas pour nettoyage de données → **data**

---

## Tableau de décision rapide (orchestrator-dev)

| Signal dans le ticket | Domaine | Seconde option si ambiguïté |
|-----------------------|---------|------------------------------|
| frontend, UI, composant, Vue, React, CSS | `frontend` | `fullstack` (si couplé back) |
| backend, service, repository, SQL migration, schéma | `backend` | `fullstack` (si couplé front) |
| fullstack, feature traversante, front + back liés | `fullstack` | — |
| API, REST, GraphQL, webhook, intégration tierce | `api` | `backend` (si API interne) |
| data, ETL, pipeline, ML, dbt, Airflow | `data` | — |
| docker, CI/CD, script shell, pipeline de build | `devops` | `platform` (si K8s/Helm) |
| mobile, React Native, Flutter, Swift, Kotlin | `mobile` | — |
| Terraform, K8s, Helm, GitOps | `platform` | — |
| refactoring, extraction, renommage, dette | `developer-refactor` (agent dédié) | — |
| migration, upgrade, version majeure | `developer-migrator` (agent dédié) | — |
| sécurité, hardening, CORS, headers, audit | `security` | `backend` (si pas suite audit) |

**En cas d'ambiguïté :** choisir le domaine `fullstack` et l'indiquer dans le compte rendu d'étape.

---

## Référence

**Source de vérité :** `skills/orchestrator/orchestrator-dev-protocol.md` — section "Matrice de routing"

**ADR :** [ADR-013 — Fusion des agents developer-*](../adr/013-developer-agent-consolidation.fr.md)
