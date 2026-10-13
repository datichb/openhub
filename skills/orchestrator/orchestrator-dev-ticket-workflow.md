---
name: orchestrator-dev-ticket-workflow
description: "Utiliser pour chaque ticket : étapes 1a à 6 (présentation, branche, délégation, pre-review, review, décision, compte rendu)."
---

## Workflow ticket par ticket

> **Un seul protocole de checkpoint** : chaque checkpoint listé dans la table des checkpoints de la carte du workflow passe **uniquement** par l'outil `workflow_checkpoint` (`id` + `summary`), appelé par toi-même — y compris quand tu es invoqué via `task` par l'orchestrator. C'est oh qui applique le mode (`pause` : l'appel attend la validation ; `auto` : il passe aussitôt ; `skip` : ne l'appelle pas). Validation reçue = suite prévue (validation du checkpoint qui déverrouille `commit` et `close` = commit puis fermeture) ; refus = suivre la consigne reçue, puis rappeler `workflow_checkpoint`. oh ferme sans effet une question qui nomme un checkpoint, et refuse `git commit` et la fermeture du ticket tant que ce checkpoint n'est pas validé. L'outil `question` ne sert qu'aux confirmations hors checkpoint (branche, CHANGELOG), sans nommer de checkpoint.

### Étape 1a — Présentation du ticket + `cp-1`

Afficher le ticket :

```
## Ticket #<ID> — <titre>

**Priorité :** P<PRIORITE> | **Type :** <type> | **Agent :** developer (<domaine>)

**Description :**
<description du ticket>

**Critères d'acceptance :**
<liste des critères>

**Notes :**
<notes et contraintes>

---
```

**Checkpoint `cp-1`** (démarrer le ticket) :

```
workflow_checkpoint({ id: "cp-1", summary: "Ticket #<ID> — <titre> : <résumé en 2 lignes>. Agent prévu : developer (<domaine>)." })
```

- Validé (ou passé automatiquement selon le mode) → mettre à jour todowrite (ticket en `in_progress`) → étape 1b
- Refusé avec une consigne (« voir le détail », « passer », « stop ») → l'appliquer : afficher `bd show <ID>` puis rappeler `cp-1` ; passer = ticket `cancelled`, ticket suivant ; stop = **Récap global — Fin de session**

**Mise à jour todowrite au démarrage du ticket — agent d'entrée uniquement (exemple : premier ticket démarre) :**

> Invoqué via `task` par l'orchestrator, cette mise à jour reste locale à la session isolée et n'est pas visible par l'utilisateur. L'orchestrator feature gère sa propre liste.

```
todowrite({
  todos: [
    { content: "#bd-12 — <titre court> [dev]", status: "in_progress", priority: "high" },  // ← label [dev] ajouté
    { content: "#bd-13 — <titre court>", status: "pending", priority: "high" },
    { content: "#bd-14 — <titre court>", status: "pending", priority: "medium" }
  ]
})
```

> Rappel : exactement une tâche `in_progress` à la fois (règle du skill `skills/posture/tool-todowrite.md`).

---

### Étape 1b — Branche dédiée ⏸️ PAUSE

> Si le premier message contient une ligne `Branche de travail : <branche>`, la session a déjà sa branche (worktree ou branche dédiée choisie au lancement) : **sauter cette étape**, sans question ni création de branche, et indiquer cette branche à l'agent développeur.
>
> Sinon, cette étape s'exécute après `cp-1`, avant toute délégation à un `developer`, quel que soit le mode. Invoqué via `task` par l'orchestrator (sans ligne `Branche de travail`) : rester sur la branche courante et le signaler dans le bloc `## Retour vers orchestrator`.

Calculer le nom de branche selon la convention `<type>/<ticket-id>-<description-courte>` à partir du type et du titre du ticket, puis demander (confirmation hors checkpoint) :

```
question({
  questions: [{
    header: "Branche — Ticket #<ID>",
    question: "Créer une branche dédiée pour le ticket #<ID> ?",
    options: [
      { label: "Oui (Recommandé)", description: "Créer et basculer sur <type>/<ticket-id>-<description-courte> avant de démarrer" },
      { label: "Non", description: "Rester sur la branche courante" }
    ]
  }]
})
```

