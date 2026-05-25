# Guide — Créer un bon agent ou skill

Ce guide couvre les décisions de design pour concevoir des agents et skills
efficaces, cohérents avec l'architecture du hub.

---

## Agent ou skill ?

La première question à se poser avant de créer quoi que ce soit.

| Critère | Agent | Skill |
|---------|-------|-------|
| A un rôle propre, une identité invocable | ✅ | ❌ |
| Contient des règles ou protocoles réutilisables | — | ✅ |
| Est invoqué directement par l'utilisateur | ✅ | ❌ |
| Est injecté dans plusieurs agents | ❌ | ✅ |
| Orchestre d'autres agents | ✅ | ❌ |
| Définit un format de sortie ou une checklist | — | ✅ |

**Règle de décision :**
- Si tu réponds à "invoque [X] pour faire Y" → **agent**
- Si tu réponds à "applique ces règles / ce protocole quand tu fais Y" → **skill**

**Exemple :** `auditor-security` est un agent (invocable), `audit-protocol` est un skill (checklist de format injectée dans tous les auditeurs).

---

## Concevoir un agent

### Responsabilité unique

Un agent a une responsabilité claire et délimitée. S'il fait "trop de choses", c'est souvent le signe qu'il devrait être scindé en deux agents ou qu'une partie de sa logique appartient à un skill.

**Bon signal :** la `description` tient en une phrase sans "et" redondant.

**Mauvais signal :** "fait X, Y, Z et aussi W selon le contexte" → scinder.

### Ce que le corps d'un agent doit contenir

1. **Identité** (1 paragraphe) — qui il est, ce qu'il fait, ses contraintes fondamentales
2. **Ce qu'il fait** — liste des responsabilités concrètes
3. **Ce qu'il NE fait PAS** — les limites explicites (aussi important que les responsabilités)
4. **Workflow** — les étapes dans l'ordre, avec les commandes Beads si applicable
5. **Focus technique** (optionnel) — les patterns spécifiques à son domaine

### Quand ajouter une contrainte dans "Ce qu'il NE fait PAS"

Ajouter une contrainte explicite si :
- L'agent pourrait naturellement tenter de la violer (ex: un auditeur qui voudrait "corriger" lui-même)
- La limite est non évidente pour un utilisateur (ex: un reviewer qui ne clôt pas les tickets)
- Un autre agent est responsable de cette action (clarifier à qui déléguer)

### Familles et placement

| Famille | Quand l'utiliser |
|---------|-----------------|
| `auditor/` | Agents en lecture seule qui analysent et rapportent |
| `design/` | Agents de conception UX/UI — ne codent pas |
| `developer/` | Agents qui implémentent du code |
| `documentation/` | Agents qui écrivent de la documentation |
| `planning/` | Agents qui orchestrent ou planifient — ne codent pas |
| `quality/` | Agents de qualité (review, QA, debug) |

Un agent qui code va dans `developer/`. Un agent qui orchestre va dans `planning/`.
Un agent qui audite (lecture seule) va dans `auditor/`.

### Skills à injecter selon le type d'agent

| Type d'agent | Skills de base recommandés |
|-------------|---------------------------|
| Developer | `dev-standards-universal`, `dev-standards-security`, `beads-plan`, `beads-dev` + skills domaine |
| Auditeur | `audit-protocol-light` + skill domaine spécifique + `posture/expert-posture` |
| Coordinateur (lecture seule) | Son protocole propre — pas de `beads-dev` |
| Agent expert conseiller | `posture/expert-posture` |
| Agent interactif primary | `posture/tool-question` (+ `permission: question: allow` dans le frontmatter) |
| Agent qui gère des tickets | `beads-plan` (lecture + création), `beads-dev` (exécution) |
| Agent qui produit du code à tester | `dev-standards-testing` |
| Agent qui commit | `dev-standards-git` |

### Skills spécifiques aux stacks (injection dynamique)

Les agents developer reçoivent automatiquement des skills spécifiques à la stack au déploiement, en fonction de ce qui est détecté dans le projet cible. Cette injection dynamique est gérée par `detect_stack()` et `resolve_stack_skills()` dans `scripts/lib/prompt-builder.sh`, configurée par `config/stack-skills.json`.

**Il n'est pas nécessaire de déclarer les skills spécifiques aux stacks dans les frontmatters des agents.** Ils sont injectés automatiquement pour les types d'agents concernés.

Le périmètre d'injection dynamique par type d'agent :

| Agent | Catégories injectées dynamiquement |
|---|---|
| `developer-frontend` | language, frontend, test, api-spec |
| `developer-backend` | language, backend, orm, test, api-spec |
| `developer-fullstack` | language, frontend, backend, orm, test, api-spec |
| `developer-mobile` | mobile, test |
| `developer-data` | language, data, test |
| `developer-devops` | infra |
| `developer-platform` | infra |

**Ajouter une nouvelle stack :** ajouter la signature de détection dans `detect_stack()` et l'entrée de mapping dans `config/stack-skills.json`. Créer le fichier skill dans `skills/developer/stacks/`. Aucun changement de frontmatter d'agent nécessaire.

---

## Concevoir un skill

### Un skill = un contrat

Un skill définit un contrat que l'agent s'engage à respecter. Il n'est pas un cours magistral — c'est un ensemble de règles opérationnelles, formats et patterns directement applicables.

**Un bon skill répond à :** "Quand tu fais X, voici exactement comment tu le fais."

### Structure recommandée d'un skill

