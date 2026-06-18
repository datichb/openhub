---
name: debugger-subagent
description: Parcours d'exécution du debugger en mode sous-agent (invoqué via task depuis l'agent orchestrator feature) — mécanisme d'interruption de session, blocs structurés Retour intermédiaire + Question pour l'agent orchestrator, task_id obligatoire. Ne jamais appeler l'outil question dans ce mode.
---

# Skill — Parcours Debugger Sous-agent

> Ce skill est chargé quand le debugger est invoqué via `task` depuis l'agent orchestrator feature. L'orchestrateur injecte `[SKILL:quality/debugger-subagent]` dans le prompt.

## Principe fondamental

Quand le debugger est invoqué via `task`, le texte de la session enfant n'est **PAS visible** par l'utilisateur dans la session parent. La seule façon de remonter du contenu est de **terminer la session** avec les blocs structurés, que l'agent orchestrator retranscrira.

**Confirmer le contexte au démarrage :**
> `[debugger] Contexte détecté : invoqué depuis l'agent orchestrator feature. Mode interruption actif — je terminerai ma session à chaque checkpoint pour remonter le récap et la question à l'agent orchestrator.`

---

## Mécanisme d'interruption — RÈGLE ABSOLUE

**À CHAQUE fin de phase ET à chaque pause ad hoc :**

1. Produire le récap de la phase en texte
2. Produire le bloc `## Retour intermédiaire vers orchestrator`
3. Produire le bloc `## Question pour l'orchestrator`
4. **TERMINER LA SESSION** — ne pas appeler l'outil `question`, ne pas continuer

L'orchestrateur :
- Affiche le `## Retour intermédiaire` en texte dans la discussion
- Lit la `## Question pour l'orchestrator`
- Pose la question à l'utilisateur via l'outil `question`
- Re-invoque le debugger avec `task_id` + la réponse → le debugger recharge l'historique et continue

---

## Autocontrôle avant chaque fin de session

> « Ai-je produit (1) le récap de la phase, (2) le bloc `## Retour intermédiaire vers orchestrator`, ET (3) le bloc `## Question pour l'orchestrator` ? »
> - **Non** → produire les blocs manquants MAINTENANT
> - **Oui** → terminer la session

---

## ✅ Checklist visuelle — AVANT CHAQUE FIN DE SESSION

**STOP — Vérifier MAINTENANT :**

| Vérification | Fait ? |
|--------------|--------|
| ✅ J'ai produit le récap complet de la phase en texte | ⬜ |
| ✅ J'ai produit le bloc `## Retour intermédiaire vers orchestrator` avec la synthèse condensée | ⬜ |
| ✅ J'ai produit le bloc `## Question pour l'orchestrator` avec question + options + instruction de reprise | ⬜ |
| ✅ Le `task_id` est renseigné dans les deux blocs | ⬜ |
| ✅ Je vais TERMINER la session — pas appeler l'outil `question` | ⬜ |

**Si une seule case est ⬜ (non cochée) → ARRÊTER et produire le contenu manquant MAINTENANT.**

---

## Format des blocs structurés

### Bloc standard (fin de phase)

```markdown
## [Phase X] <titre du récap>

<contenu complet du récap — observations, découvertes, décisions — JAMAIS résumé>

---

## Retour intermédiaire vers orchestrator

**Agent :** debugger
**Phase :** X — <titre>
**task_id :** <sessionID courant>

**Résumé :** <2-3 phrases décrivant ce qui a été fait dans cette phase>
**Points clés :** <liste courte — découvertes importantes, décisions prises, hypothèses formulées>
**Zones d'ombre / Blocages :** <si applicable, sinon omettre>

---

## Question pour l'orchestrator

**Phase :** X
**task_id :** <sessionID courant>

**Contexte :** <pourquoi cette question — ce qui a été découvert, ce qui bloque ou nécessite validation>

**Question :** <texte exact de la question à poser à l'utilisateur>

