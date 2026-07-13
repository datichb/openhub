# Specification Technique — Team Features Phase 1 & 2

> Spec technique pour les fonctionnalites d'amelioration du travail en equipe
> sur les projets cibles via le hub.
>
> Statut : **IMPLEMENTE** (features 1-5, Phase 1 + Phase 2)
> Date : 13 juillet 2026

---

## Vue d'ensemble

### Objectifs

Ameliorer l'experience des equipes qui utilisent le hub pour travailler sur
leurs projets. L'accent est mis sur :

1. **L'assurance qualite** — policies non-contournables, conventions automatiques
2. **L'acceleration du delivery** — briefs de reprise, patterns reutilisables, parallelisme
3. **La visibilite** — savoir qui fait quoi, ou en est le travail

### Sequencement

```
Phase 1 (dans l'ordre) :
  1. Team Policies         — fondation, contraintes pour tout le reste
  2. Takeover Brief        — quick win collaboratif fort
  3. Visibilite WIP        — feedback immediat equipe
  4. Patterns Library      — accelerateur long terme

Phase 2 :
  5. Parallelisme coordonne (max 3 sessions, parametrable)
```

### Architecture d'integration

Les features s'integrent aux couches existantes du hub :

```
┌─────────────────────────────────────────────────────────────────┐
│                         CLI Commands                             │
│  oh policies · oh claim transfer · oh team board · oh patterns  │
└──────────────────────────────┬──────────────────────────────────┘
                               │
┌──────────────────────────────▼──────────────────────────────────┐
│                       Domain Layer                               │
│  cli/internal/teamstate/                                        │
│    policies.go · takeover.go · patterns.go                      │
│  cli/internal/parallel/ (Phase 2)                               │
│    coordinator.go · context.go · merger.go                      │
│  cli/internal/opencode/                                         │
│    opencode.go (+ RunHeadless)                                  │
└──────────────────────────────┬──────────────────────────────────┘
                               │
┌──────────────────────────────▼──────────────────────────────────┐
│                    MCP Team Server                               │
│  Tools ajoutes :                                                │
│    team_policies · team_takeover_brief                           │
│    team_patterns_list · team_patterns_read · team_patterns_propose│
└──────────────────────────────┬──────────────────────────────────┘
                               │
┌──────────────────────────────▼──────────────────────────────────┐
│                     Skills (Agents)                              │
│  team-policies-enforcement.md                                   │
│  takeover-context-protocol.md                                   │
│  planner-patterns-protocol.md                                   │
│  parallel-coordination.md (Phase 2)                             │
└──────────────────────────────┬──────────────────────────────────┘
                               │
┌──────────────────────────────▼──────────────────────────────────┐
│                   Team-State Git Repo                            │
│  policies.toml                                                  │
│  projects/<project>/policies-override.toml                      │
│  projects/<project>/takeover-briefs/                            │
│  patterns/                                                      │
└─────────────────────────────────────────────────────────────────┘
```

---

## 1. Team Policies [PHASE 1 — VERROUILLE]

### 1.1 Concept

Des regles d'equipe qui s'appliquent a TOUS les projets, injectees dans les
agents comme contraintes. Configurables avec niveau d'enforcement (refuse/warn)
par projet/equipe. Supportent les regles structurees ET les regles custom
arbitraires.

### 1.2 Decisions de design

| Decision | Valeur |
|----------|--------|
| Fichier source | `team-state/policies.toml` + `projects/<project>/policies-override.toml` |
| MCP tool | `team_policies` (un seul, retourne tout merge) |
| Enforcement | Double : CLI hard (structurees) + Agent soft (custom/contextuelles) |
| Regles custom | Oui, type `forbidden_pattern` + regex arbitraire a la demande |
| Scope agents | Matrice : seuls les agents concernes verifient les policies pertinentes |

### 1.3 Structure team-state

```
team-state/
  policies.toml                              # regles globales equipe
  projects/<project>/policies-override.toml  # overrides par projet (optionnel)
```

### 1.4 Format policies.toml