```markdown
---
name: nom-du-skill
description: Une phrase — ce que ce skill apporte à l'agent qui l'injecte.
---

# Skill — Titre

## Rôle
Ce skill définit... Il complète <autre-skill> si applicable.

---

## [Section thématique 1]
<règles + exemples de code>

---

## [Section thématique N]
<règles + exemples de code>

---

## Ce que ce skill ne remplace pas (optionnel)
<limites explicites — à qui déléguer pour aller plus loin>
```

### Règles de contenu

- **Concret avant abstrait** : commencer par les règles, pas par la philosophie
- **Exemples de code** : montrer un ✅ bon exemple et un ❌ mauvais exemple pour les règles non triviales
- **Pas de duplication** : si une règle existe dans `dev-standards-universal`, ne pas la répéter — la référencer
- **Description dans le frontmatter** : phrase courte, orientée bénéfice pour l'agent consommateur

### Granularité

**Trop large :** un skill qui couvre "tout le backend" — impossible à injecter sélectivement.
**Trop fin :** un skill qui ne couvre qu'une seule règle de 3 lignes — ne justifie pas un fichier séparé.

**Bonne granularité :** un domaine cohérent que plusieurs agents pourraient partager, avec 5 à 15 règles concrètes.

### Quand créer un nouveau skill vs enrichir un existant

| Situation | Action |
|-----------|--------|
| Nouvelles règles dans le même domaine | Enrichir le skill existant |
| Règles utilisées par un sous-ensemble différent d'agents | Nouveau skill |
| Règles qui seraient injectées dans plus de 3 agents distincts | Nouveau skill |
| Protocole de format de sortie spécifique à un agent | Nouveau skill dédié |
| Règles de domaine technique distinct (ex: API vs backend) | Nouveau skill |

---

## Checklist avant de créer

### Agent

- [ ] La `description` tient en une phrase sans "et" abusif
- [ ] La famille est correcte (placement dans le bon sous-dossier)
- [ ] Le champ `mode:` est défini : `primary` pour un agent invocable directement, `subagent` pour un spécialiste délégué
- [ ] Les skills injectés sont cohérents avec le type d'agent (voir tableau ci-dessus)
- [ ] Le corps contient : identité + ce qu'il fait + ce qu'il NE fait PAS + workflow
- [ ] Les limites explicites pointent vers le bon agent alternatif si applicable
- [ ] `posture/expert-posture` est injecté si l'agent a un rôle de conseil ou d'expertise
- [ ] `posture/tool-question` est injecté **et** `permission: question: allow` est dans le frontmatter si l'agent `primary` a besoin de poser des questions structurées à l'utilisateur
- [ ] `beads-plan` est injecté si l'agent lit ou crée des tickets Beads
- [ ] `beads-dev` est injecté en plus si l'agent exécute (clame, implémente, clôt) des tickets
- [ ] La matrice de dépendances dans `docs/architecture/skills.md` est mise à jour

### Skill

- [ ] La `description` dans le frontmatter est renseignée
- [ ] Le contenu est opérationnel (règles + exemples) — pas théorique
- [ ] Pas de duplication avec les skills existants
- [ ] Le skill est ajouté dans le tableau du bon domaine dans `docs/architecture/skills.md`
- [ ] **Skills génériques** (`developer/`) : les agents qui en ont besoin l'ont dans leur frontmatter `skills` ; matrice de dépendances mise à jour
- [ ] **Skills spécifiques aux stacks** (`developer/stacks/`) : détection ajoutée dans `detect_stack()`, mapping ajouté dans `config/stack-skills.json` — aucun changement de frontmatter d'agent nécessaire

---

## Exemple commenté — Créer un agent `developer-security`

```markdown
---
id: developer-security                    # ← kebab-case, unique
label: DeveloperSecurity                  # ← PascalCase, affiché dans l'outil
description: Assistant de développement   # ← une phrase, orientée usage
  sécurité applicative — [...]
mode: subagent                            # ← subagent : invocable uniquement via orchestrator-dev
targets: [opencode]  # ← les 2 cibles supportées
skills:                                   # ← du plus générique au plus spécifique
  - developer/dev-standards-universal     #   standards communs à tous les devs
  - developer/dev-standards-security      #   sécurité préventive
  - developer/dev-standards-security-hardening  # domaine spécifique de l'agent
  - developer/dev-standards-backend       #   contexte d'application
  - developer/dev-standards-testing       #   il écrit des tests
  - developer/dev-standards-git           #   il commit
  - developer/beads-plan                  #   il lit et crée des tickets
  - developer/beads-dev                   #   il exécute des tickets
---
```

**Ordre des skills recommandé :** universel → sécurité → domaine spécifique → contexte → tests → git → beads.

---

## Exemple commenté — Créer un skill `dev-standards-api`

```markdown
---
name: dev-standards-api                   # ← kebab-case, lisible
description: Standards spécifiques aux    # ← bénéfice concret pour l'agent
  APIs — versioning, pagination, [...]
---

# Skill — Standards API

## Rôle
Ce skill définit les bonnes pratiques pour les APIs publiques.
Il complète `dev-standards-backend.md`.  # ← pointer les complémentaires

## Versioning                             # ← une section = un thème
- Prefixe d'URL recommandé...

## Pagination                             # ← exemples de code concrets
```json
{ "data": [...], "pagination": { ... } }
```
```

**À ne pas faire :**
```markdown
## Introduction
Dans le monde des APIs modernes, il est crucial de...  # ← pas de cours magistral
```
