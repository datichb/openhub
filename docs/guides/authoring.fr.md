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

**Exemple :** `auditor-subagent` est un agent (invocable), `audit-protocol` est un skill (checklist de format injectée dans tous les auditeurs).

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

Le hub utilise une architecture hybride — voir [ADR-010](../architecture/adr/010-hybrid-skills-architecture.fr.md) :
- **Bucket A** (`skills: [...]`) — toujours actif, inline. Protocoles de workflow, formats de handoff, principes universels.
- **Bucket B** (`native_skills: [...]`) — à la demande via l'outil `skill`. Standards de domaine, stack skills, checklists.

Les agents utilisant des skills natives doivent avoir `permission: skill: allow`. Les coordinateurs définissent `permission: skill: deny`.

| Type d'agent | Bucket A (toujours inline) | Bucket B (natif, à la demande) |
|-------------|---------------------------|-------------------------------|
| Developer | `dev-standards-universal`, `dev-standards-simplicity`, `beads-plan`, `beads-dev` | `dev-standards-security`, skills de domaine, stack skills |
| Sous-agent auditeur | `audit-protocol-light`, `audit-handoff-format`, `posture/expert-posture` | Checklist d'audit du domaine (`audit-security`, `audit-accessibility`, etc.) |
| Coordinateur (lecture seule) | Son protocole propre + formats de handoff consommés — pas de `beads-dev` | aucune (`skill: deny`) |
| Agent expert conseiller | `posture/expert-posture` | — |
| Agent interactif primary | `posture/tool-question` (+ `permission: question: allow`) | — |
| Agent qui gère des tickets | `beads-plan` (lecture + création), `beads-dev` (exécution) | — |
| Agent qui commit | Inline via `beads-dev` | `dev-standards-git` (natif) |

### Stack skills (Bucket B — natif)

Les stack skills sont **toujours Bucket B**. Au déploiement, `deploy_native_skills()` les déploie vers `.opencode/skills/` en fonction de la stack détectée dans le projet cible. Le LLM charge ceux qui sont pertinents à la demande lors de l'inférence.

**Il n'est pas nécessaire de déclarer les skills spécifiques aux stacks dans les frontmatters des agents.** Ils sont déployés automatiquement pour les types d'agents concernés (ceux dont le scope `native_skills` couvre cette catégorie de stack, selon `config/stack-skills.json`).

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

**Ajouter une nouvelle stack :** ajouter la signature de détection dans `detect_stack()` et l'entrée de mapping dans `config/stack-skills.json`. Créer le fichier skill dans `skills/developer/stacks/`. Aucun changement de frontmatter d'agent nécessaire — le skill sera automatiquement déployé comme skill natif pour les agents concernés.

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

### Bucket A ou Bucket B ?

| Critère | Bucket A (inline) | Bucket B (natif) |
|---------|------------------|-----------------|
| Doit être actif dès le premier token | ✅ | ❌ |
| Définit un contrat de handoff entre deux agents | ✅ | ❌ |
| Workflow / protocole universel (toujours applicable) | ✅ | ❌ |
| Spécifique à un domaine — pertinent uniquement pour un sous-ensemble de tâches | ❌ | ✅ |
| Convention spécifique à une stack | ❌ | ✅ |
| Checklist d'audit pour un domaine spécifique | ❌ | ✅ |
| Standard de type de documentation (ADR, API, changelog…) | ❌ | ✅ |
| Protocole de recherche contextuelle | ❌ | ✅ |

---

## Checklist avant de créer

### Agent

