# Fiabilisation agents & skills — Suivi d'avancement

## Contexte

Analyse exhaustive de l'ensemble des agents (22) et skills (~120) du hub.
26 zones d'ambiguïté détectées, classées par criticité.

**Date d'analyse :** 2026-06-18
**Périmètre :** tous les agents sous `agents/`, toutes les skills sous `skills/`

---

## Légende

| Statut | Signification |
|--------|--------------|
| ✅ Corrigé | Fix commité |
| 🔴 Critique | À traiter en priorité — impact fonctionnel avéré |
| 🟠 Majeur | Décalage de portée, skill ou fichier manquant |
| 🟡 Mineur | Ambiguïté documentaire, terminologie, redondance |

---

## Corrections effectuées

### Session 2026-06-18

| ID | Problème | Fichiers modifiés | Commit |
|----|---------|-------------------|--------|
| ✅ **C-1** | `orchestrator` : `skill: deny` bloquait l'intégralité des skills — confirmé par le code source OpenCode (`session/system.ts` + `skill/index.ts`). L'orchestrator tournait sans aucune posture ni format handoff. | `agents/planning/orchestrator.md` | `00ef56c` |
| ✅ **C-4** | `coordination-only.md` : tableau récap `bash ❌` sans nuance → contradiction avec les exceptions worktree déclarées dans `orchestrator-dev`. | `skills/posture/coordination-only.md` | `00ef56c` |
| ✅ **subagent-concision** | Création du skill `subagent-concision-posture` (niveau compact, machine-to-machine). Câblage sur les 11 agents `mode: subagent`. Allègement de `concision-posture` (portée resserrée aux agents primaires). ADR-015 mis à jour (fr + en). | `skills/posture/subagent-concision-posture.md`, `skills/posture/concision-posture.md`, 11 agents, `config/hub.json`, ADR-015 | `c5f61a4` |
| ✅ **C-2** | Conflit `expert-posture` vs `subagent-concision-posture` sur les auditors — résolu par ADR-017 : consolidation des 7 `auditor-*` en un seul `auditor-subagent`. L'agent unique reçoit les deux skills sans conflit structurel car le domaine est injecté dynamiquement ; les risques critiques remontent via le champ `risques` du bloc handoff. | `agents/auditor/auditor-subagent.md`, docs architecture/skills, docs guides/workflows | ADR-017 |
| ✅ **C-5** | Wildcard `"auditor-*": allow` dans les permissions `task` de l'`auditor` — résolu par ADR-017 : remplacement par `"auditor-subagent": allow` (permission explicite, sans wildcard). | `agents/auditor/auditor.md`, `docs/architecture/task-delegation.fr.md` | ADR-017 |
| ✅ **m-3** | `subagent-concision-posture` listait `debugger` dans sa portée (hybride standalone/subagent) — résolu conjointement avec C-3 (debugger → `mode: primary`) et ADR-017 (suppression des 7 `auditor-*` de la portée, remplacés par `auditor-subagent`). | `skills/posture/subagent-concision-posture.md` | ADR-017 |
| ✅ **C-3** | `debugger` : `mode: subagent` → `mode: primary`. Retrait de `subagent-concision-posture` des skills. Section "Contexte d'invocation" remplacée par le pattern double-rôle (condition sur `[SKILL:quality/debugger-subagent]`). `debugger-workflow.md` mis à jour (détection signal). | `agents/quality/debugger.md`, `skills/quality/debugger-workflow.md` | voir M-9 |
| ✅ **M-9** | Création de `skills/quality/debugger-subagent.md` — parcours sous-agent calqué sur `planner-subagent` (mécanisme d'interruption de session, blocs `## Retour intermédiaire` + `## Question pour l'orchestrateur` par phase). Retrait de `debugger` de la portée de `subagent-concision-posture`. Documentation mise à jour (ADR-015 FR+EN, skills.fr.md, skills.en.md). | `skills/quality/debugger-subagent.md`, `skills/posture/subagent-concision-posture.md`, `docs/architecture/adr/015-concision-posture.fr.md`, `docs/architecture/adr/015-concision-posture.en.md`, `docs/architecture/skills.fr.md`, `docs/architecture/skills.en.md` | — |

---

## Points restants — 🔴 Critique (0)

*Tous les points critiques ont été résolus.*

---

## Points restants — 🟠 Majeur (9)

### M-1 — `expert-posture` + `concision-posture` sans règle de priorité

**Agents concernés :** `qa-engineer`, `planner`

**Problème :** Les deux skills coexistent sans hiérarchie. `expert-posture` prescrit des recommandations argumentées en prose ; `concision-posture` prescrit d'éliminer les phrases sans valeur immédiate. Un agent peut légitimement interpréter une recommandation argumentée comme du "filler" à supprimer.

**Résolution :** Ajouter dans `concision-posture` une note de priorité explicite : "Ce skill ne supprime jamais les justifications de décision, avertissements ou recommandations prescrits par `expert-posture`."

---

### M-2 — `shared/wiki-navigation` absent de `developer-migrator` et `developer-refactor`

**Agents concernés :** `agents/developer/developer-migrator.md`, `agents/developer/developer-refactor.md`

**Problème :** `developer.md` charge `wiki-navigation` pour lire les conventions avant d'implémenter. Les deux variantes spécialisées ne l'ont pas — agents aveugles aux conventions wiki alors qu'une migration ou un refactoring sans connaître les conventions est plus risqué que l'implémentation de base.

**Résolution :** Ajouter `shared/wiki-navigation` dans les `skills:` des deux agents.

---

### M-3 — `onboarder` : `read` non déclaré mais workflow Phase 5 nécessite de lire les pages wiki

**Agent concerné :** `agents/planning/onboarder.md`

**Problème :** La Phase 5 du workflow onboarder dit "Si `docs/wiki/index.md` existe déjà → lire la page existante (Read)". La permission `read` n'est pas déclarée dans le frontmatter. Si `deny` est la valeur par défaut pour les outils non déclarés, le mode enrichissement incrémental est techniquement bloqué.

**À vérifier :** comportement OpenCode pour les outils non déclarés (`allow` ou `deny` par défaut ?).

**Résolution probable :** Ajouter `read: allow` (et `glob: allow`, `grep: allow`) dans les permissions de l'onboarder.

---

### M-4 — `reviewer` : 7 `native_skills` standards dev sans adaptation au contexte review

**Agent concerné :** `agents/quality/reviewer.md`

**Problème :** Les standards `dev-standards-backend`, `dev-standards-frontend`, etc. sont écrits en mode prescriptif ("tu dois faire X"). Le reviewer les charge pour juger, pas pour appliquer. Aucun de ces skills ne contient de section "en review, tu signales seulement". Risque de confusion entre signaler (rôle reviewer) et corriger (rôle developer).

**Résolution :** Ajouter un préambule dans le body du reviewer : "Tu charges ces standards pour référence uniquement. Tu ne les appliques jamais — tu signales les violations, tu ne les corriges pas."

---

### M-5 — `planner` : référence à `retranscription-coordinateur` dans le body mais skill absent

**Agent concerné :** `agents/planning/planner.md` (ligne ~104)

**Problème :** Le body du planner dit "règle absolue définie dans le skill `posture/retranscription-coordinateur`" — mais ce skill n'est pas dans ses `skills:`. Référence cassée vers un skill non chargé.

**Résolution :** Soit supprimer la référence et inliner la règle dans le body, soit clarifier que le planner est "producteur" du format et n'a pas besoin de charger le skill de retransmission (qui s'applique aux coordinateurs consommateurs).

---

### M-6 — `documentarian` : `beads-dev` référence `living-docs-enrichment` mais skill absent

**Agent concerné :** `agents/documentation/documentarian.md`

**Problème :** `beads-dev.md` (chargé par le documentarian) contient en fin de workflow "Appliquer le skill `shared/living-docs-enrichment`". Ce skill n'est pas dans les `skills:` du documentarian. Référence vers un skill non disponible dans ce contexte.

**Résolution :** Ajouter `shared/living-docs-enrichment` dans les skills du documentarian, ou clarifier que la référence dans `beads-dev` s'applique au contexte de l'invocateur (le developer), pas du documentarian.

---

### M-7 — Skills `ux-subagent` et `ui-subagent` inexistants

**Agents concernés :** `ux-designer`, `ui-designer` (via `orchestrator-protocol`)

**Problème :** `orchestrator-protocol.md` injecte `[SKILL:designer/ux-subagent]` et `[SKILL:designer/ui-subagent]` dans les invocations des designers. Ces fichiers n'existent pas.

**Résolution :** Créer `skills/designer/ux-subagent.md` et `skills/designer/ui-subagent.md` avec les règles de comportement en mode sous-agent (format handoff, pas de question directe à l'utilisateur, etc.).

---

### M-8 — `orchestrator` avec `skill: allow` : risque de chargement de skills non pertinents

**Agent concerné :** `agents/planning/orchestrator.md`

**Contexte :** Fix C-1 a passé l'orchestrator de `skill: deny` à `skill: allow`. L'orchestrator voit désormais **tous** les skills disponibles dans son system prompt (via `Skill.fmt`).

**Risque modéré :** L'orchestrator pourrait charger des skills techniques (ex: `developer/dev-standards-backend`) par confusion. À monitorer en utilisation réelle.

**Résolution si nécessaire :** Ajouter une règle dans le body de l'orchestrator : "Tu ne charges que les skills de posture et de format — jamais les skills techniques (dev-standards-*, audit-*, qa-*)."

---

### M-9 — ✅ `skills/quality/debugger-subagent.md` inexistant *(résolu — fix C-3)*

**Résolution :** Création de `skills/quality/debugger-subagent.md` — parcours sous-agent calqué sur `planner-subagent`, chargé conditionnellement quand l'orchestrateur injecte `[SKILL:quality/debugger-subagent]`.

---

## Points restants — 🟡 Mineur (12)

| ID | Problème | Fichier(s) concerné(s) |
|----|---------|----------------------|
| m-1 | Terminologie `orchestrateur` vs `orchestrator` (avec/sans accent) dans les templates de blocs structurés — double nomenclature | Multiples skills et agents |
| m-2 | `concision-posture` : liste d'agents éligibles figée dans le texte — pas auto-extensible à de nouveaux agents | `skills/posture/concision-posture.md` |
| m-3 | ✅ `subagent-concision-posture` : `debugger` retiré de la portée (résolu conjointement avec C-3 + ADR-017 — les 7 `auditor-*` supprimés, remplacés par `auditor-subagent`) | `skills/posture/subagent-concision-posture.md` |
| m-4 | `planner` : `expert-posture` + `concision-posture` sans priorité (identique à M-1) | `agents/planning/planner.md` |
| m-5 | `onboarder` absent de la portée de `concision-posture` sans justification documentée, alors que `planner` et `pathfinder` y sont | `skills/posture/concision-posture.md` |
| m-6 | Chaîne `living-docs-enrichment` → documentarian → wiki-navigation → index : complexité de dépendances non documentée | `skills/shared/living-docs-enrichment.md` |
| m-7 | `qa-engineer` : `edit: deny` + `write: allow` → pour ajouter un test dans un fichier existant, le QA doit réécrire le fichier entier via `write` | `agents/quality/qa-engineer.md` |
| m-8 | `developer` : `living-docs-enrichment` référencé dans ses `skills:` ET dans `beads-dev` — doublon de référence | `agents/developer/developer.md` |
| m-9 | `auditor-workflow` contient une note "ne pas dupliquer les règles de parcours" puis les duplique dans le même skill | `skills/auditor/auditor-workflow.md` |
| m-10 | `orchestrator-dev` : pas d'`expert-posture` → règle d'interdiction `git push` non couverte par une posture générique | `agents/planning/orchestrator-dev.md` |
| m-11 | `pathfinder` : permission `ask` (confirmation système) + outil `question` — double mécanisme de confirmation, relation non définie | `agents/planning/pathfinder.md` |
| m-12 | `concision-posture` ne documente pas pourquoi `auditor` (mode: primary, coordinateur) est exclu de sa portée | `skills/posture/concision-posture.md` |

---

## Priorités recommandées (prochaines sessions)

### Lot 1 — Rapide, fort impact (< 30 min)

1. **M-2** : ajouter `shared/wiki-navigation` à `developer-migrator` et `developer-refactor` (2 lignes)

### Lot 2 — Fichiers manquants (1-2h)

2. **M-7** : créer `skills/designer/ux-subagent.md` et `skills/designer/ui-subagent.md`

### Lot 3 — Conflits de posture (1-2h)

3. **M-1 / m-4** : note de priorité `expert-posture` vs `concision-posture`

### Lot 4 — Vérifications système (30 min)

4. **M-3** : vérifier comportement `read` non déclaré dans OpenCode (allow ou deny par défaut ?)

### Lot 5 — Nettoyage documentaire (1h)

5. **M-5** : planner / référence `retranscription-coordinateur`
6. **M-6** : documentarian / `living-docs-enrichment`
7. **m-1** : normaliser terminologie `orchestrateur` / `orchestrator`
8. Remaining mineurs

---

## Notes techniques — Apprentissages OpenCode

> Ces découvertes ont été faites en lisant le code source d'OpenCode pendant cette session.

- **`skills:` dans le frontmatter agent n'est PAS un champ natif OpenCode.** `ConfigAgentV1.Info` ne le reconnaît pas. C'est une convention documentaire du hub — les skills ne sont pas injectés automatiquement.
- **`skill: deny` bloque tout** : supprime la section "Available Skills" du system prompt (`session/system.ts` ligne `if (Permission.disabled(["skill"]...)`), et bloque l'outil `skill`. Un agent avec `skill: deny` tourne sans aucun skill.
- **`native_skills:` n'est pas non plus un champ natif OpenCode.** Convention documentaire du hub.
- **`skill: allow` est requis** pour que les agents voient la liste des skills disponibles et puissent les charger via l'outil `skill`.
- **Le wildcard dans `task: { "auditor-*": allow }` : résolu par ADR-017** — remplacé par `"auditor-subagent": allow` (permission explicite).
