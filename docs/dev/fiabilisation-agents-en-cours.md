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
| ✅ **M-1 / m-4** | Conflit `expert-posture` + `concision-posture` sans règle de priorité — résolu par déclaration explicite de dépendance dans les deux skills : `expert-posture` déclare sa priorité sur `concision-posture` ; `concision-posture` liste exhaustivement les formats `expert-posture` non suppressibles. | `skills/posture/expert-posture.md`, `skills/posture/concision-posture.md` | voir commit suivant |
| ✅ **M-2** | `shared/wiki-navigation` absent de `developer-migrator` et `developer-refactor` — ajouté dans les `skills:` des deux agents. | `agents/developer/developer-migrator.md`, `agents/developer/developer-refactor.md` | voir commit suivant |
| ✅ **M-7** | Skills `designer/ux-subagent` et `designer/ui-subagent` inexistants — créés, calqués sur `pathfinder-subagent` (session unique sans interruption de phase, sauf clarification critique). Agents `ux-designer` et `ui-designer` mis à jour avec le pattern double-rôle (`[SKILL:designer/ux-subagent]`). `design-handoff-format.md` migré de la détection `[CONTEXTE]` vers `[SKILL:]`. | `skills/designer/ux-subagent.md`, `skills/designer/ui-subagent.md`, `agents/design/ux-designer.md`, `agents/design/ui-designer.md`, `skills/design/design-handoff-format.md` | voir commit suivant |
| ✅ **M-3** | `onboarder` : `read` non déclaré — vérifié sur le code source OpenCode (v1.17.5). Les defaults système injectent `read: { "*": "allow" }` pour tous les agents customs avant fusion avec le frontmatter. Permission `read` est `allow` par défaut. Aucune modification nécessaire. | — | — |
| ✅ **M-5** | `planner` : références à `posture/retranscription-coordinateur` dans le body (L.104, L.168) — skill non chargé par le planner (il est producteur des blocs, pas consommateur). Références remplacées par du texte inline autonome. | `agents/planning/planner.md` | voir commit suivant |
| ✅ **M-6** | `documentarian` : `beads-dev` référence `living-docs-enrichment` — la référence s'applique au developer (qui charge ce skill), pas au documentarian. Formulation clarifiée dans `beads-dev.md` pour lever l'ambiguïté. | `skills/developer/beads-dev.md` | voir commit suivant |
| ✅ **M-4** | `reviewer` : standards dev prescriptifs sans adaptation au contexte review — préambule ajouté dans le body : usage des standards pour référence uniquement, signalement des violations, correction déléguée au developer. | `agents/quality/reviewer.md` | voir commit suivant |

---

## Points restants — 🔴 Critique (0)

*Tous les points critiques ont été résolus.*

---

## Points restants — 🟠 Majeur (1)

### ✅ M-1 — `expert-posture` + `concision-posture` sans règle de priorité *(résolu)*