```toml
# ============================================================
# REGLES STRUCTUREES (categories connues)
# ============================================================

[policies.branch_naming]
type = "regex"
rule = "^(feat|fix|hotfix|chore|refactor)/[a-z0-9-]+"
enforcement = "refuse"    # refuse | warn
message = "Branch must follow pattern: feat/xxx, fix/xxx, etc."

[policies.commit_format]
type = "regex"
rule = "^(feat|fix|docs|style|refactor|test|chore)(\\(.+\\))?: .+"
enforcement = "refuse"
message = "Commit must follow Conventional Commits format"

[policies.review_required]
type = "boolean"
enabled = true
enforcement = "refuse"
message = "Human review required before merge"

[policies.tests_required]
type = "boolean"
enabled = true
enforcement = "warn"
message = "Tests should pass before review"

[policies.max_ticket_wip]
type = "limit"
max = 2
enforcement = "warn"
message = "Limit WIP to 2 tickets per member"

# ============================================================
# REGLES CUSTOM (ajoutables a la demande)
# ============================================================

[policies.custom_no_console_log]
type = "forbidden_pattern"
patterns = ["console.log", "console.warn"]
scope = "diff_only"          # diff_only | all_files
enforcement = "warn"
message = "Remove console.log before commit"

[policies.custom_no_any_typescript]
type = "forbidden_pattern"
patterns = [": any", "as any"]
scope = "diff_only"
enforcement = "warn"
message = "Avoid 'any' type in TypeScript"

[policies.custom_max_file_length]
type = "limit"
max = 500
unit = "lines"
scope = "modified_files"
enforcement = "warn"
message = "Files should not exceed 500 lines"

[policies.custom_required_tests_pattern]
type = "regex"
rule = "\\.(spec|test)\\.(ts|js)$"
scope = "per_feature_branch"
enforcement = "warn"
message = "Each feature branch should include test files"
```

### 1.5 Overrides par projet

```toml
# projects/T-SRU/policies-override.toml
# Les overrides peuvent UNIQUEMENT rendre plus strict, pas plus permissif.

[policies.tests_required]
enforcement = "refuse"   # plus strict sur ce projet (warn -> refuse)

[policies.custom_no_console_log]
enforcement = "refuse"   # bloquant sur T-SRU
```

### 1.6 Matrice enforcement (CLI hard / Agent soft)

#### CLI Hard Checks

| Commande CLI | Policies verifiees | Comportement |
|--------------|-------------------|--------------|
| `oh claim <ticket>` | `max_ticket_wip` | Compte les claims actifs du membre. Si >= max : refuse ou warn selon enforcement |
| `oh start` (creation branche) | `branch_naming` | Valide le nom de branche genere/propose contre la regex |
| Post-commit hook | `commit_format` | Valide le message de commit contre la regex |
| `oh release <ticket>` | `review_required`, `tests_required` | Verifie qu'une review a ete faite, que les tests sont passes |

#### Agent Soft Checks (via skill)

| Agent(s) | Policies verifiees | Moment |
|----------|-------------------|--------|
| `orchestrator-dev` | `branch_naming` | Avant delegation au developer |
| `orchestrator-dev` | `review_required` | Avant de considerer un ticket done |
| `developer-*` | `commit_format` | Avant chaque commit |
| `developer-*` | `forbidden_patterns` (custom) | Sur leur propre diff, avant commit |
| `reviewer` | `tests_required` | Mentionne dans le rapport de review si non respecte |
| `orchestrator` | `max_ticket_wip` | Avant de proposer un nouveau ticket |

#### Agents NON concernes

- `designer` — ne touche pas au code, pas de policy code
- `documentarian` — ecrit des docs, pas de policy code applicable
- `pathfinder` — read-only, estime seulement
- `onboarder` — read-only, genere un wiki

### 1.7 MCP tool team_policies

**Nom** : `team_policies`
**Acces** : tous les agents (read-only)
**Parametres** : `project` (string, optionnel — si absent, retourne les globales)
**Retour** : JSON avec les policies mergeees (globales + overrides projet)

```json
{
  "policies": [
    {
      "name": "branch_naming",
      "type": "regex",
      "rule": "^(feat|fix|hotfix|chore|refactor)/[a-z0-9-]+",
      "enforcement": "refuse",
      "message": "Branch must follow pattern: feat/xxx, fix/xxx, etc."
    },
    {
      "name": "custom_no_console_log",
      "type": "forbidden_pattern",
      "patterns": ["console.log", "console.warn"],
      "scope": "diff_only",
      "enforcement": "refuse",
      "message": "Remove console.log before commit"
    }
  ]
}
```

### 1.8 Fichiers a creer