**Options :**
- `<label-option-a>` — <description de l'option>
- `<label-option-b>` — <description de l'option>

**Instruction de reprise :** "Réponse Phase X debugger : [option]. Reprendre depuis <contexte précis>."
```
→ **TERMINER LA SESSION**

---

### Bloc pause ad hoc (information manquante critique)

> ⚠️ Réserver aux vrais blockers — pas aux détails. Si une hypothèse documentée permet de continuer, continuer.

```markdown
## ⏸️ Pause — Phase X — <sujet de la pause>

Pendant [l'analyse de / l'exploration de] [artefact/fichier], j'ai détecté que [description précise du problème].

**Impact :** Sans cette information, [conséquence concrète sur le diagnostic].

**Hypothèse possible :** [formulation de l'hypothèse si l'utilisateur souhaite continuer sans info]

---

## Retour intermédiaire vers orchestrator

**Agent :** debugger
**Phase :** X — Pause (information manquante critique)
**task_id :** <sessionID courant>

**Résumé :** <description en 1-2 phrases du problème détecté>
**Impact :** <conséquence concrète sur le diagnostic si non résolu>

---

## Question pour l'orchestrator

**Phase :** X — Pause
**task_id :** <sessionID courant>

**Contexte :** <description du problème détecté et de son impact>

**Question :** <question précise>

**Options :**
- `fournir-information` — Fournir l'information maintenant
- `continuer-hypothese` — Continuer avec l'hypothèse : [formulation]

**Instruction de reprise :** "Réponse pause Phase X debugger : [option]. [Information fournie si applicable]. Reprendre depuis le point d'interruption."
```
→ **TERMINER LA SESSION**

---

## Blocs par phase

### Phase 0 — Artefacts insuffisants

```markdown
## Retour intermédiaire vers orchestrator

**Agent :** debugger
**Phase :** 0 — Artefacts insuffisants
**task_id :** <sessionID courant>

## ⏸️ Phase 0 — Artefacts insuffisants

Pour conduire un diagnostic sérieux, j'ai besoin des informations suivantes :
1. <information manquante 1 — ex : stacktrace complète>
2. <information manquante 2 — ex : conditions de déclenchement>
3. <information manquante 3 — ex : logs applicatifs>

**Impact :** Sans ces éléments, le diagnostic sera partiel et formulé en hypothèses.

---

## Question pour l'orchestrator

**Phase :** 0
**task_id :** <sessionID courant>

**Contexte :** Les artefacts fournis sont insuffisants pour conduire un diagnostic sérieux.

**Question :** Pour conduire un diagnostic sérieux, j'ai besoin de : <liste>. Comment souhaitez-vous procéder ?

**Options :**
- `fournir-informations` — Copier les logs, la stacktrace ou décrire le scénario de reproduction précis
- `continuer-quand-meme` — Démarrer le diagnostic avec les éléments disponibles — le rapport sera partiel

**Instruction de reprise :** "Réponse Phase 0 debugger : [option]. Reprendre depuis Phase 0 — artefacts insuffisants."
```
→ **TERMINER LA SESSION**

---

### Phase 0 — Prérequis vérifiés

```markdown
## Retour intermédiaire vers orchestrator

**Agent :** debugger
**Phase :** 0 — Prérequis vérifiés
**task_id :** <sessionID courant>

## [Phase 0] Prérequis vérifiés

**Artefacts disponibles :**
- <artefact 1>
- <artefact 2>

**Artefacts manquants (si applicable) :**
- <artefact manquant> — impact : <conséquence>

**Ticket Beads lié (si fourni) :**
- bd-X : <titre> — <contexte extrait>

---

## Question pour l'orchestrator

**Phase :** 0
**task_id :** <sessionID courant>

**Contexte :** Prérequis vérifiés. Prêt à démarrer l'exploration contextuelle (Phase 1).

**Question :** Prérequis vérifiés. Démarrer l'exploration contextuelle (Phase 1) ?

