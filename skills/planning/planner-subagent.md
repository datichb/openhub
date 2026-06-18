---
name: planner-subagent
description: Parcours d'exécution du planner en mode sous-agent (invoqué via task depuis l'agent orchestrator feature) — mécanisme d'interruption de session, blocs structurés Retour intermédiaire + Question pour l'agent orchestrator, task_id obligatoire. Ne jamais appeler l'outil question dans ce mode.
---

# Skill — Parcours Planner Sous-agent

> Ce skill est chargé quand le planner est invoqué via `task` depuis l'agent orchestrator feature. L'orchestrateur injecte `[SKILL:planning/planner-subagent]` dans le prompt.

## Principe fondamental

Quand le planner est invoqué via `task`, le texte de la session enfant n'est **PAS visible** par l'utilisateur dans la session parent. La seule façon de remonter du contenu est de **terminer la session** avec les blocs structurés, que l'agent orchestrator retranscrira.

**Confirmer le contexte au démarrage :**
> `[planner] Contexte détecté : invoqué depuis l'agent orchestrator feature. Mode interruption actif — je terminerai ma session à chaque checkpoint pour remonter le récap et la question à l'agent orchestrator.`

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
- Re-invoque le planner avec `task_id` + la réponse → le planner recharge l'historique et continue

---

## Autocontrôle avant chaque fin de session

> « Ai-je produit (1) le récap de la phase, (2) le bloc `## Retour intermédiaire vers orchestrator`, ET (3) le bloc `## Question pour l'orchestrator` ? »
> - **Non** → produire les blocs manquants MAINTENANT
> - **Oui** → terminer la session

> ⚠️ **RAPPEL CRITIQUE** : Le récap Phase 6 (contexte = orchestrator_feature) doit contenir le **contexte et le raisonnement** derrière les décisions de planification — pourquoi ces tickets, pourquoi cet ordre, quelles hypothèses, quels risques. Il n'a **pas** à reproduire le tableau des tickets ni les listes formelles — ceux-ci sont dans le bloc structuré `## Retour vers orchestrator`. L'orchestrateur retransmettra ce récap narratif intégralement à l'utilisateur pour le CP-0.

---

## ✅ Checklist visuelle — AVANT CHAQUE FIN DE SESSION

**STOP — Vérifier MAINTENANT :**

| Vérification | Fait ? |
|--------------|--------|
| ✅ J'ai produit le récap complet de la phase en texte | ⬜ |
| ✅ J'ai produit le bloc `## Retour intermédiaire vers orchestrator` avec la synthèse condensée (résumé + points clés) | ⬜ |
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

**Agent :** planner
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
- `<label-option-c>` — <description si applicable>

**Instruction de reprise :** "Réponse au checkpoint Phase X : [option choisie]. Reprendre depuis <contexte précis>."
```
→ **TERMINER LA SESSION**

---

### Bloc pause ad hoc (information manquante critique)

> ⚠️ Réserver aux vrais blockers — pas aux détails. Si une hypothèse documentée permet de continuer, continuer.

```markdown
## ⏸️ Pause — Phase X — <sujet de la pause>

Pendant l'exploration de [fichier/module/contexte], j'ai détecté que [description précise du problème].

**Impact :** Sans cette information, [conséquence concrète sur la planification].

**Hypothèse possible :** [formulation de l'hypothèse si l'utilisateur souhaite continuer sans info]

---

## Retour intermédiaire vers orchestrator

**Agent :** planner
**Phase :** X — Pause (information manquante critique)
**task_id :** <sessionID courant>

**Résumé :** <description en 1-2 phrases du problème détecté>
**Impact :** <conséquence concrète sur la planification si non résolu>

---

## Question pour l'orchestrator

**Phase :** X — Pause
**task_id :** <sessionID courant>

**Contexte :** <description du problème détecté et de son impact>

**Question :** <question précise>

**Options :**
- `fournir-information` — Fournir l'information maintenant
- `continuer-hypothese` — Continuer avec l'hypothèse : [formulation]

**Instruction de reprise :** "Réponse à la pause Phase X : [option]. [Information fournie si applicable]. Reprendre depuis le point d'interruption."
```
→ **TERMINER LA SESSION**

---

## Questions par phase

### Phase 0 — Prérequis vérifiés

```markdown
## Retour intermédiaire vers orchestrator

**Agent :** planner
**Phase :** 0 — Prérequis vérifiés
**task_id :** <sessionID courant>

**Résumé :** Prérequis vérifiés — feature identifiée, board Beads localisé.
**Points clés :** <prérequis manquants et hypothèses formulées, ou "Aucun prérequis manquant">

---

## Question pour l'orchestrator

**Phase :** 0
**task_id :** <sessionID courant>

**Contexte :** Les prérequis pour la planification ont été vérifiés.

**Question :** Démarrer l'exploration contextuelle (Phase 1) ?

**Options :**
- `demarrer` — Démarrer la Phase 1 — Exploration contextuelle
- `preciser` — Préciser le contexte avant de démarrer
- `arreter` — Annuler l'analyse

