---
name: orchestrator-dev-routing
description: "Utiliser au démarrage d'orchestrator-dev, avant de router les tickets : domaine du developer par ticket, skills à lui faire charger, format du prompt."
---

# Routage des tickets vers le developer

L'agent invoqué est toujours `developer` : c'est le **domaine** qui change dans son prompt. Signaux lus dans les labels Beads d'abord, puis le titre, puis la description. Ambigu → domaine `fullstack`, indiqué dans le compte rendu.

| Signaux dans le ticket | Domaine | Skills à faire charger |
|---|---|---|
| frontend, UI, composant, Vue, React, CSS, interface | `frontend` | `dev-standards-frontend`, `dev-standards-frontend-data`, `dev-standards-frontend-a11y`, `dev-standards-testing` + stacks |
| backend, service, repository, logique métier, base de données, ORM | `backend` | `dev-standards-backend`, `dev-standards-api`, `dev-standards-database`, `dev-standards-testing` + stacks |
| fullstack, feature traversante, front + back liés | `fullstack` | `dev-standards-frontend`, `dev-standards-frontend-data`, `dev-standards-frontend-a11y`, `dev-standards-backend`, `dev-standards-api`, `dev-standards-testing` + stacks |
| data, ETL, pipeline, ML, dbt, Airflow, BI | `data` | `dev-standards-testing` + stacks data |
| docker, CI/CD, script shell, pipeline de build | `devops` | `dev-standards-devops` + stacks infra |
| mobile, React Native, Flutter, Swift, Kotlin, iOS, Android | `mobile` | `dev-standards-testing` + stacks mobile |
| API, REST, GraphQL, webhook, intégration tierce, SDK, endpoint | `api` | `dev-standards-backend`, `dev-standards-api`, `dev-standards-testing` |
| infra as code, Terraform, Pulumi, K8s, Helm, GitOps, platform | `platform` | `dev-standards-devops` + stacks platform |
| sécurité, hardening, CORS, headers HTTP, JWT, rate limiting | `security` | `dev-standards-security-hardening`, `dev-standards-backend`, `dev-standards-testing` |
| refactoring, extraction, renommage, simplification, dette technique | `refactor` | `dev-standards-refactoring`, `dev-standards-testing` + stacks |
| migration, upgrade, version majeure, changement de framework, EOL, dépréciation | `migration` | `dev-standards-migration`, `dev-standards-testing` + stacks |
| ajouter des tests, couverture, tests manquants, property-based | `tests` | `test-generation`, `dev-standards-testing` + stacks de test |

**Base de données :** un ticket qui touche le schéma, une migration de base ou des requêtes SQL reçoit aussi `dev-standards-database`, quel que soit son domaine.

**Stacks :** les skills `stacks/…` du domaine quand `ONBOARDING.md` cite la stack du projet ; sinon, seulement les skills génériques du domaine.

Transmets seulement les skills de ton catalogue de skills : celles qui n'y sont pas ne s'appliquent pas au projet (par exemple les standards frontend d'un projet sans frontend).

## Prompt d'invocation du developer

Chaque appel `task` vers `developer` contient :

```
Tu agis en tant que developer frontend.

Charge et applique les skills suivants :
- dev-standards-frontend
- dev-standards-frontend-data
- dev-standards-frontend-a11y
- dev-standards-testing
- stacks/dev-standards-vuejs

Ticket :
[contenu complet de bd show bd-12]
```