**Résolution :** Déclaration explicite de dépendance dans les deux skills. `expert-posture` déclare sa priorité ; `concision-posture` liste exhaustivement les formats `expert-posture` non suppressibles (bloc `⚠️ Recommandation contraire`, trade-offs, formulation à la première personne, zones d'incertitude, pauses de confirmation).

---

### ✅ M-2 — `shared/wiki-navigation` absent de `developer-migrator` et `developer-refactor` *(résolu)*

**Résolution :** `shared/wiki-navigation` ajouté dans les `skills:` des deux agents.

---

### ✅ M-3 — `onboarder` : `read` non déclaré *(résolu — aucune modification nécessaire)*

**Vérification :** Les defaults système OpenCode (v1.17.5) injectent `read: { "*": "allow" }` pour tous les agents customs avant toute fusion avec le frontmatter. La permission `read` est `allow` par défaut — le mode enrichissement incrémental de la Phase 5 fonctionne sans déclaration explicite.

---

### ✅ M-4 — `reviewer` : standards dev prescriptifs sans adaptation au contexte review *(résolu)*

**Résolution :** Préambule "Usage des standards de développement" ajouté dans le body du reviewer : les standards sont chargés pour référence uniquement, les violations sont signalées (pas corrigées), la correction reste le rôle du developer.

---

### ✅ M-5 — `planner` : référence à `retranscription-coordinateur` *(résolu)*

**Résolution :** Le planner est producteur des blocs structurés, pas consommateur — `retranscription-coordinateur` ne s'applique pas à lui. Les deux références (L.104, L.168) remplacées par du texte inline autonome décrivant la règle directement.

---

### ✅ M-6 — `documentarian` : `beads-dev` référence `living-docs-enrichment` *(résolu)*

**Résolution :** La référence dans `beads-dev.md` s'applique au developer (qui charge ce skill), pas au documentarian. Formulation clarifiée : "Le skill `shared/living-docs-enrichment` est chargé par le developer — appliquer ses règles ici." Aucune modification du documentarian.

---

### ✅ M-7 — Skills `ux-subagent` et `ui-subagent` inexistants *(résolu)*

**Résolution :** Création de `skills/designer/ux-subagent.md` et `skills/designer/ui-subagent.md`, calqués sur `pathfinder-subagent`. Agents mis à jour avec le pattern double-rôle. `design-handoff-format.md` migré vers la détection par `[SKILL:]`.

---

### M-8 — `orchestrator` avec `skill: allow` : risque de chargement de skills non pertinents *(surveillance)*

**Agent concerné :** `agents/planning/orchestrator.md`

**Contexte :** Fix C-1 a passé l'orchestrator de `skill: deny` à `skill: allow`. L'orchestrator voit désormais **tous** les skills disponibles dans son system prompt.

**Risque modéré — à monitorer en utilisation réelle.** Résolution si observé en pratique : ajouter une règle dans le body de l'orchestrator : "Tu ne charges que les skills de posture et de format — jamais les skills techniques (dev-standards-*, audit-*, qa-*)."

---

### ✅ M-9 — `skills/quality/debugger-subagent.md` inexistant *(résolu — fix C-3)*

**Résolution :** Création de `skills/quality/debugger-subagent.md` — parcours sous-agent calqué sur `planner-subagent`, chargé conditionnellement quand l'orchestrateur injecte `[SKILL:quality/debugger-subagent]`.

---

## Points restants — 🟡 Mineur (12)

| ID | Problème | Fichier(s) concerné(s) |
|----|---------|----------------------|
| m-1 | Terminologie `orchestrateur` vs `orchestrator` (avec/sans accent) dans les templates de blocs structurés — double nomenclature | Multiples skills et agents |
| m-2 | `concision-posture` : liste d'agents éligibles figée dans le texte — pas auto-extensible à de nouveaux agents | `skills/posture/concision-posture.md` |
| m-3 | ✅ `subagent-concision-posture` : `debugger` retiré de la portée (résolu conjointement avec C-3 + ADR-017 — les 7 `auditor-*` supprimés, remplacés par `auditor-subagent`) | `skills/posture/subagent-concision-posture.md` |
| m-4 | ✅ `planner` : `expert-posture` + `concision-posture` sans priorité — résolu conjointement avec M-1 | `skills/posture/expert-posture.md`, `skills/posture/concision-posture.md` |
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

*(lot terminé)*

### Lot 2 — Fichiers manquants (1-2h)

*(lot terminé)*

### Lot 4 — Vérifications système (30 min)

*(lot terminé)*

### Lot 5 — Nettoyage documentaire (1h)

5. **m-1** : normaliser terminologie `orchestrateur` / `orchestrator`
6. Remaining mineurs

---

## Notes techniques — Apprentissages OpenCode

> Ces découvertes ont été faites en lisant le code source d'OpenCode pendant cette session.

- **`skills:` dans le frontmatter agent n'est PAS un champ natif OpenCode.** `ConfigAgentV1.Info` ne le reconnaît pas. C'est une convention documentaire du hub — les skills ne sont pas injectés automatiquement.
- **`skill: deny` bloque tout** : supprime la section "Available Skills" du system prompt (`session/system.ts` ligne `if (Permission.disabled(["skill"]...)`), et bloque l'outil `skill`. Un agent avec `skill: deny` tourne sans aucun skill.
- **`native_skills:` n'est pas non plus un champ natif OpenCode.** Convention documentaire du hub.
- **`skill: allow` est requis** pour que les agents voient la liste des skills disponibles et puissent les charger via l'outil `skill`.
- **Le wildcard dans `task: { "auditor-*": allow }` : résolu par ADR-017** — remplacé par `"auditor-subagent": allow` (permission explicite).
- **Permission non déclarée dans un agent custom = `allow` par défaut** (via la règle catch-all `"*": "allow"` dans les defaults système de `agent.ts`). Exception : `question`, `plan_enter`, `plan_exit` → `deny` par défaut. Fichiers `.env` → `ask`. Le fallback ultime (aucune règle ne matche) est `ask`, jamais `deny`.
