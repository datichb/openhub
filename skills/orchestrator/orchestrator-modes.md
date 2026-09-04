---
name: orchestrator-modes
description: Détail des 5 modes d'entrée de l'orchestrator — Mode D (bug), Mode E (pathfinder), Mode C (projet inconnu), Mode A (feature NL), Mode B (tickets existants).
---

## Ordre de priorité des modes

Quand plusieurs conditions de déclenchement sont vraies simultanément, appliquer l'ordre de priorité suivant :

1. **Mode D (bug)** — priorité la plus haute
   - Si l'utilisateur signale un bug ou une anomalie, invoquer le debugger immédiatement, même si le projet est inconnu
   - L'onboarding peut intervenir après le diagnostic si nécessaire

2. **Mode C (onboarding)** — priorité intermédiaire
   - Si aucun bug n'est signalé mais que le projet est inconnu, proposer l'onboarding

3. **Mode A / Mode B** — priorité par défaut
   - Si le projet est connu et qu'aucun bug n'est signalé, router vers planification ou tickets existants

> **Exemple de conflit résolu :** « J'ai un bug sur ce projet que je découvre » → Mode D (debugger) d'abord, puis proposition d'onboarding après le rapport de diagnostic si ONBOARDING.md est toujours absent.

---

### Mode D — Bug / Problème isolé signalé par l'utilisateur

À utiliser quand l'utilisateur ouvre une session en décrivant un problème, une anomalie,
un comportement inattendu ou un bug — sans contexte de feature en cours.

**Condition de déclenchement — activer le Mode D si :**
- L'utilisateur décrit un bug, une erreur ou un comportement anormal
- L'utilisateur dit "ça plante", "j'ai un souci", "ça ne fonctionne pas" ou équivalent
- Le message d'entrée n'est pas une feature à implémenter mais un problème à diagnostiquer

**Action immédiate — sans analyse ni tentative de correction :**

> « Je détecte un problème à diagnostiquer. Je délègue immédiatement à l'agent `debugger`. »

Invoquer le `debugger` en lui transmettant :
- Le problème tel que décrit par l'utilisateur (verbatim)
- Tout contexte disponible (fichier mentionné, comportement attendu vs observé, stacktrace)
- **Le marqueur de contexte d'invocation (obligatoire) :**
  > `[CONTEXTE] Invoqué depuis l'orchestrateur feature. Tu dois utiliser le mécanisme d'interruption de session à chaque checkpoint et produire le bloc ## Retour vers orchestrator en fin de session.`
- **Le skill de parcours (obligatoire) :**
  > `[SKILL:quality/debugger-subagent]`

À la réception du résultat, **détecter le type de retour** :

**Cas A — retour final :** le résultat contient `## Retour vers orchestrator` mais **pas** de `## Question pour l'orchestrator`
→ Effectuer les vérifications :

1. **Détecter la présence des blocs `## Retour intermédiaire vers orchestrator`** (récaps de phases accumulés) :
   - **Présents** → les afficher intégralement en texte dans la discussion, dans l'ordre, AVANT le reste
   - **Absents** → continuer directement

2. **Détecter la présence du bloc `## Retour vers orchestrator`** avec sa section `### Rapport de diagnostic complet` :
   - **Présent** → continuer
   - **Absent** → demander explicitement au debugger de produire le bloc complet avant de continuer.
   - **`### Rapport de diagnostic complet` absent dans le bloc** → demander au debugger de compléter le bloc.

**Cas B — question montante :** le résultat contient `## Question pour l'orchestrator`
→ Voir section "Réception d'une question montante depuis le debugger" ci-dessous.

> Template de retranscription, checklist et vérifications complets → skill `posture/retranscription-coordinateur`.

**Spécificités debugger (Cas A) :**
- Sections critiques : `### Actions d'urgence si bug en prod`, `### Impact et régressions potentielles`
- Présenter les actions d'urgence **en premier** si renseignées — elles priment sur toute autre décision
- Proposer d'intégrer les tickets créés dans le workflow (Mode A ou B) si applicable

---

### Réception d'une question montante depuis le debugger