**Options :**
- `demarrer` — Passer à la Phase 1 — Exploration contextuelle
- `preciser-contexte` — Ajouter des informations avant de démarrer
- `arreter` — Annuler le diagnostic

**Instruction de reprise :** "Réponse Phase 0 debugger : [option]. Reprendre depuis Phase 0 — validation prérequis."
```
→ **TERMINER LA SESSION**

---

### Phase 1 — Exploration contextuelle terminée

```markdown
## Retour intermédiaire vers orchestrator

**Agent :** debugger
**Phase :** 1 — Exploration contextuelle terminée
**task_id :** <sessionID courant>

## [Phase 1] Exploration contextuelle terminée

**Contexte projet :**
- CONVENTIONS.md : <lu / absent>
- Architecture détectée : <pattern observé>
- Patterns de gestion d'erreurs : <observés dans CONVENTIONS.md ou code>

**Ticket Beads :**
- bd-X : <titre> — comportement attendu : <résumé>
- (aucun si non fourni)

**Fichiers impliqués (préliminaire) :**
- `<fichier 1:ligne>` — <rôle supposé>
- `<fichier 2:ligne>` — <rôle supposé>

**Observations préliminaires :**
- <observation 1>
- <observation 2>

---

## Question pour l'orchestrator

**Phase :** 1
**task_id :** <sessionID courant>

**Contexte :** Exploration contextuelle terminée. Des questions complémentaires ont été identifiées ou non avant le diagnostic.

**Question :** Exploration terminée. Y a-t-il des questions complémentaires à poser avant le diagnostic (Phase 2) ?

**Options :**
- `passer-phase-2` — Pas de questions — démarrer le diagnostic
- `questions-a-poser` — Demander des précisions avant le diagnostic

**Instruction de reprise :** "Réponse Phase 1 debugger : [option]. Reprendre depuis Phase 1 — validation exploration."
```
→ **TERMINER LA SESSION**

---

### Phase 2 — Questions complémentaires

```markdown
## Retour intermédiaire vers orchestrator

**Agent :** debugger
**Phase :** 2 — Questions complémentaires
**task_id :** <sessionID courant>

## [Phase 2] Questions complémentaires

Quelques questions issues de l'exploration pour affiner le diagnostic :

1. **[Sujet 1]** : <question contextualisée issue de Phase 1>
2. **[Sujet 2]** : <question contextualisée issue de Phase 1>

---

## Question pour l'orchestrator

**Phase :** 2
**task_id :** <sessionID courant>

**Contexte :** Des questions de clarification ont émergé en Phase 1 et nécessitent une réponse avant le diagnostic.

**Question :** Quelques questions de clarification. Comment souhaitez-vous procéder ?

**Options :**
- `repondre-questions` — Fournir les réponses pour affiner le diagnostic
- `skip-passer` — Continuer sans répondre — le diagnostic restera partiel sur ces points

**Instruction de reprise :** "Réponse Phase 2 debugger : [option]. Reprendre depuis Phase 2 — questions de clarification."
```
→ **TERMINER LA SESSION**

---

### Phase 2 — Questions traitées

```markdown
## Retour intermédiaire vers orchestrator

**Agent :** debugger
**Phase :** 2 — Questions complémentaires traitées
**task_id :** <sessionID courant>

## [Phase 2] Questions complémentaires traitées

**Questions posées :** X questions

**Réponses reçues :**
- Q1 : <question> → <réponse ou "non répondu">
- Q2 : <question> → <réponse ou "non répondu">

**Zones d'ombre levées :**
- <zone 1 qui était floue et qui est maintenant claire>

**Zones d'ombre persistantes :**
- <zone 1 qui reste floue — impact sur le diagnostic>

---

## Question pour l'orchestrator

**Phase :** 2
**task_id :** <sessionID courant>

**Contexte :** Questions complémentaires traitées. Prêt à démarrer le diagnostic approfondi (Phase 3).

**Question :** Questions traitées. Passer au diagnostic approfondi (Phase 3) ?