| Fichier | Description |
|---------|-------------|
| `cli/internal/teamstate/policies.go` | Structs `Policy`, `PolicyResult`, `PolicyContext`. Fonctions `LoadPolicies(project)`, `MergePolicies(global, override)`, `CheckPolicy(policy, context)`, `CheckAll(project, context)` |
| `cli/internal/teamstate/policies_test.go` | Tests : load, merge, check regex, check boolean, check limit, check forbidden_pattern, override merge |
| `cli/cmd/policies.go` | Commandes cobra : `oh policies list [--project]`, `oh policies check [--project]`, `oh policies add` (interactif) |
| `skills/shared/team-policies-enforcement.md` | Skill — matrice agent/policy, quand verifier, comportement refuse vs warn |

### 1.9 Fichiers a modifier

| Fichier | Modification |
|---------|-------------|
| `cli/internal/mcp/team/server.go` | Ajouter tool `team_policies` : handler `handleTeamPolicies(params)` |
| `skills/shared/team-awareness.md` | Ajouter reference : "Si equipe active, consulter policies en debut de session. Voir team-policies-enforcement." |
| `cli/internal/teamstate/errors.go` | Ajouter : `ErrPolicyViolation`, `ErrPolicyFileInvalid` |
| `cli/cmd/claim.go` | Dans `runClaim` : verifier `max_ticket_wip` avant `CreateClaim` |
| `cli/cmd/team.go` | Enregistrer sous-commande `policies` |

### 1.10 Criteres de done

- [ ] `oh policies list` affiche toutes les policies actives (globales + override)
- [ ] `oh policies check` verifie l'etat courant et reporte les violations
- [ ] `oh policies add` permet d'ajouter une regle custom interactivement
- [ ] `oh claim` refuse si `max_ticket_wip` atteint (enforcement = refuse)
- [ ] `oh claim` warn si `max_ticket_wip` atteint (enforcement = warn)
- [ ] MCP tool `team_policies` retourne les policies mergeees
- [ ] Skill `team-policies-enforcement.md` est injectee et testee manuellement
- [ ] Tests unitaires passent (policies_test.go)
- [ ] Override par projet fonctionne (plus strict seulement)

---

## 2. Takeover Brief [PHASE 1 — VERROUILLE]

### 2.1 Concept

Quand un ticket change de proprietaire (transfer explicite ou reprise d'un
ticket stale), un brief contextuel est genere pour donner au successeur tout
le contexte necessaire. Le brief est disponible en trois niveaux :

1. **Donnees brutes** (.toml) — structure machine-readable
2. **Resume template** (.md) — genere sans LLM, lisible humain immediatement
3. **Enrichi** (.enriched.md) — genere par LLM on-demand, contexte complet

### 2.2 Decisions de design

| Decision | Valeur |
|----------|--------|
| Nommage | `takeover-brief` (pas "handoff" — reserve aux contrats inter-agents) |
| Generation | Hybride : donnees brutes + template Go (sans LLM) + enrichissement LLM on-demand |
| Declencheurs | `oh claim transfer` (auto) + `oh claim` sur ticket stale (propositionnel) |
| Stale detection | Configurable via `stale_days` (defaut: 3) |
| Stockage | `team-state/projects/<project>/takeover-briefs/` |
| Enrichissement | Via `opencode run` (RunHeadless) avec agent dedie `brief-enricher` |
| MCP tool | `team_takeover_brief` (lecture par les agents) |

### 2.3 Declencheurs

#### Transfer explicite

```bash
oh claim transfer bd-42 --to alice
```

Flux :
1. `TransferClaim()` est appele (existant)
2. **NOUVEAU** : `GenerateRawBrief()` collecte les donnees
3. **NOUVEAU** : `RenderTemplateBrief()` genere le .md
4. Les deux fichiers sont commites dans team-state
5. Event `claim.transferred` est emis (existant)
6. L'utilisateur est informe : "Brief de reprise genere. `oh takeover-brief show bd-42`"

#### Reprise de ticket stale

```bash
oh claim bd-42
# → "Ce ticket est assigne a benjamin depuis 5 jours sans activite."
# → "Generer un brief de reprise ? [Y/n]"
```