Quand le debugger atteint un checkpoint (fin de phase, pause, confirmation d'action irréversible), il termine sa session avec un bloc `## Question pour l'orchestrator`.

> ⚠️ **RAPPEL IMPÉRATIF** : Afficher le `## Retour intermédiaire vers orchestrator` AVANT d'appeler l'outil `question`.

**Comportement obligatoire :**

1. **Afficher intégralement le bloc `## Retour intermédiaire vers orchestrator`** dans la discussion — ne jamais résumer.

2. **Lire le bloc `## Question pour l'orchestrator`** — récupérer : question, options, `task_id`, instruction de reprise.

3. **Poser la question à l'utilisateur** via l'outil `question` :

   ```
   question({
     questions: [{
       header: "[Debugger] Phase X — Bug : <titre>",
       question: "[Debugger — Phase X | Bug : <titre>]\n<question exacte du bloc>",
       options: [
         { label: "<label-option-1>", description: "<description du bloc>" },
         { label: "<label-option-2>", description: "<description du bloc>" }
       ]
     }]
   })
   ```

4. **Ré-invoquer le debugger avec `task_id`** :

   ```
   task(
     subagent_type: "debugger",
     task_id: "<task_id du bloc>",
     prompt: "<Instruction de reprise du bloc>. Réponse : <option choisie>."
   )
   ```

5. **Attendre le nouveau résultat** et recommencer la détection (Cas A ou Cas B).

---

Le format attendu et les définitions des statuts du debugger sont définis dans le skill `quality/debugger-handoff-format` — s'y référer comme source de vérité.

> ❌ Ne jamais accepter un bloc handoff sans la section `### Rapport de diagnostic complet` — elle doit être intégrée dans le bloc.

⚠️ Ne jamais tenter de :
- Lire les fichiers concernés pour comprendre le bug
- Formuler une hypothèse de cause racine
- Proposer une correction, même partielle, même "pour débloquer"

Le `debugger` prend en charge l'analyse complète et la création du ticket de correction.

---

### Mode C — Projet inconnu (pré-phase optionnelle)

À utiliser uniquement quand le contexte projet est absent de la session.

**Vérification préalable obligatoire — avant toute proposition d'onboarding :**

Le contexte projet est injecté automatiquement dans la session via le champ `instructions` de `opencode.json` :
- **Présent** (cache `.opencode/context.json` valide, ou `ONBOARDING.md`/`CONVENTIONS.md` détectés au démarrage) → le contexte est disponible dans la session. Passer directement en Mode A ou Mode B.
- **Absent** (aucun fichier injecté) → évaluer les conditions ci-dessous.

**Condition de déclenchement — proposer le Mode C si ET SEULEMENT SI :**
- Aucun contexte projet n'est disponible dans la session
- ET l'utilisateur ne donne aucun contexte projet dans son message (feature brute sans contexte)
  ou dit explicitement "je découvre ce projet" ou équivalent

**Proposer à l'utilisateur via l'outil `question` :**

```
question({
  questions: [{
    header: "Onboarding projet",
    question: "Aucun fichier de contexte (ONBOARDING.md, CONVENTIONS.md) n'existe sur ce projet. Lancer l'onboarder pour établir le contexte avant de démarrer la feature ?",
    options: [
      { label: "Oui — lancer l'onboarder (Recommandé)", description: "Invoquer l'onboarder pour analyser le projet et établir le contexte" },
      { label: "Non — skip", description: "Passer directement à la feature (à utiliser si tu connais déjà le projet)" }
    ]
  }]
})
```

- **Oui** → Invoquer l'`onboarder`, attendre le rapport complet.

  Invoquer avec le marqueur de contexte (obligatoire) :
  > `[CONTEXTE] Invoqué depuis l'orchestrateur feature. Tu dois utiliser le mécanisme d'interruption de session à chaque fin de phase et produire le bloc ## Retour vers orchestrator en fin de session.`
  > `[SKILL:planning/onboarder-subagent]`

  À la réception du résultat, **détecter le type de retour** :

  **Cas A — retour final :** contient `## Retour vers orchestrator` mais **pas** de `## Question pour l'orchestrator`
  → Effectuer les vérifications dans l'ordre :

  1. **Détecter la présence des blocs `## Retour intermédiaire vers orchestrator`** :
     - **Présents** → les afficher intégralement en texte dans l'ordre, AVANT le reste
     - **Absents** → continuer directement

  2. **Détecter la présence du bloc `## Retour vers orchestrator`** avec sa section `### Rapport d'onboarding` :
     - **Présent** → continuer
     - **Absent** → demander explicitement à l'onboarder de produire le bloc complet avant de continuer.
     - **`### Rapport d'onboarding` absent dans le bloc** → demander à l'onboarder de compléter le bloc.

  **Cas B — question montante :** contient `## Question pour l'orchestrator`
  → Voir section "Réception d'une question montante depuis l'onboarder" ci-dessous.

> Template de retranscription, checklist et vérifications complets → skill `posture/retranscription-coordinateur`.

**Spécificités onboarder (Cas A) :**
- Sections critiques : `### Zones d'incertitude`, `### Dette technique détectée`
- Présenter les zones d'incertitude à l'utilisateur pour décision avant de démarrer la feature
- Signaler la dette technique 🔴 qui pourrait impacter la feature

---

### Réception d'une question montante depuis l'onboarder

Quand l'onboarder atteint un checkpoint (fin de phase), il termine sa session avec un bloc `## Question pour l'orchestrator`.

> ⚠️ **RAPPEL IMPÉRATIF** : Afficher le `## Retour intermédiaire vers orchestrator` AVANT d'appeler l'outil `question`.

**Comportement obligatoire :**

1. **Afficher intégralement le bloc `## Retour intermédiaire vers orchestrator`** dans la discussion.

2. **Lire le bloc `## Question pour l'orchestrator`** — récupérer : question, options, `task_id`, instruction de reprise.

3. **Poser la question à l'utilisateur** via l'outil `question` :

   ```
   question({
     questions: [{
       header: "[Onboarder] Phase X — <nom projet>",
       question: "[Onboarder — Phase X | Projet : <nom>]\n<question exacte du bloc>",
       options: [
         { label: "<label-option-1>", description: "<description du bloc>" },
         { label: "<label-option-2>", description: "<description du bloc>" }
       ]
     }]
   })
   ```

4. **Ré-invoquer l'onboarder avec `task_id`** :

   ```
   task(
     subagent_type: "onboarder",
     task_id: "<task_id du bloc>",
     prompt: "<Instruction de reprise du bloc>. Réponse : <option choisie>."
   )
   ```

5. **Attendre le nouveau résultat** et recommencer la détection (Cas A ou Cas B).

---

  Le format attendu et les définitions des statuts de l'onboarder sont définis dans le skill `planning/onboarder-handoff-format` — s'y référer comme source de vérité.

  > ❌ Ne jamais accepter un bloc handoff sans la section `### Rapport d'onboarding` — elle doit être intégrée dans le bloc.

  **[CP-onboard]** — Après avoir affiché le rapport et le bloc dans le texte de la discussion (ne pas inclure dans l'outil `question`), utiliser l'outil `question` :

  ```
  question({
    questions: [{
      header: "CP-onboard",
      question: "Contexte établi pour [Nom du projet]. Le contexte est-il suffisant pour démarrer la feature ?",
      options: [
        { label: "Oui — démarrer la feature", description: "Continuer en Mode A ou Mode B avec le contexte établi" },
        { label: "Non — questions complémentaires", description: "Poser des questions avant de démarrer" }
      ]
    }]
  })
  ```

- **Non / skip** → Passer directement en Mode A ou Mode B.

Le Mode C est toujours optionnel et sautables — ne jamais le forcer.

---

### Mode A — Feature en langage naturel

L'utilisateur décrit une feature, un besoin ou un chantier.

**Étapes :**

1. Déléguer au `planner` :
   > « Je délègue la planification au `planner` pour la feature : <nom de la feature>.
   > Le planner va explorer le projet, poser des questions de contexte et produire les tickets — les récaps et questions apparaîtront ici avec leur contexte identifié. »

   Invoquer le planner en transmettant :
   - La feature à planifier (description verbatim de l'utilisateur)
   - **Le marqueur de contexte d'invocation (obligatoire) :**
     > `[CONTEXTE] Invoqué depuis l'orchestrateur feature. Tu dois utiliser le mécanisme d'interruption de session (blocs ## Retour intermédiaire vers orchestrator + ## Question pour l'orchestrator) à chaque fin de phase et chaque pause, et produire le bloc ## Retour vers orchestrator en fin de planification — sans exception.`
   - **Le skill de parcours (obligatoire) :**
     > `[SKILL:planning/planner-subagent]`

2. À la réception du résultat du `planner`, **détecter le type de retour** :

   **Cas A — retour final :** le résultat contient `## Retour vers orchestrator` mais **pas** de `## Question pour l'orchestrator`
   → Effectuer les vérifications suivantes dans l'ordre :

   1. **Détecter la présence des blocs `## Retour intermédiaire vers orchestrator`** (récaps de phases accumulés) :
      - **Présents** → les afficher intégralement en texte dans la discussion, dans l'ordre, AVANT le reste
      - **Absents** → continuer directement avec le récapitulatif final

   2. **Détecter la présence du bloc `## Retour vers orchestrator`** avec sa section `### Récapitulatif de planification` :
      - **Présent** → continuer
      - **Absent** → demander explicitement au planner de produire le bloc complet avant de continuer.
      - **`### Récapitulatif de planification` absent dans le bloc** → demander au planner de compléter le bloc.

   **Cas B — question montante :** le résultat contient `## Question pour l'orchestrator`
   → Voir section "Réception d'une question montante depuis le planner" ci-dessous.

> Template de retranscription, checklist et vérifications complets → skill `posture/retranscription-coordinateur`.

**Spécificités planner (Cas A) :**
- Sections critiques : `### Hypothèses et ambiguïtés`, `### Risques identifiés`, `### Ordre de traitement`
- Présenter hypothèses et risques à l'utilisateur avant le CP-0

---

### Réception d'une question montante depuis le planner

Quand le planner atteint un checkpoint (fin de phase ou clarification critique), il termine sa session avec un bloc question. **Détecter le type de bloc** pour appliquer le bon traitement :

| Bloc reçu | Type | Traitement |
|-----------|------|------------|
| `## Question pour l'orchestrator` | Question unitaire (fin de phase) | → **Cas B** ci-dessous |
| `## Question batch pour l'orchestrator` | Questions multiples (Phase 2) | → **Cas C** ci-dessous |

> ⚠️ **RAPPEL IMPÉRATIF** : Tu DOIS afficher le contenu du bloc `## Retour intermédiaire vers orchestrator` AVANT d'appeler l'outil `question`. Ne jamais appeler `question` sans avoir d'abord affiché le récap en texte.

---

**Cas B — Question unitaire (fin de phase) :**

Le résultat contient `## Question pour l'orchestrator` (mais PAS `## Question batch`).

1. **Afficher intégralement le bloc `## Retour intermédiaire vers orchestrator`** dans la discussion — ne jamais résumer ni abréger.

2. **Lire le bloc `## Question pour l'orchestrator`** — récupérer : question, options, `task_id`, instruction de reprise.

3. **Poser la question à l'utilisateur** via l'outil `question` en reprenant exactement la question et les options du bloc :

   ```
   question({
     questions: [{
       header: "[Planner] Phase X — Feature : <nom>",
       question: "[Planner — Phase X | Feature : <nom>]\n<question exacte du bloc>",
       options: [
         { label: "<label-option-1>", description: "<description du bloc>" },
         { label: "<label-option-2>", description: "<description du bloc>" }
       ]
     }]
   })
   ```

4. **Ré-invoquer le planner avec `task_id`** (valeur dans le bloc `## Question pour l'orchestrator`) en transmettant la réponse :

   ```
   task(
     subagent_type: "planner",
     task_id: "<task_id du bloc>",
     prompt: "<Instruction de reprise du bloc>. Réponse : <option choisie>. [Information fournie si applicable.]"
   )
   ```

   > **Toujours reprendre le marqueur `[CONTEXTE]` et le skill :** si l'instruction de reprise ne les contient pas déjà, ajouter :
   > `[CONTEXTE] Invoqué depuis l'orchestrateur feature. Mécanisme d'interruption actif.`
   > `[SKILL:planning/planner-subagent]`

5. **Attendre le nouveau résultat** et recommencer la détection (Cas A, B ou C).

---

**Cas C — Question batch montante (Phase 2 — questions de clarification) :**

Le résultat contient `## Question batch pour l'orchestrator`. Ce bloc est produit par le planner en Phase 2 et contient **plusieurs questions de clarification** à poser à l'utilisateur en un seul appel.

> ⚠️ Ce cas est **spécifique à la Phase 2** du planner. C'est le seul moment où un batch de questions remonte.

**Comportement obligatoire :**

1. **Afficher intégralement le bloc `## Retour intermédiaire vers orchestrator`** dans la discussion — y compris le contexte global (observations Phase 1).

2. **Lire le bloc `## Question batch pour l'orchestrator`** — récupérer : le contexte global, chaque question (header, question, options), `task_id`, instruction de reprise.

3. **Afficher le contexte global** du batch dans la discussion (résumé de l'exploration Phase 1) — ne pas inclure dans l'outil `question`.

4. **Poser TOUTES les questions à l'utilisateur** via un **seul appel `question`** avec une entrée par question du batch :

   ```
   question({
     questions: [
       {
         header: "<header Q1 — ex: Objectif métier>",
         question: "[Planner — Phase 2 | Feature : <nom>]\n<question exacte de Q1>",
         options: [
           { label: "<label-a>", description: "<description>" },
           { label: "<label-b>", description: "<description>" }
         ]
       },
       {
         header: "<header Q2 — ex: Hors périmètre>",
         question: "[Planner — Phase 2 | Feature : <nom>]\n<question exacte de Q2>",
         options: [
           { label: "<label-a>", description: "<description>" },
           { label: "<label-b>", description: "<description>" }
         ]
       },
       // ... une entrée par question du batch
       {
         header: "Skip questions",
         question: "[Planner — Phase 2 | Feature : <nom>]\nSi vous préférez ne pas répondre, vous pouvez passer cette étape.",
         options: [
           { label: "J'ai répondu", description: "Continuer avec mes réponses" },
           { label: "Skip toutes", description: "Passer les clarifications" }
         ]
       }
     ]
   })
   ```

   > **Règle critique :** reproduire **chaque question** du batch dans l'appel `question` — ne jamais les résumer en une seule question "Voulez-vous répondre aux questions ?", et ne jamais demander "ignorer ou répondre".

5. **Ré-invoquer le planner avec `task_id`** en transmettant **toutes les réponses** :

   ```
   task(
     subagent_type: "planner",
     task_id: "<task_id du bloc>",
     prompt: "<Instruction de reprise du bloc>. Réponses Phase 2 : [Q1 (<header>): <réponse>, Q2 (<header>): <réponse>, ...]. [CONTEXTE] Invoqué depuis l'orchestrateur feature. Mécanisme d'interruption actif. [SKILL:planning/planner-subagent]"
   )
   ```

6. **Attendre le nouveau résultat** et recommencer la détection (Cas A, B ou C).

---

**Cas D — session introuvable :** si la ré-invocation avec `task_id` ne produit pas de résultat :

```
question({
  questions: [{
    header: "Session planner perdue",
    question: "[Orchestrator — Session introuvable | Planner]\nLa session planner a été perdue (redémarrage probable). Comment reprendre ?",
    options: [
      { label: "Relancer depuis le début (Recommandé)", description: "Invoquer une nouvelle session planner" },
      { label: "Stop", description: "Arrêter la planification" }
    ]
  }]
})
```

---

   Le format attendu, les champs obligatoires et les définitions des statuts du planner sont définis dans le skill `planning/planner-handoff-format` — s'y référer comme source de vérité.

   > ❌ Ne jamais accepter un bloc handoff final sans la section `### Récapitulatif de planification` — elle doit être intégrée dans le bloc.

3. **Récupérer les instructions de routing depuis le retour planner :**
   - Lire le champ `Agent prévu` dans le tableau `### Tickets créés` pour chaque ticket — c'est l'agent à utiliser
   - Lire la section `### Ordre de traitement` pour la séquence d'exécution
   - Noter la présence du label `tdd` depuis la colonne `TDD` du tableau
   - *Voir règles de routing dans le noyau `orchestrator.md`*

4. **Initialiser la liste todowrite** — construire la liste avec 1 tâche par phase ET 1 tâche par ticket dev :

   > ⚠️ **Contrainte d'isolation des sessions :** la todo list d'`orchestrator-dev` (invoqué via `task`) est dans une session isolée — elle n'est pas visible par l'utilisateur. L'orchestrator feature est le seul responsable de la liste visible. Il doit donc maintenir une granularité suffisante pour refléter l'avancement réel ticket par ticket.
   >
   > Référence : `skills/posture/tool-todowrite.md` section "Usage par type d'agent".

   ```
   todowrite({
     todos: [
       { content: "Planification feature", status: "completed", priority: "high" },
       // Inclure si tickets spec-ux identifiés — une tâche par ticket :
       { content: "Spec UX — #bd-10 <titre court>", status: "pending", priority: "high" },
       // Inclure si tickets spec-ui identifiés — une tâche par ticket :
       { content: "Spec UI — #bd-11 <titre court>", status: "pending", priority: "high" },
       // Inclure si tickets audit identifiés — une tâche par ticket :
       { content: "Audit sécurité — #bd-13 <titre court>", status: "pending", priority: "medium" },
       // Une tâche par ticket dev (agent prévu = orchestrator-dev) :
       { content: "#bd-12 — <titre court>", status: "pending", priority: "high" },
       { content: "#bd-14 — <titre court>", status: "pending", priority: "medium" }
     ]
   })
   ```

   **Règles de construction :**
   - Tickets `spec-ux` → `"Spec UX — #bd-XX <titre>"` (une tâche par ticket)
   - Tickets `spec-ui` → `"Spec UI — #bd-XX <titre>"` (une tâche par ticket)
   - Tickets `audit` → `"Audit <domaine> — #bd-XX <titre>"` (une tâche par ticket)
   - Tickets `dev` (routés vers orchestrator-dev) → `"#bd-XX — <titre>"` (une tâche par ticket)
   - Priorité : mapping direct depuis la priorité Beads (P0/P1 → `high`, P2 → `medium`, P3 → `low`)

   > La phase "Planification" est immédiatement `completed` puisqu'on vient de la terminer.

5. **[CP-0]** — voir section CP-0 ci-dessous.

---

### Mode B — Tickets Beads existants

L'utilisateur fournit directement un ou plusieurs IDs de tickets.

**Étapes :**

1. **Invoquer le planner en mode classification** directement avec les IDs fournis par l'utilisateur :
   > « Je délègue la classification au `planner` pour les tickets : [IDs].
   > Le planner va déterminer l'agent approprié et l'ordre de traitement pour chaque ticket. »

   Transmettre au planner : `Mode classification — déterminer l'agent et l'ordre de traitement pour les tickets : [IDs]`

   > ❌ Ne jamais faire `bd show <ID>` avant de transmettre au planner — les IDs sont fournis par l'utilisateur, le planner lit les tickets lui-même.

2. À la réception du résultat du planner, lire le champ `Agent prévu` pour chaque ticket et la section `### Ordre de traitement`.

3. **Initialiser la liste todowrite** — construire la liste avec 1 tâche par ticket :

   ```
   todowrite({
     todos: [
       // Inclure si tickets audit identifiés — une tâche par ticket :
       { content: "Audit sécurité — #bd-09 <titre court>", status: "pending", priority: "medium" },
       // Une tâche par ticket dev (routé vers orchestrator-dev) :
       { content: "#bd-10 — <titre court>", status: "pending", priority: "high" },
       { content: "#bd-11 — <titre court>", status: "pending", priority: "medium" }
     ]
   })
   ```

   > En Mode B, la planification n'a pas lieu — les tickets existent déjà. La liste ne contient que les phases audit (si applicable) et les tickets dev, au même format granulaire que le Mode A.

4. **[CP-0]** — voir section CP-0 ci-dessous.
