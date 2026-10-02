---
name: planner-phase-3-4
description: Phase 3 (plan hiérarchique — epics, tickets, ordre, risques) et Phase 4 (détection des cas particuliers) du workflow planner.
---

# Phase 3 — Analyse approfondie : Plan hiérarchique

## Objectif
Décomposer la feature en epics et tickets structurés, avec ordre d'implémentation et risques identifiés.

## Format de présentation

```markdown
## [Phase 3] Plan hiérarchique — <nom de la feature>

### Contexte métier
[1-2 phrases : pourquoi cette feature, quelle valeur pour l'utilisateur]

### Epic 1 — [Nom de l'epic]
*Objectif : [phrase courte décrivant la valeur de cet epic]*

  #### Story 1.1 — [Nom de la story] *(optionnel — omettre si granularité inutile)*

  - [ ] Ticket 1.1.1 (P1, feature, ~[Xh]) — [Titre du ticket]
    → [Description courte en 1 phrase : état actuel → état cible]
    → Contexte métier : [pourquoi ce ticket existe]
    → Couches touchées : [use case / DTO / API / composant / store / etc.]
    → Tests attendus : [type de test + frontière de test (si identifiable) + cas à couvrir]
    → Acceptance : [critère 1] / [critère 2] / [critère 3]
    → Dépend de : —

  - [ ] Ticket 1.1.2 (P2, task, ~[Xh]) — [Titre du ticket]
    → [Description courte]
    → Couches touchées : [...]
    → Tests attendus : [type de test + frontière de test (si identifiable) + cas à couvrir]
    → Acceptance : [critère]
    → Dépend de : Ticket 1.1.1

### Epic 2 — [Nom de l'epic]
  ...

---

### Ordre d'implémentation suggéré
1. [Ticket X] — bloquant (tous les autres en dépendent)
2. [Ticket Y], [Ticket Z] — parallélisables
3. [Ticket W] — après Y et Z
...

### Risques identifiés
- [Risque 1 — impact potentiel + mitigation suggérée]
- [Risque 2 — impact potentiel + mitigation suggérée]

### Résumé
Epics : N | Tickets : M | Estimation totale : ~Xh
Epics dans Beads : [oui / non / à confirmer]
```

## Règle — Epics dans Beads

- **> 5 tickets** → les epics sont créés dans Beads avec `bd create -t epic`. Annoncer :
  > "La feature comporte N tickets. Je vais créer les epics dans Beads pour structurer la hiérarchie."

- **≤ 5 tickets** → demander explicitement :
  > "La feature est courte (N tickets). Voulez-vous quand même créer les epics dans Beads pour la hiérarchie, ou préférez-vous rester à plat ?"

## Règle — Granularité des tickets

**Un ticket unique est toujours acceptable** si la demande est clairement délimitée (bug isolé, ajout UI simple, tâche technique ciblée, etc.). Ne pas découper par défaut.

Un découpage peut être **suggéré** (jamais imposé) si **plusieurs** de ces critères sont vrais simultanément :
- Plus de 3 critères d'acceptance complexes
- Estimation > 1 jour de travail
- Implique des modifications dans > 3 couches (ex : BDD + service + API + frontend + tests)

Un seul critère ne suffit pas à proposer un découpage. Si un découpage semble pertinent, le **signaler comme option** à l'utilisateur sans l'inclure dans le plan par défaut. L'utilisateur décide toujours.

## Règle — Tracer bullet (premier ticket)

Le premier ticket de chaque epic devrait être un **tracer bullet** : une tranche verticale mince qui traverse toutes les couches concernées de bout en bout, même si chaque couche est minimale.

**Objectif :** prouver que le chemin d'intégration fonctionne avant d'élargir chaque couche individuellement. Les tickets suivants enrichissent et étendent — ils ne découvrent pas les problèmes d'intégration.

**Signaux d'un bon tracer bullet :**
- Il touche toutes les couches listées dans l'epic (DB + service + API + UI si applicable)
- Il a un comportement observable testable de bout en bout
- Il est le plus petit possible tout en traversant toutes les couches
- Les tickets suivants élargissent chaque couche sans toucher l'intégration

**Signaux d'un mauvais découpage (horizontal slicing) :**
- Ticket 1 = DB uniquement, Ticket 2 = API uniquement, Ticket 3 = UI uniquement
- Le premier ticket n'a pas de valeur testable isolément
- L'intégration n'est prouvée qu'au dernier ticket

**Exceptions acceptées :**
- Un ticket technique fondation (migration DB, setup CI, config infra) peut précéder le tracer bullet si l'intégration n'est pas possible sans ce prérequis
- Un epic mono-couche (ex : refactoring pur backend) n'a pas besoin de tracer bullet — le premier ticket est le cas nominal de cette couche
- L'utilisateur peut préférer un découpage horizontal — la règle est une recommandation, pas une obligation

## Récap de fin de Phase 3

(Le récap est le plan lui-même tel que présenté ci-dessus)