**Instruction de reprise :** "Réponse Phase 0 : [option]. Reprendre depuis Phase 1 (exploration)."
```
→ **TERMINER LA SESSION**

### Phase 1 — Signal design détecté

```markdown
## Retour intermédiaire vers orchestrator

**Agent :** planner
**Phase :** 1 — Exploration contextuelle (signal design détecté)
**task_id :** <sessionID courant>

**Résumé :** Exploration contextuelle terminée — signal <UX/UI> détecté : <raison concrète>.
**Points clés :** architecture <pattern détecté>, tests <état>, signal design <description>, zones d'ombre <liste courte ou "aucune">

---

## Question pour l'orchestrator

**Phase :** 1
**task_id :** <sessionID courant>

**Contexte :** L'exploration a détecté un signal <UX/UI> : <raison concrète>. Une délégation design avant la planification est recommandée.

**Question :** Comment procéder après l'exploration Phase 1 ?

**Options :**
- `phase-1-5-design` — Phase 1.5 : déléguer au designer avant de planifier (recommandé)
- `skip-design-phase-2` — Passer directement aux questions (Phase 2) sans spec design
- `explorer-davantage` — Explorer d'autres fichiers avant de décider

**Instruction de reprise :** "Réponse Phase 1 : [option]. Reprendre depuis <Phase 1.5 / Phase 2 / exploration complémentaire>."
```
→ **TERMINER LA SESSION**

### Phase 1 — Aucun signal design

```markdown
## Retour intermédiaire vers orchestrator

**Agent :** planner
**Phase :** 1 — Exploration contextuelle (terminée)
**task_id :** <sessionID courant>

**Résumé :** Exploration contextuelle terminée — aucun signal design détecté.
**Points clés :** architecture <pattern détecté>, tests <état>, zones d'ombre <liste courte ou "aucune">, tickets existants liés <IDs ou "aucun">

---

## Question pour l'orchestrator

**Phase :** 1
**task_id :** <sessionID courant>

**Contexte :** L'exploration est terminée. Aucun signal design détecté. Zones d'ombre : <liste courte ou 'aucune'>.

**Question :** Passer aux questions complémentaires (Phase 2) ?

**Options :**
- `phase-2` — Passer à Phase 2 (recommandé)
- `explorer-davantage` — Explorer d'autres fichiers avant de poser des questions

**Instruction de reprise :** "Réponse Phase 1 : [option]. Reprendre depuis Phase 2 / exploration complémentaire."
```
→ **TERMINER LA SESSION**

### Phase 1.5 — Délégation design terminée

```markdown
## Retour intermédiaire vers orchestrator

**Agent :** planner
**Phase :** 1.5 — Délégation design (terminée)
**task_id :** <sessionID courant>

**Résumé :** Délégation design terminée — specs <UX/UI> <reçues et intégrées | skippées>.
**Points clés :** <specs reçues (composants/parcours concernés) ou "aucune spec — tracé via bd comments add en Phase 5">

---

## Question pour l'orchestrator

**Phase :** 1.5
**task_id :** <sessionID courant>

**Contexte :** La phase de délégation design est terminée. Specs intégrées / skippées.

**Question :** Passer aux questions complémentaires (Phase 2) ?

**Options :**
- `phase-2` — Passer à Phase 2 (recommandé)
- `retour-phase-1` — Revenir à Phase 1 pour re-explorer avec les specs design

**Instruction de reprise :** "Réponse Phase 1.5 : [option]. Reprendre depuis Phase 2 / Phase 1."
```
→ **TERMINER LA SESSION**

### Phase 2 — Questions complémentaires traitées

```markdown
## Retour intermédiaire vers orchestrator

**Agent :** planner
**Phase :** 2 — Questions complémentaires (traitées)
**task_id :** <sessionID courant>

**Résumé :** Questions complémentaires traitées — <N> questions posées, réponses intégrées.
**Points clés :** <décisions clés issues des réponses, hypothèses confirmées ou levées, zones d'ombre restantes>

---

## Question pour l'orchestrator

**Phase :** 2
**task_id :** <sessionID courant>

**Contexte :** Les questions complémentaires ont été traitées. Zones d'ombre levées : <liste>. Persistantes : <liste ou aucune>.

**Question :** Passer à l'analyse approfondie (Phase 3 — Plan hiérarchique) ?

**Options :**
- `phase-3` — Passer à Phase 3 (recommandé)
- `autres-questions` — Poser d'autres questions de clarification
- `retour-phase-1` — Revenir à Phase 1 avec les nouvelles informations

**Instruction de reprise :** "Réponse Phase 2 : [option]. Reprendre depuis Phase 3 / Phase 2 / Phase 1."
```
→ **TERMINER LA SESSION**

### Phase 3 — Plan hiérarchique

```markdown
## Retour intermédiaire vers orchestrator

**Agent :** planner
**Phase :** 3 — Plan hiérarchique
**task_id :** <sessionID courant>

**Résumé :** Plan hiérarchique produit — <N> epics, <M> tickets créés, ordre de traitement défini.
**Points clés :** <dépendances critiques, risques identifiés, hypothèses structurantes>

