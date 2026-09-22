---
name: gitlab-onboarder-protocol
description: Protocole d'intégration GitLab pour l'agent Onboarder — cartographie du projet via labels, milestones actifs et tickets récents pour enrichir ONBOARDING.md et CONVENTIONS.md
---

# Skill — GitLab Onboarder Protocol (v1)

## Rôle

Ce skill enrichit la Phase 1 du workflow Onboarder avec les données GitLab pour documenter :
- La taxonomie des labels du projet (comment les tickets sont classifiés)
- Les milestones actifs (contexte de release/sprint)
- L'état du backlog (volume et répartition des tickets ouverts)

## Phase 1.4bis — Exploration GitLab (optionnelle)

### Déclencheur

Lancer Phase 1.4bis si :
- Le projet est déclaré avec `Tracker: gitlab` dans `projects/projects.md`
- OU un fichier `.gitlab-ci.yml` est détecté dans la codebase
- OU l'utilisateur mentionne un projet GitLab pendant l'onboarding

**Si pas de GitLab détecté → skiper Phase 1.4bis, passer à la suite.**

### Workflow

#### Étape 1 : Cartographie des labels

**Annoncer avant d'explorer :**
> "Je vais explorer la taxonomie GitLab du projet."

```
Utiliser l'outil : list_gitlab_labels
Argument : project_path
→ Obtenir : tous les labels avec description et compteurs d'usage
```

**Analyser et regrouper les labels par catégorie :**

| Catégorie détectée | Exemples typiques |
|---|---|
| Type de ticket | `type::bug`, `type::feature`, `type::chore` |
| Priorité | `priority::critical`, `P0`, `urgent` |
| Domaine fonctionnel | `area::frontend`, `area::backend`, `area::infra` |
| Statut workflow | `needs-review`, `blocked`, `in-progress` |
| Qualité | `tech-debt`, `breaking-change`, `security` |

**Si aucun label → noter "aucune taxonomie de labels définie".**

#### Étape 2 : Milestones actifs

```
Utiliser l'outil : list_gitlab_milestones
Arguments : project_path, state: "active"
→ Obtenir : sprints/releases en cours avec dates
```

**Exploiter pour :**
- Identifier la **cadence de release** (sprints de 2 semaines ? releases mensuelles ?)
- Situer le **sprint actuel** et sa date de fin
- Évaluer la **maturité du projet** (milestone v0.1 vs v5.2)

#### Étape 3 : Aperçu du backlog (optionnel)

```
Utiliser l'outil : list_gitlab_issues
Arguments : project_path, state: "opened", per_page: 20
→ Obtenir : aperçu des 20 premiers tickets ouverts
```

**Exploiter uniquement pour :**
- Évaluer le **volume de backlog** (petit / moyen / large)
- Identifier les **domaines les plus actifs** (labels fréquents)
- Détecter des **patterns récurrents** (`type::bug` majoritaire = projet instable ?)

**Ne pas lister tous les tickets dans ONBOARDING.md** — seulement les métriques globales.

#### Étape 4 : Enrichissement de ONBOARDING.md

Ajouter cette section si données GitLab disponibles :

```markdown
## Gestion de projet GitLab

**Instance :** <GITLAB_BASE_URL ou gitlab.com>
**Projet :** <project_path>

### Taxonomie des labels

| Catégorie | Labels |
|-----------|--------|
| Type | `type::feature`, `type::bug`, `type::chore` |
| Priorité | `priority::high`, `priority::medium`, `priority::low` |
| Domaine | `area::frontend`, `area::backend` |
| Workflow | `needs-review`, `blocked` |

> Labels hérités du groupe : <liste si applicable>

### Cadence de livraison

- **Type :** <sprints 2 semaines / releases mensuelles / ad-hoc>
- **Milestone actuel :** <titre> (échéance : <date>)
- **Prochaine release :** <titre si disponible>

### État du backlog

- **Tickets ouverts :** ~<N>
- **Domaines les plus actifs :** <labels fréquents>
- **Tendance :** <stable / bugs fréquents / dette technique visible>
```

**Si aucune donnée GitLab :** ne pas inclure cette section.

#### Étape 5 : Enrichissement de CONVENTIONS.md

Ajouter cette section si labels structurés détectés :

```markdown
## Conventions GitLab

### Labels obligatoires à l'ouverture d'un ticket

- **Type :** `type::feature` | `type::bug` | `type::chore`
- **Priorité :** `priority::high` | `priority::medium` | `priority::low`
- **Domaine :** `area::frontend` | `area::backend` | `area::infra` (si applicable)

### Workflow des tickets

1. Ticket créé → label `needs-triage`
2. En cours → label `in-progress` + assigné
3. En review → label `needs-review` + MR liée
4. Terminé → ticket fermé à la merge de la MR

> Adapter selon les conventions réelles observées dans les labels du projet.
```

### Gestion des erreurs

> **Protocole d'intégration tracker :** voir skill `shared/tracker-integration-protocol` pour la gestion d'erreurs commune.
>
> Paramètres pour cet adapter : `{platform}` = GitLab, `{platform_emoji}` = 🦊, `{tool_read_issue}` = get_gitlab_issue, `{tool_list_issues}` = list_gitlab_issues.
