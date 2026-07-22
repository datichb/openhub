---
name: github-onboarder-protocol
description: Protocole d'intégration GitHub pour l'agent Onboarder — cartographie du projet via labels, milestones actifs et tickets récents pour enrichir ONBOARDING.md et CONVENTIONS.md
---

# Skill — GitHub Onboarder Protocol (v1)

## Rôle

Ce skill enrichit la Phase 1 du workflow Onboarder avec les données GitHub pour documenter :
- La taxonomie des labels du projet (comment les tickets sont classifiés)
- Les milestones actifs (contexte de release/sprint)
- L'état du backlog (volume et répartition des tickets ouverts)

## Phase 1.4bis — Exploration GitHub (optionnelle)

### Déclencheur

Lancer Phase 1.4bis si :
- Le projet est déclaré avec `Tracker: github` dans `projects/projects.md`
- OU un fichier `.github/workflows/` est détecté dans la codebase
- OU l'utilisateur mentionne un dépôt GitHub pendant l'onboarding

**Si pas de GitHub détecté → skiper Phase 1.4bis, passer à la suite.**

### Workflow

#### Étape 1 : Informations générales du dépôt

**Annoncer avant d'explorer :**
> "Je vais explorer le dépôt GitHub du projet."

```
Utiliser l'outil : github_get_repo
Arguments : owner, repo
→ Obtenir : description, langue principale, topics, visibilité, stars, forks
```

**Exploiter pour :**
- Comprendre la nature du projet (bibliothèque, application, service)
- Identifier la **langue principale** et l'ecosystème
- Évaluer la **maturité du projet** (stars, activité récente)

#### Étape 2 : Aperçu du backlog et taxonomie des labels

```
Utiliser l'outil : github_list_issues
Arguments : owner, repo, state: "open", per_page: 20
→ Obtenir : aperçu des 20 premiers tickets ouverts avec leurs labels
```

**Analyser et regrouper les labels par catégorie :**

| Catégorie détectée | Exemples typiques |
|---|---|
| Type de ticket | `bug`, `enhancement`, `chore` |
| Priorité | `priority: critical`, `P0`, `urgent` |
| Domaine fonctionnel | `area: frontend`, `area: backend`, `area: infra` |
| Statut workflow | `needs-review`, `blocked`, `in-progress` |
| Qualité | `tech-debt`, `breaking-change`, `security` |

**Si aucun label → noter "aucune taxonomie de labels définie".**

#### Étape 3 : Milestones actifs

```
Utiliser l'outil : github_list_issues
Arguments : owner, repo, state: "open", milestone: <milestone_number>
→ Vérifier les tickets rattachés au milestone en cours
```

**Exploiter pour :**
- Identifier la **cadence de release** (sprints de 2 semaines ? releases mensuelles ?)
- Situer le **milestone actuel** et sa date de fin
- Évaluer la **maturité du projet** (milestone v0.1 vs v5.2)

#### Étape 4 : Aperçu des workflows CI/CD (optionnel)

```
Utiliser l'outil : github_list_workflows
Arguments : owner, repo
→ Obtenir : workflows CI/CD configurés
```

**Exploiter uniquement pour :**
- Identifier les **pipelines de déploiement** (staging, production)
- Détecter les **étapes de validation** (tests, lint, build)
- Documenter la **chaîne de livraison** dans ONBOARDING.md

#### Étape 5 : Enrichissement de ONBOARDING.md

Ajouter cette section si données GitHub disponibles :

```markdown
## Gestion de projet GitHub

**Instance :** github.com
**Dépôt :** <owner/repo>

### Taxonomie des labels

| Catégorie | Labels |
|-----------|--------|
| Type | `enhancement`, `bug`, `chore` |
| Priorité | `priority: high`, `priority: medium`, `priority: low` |
| Domaine | `area: frontend`, `area: backend` |
| Workflow | `needs-review`, `blocked` |

### Cadence de livraison

- **Type :** <sprints 2 semaines / releases mensuelles / ad-hoc>
- **Milestone actuel :** <titre> (échéance : <date>)
- **Prochaine release :** <titre si disponible>

### État du backlog

- **Tickets ouverts :** ~<N>
- **Domaines les plus actifs :** <labels fréquents>
- **Tendance :** <stable / bugs fréquents / dette technique visible>
```

**Si aucune donnée GitHub :** ne pas inclure cette section.

#### Étape 6 : Enrichissement de CONVENTIONS.md

Ajouter cette section si labels structurés détectés :

```markdown
## Conventions GitHub

### Labels obligatoires à l'ouverture d'un ticket

- **Type :** `enhancement` | `bug` | `chore`
- **Priorité :** `priority: high` | `priority: medium` | `priority: low`
- **Domaine :** `area: frontend` | `area: backend` | `area: infra` (si applicable)

### Workflow des tickets

1. Ticket créé → label `needs-triage`
2. En cours → label `in-progress` + assigné
3. En review → label `needs-review` + PR liée
4. Terminé → ticket fermé à la merge de la PR

> Adapter selon les conventions réelles observées dans les labels du projet.
```

### Gestion des erreurs

| Erreur | Comportement |
|---|---|
| Token invalide / expiré | Afficher : `⚠️ Token GitHub invalide — vérifier : GITHUB_TOKEN` |
| Dépôt non trouvé (404) | Mentionner dans ONBOARDING.md : "Dépôt GitHub non accessible" |
| Pas de credentials | Skiper silencieusement Phase 1.4bis |