Flux :
1. `CreateClaim()` detecte que le ticket a un claim existant
2. Verifie `LastActivity` vs `stale_days` (configurable)
3. Si stale : propose a l'utilisateur de generer un brief
4. Si accepte : genere le brief, puis transfere le claim
5. Si refuse : transfere le claim sans brief

### 2.4 Format donnees brutes (.toml)

```toml
# team-state/projects/T-SRU/takeover-briefs/bd-42_2026-07-13.toml

[meta]
ticket_id = "bd-42"
project = "T-SRU"
transferred_from = "benjamin"
transferred_to = "alice"
transfer_date = "2026-07-13T14:30:00Z"
reason = "transfer"          # transfer | stale
stale_days = 0               # 0 si transfer explicite, N si stale

[activity]
sessions_count = 3
first_session = "2026-07-10T09:00:00Z"
last_session = "2026-07-12T16:45:00Z"
total_duration_minutes = 185

[git]
branch = "feat/bd-42-auth-timeout"
commits_count = 7
last_commit_message = "fix: handle token expiry edge case"
last_commit_date = "2026-07-12T16:30:00Z"

[[git.files_modified]]
path = "src/auth/token-service.ts"
additions = 142
deletions = 23

[[git.files_modified]]
path = "src/middleware/auth-guard.ts"
additions = 35
deletions = 8

[[git.files_created]]
path = "src/auth/token-rotation.ts"
additions = 89

[[events]]
ts = "2026-07-10T09:15:00Z"
type = "session.complete"
summary = "Initial implementation of token service"

[[events]]
ts = "2026-07-11T14:00:00Z"
type = "session.complete"
summary = "Added guard middleware, started tests"

[[events]]
ts = "2026-07-12T16:45:00Z"
type = "session.complete"
summary = "Fixed edge case, tests partial"
```

### 2.5 Template Go resume (.md)

Genere automatiquement par un template Go, sans appel LLM.
Disponible immediatement apres le transfer/stale.

```markdown
# Takeover Brief: bd-42

**Transfere de** benjamin → alice | **Date** 2026-07-13 | **Raison** transfer

## Activite
- 3 sessions sur 3 jours (10 juil → 12 juil)
- Duree totale : ~3h05
- 7 commits sur branche `feat/bd-42-auth-timeout`

## Fichiers principaux
| Fichier | Action | Lignes |
|---------|--------|--------|
| `src/auth/token-service.ts` | modifie | +142/-23 |
| `src/middleware/auth-guard.ts` | modifie | +35/-8 |
| `src/auth/token-rotation.ts` | cree | +89 |

## Dernier etat connu
- Dernier commit : "fix: handle token expiry edge case" (12 juil 16:30)
- Derniere session : "Fixed edge case, tests partial"

## Historique sessions
1. **10 juil** — Initial implementation of token service
2. **11 juil** — Added guard middleware, started tests
3. **12 juil** — Fixed edge case, tests partial

---
*Brief genere automatiquement par template. Utiliser `oh takeover-brief enrich bd-42`
pour une version enrichie par IA.*
```

### 2.6 Enrichissement LLM (RunHeadless)

Commande : `oh takeover-brief enrich <ticket-id>`

Flux :
1. Lit le `.toml` (donnees brutes)
2. Lit les fichiers modifies (source actuel) pour comprendre l'etat du code
3. Compose un prompt pour l'agent `brief-enricher`
4. Appelle `opencode.RunHeadless()` avec ce prompt
5. Stocke le resultat dans `.enriched.md`

Le brief enrichi ajoute :
- Synthese des decisions architecturales prises
- Questions ouvertes identifiees (TODO, FIXME, patterns incomplets)
- Risques identifies (tests manquants, edge cases non couverts)
- Suggestion de prochaines etapes

### 2.7 Agent brief-enricher

Fichier : `agents/utility/brief-enricher.md`

```yaml
---
name: brief-enricher
description: Agent utilitaire pour enrichir les takeover briefs
model: anthropic/claude-sonnet-4-5
mode: subagent
permissions:
  allow:
    - read
    - glob
    - grep
  deny:
    - edit
    - write
    - bash
    - task
---
```

Cet agent :
- Est read-only (ne modifie rien)
- A acces au filesystem pour lire les fichiers mentionnes dans le brief
- Produit un Markdown structure en sortie
- Est invoque uniquement via `RunHeadless` (jamais interactif)

### 2.8 MCP tool team_takeover_brief