- **Oui** → transmettre le nom de branche à l'agent développeur avec l'instruction :
  > « Crée et bascule sur la branche `<nom>` avant de démarrer :
  > `git switch -c <nom>` »

  > Un worktree se choisit au lancement de la session (emplacement proposé par oh) : ne crée pas de worktree toi-même.

- **Non** → continuer sur la branche courante, ne pas créer de branche

→ étape 2

---

### Étape 2 — Délégation de l'implémentation

1. Annoncer la délégation :
   > « Je délègue l'implémentation du ticket #<ID> à `developer` (domaine <domaine>). »

2. Invoquer l'agent développeur identifié, en fournissant :
   - L'ID du ticket (`bd show <ID>`)
   - Le contexte de la feature si disponible (specs UX/UI validées, rapports d'audit)
   - Si le ticket porte le label `tdd` → préciser explicitement :
     > « Ce ticket est en TDD — écrire les tests rouges couvrant les critères d'acceptance **avant** d'implémenter. »

3. L'agent développeur délégué exécute son workflow Beads complet de manière autonome.
   (bd claim → **[TDD : tests rouges d'abord]** → implémenter → tester → bd update --add-label ready-for-review)
   orchestrator-dev attend le compte rendu — il n'exécute aucune de ces étapes lui-même.

4. À la réception du résultat, effectuer les vérifications suivantes dans l'ordre :

   1. **Détecter la présence du compte rendu d'implémentation complet** (description de ce qui a été fait, fichiers modifiés, tests écrits) :
      - **Présent** → continuer la vérification suivante
      - **Absent** → appliquer le protocole de retry (voir skill `orchestrator/error-recovery-protocol`) : max 2 re-invocations avec `task_id` + instruction corrective. Après 2 échecs → escalader à l'utilisateur.

   2. **Détecter la présence du bloc `## Retour vers orchestrator-dev`** :
       - **Présent** → lire le `### Statut` :
         - `implémenté` ou `partiellement-implémenté` → continuer vers l'étape 3
         - `bloqué` → traiter comme un "Ticket bloqué en cours d'implémentation" (voir section dédiée)
          - `BLOCKED_ARCHITECTURE` → **charger puis appliquer le skill de gestion de dérive** :
            0. Charger la skill `dev-drift-detection` (outil `skill`) pour appliquer le protocole
            1. Lire le rapport de dérive fourni par le developer
            2. Présenter les 3 options à l'utilisateur (réviser scope / revert / bifurquer) via l'outil `question` ou bloc handoff selon le contexte
            3. Appliquer la décision : modifier le ticket Beads (Option A), relancer depuis l'étape 1b (Option B), ou créer le ticket de refactoring et mettre le ticket courant en `blocked` (Option C)
       - **Absent** → appliquer le protocole de retry (voir skill `orchestrator/error-recovery-protocol`) : max 2 re-invocations avec `task_id` + instruction corrective. Si le résultat est tronqué (indicateurs de progression mais pas de bloc de clôture) → suspecter un context window exhaustion et ré-invoquer dans une **nouvelle session** avec un prompt réduit. Après épuisement des retries → escalader à l'utilisateur.

   Le format attendu et les définitions des statuts sont définis dans le skill `developer/developer-handoff-format` — s'y référer comme source de vérité.

   3. **Vérifier `### Questions bloquantes`** :
      - `Aucune.` → continuer vers l'étape 3
      - Non-vide → **escalader les questions à l'utilisateur** (via `question` en agent d'entrée, via `## Question pour l'orchestrator` en sous-agent) avant de continuer. Ne jamais ignorer, ne jamais auto-résoudre. Après résolution, ré-invoquer le developer avec `task_id` + les réponses si des modifications sont nécessaires.

   > ❌ Ne jamais passer à l'étape 3 sans avoir reçu à la fois le compte rendu d'implémentation ET le bloc `## Retour vers orchestrator-dev`.
   > ❌ Ne jamais ignorer des questions bloquantes non résolues — elles doivent être traitées avant la review.

---

### Étape 3 — Pre-review (faite par le developer)

**Rôle :** les vérifications automatiques (lint, types, tests, format) sont un **critère de fin du developer** : il les lance lui-même avec les commandes du projet avant de rendre la main, applique les corrections déterministes (`--fix`, formatage) et les rapporte dans son bloc `## Retour vers orchestrator-dev` (section des tests et vérifications). Tu ne lances aucune commande et tu ne lis pas `package.json` : tu lis le résultat dans la passation.

> Référence pour les corrections auto-applicables : skill `developer/quick-fix` (lint fix, formatage, point-virgule ; jamais de renommage, refactoring ni logique métier).

#### Lecture du résultat

| Résultat rapporté par le developer | Action |
|----------|--------|
| Tous les checks ✅ (avec ou sans auto-fix) | → Étape 4 (Review) |
| Un check en échec non auto-fixable (types, tests, lint sans `--fix`, syntaxe) | Re-délégation au developer (ci-dessous) puis de nouveau étape 3 |
| Checks absents de la passation | Re-délégation : « Lance les vérifications du projet (lint, types, tests, format) et rapporte leur résultat. » |

Prompt de re-délégation en cas d'échec :

```
[Pre-review échouée]
Ticket : <ID>
Erreur(s) rapportée(s) :
- <check> : <message d'erreur>

Action requise :
1. bd comments add <ID> "Pre-review échouée : <détail de l'erreur>"
2. Corriger les erreurs
3. Relancer les vérifications du projet et repasser en review (bd update <ID> --add-label ready-for-review)
```

> **Compteur de cycles :** les boucles « pre-review échoue → retour developer » ne comptent **pas** dans la limite des 3 cycles de review (étape 4). Après **5 cycles** sans résolution, escalader à l'utilisateur (skill `orchestrator/error-recovery-protocol`, « Boucle pre-review infinie ») : intervenir manuellement / passer à la review malgré les erreurs / passer ce ticket / stop.

---

### Étape 4 — Review automatique

Dès que la pre-review rapportée par le developer est verte, invoquer **automatiquement** le `reviewer` :

**Mise à jour todowrite — agent d'entrée uniquement :**

```
todowrite({
  todos: [
    { content: "#bd-12 — <titre court> [review]", status: "in_progress", priority: "high" },  // ← label [review]
    { content: "#bd-13 — <titre court>", status: "pending", priority: "high" },
    { content: "#bd-14 — <titre court>", status: "pending", priority: "medium" }
  ]
})
```

> « Implémentation terminée — je soumets au reviewer. »

Fournir au reviewer :
- Le nom de la branche produite (le reviewer récupère lui-même le diff complet via `git diff` — ne pas tenter de construire ou transmettre le diff depuis orchestrator-dev)
- L'ID du ticket Beads pour contexte (`bd show <ID>`)
- Si disponible depuis le retour developer : les `### Points d'attention pour la review` du developer
- **Le skill de parcours (obligatoire) :**
  > `[SKILL:reviewer/reviewer-subagent]`

À la réception du résultat, effectuer les vérifications suivantes dans l'ordre :

1. **Détecter la présence du bloc `## Retour vers orchestrator-dev`** avec sa section `### Rapport complet` :
   - **Présent** → lire le `### Verdict` pour préparer `cp-2` :
     - `commit` → résumé « reviewer approuve — aucun problème bloquant »
     - `corriger` ou `corriger-sécurité` → résumé avec la synthèse des problèmes + routing recommandé
   - **Absent** → appliquer le protocole de retry (voir skill `orchestrator/error-recovery-protocol`) : max 2 re-invocations avec `task_id` + instruction corrective. Après 2 échecs → escalader à l'utilisateur.
   - **`### Rapport complet` absent dans le bloc** → appliquer le protocole de retry : max 2 re-invocations avec `task_id` + instruction listant les champs manquants. Après 2 échecs → escalader.

Le format attendu, les définitions des verdicts et du routing sont définis dans le skill `reviewer/reviewer-handoff-format` — s'y référer comme source de vérité.

2. **Vérifier `### Questions bloquantes`** :
   - `Aucune.` → continuer vers `cp-2`
   - Non-vide → escalader les questions à l'utilisateur avant d'appeler `cp-2`.

> ❌ Ne jamais passer à l'étape 5 sans avoir reçu à la fois le rapport de review complet ET le bloc `## Retour vers orchestrator-dev`.
> ❌ Ne jamais ignorer des questions bloquantes du reviewer — elles doivent être traitées avant `cp-2`.

---

### Étape 5 — Décision après review

Afficher le rapport de review intégralement dans le texte de la discussion.

**Checkpoint `cp-2`** (commit ou correction), **un appel par ticket** :

```
workflow_checkpoint({ id: "cp-2", summary: "Ticket #<ID> — <titre>. Verdict du reviewer : <commit | corriger | corriger-sécurité> (<X> problèmes 🔴/🟠). Recommandé : <commit | corriger>. Validation = commit puis fermeture ; refus = correction." })
```

X = nombre de 🔴 Critique + 🟠 Majeur de `### Synthèse des problèmes` (sans 🟡 Mineur ni 💡 Suggestion). Le checkpoint attend la validation de l'utilisateur dans tous les modes. Validé → **commit** ci-dessous, aussitôt ; refusé avec une consigne → **corriger** avec cette consigne.

**Après chaque décision sur `cp-2`, si l'outil `team_review_verdict` est disponible :**
- Sur **commit** : `team_review_verdict({ project, ticket_id, verdict: "approved", reviewer_verdict: "<verdict original>", cycle: <N> })`
- Sur **corriger** : `team_review_verdict({ project, ticket_id, verdict: "rejected", reason: "corrections-requises", reviewer_verdict: "<verdict original>", cycle: <N> })`

> Cet appel enregistre le verdict dans le journal d'équipe, transite le statut du claim, ajoute le label `agent-reviewed` (si approved), et notifie l'équipe.

- **commit** →
  1. Formuler le message de commit selon Conventional Commits :
     `<type>(<scope>): <description> (<ID>)` — basé sur le type du ticket et son titre ; l'ID entre parenthèses en fin de ligne relie le commit au ticket (oh doctor signale ainsi les tickets commités mais restés ouverts)
  2. Relancer **aussitôt** le developer (même `task_id`) avec l'instruction — c'est lui qui commite et clôt, jamais avant cette instruction :
     > « Crée le commit final et clos le ticket :
     > 1. `git commit -m "<type>(<scope>): <description> (<ID>)"`
     > 2. `bd close <ID> --reason "Implemented in commit <hash>" --suggest-next` »
  → étape 6

- **corriger** → transmettre les retours reviewer au developer dans le prompt de re-délégation.
  Le developer est responsable de poser le commentaire Beads et de reprendre le ticket.

  Transmettre au developer dans le prompt de re-délégation :

  ```
  [Retours reviewer]
  Ticket : <ID>
  
  Action requise :
  1. bd comments add <ID> "Retours reviewer : <contenu intégral de ### Corrections requises — copier tel quel, sans résumer>"
  2. Appliquer les corrections ci-dessous
  3. Repasser en review (bd update <ID> --add-label ready-for-review)
  
  ### Corrections requises
  <contenu intégral du champ ### Corrections requises du retour reviewer>
  ```

  > **Règle de transmission :** copier les `### Corrections requises` telles quelles dans le prompt — ne jamais résumer ni reformuler.

  **Routing de la correction — basé sur le `### Routing recommandé` du retour reviewer :**
  - `domaine-security` → router vers `developer` (domaine `security`)
    > « La correction est de nature sécurité — je route vers `developer` (domaine security). »
  - `retour-initial` → retourner à l'agent `developer` avec le même domaine initial

  > « Je retourne le ticket à `developer` (domaine <xxx>) avec les corrections demandées. »
  > Puis repasser étape 3 (pre-review rapportée par le developer) → étape 4 (review).

  ⚠️ Limite : après 3 cycles sans résolution, signaler le blocage et demander si une intervention manuelle est nécessaire.

⏸️ **Attendre la réponse de `workflow_checkpoint`** : c'est la décision de l'utilisateur.

---

### Étape 6 — Compte rendu d'étape

Construire le compte rendu en agrégeant les données structurées collectées aux étapes précédentes :

```
## ✅ Ticket #<ID> terminé — <titre>

**Agent :** developer (<domaine>)
**Cycles de review :** <NB_CYCLES>
**Corrections demandées :** <oui/non>
**Statut Beads :** clos

**Changements par fichier :**
<bloc `**Changements par fichier :**` intégral issu du retour developer — si disponible>

**Couverture des critères d'acceptance :** <tous couverts | partielle — <critères non couverts>>
<issue du ### Critères d'acceptance couverts du retour developer>

**Points d'attention techniques :**
<issue du ### Points d'attention pour la review du retour developer — si renseigné>
<"Aucun" si aucun point d'attention signalé>

**Données structurées du bloc developer :**
<champs `### Contexte et décisions`, `### Implémentation`, `### Points d'attention pour la review` extraits du bloc `## Retour vers orchestrator-dev` du developer — stockés pour inclusion dans le récap global.>

---

**Tickets restants :** <NB_RESTANTS> | **Traités :** <NB_TRAITES> | **Ignorés :** <NB_IGNORES>
```

Si le ticket est de type `feature` ou `fix` (visible utilisateur), utiliser l'outil `question` :

```
question({
  questions: [{
    header: "CHANGELOG",
    question: "Ce ticket est de type feature/fix. Mettre à jour le CHANGELOG via le documentarian ?",
    options: [
      { label: "Non (Recommandé)", description: "Passer au ticket suivant sans mettre à jour le CHANGELOG" },
      { label: "Oui", description: "Invoquer le documentarian pour mettre à jour le CHANGELOG" }
    ]
  }]
})
```
Invoquer `documentarian` uniquement si l'utilisateur répond "Oui".

À la réception du résultat du documentarian, effectuer les vérifications suivantes :

1. **Détecter la présence du contenu de documentation complet** (présenté avant le bloc) :
   - **Présent** → continuer la vérification suivante
   - **Absent** → demander explicitement au documentarian de présenter le contenu complet avant de continuer.

2. **Détecter la présence du bloc de retour** (titre commençant par `## Retour vers`) :
   - **Présent** → lire le `### Statut` et intégrer le `### Résumé de l'entrée` dans le compte rendu d'étape
   - **Absent** → demander explicitement au documentarian de produire le bloc avant de continuer.

Le format attendu et les définitions des statuts sont définis dans le skill `documentarian/documentarian-handoff-format` — s'y référer comme source de vérité.

**Mise à jour todowrite (fin de ticket) :**

Mettre à jour todowrite avec le ticket passé en `completed` — le suffixe de phase est retiré :

```
todowrite({
  todos: [
    { content: "#bd-12 — <titre court>", status: "completed", priority: "high" },  // ← sans suffixe, completed
    { content: "#bd-13 — <titre court>", status: "completed", priority: "high" },  // ← ticket terminé
    { content: "#bd-14 — <titre court>", status: "pending", priority: "medium" }
  ]
})
```

> Invoqué via `task` par l'orchestrator, cette mise à jour est locale à la session isolée. L'orchestrator feature met à jour sa propre liste en recevant le récap via les blocs de handoff.
> Agent d'entrée de la session : cette mise à jour est immédiatement visible par l'utilisateur.

**Checkpoint `cp-3`** (ticket suivant) :

```
workflow_checkpoint({ id: "cp-3", summary: "Ticket #<ID> terminé (commit <sha court>). Suivant : #<ID-suivant> — <titre>." })
```

- Validé (ou passé automatiquement selon le mode) → ticket suivant
- Refusé avec « stop » → **Récap global — Fin de session**
