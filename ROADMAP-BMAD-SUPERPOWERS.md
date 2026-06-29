# Roadmap — Opportunités BMAD × Superpowers

Analyse comparative de [BMAD Method](https://github.com/bmad-code-org/bmad-method) et [Superpowers](https://github.com/obra/Superpowers) pour enrichissement du hub.

**Date d'analyse :** 2026-06-24  
**Version hub au moment de l'analyse :** 1.5.0

---

## Statuts

| Symbole | Signification |
|---|---|
| `[ ]` | À faire |
| `[~]` | En cours |
| `[x]` | Terminé |
| `[-]` | Abandonné |

---

## Phase 1 — Enrichissement skills (aucun agent touché)

Skills purs, aucune dépendance inter-items, entièrement parallélisables.

### G — SDO Frontmatter (Skill Discovery Optimization) ✅
**Source :** Superpowers  
**Principe :** Ajouter un champ `description:` structuré dans le frontmatter YAML de chaque SKILL.md pour que l'IA trouve le bon skill au bon moment.  
**Format :** `description: "Déclenche quand... Couvre... Mots-clés : ..."`

- [x] Auditer tous les SKILL.md sans `description:` dans leur frontmatter
- [x] Ajouter `description:` dans `skills/developer/*.md`
- [x] Ajouter `description:` dans `skills/developer/stacks/*.md`
- [x] Ajouter `description:` dans `skills/auditor/*.md`
- [x] Ajouter `description:` dans `skills/qa/*.md`
- [x] Ajouter `description:` dans `skills/reviewer/*.md`
- [x] Ajouter `description:` dans `skills/planning/*.md`
- [x] Ajouter `description:` dans `skills/orchestrator/*.md`
- [x] Ajouter `description:` dans `skills/design/*.md`
- [x] Ajouter `description:` dans `skills/designer/*.md`
- [x] Ajouter `description:` dans `skills/documentarian/*.md`
- [x] Ajouter `description:` dans `skills/posture/*.md`
- [x] Ajouter `description:` dans `skills/shared/*.md`
- [x] Ajouter `description:` dans `skills/adapters/*.md`

**Critère de complétion :** 100% des SKILL.md ont un champ `description:` non-vide.

> **Note :** 132/133 fichiers avaient déjà un `description:` de qualité. Seul `skills/planning/websearch-stack-research.md` était sans frontmatter — corrigé le 2026-06-24.

---

### B — `reviewer-reception` skill ✅
**Source :** Superpowers (`receiving-code-review`)  
**Principe :** Compléter le cycle review. Le hub a un `reviewer` mais aucun skill pour guider le developer à *répondre* à une review.  
**Position dans le workflow :** reviewer → handoff → orchestrator-dev → brief → **developer (utilise ce skill)**

- [x] Créer `skills/reviewer/reviewer-reception.md`
  - 6-step pattern : READ → RESTATE → VERIFY → EVALUATE → RESPOND → IMPLEMENT
  - Gestion du pushback (quand et comment contester)
  - YAGNI check pour les suggestions "professionnelles" non demandées
  - Exemples de réponses (acknowledge, pushback technique, clarification)
  - Forbidden responses (accord performatif sans vérification)
- [x] Déclarer bucket-B dans `native_skills` du developer

**Fichiers créés :** `skills/reviewer/reviewer-reception.md`  
**Fichiers modifiés :** `agents/developer/developer.md` (ajout `native_skills`)

---

### I — Reviewer modes spécialisés ✅
**Source :** BMAD (`bmad-review-adversarial-general`, `bmad-review-edge-case-hunter`)  
**Principe :** Deux modes de revue complémentaires à la revue standard, invocables explicitement.

#### I.1 — `reviewer-adversarial`
- [x] Créer `skills/reviewer/reviewer-adversarial.md`
  - Posture : scepticisme extrême, suppose que des problèmes existent
  - Minimum 10 findings obligatoires
  - HALT si zéro finding (re-analyser ou demander guidance)
  - Périmètre : architecture, choix techniques, sécurité, performance, maintenabilité

#### I.2 — `reviewer-edge-case`
- [x] Créer `skills/reviewer/reviewer-edge-case.md`
  - Exhaustive path analysis : control flow, boundaries, race conditions
  - Classes d'edge cases : null/empty inputs, off-by-one, arithmetic overflow, type coercion implicite, timeout gaps
  - Ne rapporter que les chemins non gérés (ignorer les gérés)
  - Validate completeness : revisit après step 2
  - Output format : findings avec chemin non géré + conséquence potentielle + suggestion

**Fichiers créés :** `skills/reviewer/reviewer-adversarial.md`, `skills/reviewer/reviewer-edge-case.md`

---

### M — `elicitation-techniques` skill ✅
**Source :** BMAD (`bmad-advanced-elicitation`, 50+ méthodes)  
**Principe :** Référence de techniques d'élicitation pour UX designer et planner quand ambiguité détectée.

- [x] Créer `skills/shared/elicitation-techniques.md`
  - Format : tableau technique → objectif → quand l'utiliser → exemple de prompt
  - 25 techniques couvrant : divergence, convergence, parties prenantes, risques, profondeur
  - Smart selection : tableau contextuel signal → technique(s) adaptée(s)
- [x] Ajouter référence dans `agents/design/ux-designer.md` (bucket-B conditionnel)
- [x] Ajouter référence dans `skills/planning/planner-workflow.md` (Phase 1 si ambiguité)

**Fichiers créés :** `skills/shared/elicitation-techniques.md`  
**Fichiers modifiés :** `agents/design/ux-designer.md`, `skills/planning/planner-workflow.md`

---

## Phase 2 — Extensions agents existants

### C — `verification-before-completion` gate
**Source :** Superpowers  
**Principe :** Pattern transversal — avant tout `DONE`/`TERMINÉ`, l'agent passe 3 checks explicites.

**3 checks obligatoires :**
1. Tests passent (ou justification documentée si pas de tests)
2. Comportement observable conforme à la spec
3. Aucune régression connue non documentée

- [x] Ajouter section "Gate de complétion" dans `skills/developer/dev-standards-universal.md`
- [x] Ajouter gate explicite avant handoff dans `skills/qa/qa-protocol.md`
- [x] Ajouter vérification avant clôture de tâche Beads dans `skills/orchestrator/orchestrator-protocol.md`

**Fichiers modifiés :** 3 fichiers skills existants

---

### D — Debugger forensique
**Source :** BMAD (`bmad-investigate`)  
**Principe :** Étendre le debugger avec un mode `--forensic` basé sur le grading d'évidence.

**Evidence grading :**
- **Confirmed** — directement observé, citer `path:line` ou commit hash
- **Deduced** — découle logiquement de preuves Confirmed, montrer la chaîne
- **Hypothesized** — plausible mais non confirmé, énoncer ce qui confirmerait ou réfuterait

**Principes clés :**
- Stronghold-first : ancrer sur une preuve Confirmed, ne jamais partir d'une théorie
- Challenge the premise : la description de l'utilisateur est une hypothèse, pas un fait
- Hypothèses jamais supprimées (update Status : Open / Confirmed / Refuted)
- Missing evidence = finding en soi
- Delegation discipline : >5 fichiers ou >10K tokens → déléguer à subagent (JSON structuré)

- [x] Étendre `skills/qa/debugger-workflow.md` avec section "Mode Forensique"
  - Case file `.investigation-{slug}.md` créé dès accord sur le slug
  - Template case file (hypothèses, évidence, timeline, conclusion)
  - Protocole de résumé à chaque reprise de session (open hypotheses, backlog, missing evidence)
- [x] Modifier `agents/quality/debugger.md` pour exposer le flag `--forensic`

**Fichiers modifiés :** `skills/qa/debugger-workflow.md`, `agents/quality/debugger.md`

---

### J — `dev-drift-detection` skill
**Source :** BMAD (`bmad-correct-course`)  
**Principe :** Détecter et gérer la dérive architecturale en cours d'implémentation.

**Déclencheurs :**
- Developer-subagent signale un blocage architectural (spec contradictoire avec réalité code)
- Orchestrateur détecte divergence entre tâche Beads et implémentation en cours
- Developer découvre que l'approche initiale est non viable

**3 options proposées à l'utilisateur :**
1. Réviser la tâche Beads (changer le scope)
2. Revenir à l'état précédent (git revert + nouvelle approche)
3. Bifurquer : créer une tâche de refactoring pré-requisite

- [x] Créer `skills/developer/dev-drift-detection.md`
  - Critères de dérive (liste de signaux)
  - Process de décision avec les 3 options
  - Template de rapport de dérive pour l'orchestrateur
- [x] Ajouter trigger de détection dans `skills/orchestrator/orchestrator-dev-protocol.md`
  - Condition : developer-subagent retourne un status `BLOCKED_ARCHITECTURE`
  - Action : appel explicite `skill("developer/dev-drift-detection")`
- [x] Documenter signal `BLOCKED_ARCHITECTURE` côté émetteur dans `skills/developer/dev-standards-universal.md`

**Fichiers créés :** `skills/developer/dev-drift-detection.md`  
**Fichiers modifiés :** `skills/orchestrator/orchestrator-dev-protocol.md`

---

### K — Scale-Domain-Adaptive Planning
**Source :** BMAD  
**Principe :** Ajouter une étape de complexity scoring au début du planner pour ajuster la profondeur de planification.

**Complexity scoring (4 critères, 1–4 pts chacun) :**

| Critère | 1 pt | 2 pts | 3 pts | 4 pts |
|---|---|---|---|---|
| Domaines techniques | 1 | 2 | 3 | 4+ |
| Intégrations tierces | 0 | 1 | 2–3 | 4+ |
| Sensibilité sécurité | Faible | Moyenne | Haute | Critique |
| Taille codebase estimée | <500 LOC | 500–5K | 5K–50K | 50K+ |

**Tiers et comportement :**
- **Small (4–6 pts)** → plan léger, pathfinder optionnel, 3–5 tâches Beads
- **Medium (7–10 pts)** → flow standard, pathfinder recommandé, 5–15 tâches
- **Large (11–13 pts)** → pathfinder obligatoire + audit pré-implem, tickets structurés
- **Enterprise (14–16 pts)** → toutes phases + onboarder pre-flight, architecture review

- [x] Modifier `skills/planning/planner-workflow.md` — ajouter Étape 0 "Complexity Scoring" avant Phase 1
  - Grille de scoring
  - Comportement conditionnel par tier
  - Intégration avec la décision pathfinder/planner direct dans `hub-workflow-reference` (Phase 3)

**Fichiers modifiés :** `skills/planning/planner-workflow.md`

---

## Phase 3 — Refactoring hub-workflow-reference (A)

**Source :** BMAD (`bmad-help`) — inspiration, implémentation hub native  
**ADR :** `docs/architecture/adr/018-hub-workflow-reference.fr.md`  
**Principe :** Extraire les descriptions de workflow dupliquées dans 4+ endroits vers un skill canonique `hub-workflow-reference`. Modèle : `orchestrator-workflow-modes.md` (pattern déjà établi dans le hub).

**Duplication actuelle identifiée :**
- `agents/planning/orchestrator.md` lignes ~40–91 — catalogue agents + heuristique routing
- `skills/orchestrator/orchestrator-protocol.md` — routing complet (version longue)
- `skills/planning/planner-workflow.md` lignes 42–110 — routing table orchestrateur
- `agents/planning/orchestrator-dev.md` — domain→agent mapping partiel

**Prérequis :** Phase 1 + Phase 2 terminées (tous les nouveaux agents/skills connus avant de figer le catalogue).

- [x] Créer `skills/shared/hub-workflow-reference.md`
  - Catalogue des agents (famille, rôle, mode primary/subagent, quand invoquer, output attendu)
  - Heuristique routing : pathfinder vs planner (critères de décision formalisés)
  - Tableau des handoffs (émetteur → format → récepteur)
  - Ordre d'enchaînement standard et variantes (solo / avec UX / avec audit)
  - Intégration du complexity scoring (K) pour conditionner le routing
- [x] Remplacer les sections dupliquées :
  - [x] `agents/planning/orchestrator.md` — remplacer catalogue+routing par pointeur vers `hub-workflow-reference`
  - [x] `skills/orchestrator/orchestrator-protocol.md` — remplacer section routing par pointeur
  - [x] `skills/planning/planner-workflow.md` lignes 42–110 — remplacer par pointeur
  - [x] `agents/planning/orchestrator-dev.md` — mapping domain→agent conservé (unique, non dupliqué)
- [x] ADR `docs/architecture/adr/018-hub-workflow-reference.fr.md` — statut Accepted
- [x] ADR `docs/architecture/adr/018-hub-workflow-reference.en.md` — statut Accepted

**Fichiers créés :** `skills/shared/hub-workflow-reference.md`, 2 ADRs  
**Fichiers modifiés :** 4 fichiers agents/skills existants

---

## Phase 4 — Méthodologie authoring skills (F)

**Source :** Superpowers (`writing-skills`)  
**Principe :** TDD appliqué à la rédaction de skills. Prérequis : G (SDO) terminé pour que le guide soit cohérent avec la pratique.

**TDD pour skills :**
- **RED** — rédiger un scénario de test en langage naturel (cas nominal + cas de rationalisation)
- **GREEN** — écrire le skill minimal qui passe le scénario
- **REFACTOR** — fermer les loopholes, ajouter la rationalization table, Red Flags list

- [ ] Créer `docs/guides/authoring-skills.md`
  - Guide complet : quand créer un skill, types (Technique / Pattern / Reference / Discipline)
  - TDD pour skills avec exemples
  - SDO checklist (description riche, keyword coverage, token efficiency, cross-references)
  - Anti-patterns (narrative, multi-language dilution, labels génériques, code dans flowcharts)
  - Rationalization table template
  - Checklist de validation finale
  - Règle : tout nouvel agent → MAJ obligatoire de `hub-workflow-reference`
- [ ] Créer `skills/shared/skill-authoring-protocol.md`
  - Version condensée invocable in-session par l'agent documentarian
  - Checklist TDD + SDO en format actionnable

**Fichiers créés :** `docs/guides/authoring-skills.md`, `skills/shared/skill-authoring-protocol.md`

---

## Récapitulatif fichiers

### Fichiers créés (11)

| Fichier | Phase | Item |
|---|---|---|
| `skills/reviewer/reviewer-reception.md` | 1 | B |
| `skills/reviewer/reviewer-adversarial.md` | 1 | I |
| `skills/reviewer/reviewer-edge-case.md` | 1 | I |
| `skills/shared/elicitation-techniques.md` | 1 | M |
| `skills/developer/dev-drift-detection.md` | 2 | J |
| `skills/shared/hub-workflow-reference.md` | 3 | A |
| `docs/architecture/adr/018-hub-workflow-reference.fr.md` | 3 | A |
| `docs/architecture/adr/018-hub-workflow-reference.en.md` | 3 | A |
| `docs/guides/authoring-skills.md` | 4 | F |
| `skills/shared/skill-authoring-protocol.md` | 4 | F |

### Fichiers modifiés (12)

| Fichier | Phase | Items |
|---|---|---|
| `config/stack-skills.json` | 1 | B |
| `agents/design/ux-designer.md` | 1 | M |
| `skills/planning/planner-workflow.md` | 1+2+3 | M, K, A |
| `skills/developer/dev-standards-universal.md` | 2 | C |
| `skills/qa/qa-protocol.md` | 2 | C |
| `skills/orchestrator/orchestrator-protocol.md` | 2+3 | C, A |
| `skills/qa/debugger-workflow.md` | 2 | D |
| `agents/quality/debugger.md` | 2 | D |
| `skills/orchestrator/orchestrator-dev-protocol.md` | 2 | J |
| `agents/planning/orchestrator.md` | 3 | A |
| `agents/planning/orchestrator-dev.md` | 3 | A |
| Tous les `skills/**/*.md` avec frontmatter | 1 | G |

---

## Séquençage et dépendances

```
Phase 1 ─────────────────────────────────── (parallélisable)
  G  SDO frontmatter          ──────────────────────────── aucune dépendance
  B  reviewer-reception       ──────────────────────────── aucune dépendance
  I  reviewer modes           ──────────────────────────── aucune dépendance
  M  elicitation-techniques   ──────────────────────────── aucune dépendance

Phase 2 ─────────────────────────────────── (certains parallélisables)
  C  verification gate        ─── après G (conventions SDO établies)
  D  forensic debugger        ──────────────────────────── aucune dépendance
  J  drift detection          ─── après D (pattern evidence établi, cohérence)
  K  adaptive planning        ──────────────────────────── aucune dépendance

Phase 3 ─────────────────────────────────── (séquentielle, après P1+P2)
  A  hub-workflow-reference   ─── après Phase 1 + 2 (catalogue complet avant de figer)

Phase 4 ─────────────────────────────────── (après G + A)
  F  authoring guide          ─── après G (SDO) + A (structure canonique établie)
```

---

## Abandons documentés

| Item | Raison |
|---|---|
| E — Sprint planning/retro | Hors scope : Beads gère les tickets, pas le cycle sprint |
| H — SPEC.md distillation | Redondant avec pathfinder-handoff dans le workflow orchestrateur |
| L — Party Mode | Complexité d'implémentation élevée, valeur incertaine |
| N — PRD / PRFAQ workflows | Hors scope : les équipes ont leurs process PM existants |
| O — Multi-plateformes | Hub est OpenCode-only par design |