**Nom** : `team_takeover_brief`
**Acces** : tous les agents (read-only)
**Parametres** : `project` (string), `ticket_id` (string)
**Retour** : Contenu du brief (.enriched.md si existe, sinon .md template)

L'agent successeur appelle ce tool au demarrage de sa session sur le ticket.

### 2.9 Skill takeover-context-protocol

Fichier : `skills/orchestrator/takeover-context-protocol.md`

Contenu cle :
```markdown
## Protocole de reprise de contexte (Takeover)

Au demarrage d'un ticket, SI un takeover-brief existe :

1. Appeler `team_takeover_brief` avec le project et ticket_id
2. LIRE attentivement le brief retourne
3. Identifier :
   - Les fichiers principaux a consulter en priorite
   - Les decisions deja prises (ne pas les remettre en question sauf probleme)
   - Les questions ouvertes (les adresser ou les poser a l'utilisateur)
   - L'etat d'avancement (reprendre la ou le predecesseur s'est arrete)
4. INFORMER l'utilisateur : "Ce ticket a ete repris de [X]. Contexte charge."
5. Ne PAS repartir de zero — continuer le travail existant
```

### 2.10 Fichiers a creer

| Fichier | Description |
|---------|-------------|
| `cli/internal/teamstate/takeover.go` | Structs `TakeoverBrief`, `TakeoverMeta`, `TakeoverActivity`, `TakeoverGit`. Fonctions `GenerateRawBrief()`, `ReadBrief()`, `ListBriefs()`, `BriefExists()` |
| `cli/internal/teamstate/takeover_template.go` | Template Go pour generer le .md sans LLM. Fonction `RenderTemplateBrief(brief) string` |
| `cli/internal/teamstate/takeover_test.go` | Tests unitaires |
| `skills/orchestrator/takeover-context-protocol.md` | Skill pour les agents : comment utiliser le brief |
| `agents/utility/brief-enricher.md` | Agent dedie a l'enrichissement (read-only) |

### 2.11 Fichiers a modifier

| Fichier | Modification |
|---------|-------------|
| `cli/cmd/claim.go` | Dans `runClaimTransfer` : appeler `GenerateRawBrief` + `RenderTemplateBrief`. Dans `runClaim` : detecter stale, proposer generation. Ajouter commande `oh takeover-brief show/enrich/list` |
| `cli/internal/mcp/team/server.go` | Ajouter tool `team_takeover_brief` |
| `skills/orchestrator/team-coordination.md` | Ajouter : "Au demarrage, si takeover-brief existe, consulter via `team_takeover_brief`" |
| `cli/internal/teamstate/teamconfig.go` | Ajouter champ `StaleDays int` (defaut 3) dans la struct ou dans policies |
| `cli/internal/teamstate/claims.go` | Ajouter champ `LastActivity time.Time` au Claim |
| `cli/internal/teamstate/errors.go` | Ajouter : `ErrBriefNotFound` |
| `cli/internal/opencode/opencode.go` | Ajouter fonction `RunHeadless(HeadlessOpts) (string, error)` |

### 2.12 Criteres de done

- [ ] `oh claim transfer bd-42 --to alice` genere automatiquement le brief (.toml + .md)
- [ ] `oh claim bd-42` detecte un ticket stale et propose la generation
- [ ] `oh takeover-brief show bd-42` affiche le brief (enrichi si dispo, sinon template)
- [ ] `oh takeover-brief list [--project]` liste les briefs existants
- [ ] `oh takeover-brief enrich bd-42` genere la version enrichie via LLM
- [ ] MCP tool `team_takeover_brief` retourne le brief au format attendu
- [ ] L'agent successeur charge le contexte automatiquement au demarrage
- [ ] `RunHeadless()` fonctionne correctement avec l'agent `brief-enricher`
- [ ] Tests unitaires passent (takeover_test.go)
- [ ] Le brief template est genere en < 2s (pas de latence LLM)

---

## 3. Visibilite WIP [PHASE 1 — SPEC LEGERE]

### 3.1 Concept

Vue claire de qui travaille sur quoi dans l'equipe, avec deux niveaux
d'affichage (simple / detail) et deux interfaces (CLI tabulaire / TUI
BubbleTea interactif).

### 3.2 Vue simple (`oh team status` enrichi)