- [ ] La `description` tient en une phrase sans "et" abusif
- [ ] La famille est correcte (placement dans le bon sous-dossier)
- [ ] Le champ `mode:` est défini : `primary` pour un agent invocable directement, `subagent` pour un spécialiste délégué
- [ ] `permission: skill: allow` est défini si l'agent utilise des skills natives (Bucket B) ; `skill: deny` pour les coordinateurs
- [ ] Les permissions ctx sont déclarées selon le type d'agent (les outils ctx ne sont PAS hérités — ils doivent être explicites) : orchestrateurs → `ctx_search`, `ctx_stats`, `ctx_batch_execute` ; développeurs → ajouter `ctx_execute`, `ctx_execute_file`, `ctx_fetch_and_index`, `ctx_index` ; qualité/audit → `ctx_search`, `ctx_execute`, `ctx_execute_file`, `ctx_batch_execute` ; design/planning → `ctx_search`, `ctx_batch_execute`
- [ ] Les skills Bucket A (`skills:`) sont cohérents avec le type d'agent : protocoles de workflow, formats de handoff, principes universels
- [ ] Les skills Bucket B (`native_skills:`) sont cohérents avec le type d'agent : standards de domaine, checklists, skills contextuelles
- [ ] Le corps contient : identité + ce qu'il fait + ce qu'il NE fait PAS + workflow + section guide des skills natives
- [ ] Les limites explicites pointent vers le bon agent alternatif si applicable
- [ ] `posture/expert-posture` est dans `skills:` (Bucket A) si l'agent a un rôle de conseil ou d'expertise
- [ ] `posture/tool-question` est dans `skills:` (Bucket A) **et** `permission: question: allow` est dans le frontmatter si l'agent `primary` a besoin de poser des questions structurées à l'utilisateur
- [ ] `beads-plan` est dans `skills:` (Bucket A) si l'agent lit ou crée des tickets Beads
- [ ] `beads-dev` est en plus dans `skills:` (Bucket A) si l'agent exécute (clame, implémente, clôt) des tickets
- [ ] La matrice de dépendances dans `docs/architecture/skills.fr.md` est mise à jour

### Skill

- [ ] La `description` dans le frontmatter est renseignée
- [ ] Le contenu est opérationnel (règles + exemples) — pas théorique
- [ ] Pas de duplication avec les skills existants
- [ ] Le skill est ajouté dans le tableau du bon domaine dans `docs/architecture/skills.fr.md` avec son marqueur de bucket (A) ou (B)
- [ ] **Skills Bucket A** : les agents qui en ont besoin l'ont dans leur frontmatter `skills:` ; matrice de dépendances mise à jour
- [ ] **Skills Bucket B** : les agents qui en ont besoin l'ont dans leur frontmatter `native_skills:` ; la section guide du corps de l'agent liste le skill avec une description du déclencheur de chargement
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
permission:
  skill: allow                            # ← active l'outil skill natif (Bucket B)
skills:                                   # ← Bucket A : toujours actif dès le premier token
  - developer/dev-standards-universal     #   principes communs à tous les devs
  - developer/dev-standards-simplicity    #   KISS/YAGNI — toujours actif
  - developer/beads-plan                  #   lit et crée des tickets
  - developer/beads-dev                   #   exécute des tickets
  - developer/developer-handoff-format    #   contrat de handoff (Bucket A — obligatoire)
native_skills:                            # ← Bucket B : chargés à la demande via l'outil skill
  - developer/dev-standards-security      #   principes de sécurité
  - developer/dev-standards-security-hardening  # spécificités de hardening
  - developer/dev-standards-backend       #   contexte d'application
  - developer/dev-standards-testing       #   il écrit des tests
  - developer/dev-standards-git           #   il commit
---
```

**Ordre recommandé pour les skills Bucket A :** universel → simplicity → beads-plan → beads-dev → format de handoff.

**Section guide des skills natives** (obligatoire dans le corps de l'agent quand `native_skills` est non vide) :

```markdown
## Skills disponibles

Chargez la ou les skills pertinentes via l'outil `skill` avant de commencer :

| Skill | Charger quand |
|-------|--------------|
| `dev-standards-security` | Toute tâche impliquant l'auth, la validation des inputs, les secrets, les injections |
| `dev-standards-security-hardening` | CORS, headers HTTP, JWT, sessions, rate limiting, chiffrement |
| `dev-standards-backend` | Architecture en couches, DTOs, services, repositories |
| `dev-standards-testing` | Écriture ou review de tests |
| `dev-standards-git` | Commit ou review de l'historique git |
```

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