> ⚠️ **RAPPEL** : En Phase 6, le récapitulatif de planification doit reprendre tous ces éléments (plan hiérarchique + dépendances + hypothèses + risques) sous forme narrative détaillée — ne pas se limiter au tableau structuré du bloc handoff.

## Question de validation obligatoire

**Si CONTEXTE = standalone :**
```
question({
  questions: [{
    header: "Validation du plan",
    question: "[Planner — Phase 3 complétée | Feature : <nom>]\nEst-ce que ce découpage vous convient ? Souhaitez-vous modifier, ajouter ou supprimer des éléments avant que je crée les tickets ?",
    options: [
      { label: "Valider le plan (Recommandé)", description: "Passer à la détection des cas particuliers (Phase 4)" },
      { label: "Modifier le plan", description: "Apporter des modifications au découpage" },
      { label: "Revenir à Phase 2", description: "Reposer des questions avant de finaliser le plan" }
    ]
  }]
})
```

**Selon la réponse (dans tous les contextes) :**
- **Valider** → Phase 4
- **Modifier** → rester en Phase 3, intégrer les modifications, re-présenter le plan
- **Revenir à Phase 2** → Phase 2 (le plan révèle de nouvelles questions)

**Ne pas continuer tant que le plan n'est pas validé.**

---

# Phase 4 — Détection des cas particuliers

## Objectif
Vérifier les cas limites qui pourraient avoir été manqués lors de la décomposition.

## Ce qu'on vérifie

**Checklist des cas particuliers :**

- ✅ **Tickets trop gros** : Y a-t-il des tickets à scinder en 2-3 sous-tickets ?
- ✅ **Doublons avec tickets existants** : Y a-t-il des tickets qui font doublon avec des tickets déjà ouverts ?
- ✅ **Dépendances circulaires** : Y a-t-il des dépendances qui forment un cycle ?
- ✅ **Logiques existantes réutilisables** : Y a-t-il un risque de dupliquer du code existant ?
- ✅ **Impacts indirects** : Y a-t-il des impacts sur d'autres parties du projet non couverts par le plan ?
- ✅ **Configurations spécifiques** : Y a-t-il des configurations (env, feature flags) qui changent le comportement ?
- ✅ **Comportements de librairies vérifiés** : Les suppositions de Phase 1.2bis ont-elles toutes été validées (✅) ou documentées comme hypothèses (`needs-clarification`) ? Y a-t-il des risques liés à des comportements non documentés ou version-dépendants ?
- ✅ **Impact en cascade complet** : Tous les consommateurs des fichiers partagés modifiés (Phase 1.2ter) ont-ils un ticket prévu ou une justification explicite d'exclusion ? Aucun consommateur classé "ticket séparé nécessaire" ne doit rester sans traitement dans le plan.
- ✅ **Concepts déjà rejetés** : Consulter `docs/wiki/decisions-rejetees.md` (si le fichier existe) et vérifier qu'aucun ticket du plan ne re-propose un concept avec statut `ACTIVE`. Pour chaque match détecté, afficher le concept rejeté avec sa raison et demander confirmation : "Ce concept a été rejeté le [date] parce que [raison]. Le contexte a-t-il changé ?" Si oui, l'entrée peut être mise à jour ou supprimée. Si le fichier n'existe pas, skip silencieusement.

## Déclencheur de pause ⏸️

Si un **cas particulier critique** est détecté (ex : doublon avéré, dépendance circulaire) :
- Afficher le contexte en texte (description du cas, impact, options)
- Puis utiliser l'outil `question` pour demander comment le traiter

## Récap de fin de Phase 4

```markdown
## [Phase 4] Détection des cas particuliers terminée

**Cas particuliers vérifiés :** X vérifications

**Cas particuliers détectés :**
- <cas 1 — description + impact + action recommandée>
- <cas 2 — description + impact + action recommandée>

**Cas particuliers écartés :**
- <cas 1 — raison de l'écarter>

**Impact sur le plan :**
- <ajustement 1 — ex : ticket bd-42 scindé en bd-42a et bd-42b>
- <ajustement 2 — ex : ajout d'une dépendance bd-X → bd-Y>
- (aucun ajustement si tous les cas écartés)
```

## Question de validation obligatoire

**Si CONTEXTE = standalone :**
```
question({
  questions: [{
    header: "Création des tickets",
    question: "[Planner — Phase 4 complétée | Feature : <nom>]\nDétection des cas particuliers terminée. Passer à la création des tickets dans Beads (Phase 5) ?",
    options: [
      { label: "Créer les tickets (Recommandé)", description: "Passer à la Phase 5 — Création dans Beads" },
      { label: "Vérifier d'autres cas", description: "Rester en Phase 4 pour vérifier d'autres cas particuliers" },
      { label: "Revenir à Phase 3", description: "Revoir le plan après détection de cas particuliers critiques" }
    ]
  }]
})
```

**Selon la réponse (dans tous les contextes) :**
- **Créer les tickets** → Phase 5
- **Vérifier d'autres cas** → rester en Phase 4, vérifier d'autres cas, re-produire le récap
- **Revenir à Phase 3** → Phase 3 (les cas particuliers nécessitent une refonte du plan)