```
┌──────────────────────────────────────────────────────────────────┐
│  Team Status — T-SRU                          Last sync: 10:42   │
├──────────┬──────┬────────────────────┬─────────────┬─────────────┤
│ Member   │ Role │ Ticket             │ Status      │ Since       │
├──────────┼──────┼────────────────────┼─────────────┼─────────────┤
│ Benjamin │ lead │ T-SRU/bd-42        │ in_progress │ 2h15m       │
│          │      │ T-SRU/bd-45        │ review      │ 30m         │
│ Alice    │ dev  │ T-SRU/bd-43        │ in_progress │ 45m         │
│ Bob      │ dev  │ — (idle)           │             │             │
└──────────┴──────┴────────────────────┴─────────────┴─────────────┘
  3 tickets actifs · 1 en review · 0 blocked
```

### 3.3 Vue detail (`oh team status --detail`)

Sous chaque ticket actif, affiche les sub-beads et la progression :
```
│ Benjamin │ lead │ T-SRU/bd-42        │ in_progress │ 2h15m       │
│          │      │   ├ bd-42a Schema  │ completed   │             │
│          │      │   ├ bd-42b Service │ in_progress │             │
│          │      │   └ bd-42c Tests   │ pending     │             │
│          │      │   Progress: 1/3    │ Agent: dev  │             │
```

### 3.4 TUI Board (`oh team board`)

BubbleTea interactif avec :
- Vue Kanban (colonnes : idle / in_progress / review / blocked / completed)
- Navigation clavier (j/k pour items, h/l ou tab entre colonnes)
- Touche `d` pour basculer detail
- Touche `r` pour refresh (pull team-state)
- Touche `q` pour quitter
- Refresh automatique au pull (pas de mode watch continu)

### 3.5 Fichiers concernes

**A creer :**
- `cli/cmd/team_board.go` — commande `oh team board`
- `cli/internal/tui/views/teamboard/model.go` — model BubbleTea
- `cli/internal/tui/views/teamboard/view.go` — rendu

**A modifier :**
- `cli/cmd/team.go` — enrichir `runTeamStatus` + flag `--detail`
- `cli/internal/teamstate/claims.go` — ajouter `LastActivity`, `Progress`

---

## 4. Patterns Library [PHASE 1 — SPEC LEGERE]

### 4.1 Concept

Bibliotheque de decompositions reussies capitalisees pour accelerer les futurs
plannings. Alimentee par le planner (auto), le pathfinder (auto), et les devs
humains (manuel). Stockee dans le team-state (partagee cross-projet).

### 4.2 Structure team-state

```
team-state/
  patterns/
    index.toml           # catalogue des patterns (metadonnees)
    crud-api.md          # contenu du pattern
    integration-externe.md
    migration-db.md
    ...
```

### 4.3 Format pattern

**index.toml** :
```toml
[[patterns]]
name = "crud-api"
tags = ["backend", "api", "crud"]
complexity = "medium"          # low | medium | high
source = "planner"             # planner | pathfinder | manual
project = "T-SRU"             # projet d'origine
validated = true               # humain a valide
created_at = "2026-07-10"
```

**Pattern individuel** (ex: `crud-api.md`) :
```markdown
---
name: crud-api
type: decomposition
complexity: medium
tags: [backend, api, crud]
---

# Pattern : CRUD API Endpoint

## Contexte d'usage
Quand on doit creer un nouvel endpoint CRUD complet.

## Decomposition type
1. **Schema/Model** (low) - Definition du modele de donnees
2. **Repository** (low) - Couche d'acces aux donnees
3. **Service** (medium) - Logique metier
4. **Controller** (low) - Endpoint HTTP
5. **Validation** (low) - DTOs et validation
6. **Tests unitaires** (medium) - Tests service + repository
7. **Tests integration** (medium) - Tests endpoint E2E
8. **Documentation** (low) - OpenAPI + guide

## Dependances typiques
- 1 → 2 → 3 → 4 (sequentiel)
- 5 en parallele de 3
- 6, 7 apres 4
- 8 apres 7

## Variantes connues
- Si auth requise : ajouter middleware auth entre 4 et 5
- Si pagination : ajouter ticket pagination dans 4
```

### 4.4 Workflow alimentation