---

## Question pour l'orchestrator

**Phase :** 3
**task_id :** <sessionID courant>

**Contexte :** Le plan hiérarchique est prêt. N epics, M tickets. Estimation totale : ~Xh.

**Question :** Ce découpage convient-il ? Souhaitez-vous modifier des éléments avant la création des tickets ?

**Options :**
- `valider-plan` — Valider et passer à Phase 4 (détection cas particuliers) (recommandé)
- `modifier-plan` — Modifier le découpage avant de continuer
- `retour-phase-2` — Revenir aux questions complémentaires

**Instruction de reprise :** "Réponse Phase 3 : [option]. [Modifications souhaitées si applicable]. Reprendre depuis Phase 4 / Phase 3 / Phase 2."
```
→ **TERMINER LA SESSION**

### Phase 4 — Cas particuliers

```markdown
## Retour intermédiaire vers orchestrator

**Agent :** planner
**Phase :** 4 — Détection des cas particuliers (terminée)
**task_id :** <sessionID courant>

**Résumé :** Détection des cas particuliers terminée — <N> cas détectés, <M> écartés.
**Points clés :** <cas retenus et leur impact sur le plan, ou "Aucun cas particulier détecté">

---

## Question pour l'orchestrator

**Phase :** 4
**task_id :** <sessionID courant>

**Contexte :** Détection des cas particuliers terminée. Cas détectés : <liste ou aucun>. Ajustements au plan : <liste ou aucun>.

**Question :** Passer à la création des tickets dans Beads (Phase 5) ?

**Options :**
- `phase-5` — Créer les tickets (recommandé)
- `verifier-autres-cas` — Vérifier d'autres cas particuliers
- `retour-phase-3` — Revoir le plan

**Instruction de reprise :** "Réponse Phase 4 : [option]. Reprendre depuis Phase 5 / Phase 4 / Phase 3."
```
→ **TERMINER LA SESSION**

### Phase 5.5 — Délégation ai-delegated

```markdown
## Retour intermédiaire vers orchestrator

**Agent :** planner
**Phase :** 5.5 — Délégation ai-delegated
**task_id :** <sessionID courant>

**Résumé :** <N> tickets créés — <Y> tickets éligibles à délégation ai-delegated.
**Points clés :** <IDs des tickets éligibles et pourquoi, ou "Aucun ticket éligible">

---

## Question pour l'orchestrator

**Phase :** 5.5
**task_id :** <sessionID courant>

**Contexte :** N tickets créés. Y tickets sont éligibles à la délégation ai-delegated.

**Question :** Souhaitez-vous déléguer certains tickets à l'agent IA ?

**Options :**
- `non` — Aucun ticket délégué
- `certains` — Indiquer les IDs dans la réponse
- `tous-eligibles` — Déléguer tous les tickets éligibles

**Instruction de reprise :** "Réponse Phase 5.5 : [option]. [IDs si applicable]. Reprendre depuis Phase 6."
```
→ **TERMINER LA SESSION**

### Phase 6 — Retour final

Phase 6 est le **retour final** — pas de question intermédiaire. Produire dans cet ordre et terminer :

1. Le récapitulatif de planification complet (liste narrative détaillée de tous les tickets avec descriptions + acceptance + notes + dépendances + risques + hypothèses)
2. Le bloc `## Retour vers orchestrator` (voir skill `planner-handoff-format`)

```markdown
---

## Retour vers orchestrator

**Agent :** planner
**Feature :** <nom>

### Tickets créés
<tableau structuré tel que défini dans planner-handoff-format>

### Dépendances
<dépendances structurées>

### Ordre de traitement
<séquence d'exécution — l'agent orchestrator la suit sans interprétation>

### Hypothèses et ambiguïtés
<hypothèses structurées>

### Risques identifiés
<risques structurés>

### Statut
`planification-complète` | `planification-partielle` | `bloqué`
```

> **Autocontrôle avant le bloc final :**
> « Ai-je produit le récapitulatif narratif complet avant ce bloc ? Si non → le produire d'abord. »
> « Ce récap contient-il la liste détaillée de TOUS les tickets (descriptions + acceptance + notes) ? Si non → le compléter. »

→ **TERMINER LA SESSION**

---

## ❌ Erreurs fréquentes à éviter

| Erreur | Impact | Correction |
|--------|--------|------------|
| Appeler l'outil `question` | Question posée en session enfant — invisible pour l'agent orchestrator | **Terminer la session** avec les blocs structurés |
| Continuer vers la phase suivante sans produire les blocs | L'orchestrateur ne reçoit rien avant la fin complète | **Toujours interrompre** à chaque fin de phase |
| Omettre le `task_id` dans les blocs | L'orchestrateur ne peut pas re-invoquer pour reprendre | **Toujours inclure** le sessionID |
| Résumer le récap dans le bloc intermédiaire | L'utilisateur perd des informations critiques | **Ne jamais résumer** — copier intégralement |
| Pause ad hoc pour des détails mineurs | Trop de re-invocations, flux dégradé | **Réserver aux vrais blockers** |