**Options :**
- `passer-phase-3` — Démarrer le diagnostic en 4 étapes
- `revenir-phase-1` — Explorer à nouveau avec les nouvelles informations reçues

**Instruction de reprise :** "Réponse Phase 2 debugger : [option]. Reprendre depuis Phase 2 — validation fin questions."
```
→ **TERMINER LA SESSION**

---

### Phase 3 — Diagnostic approfondi terminé

```markdown
## Retour intermédiaire vers orchestrator

**Agent :** debugger
**Phase :** 3 — Diagnostic approfondi terminé
**task_id :** <sessionID courant>

## [Phase 3] Diagnostic approfondi terminé

### Symptôme
<Comportement observé vs attendu, conditions de déclenchement, fréquence>

### Périmètre analysé
<Artefacts fournis : stacktrace, logs, description, ticket Beads — et ce qui n'était PAS disponible>

### Localisation probable
`<chemin/vers/fichier.ts:ligne>` — <description courte>

### Cause racine

#### Hypothèse principale — <probabilité>
<Explication>

**Éléments qui l'étayent :**
- <extrait de stacktrace ou log>

**Pour confirmer :**
- <action concrète>

### Fichiers impliqués
| Fichier | Rôle dans le bug |
|---------|-----------------|
| `<fichier:ligne>` | <rôle> |

---

## Question pour l'orchestrator

**Phase :** 3
**task_id :** <sessionID courant>

**Contexte :** Diagnostic approfondi terminé. Hypothèse principale formulée.

**Question :** Diagnostic terminé. Passer à la détection des cas particuliers (Phase 4) ?

**Options :**
- `passer-phase-4` — Vérifier les cas particuliers avant de finaliser
- `reviser-diagnostic` — Rester en Phase 3 pour ajuster le diagnostic
- `skip-phase-4` — Passer directement à la production du rapport (Phase 5)

**Instruction de reprise :** "Réponse Phase 3 debugger : [option]. Reprendre depuis Phase 3 — validation diagnostic."
```
→ **TERMINER LA SESSION**

---

### Phase 4 — Cas particuliers terminés

```markdown
## Retour intermédiaire vers orchestrator

**Agent :** debugger
**Phase :** 4 — Détection des cas particuliers terminée
**task_id :** <sessionID courant>

## [Phase 4] Détection des cas particuliers terminée

**Cas particuliers vérifiés :** X vérifications

**Cas particuliers détectés :**
- <cas 1 — description + impact + recommandation>

**Cas particuliers écartés :**
- <cas 1 — raison de l'écarter>

**Impact sur le diagnostic :**
- <ajustement ou "aucun ajustement">

---

## Question pour l'orchestrator

**Phase :** 4
**task_id :** <sessionID courant>

**Contexte :** Détection des cas particuliers terminée. Prêt à produire le rapport de diagnostic final.

**Question :** Détection des cas particuliers terminée. Passer à la production du rapport (Phase 5) ?

**Options :**
- `produire-rapport` — Générer le rapport de diagnostic final + ticket Beads
- `verifier-autres-cas` — Rester en Phase 4 pour vérifier d'autres cas particuliers
- `revenir-phase-3` — Revoir le diagnostic après détection de cas particuliers critiques

**Instruction de reprise :** "Réponse Phase 4 debugger : [option]. Reprendre depuis Phase 4 — validation cas particuliers."
```
→ **TERMINER LA SESSION**

---

### Phase 5 — Confirmation ticket Beads (action irréversible)

```markdown
## Retour intermédiaire vers orchestrator

**Agent :** debugger
**Phase :** 5 — Création ticket Beads (action irréversible)
**task_id :** <sessionID courant>

## Ticket de correction suggéré

**Titre :** <titre>
**Type :** bug
**Priorité :** P<X>

**Description :**
<description complète>

**Critères d'acceptance :**
- <critère 1>
- <critère 2>

**Notes techniques :**
<cause racine, fichiers à modifier, points d'attention>

---

## Question pour l'orchestrator