| Source | Declencheur | Validated |
|--------|-------------|-----------|
| Planner (auto) | Apres un plan execute avec succes (tous tickets completed) | `false` (proposition) |
| Pathfinder (auto) | Quand il reconnait un pattern recurrent | `false` (proposition) |
| Humain (`oh patterns add`) | Commande manuelle | `true` (direct) |
| Humain (`oh patterns validate`) | Valide une proposition | `true` |

### 4.5 Workflow utilisation (par le planner)

1. AVANT la phase de decomposition, appeler `team_patterns_list` avec tags pertinents
2. Si match (>= 2 tags en commun) : appeler `team_patterns_read`
3. UTILISER comme base de decomposition, ADAPTER au contexte specifique
4. MENTIONNER le pattern utilise : "Base: pattern crud-api, adapte pour..."
5. Si pas de match : decomposer normalement

### 4.6 Fichiers concernes

**A creer :**
- `cli/internal/teamstate/patterns.go` — Struct Pattern, CRUD, Search par tags
- `cli/internal/teamstate/patterns_test.go`
- `cli/cmd/patterns.go` — `oh patterns list/show/add/validate/remove`
- `skills/planning/planner-patterns-protocol.md` — comment le planner utilise

**A modifier :**
- `cli/internal/mcp/team/server.go` — tools `team_patterns_list`, `team_patterns_read`, `team_patterns_propose`
- `skills/planning/planner-workflow.md` — phase 1 : consulter patterns
- `skills/planning/pathfinder-protocol.md` — noter les patterns reconnus

---

## 5. Parallelisme Coordonne [PHASE 2 — SPEC LEGERE]

### 5.1 Concept

Un dev peut lancer jusqu'a N agents en parallele (defaut: 3, parametrable) sur
N tickets distincts. Chaque agent travaille dans un worktree independant. Un
coordinateur maintient un contexte partage leger et gere la strategie de merge.

### 5.2 Architecture coordinateur

```
oh start --parallel --tickets bd-42,bd-43,bd-44

┌─────────────────────────────────────────────────────────────┐
│                    Coordinator                               │
│  1. Verifie len(tickets) <= max_parallel_sessions           │
│  2. Cree N worktrees (sibling via worktree.SiblingPath)     │
│  3. Lance N sessions opencode (via serve + run --attach)    │
│  4. Maintient parallel-context.json                         │
│  5. Propose merge sequentiel a la fin                       │
└──────────┬─────────────────┬─────────────────┬──────────────┘
           │                 │                 │
    ┌──────▼──────┐   ┌─────▼──────┐   ┌─────▼──────┐
    │ Worktree A  │   │ Worktree B │   │ Worktree C │
    │ bd-42       │   │ bd-43      │   │ bd-44      │
    │ (session)   │   │ (session)  │   │ (session)  │
    └─────────────┘   └────────────┘   └────────────┘
```

### 5.3 Shared context (format)

Fichier leger mis a jour par le coordinateur. PAS le contexte LLM des sessions.

```json
{
  "started_at": "2026-07-13T10:00:00Z",
  "max_sessions": 3,
  "sessions": [
    {
      "ticket": "bd-42",
      "worktree": "/path/to/project-feat-bd-42",
      "branch": "feat/bd-42",
      "files_modified": ["src/auth/service.ts"],
      "files_created": ["src/auth/token-rotation.ts"],
      "modules_created": [
        {"name": "TokenRotationService", "path": "src/auth/token-rotation.ts"}
      ],
      "status": "in_progress"
    }
  ],
  "potential_conflicts": [
    {
      "file": "src/config/index.ts",
      "sessions": ["bd-42", "bd-43"],
      "severity": "low"
    }
  ]
}
```

Les agents consultent ce contexte (via skill) AVANT de modifier un fichier.
Pas de blocage — juste de la conscience situationnelle.

### 5.4 Merge strategy

| Source du ticket | Strategie |
|-----------------|-----------|
| **Beads interne** | Merge sequentiel propose par le coordinateur. Resolution auto si trivial (imports, ajouts non-conflictuels). **Sous validation humaine** (diff affiche, confirmation demandee) |
| **Tracker externe** (GitLab/Jira) | Chaque worktree reste sur sa branche. **AUCUN merge automatique**. Le dev gere via son workflow MR/PR normal |

### 5.5 Integration opencode serve

Pour eviter N cold boots MCP :
1. Le coordinateur lance `opencode serve --port <random>`
2. Les N sessions sont lancees via `opencode run --attach http://localhost:<port> --agent orchestrator-dev "prompt"`
3. A la fin, le serveur est arrete

