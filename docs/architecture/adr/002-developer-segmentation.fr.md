# ADR-002 — Segmentation du Developer en 7 agents spécialisés

## Statut

~~Accepté~~ **Remplacé par [ADR-013](./013-developer-agent-consolidation.fr.md)**

Les 9 agents spécialisés ont été fusionnés en un unique agent `developer` générique.
La spécialisation est désormais transmise au moment de l'invocation via le prompt (domaine + liste de native_skills),
en s'appuyant sur l'architecture Bucket B introduite par l'ADR-010.

---

## Contexte

Le hub démarrait avec un unique agent `developer.md` polyvalent. Avec l'ajout des
skills (`dev-standards-backend`, `dev-standards-frontend`, etc.), cet agent accumulait
tous les skills de tous les domaines, ce qui posait plusieurs problèmes :

- Le contexte injecté dans l'outil IA devenait très long (tous les standards en même temps)
- L'agent ne pouvait pas avoir une identité focalisée : il était à la fois frontend,
  backend, data, devops, mobile et API
- La matrice de routing de l'orchestrateur ne pouvait pas déléguer avec précision

## Décision

L'agent `developer.md` est supprimé et remplacé par 7 agents spécialisés :

| Agent | Domaine |
|-------|---------|
| `developer-frontend` | UI, composants, Vue.js, CSS, accessibilité |
| `developer-backend` | Services, repositories, migrations, logique métier |
| `developer-fullstack` | Features traversant les deux couches |
| `developer-data` | Pipelines, ETL, ML, dbt, Airflow |
| `developer-devops` | Docker, CI/CD, scripts shell, infra |
| `developer-mobile` | React Native, Flutter, iOS, Android |
| `developer-api` | REST, GraphQL, webhooks, intégrations tierces |

Chaque agent spécialisé n'injecte que les skills pertinents à son domaine.
Tous partagent `dev-standards-universal`, `dev-standards-git`, `beads-plan` et `beads-dev`.

## Conséquences

### Positives

- Contexte injecté réduit et pertinent pour chaque domaine
- L'orchestrateur peut router avec précision via la matrice de routing
- Chaque agent a une identité claire et des règles adaptées à son domaine
- Facilite l'ajout d'un nouveau domaine sans modifier les agents existants

### Négatives / compromis

- 7 fichiers agents à maintenir au lieu d'un seul
- Le `developer-fullstack` reste ambigu par nature — il couvre le cas "je ne sais pas"
- La frontière entre `developer-api` et `developer-backend` peut être floue

## Alternatives rejetées

**Agent unique avec routing interne** : un seul agent qui détecte le domaine et adapte
son comportement. Rejeté car cela réintroduit la complexité dans l'agent et ne réduit
pas le contexte injecté.

**Agents par framework** (developer-vuejs, developer-laravel, etc.) : trop granulaire,
explose le nombre d'agents, ne correspond pas à la façon dont les tickets sont formulés.