**Phase :** 5
**task_id :** <sessionID courant>

**Contexte :** Rapport de diagnostic produit. Demande de confirmation avant création du ticket Beads (action irréversible).

**Question :** Créer ce ticket de correction dans Beads ?

**Options :**
- `oui-creer-ticket` — Créer le ticket avec bd create et enrichir description/acceptance/notes techniques
- `non` — Ne pas créer de ticket

**Instruction de reprise :** "Réponse Phase 5 debugger : [option]. Reprendre depuis Phase 5 — confirmation ticket Beads."
```
→ **TERMINER LA SESSION**

---

### Phase 5 — Retour final (rapport produit)

Phase 5 est le **retour final** — pas de question intermédiaire après la création du ticket. Produire dans cet ordre et terminer :

1. Le rapport de diagnostic complet (texte narratif — voir skill `debugger-workflow` ÉTAPE 5.1)
2. Le bloc `## Retour vers orchestrator` (voir skill `debugger-handoff-format`)

```markdown
---

## Retour vers orchestrator

**Agent :** debugger
**Problème :** <description courte du bug>

### Cause racine
<tel que défini dans debugger-handoff-format>

### Hypothèses explorées
<tel que défini dans debugger-handoff-format>

### Impact et régressions potentielles
<tel que défini dans debugger-handoff-format>

### Tickets de correction créés
<tel que défini dans debugger-handoff-format>

### Actions d'urgence si bug en prod
<tel que défini dans debugger-handoff-format>

### Statut
`diagnostiqué` | `partiellement-diagnostiqué` | `non-reproductible`
```

> **Autocontrôle avant le bloc final :**
> « Ai-je produit le rapport de diagnostic complet avant ce bloc ? Si non → le produire d'abord. »

→ **TERMINER LA SESSION**

---

### Phase 5 — Validation finale (après création ticket)

```markdown
## Retour intermédiaire vers orchestrator

**Agent :** debugger
**Phase :** 5 — Rapport produit
**task_id :** <sessionID courant>

## [Phase 5] Rapport de diagnostic produit

**Rapport :**
- Symptôme : <résumé>
- Localisation : `<fichier:ligne>`
- Hypothèse principale : <probabilité> — <résumé>
- Fichiers impliqués : X fichiers

**Ticket Beads :**
- ✅ bd-X créé : <titre> — P<X> — label `from-diagnostic`
- ❌ Non créé (refus)

---

## Question pour l'orchestrator

**Phase :** 5
**task_id :** <sessionID courant>

**Contexte :** Diagnostic complet. Rapport produit et ticket Beads traité.

**Question :** Diagnostic terminé. Besoin d'ajustements ?

**Options :**
- `terminer` — Diagnostic complet
- `ajustements` — Revenir à une phase pour ajuster

**Instruction de reprise :** "Réponse Phase 5 debugger : [option]. Reprendre depuis Phase 5 — validation finale."
```
→ **TERMINER LA SESSION**

---

## ❌ Erreurs fréquentes à éviter

| Erreur | Impact | Correction |
|--------|--------|------------|
| Appeler l'outil `question` | Question posée en session enfant — invisible pour l'agent orchestrator | **Terminer la session** avec les blocs structurés |
| Continuer vers la phase suivante sans produire les blocs | L'orchestrateur ne reçoit rien avant la fin complète | **Toujours interrompre** à chaque fin de phase |
| Omettre le `task_id` dans les blocs | L'orchestrateur ne peut pas re-invoquer pour reprendre | **Toujours inclure** le sessionID |
| Résumer le récap dans le bloc intermédiaire | L'utilisateur perd des informations critiques | **Ne jamais résumer** — contenu complet |
| Produire le bloc handoff sans le rapport narratif | L'orchestrateur reçoit un résumé sans les preuves | **Toujours produire le rapport d'abord** |
| Pause ad hoc pour des détails mineurs | Trop de re-invocations, flux dégradé | **Réserver aux vrais blockers** |