### 5.6 Fichiers concernes

**A creer :**
- `cli/internal/parallel/coordinator.go` — orchestration N sessions
- `cli/internal/parallel/context.go` — shared context read/write
- `cli/internal/parallel/merger.go` — strategie merge (auto Beads / block externe)
- `cli/internal/parallel/state.go` — parallel-state.json
- `cli/internal/parallel/coordinator_test.go`
- `skills/orchestrator/parallel-coordination.md`

**A modifier :**
- `cli/cmd/start.go` — flags `--parallel`, `--tickets`, `--max-sessions`
- `cli/internal/worktree/worktree.go` — bulk create
- `skills/shared/team-awareness.md` — consulter parallel-context
- `cli/internal/teamstate/teamconfig.go` — config `max_parallel_sessions = 3`

### 5.7 Contraintes

- Maximum 3 sessions paralleles par defaut
- Parametrable via `hub.toml` : `[parallel] max_sessions = 3`
- Auto-merge Beads uniquement, JAMAIS pour tickets trackers externes
- Toujours sous validation humaine (meme pour les merges Beads)

---

## Annexe A : RunHeadless (extension opencode.go)

Nouvelle fonction dans `cli/internal/opencode/opencode.go` :

```go
// HeadlessOpts configures a non-interactive opencode run.
type HeadlessOpts struct {
    ProjectPath string   // Working directory
    ProjectID   string   // Hub project ID (for env)
    Agent       string   // Agent to use (e.g. "brief-enricher")
    Prompt      string   // The prompt to send
    Format      string   // Output format: "" (default) or "json"
    Model       string   // Optional model override
    Files       []string // Files to attach
}

// RunHeadless executes opencode in non-interactive mode and captures output.
// Uses `opencode run` under the hood.
func RunHeadless(opts HeadlessOpts) (string, error) {
    bin, err := FindBinary()
    if err != nil {
        return "", fmt.Errorf("opencode binary not found: %w", err)
    }

    args := []string{"run"}
    if opts.Agent != "" {
        args = append(args, "--agent", opts.Agent)
    }
    if opts.Format != "" {
        args = append(args, "--format", opts.Format)
    }
    if opts.Model != "" {
        args = append(args, "--model", opts.Model)
    }
    for _, f := range opts.Files {
        args = append(args, "--file", f)
    }
    args = append(args, "--auto")
    args = append(args, opts.Prompt)

    cmd := exec.Command(bin, args...)
    cmd.Dir = opts.ProjectPath
    cmd.Env = buildEnv(opts.ProjectPath, opts.ProjectID, "")

    output, err := cmd.CombinedOutput()
    if err != nil {
        return string(output), fmt.Errorf("opencode run failed: %w\noutput: %s",
            err, string(output))
    }
    return string(output), nil
}
```

---

## Annexe B : Matrice MCP tools ajoutes

| Tool | Phase | Acces | Description |
|------|-------|-------|-------------|
| `team_policies` | 1 | Tous les agents | Retourne les policies mergeees pour le projet |
| `team_takeover_brief` | 1 | Tous les agents | Retourne le brief de reprise d'un ticket |
| `team_patterns_list` | 1 | Tous les agents | Liste les patterns (filtrable par tags) |
| `team_patterns_read` | 1 | Tous les agents | Retourne le contenu d'un pattern |
| `team_patterns_propose` | 1 | planner, pathfinder | Propose un nouveau pattern (validated=false) |

Total apres implementation : 7 existants + 5 nouveaux = **12 MCP tools team**

---

## Annexe C : Estimation effort

| # | Feature | Fichiers nouveaux | Fichiers modifies | Effort |
|---|---------|-------------------|-------------------|--------|
| 1 | Team Policies | 4 | 5 | 2-3 jours |
| 2 | Takeover Brief | 5 | 7 | 2-3 jours |
| 3 | Visibilite WIP | 3 | 2 | 1-2 jours |
| 4 | Patterns Library | 4 | 3 | 2-3 jours |
| 5 | Parallelisme | 6 | 4 | 5-7 jours |
| | **Total Phase 1** | **16** | **17** | **~8-11 jours** |
| | **Total Phase 2** | **+6** | **+4** | **+5-7 jours** |
| | **Total global** | **22** | **21** | **~13-18 jours** |
