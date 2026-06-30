---
name: planner-design-templates
description: Templates de délégation design Phase 1.5 — options A/B/C pour UX et UI, contextes à transmettre aux sous-agents, instructions de reprise après spec reçue. Chargé à la demande quand des signaux design sont détectés en Phase 1.
bucket: B
---

# Skill — Planner : Templates de délégation design (Phase 1.5)

## Contexte d'usage

Ce skill est chargé par `planner-workflow` en Phase 1.5 quand un signal UX ou UI
a été détecté en Phase 1 (exploration contextuelle).

Il fournit :
- Les templates de présentation (option A/B/C) pour UX et UI
- Les contextes à transmettre aux sous-agents
- Les instructions de reprise après réception de la spec

---

## Phase 1.5 — Délégation design (optionnelle)

**Déclenchée si :** signal UX ou UI détecté en Phase 1.

Cette phase se place **avant** Phase 2 car les specs UX/UI influencent directement le découpage en tickets.
Elle se traite en sessions séparées — le planner ne continue pas tant que l'utilisateur n'a pas rapporté les specs (ou explicitement décidé de les ignorer).

---

### Délégation UX

**Condition** : signal UX détecté (parcours multi-étapes, changement d'interaction, formulaire complexe, flow critique).

Présenter le message suivant :

```markdown
## ⚠️ Spec UX recommandée avant planification

Cette feature [modifie le parcours de sélection / introduit un flow multi-étapes / change une interaction existante].
Planifier sans spec UX risque de découper les tickets selon la logique technique
plutôt que selon la logique utilisateur.

Je recommande d'invoquer l'UX Designer en premier pour :
- Modéliser le user flow (nominal + alternatifs + états d'erreur)
- Identifier les frictions et les cas limites du parcours
- Produire des critères d'acceptance orientés utilisateur

Ces éléments alimenteront directement le découpage en tickets et leurs critères d'acceptance.

### Comment souhaitez-vous procéder ?

**Option A — Je l'invoque directement** *(recommandé)*
> Tapez "invoquer UX" — j'invoque l'agent **designer** en sous-agent maintenant,
> avec le contexte complet de la feature, et j'intègre sa spec dès qu'il a terminé.

**Option B — Vous l'invoquez vous-même**
> Ouvrez une session avec l'agent **designer** et donnez-lui ce contexte :
> ---
> Feature : [nom de la feature]
> Contexte métier : [résumé du besoin collecté]
> Utilisateurs concernés : [rôles / personas identifiés]
> Interaction à analyser : [description précise du parcours ou de l'écran concerné]
> Tickets existants liés : [IDs si applicable]
> ---
> Demandez : "Spec UX pour [nom de la feature]"
> Puis revenez ici en disant : "Voici la spec UX — continue la planification avec ce contexte."

**Option C — Continuer sans spec UX**
> Tapez "continuer sans UX" — je procéderai avec le contexte disponible
> et signalerai les critères d'acceptance UX à compléter ticket par ticket.
```

**Si l'utilisateur choisit l'Option A ("invoquer UX") :**

Annoncer puis invoquer directement :
> "J'invoque l'agent **designer** avec le contexte de la feature."

Transmettre au sous-agent :
```
Feature : [nom de la feature]
Contexte métier : [résumé du besoin collecté en Phase 1]
Utilisateurs concernés : [rôles / personas identifiés]
Interaction à analyser : [description précise du parcours ou de l'écran concerné]
Tickets existants liés : [IDs si applicable]

Demande : Spec UX pour [nom de la feature]
```

Attendre la réponse de **designer** au format `## SPEC UX — [feature]` puis reprendre directement avec la section "Reprise après spec UX" ci-dessous.

**Reprise après spec UX** — quand l'utilisateur rapporte la spec UX :

1. Lire le user flow nominal et les flows alternatifs
2. En déduire les tickets supplémentaires si des étapes ou cas d'erreur non prévus apparaissent
3. Intégrer les critères d'acceptance UX dans la section `## Comportement fonctionnel` des tickets concernés
4. Mentionner dans les notes des tickets : `User flow : [résumé du flow nominal en 1-2 phrases]`
5. Annoncer : "J'ai intégré la spec UX. Je continue vers Phase 2."

---

### Délégation UI

**Condition** : signal UI détecté (nouveau composant, composant profondément modifié, variantes à spécifier).

Présenter le message suivant **en même temps que la délégation UX** si les deux sont nécessaires, ou seul sinon :

```markdown
## ⚠️ Spec UI recommandée avant planification

Cette feature [crée un nouveau composant / modifie profondément [NomComposant] / nécessite des variantes visuelles].
Sans spec UI, le champ `--design` des tickets sera incomplet et le développeur frontend
devra prendre seul les décisions visuelles (composants DSFR, états, accessibilité).

Je recommande d'invoquer l'UI Designer pour chaque composant concerné :
- Identifier les composants DSFR à utiliser (et leurs variantes)
- Spécifier les états visuels (default, hover, focus, disabled, error, loading)
- Définir les règles d'accessibilité (ARIA, contraste, navigation clavier)

### Comment souhaitez-vous procéder ?

**Option A — Je l'invoque directement** *(recommandé)*
> Tapez "invoquer UI" — j'invoque l'agent **designer** en sous-agent maintenant,
> composant par composant, et j'intègre ses specs dès qu'il a terminé.

**Option B — Vous l'invoquez vous-même**
> Pour chaque composant concerné, ouvrez une session avec l'agent **designer**
> et donnez-lui ce contexte :
> ---
> Composant : [NomDuComposant.vue]
> Feature : [nom de la feature]
> Comportement attendu : [description fonctionnelle du composant]
> Design system en place : [DSFR / autre — préciser si connu]
> Spec UX associée : [coller le user flow si déjà produit]
> ---
> Demandez : "Spec UI pour [NomComposant]"
> Puis revenez ici en disant : "Voici la spec UI pour [composant] — continue la planification avec ce contexte."

**Option C — Continuer sans spec UI**
> Tapez "continuer sans UI" — je remplirai le champ `--design` avec le contexte disponible
> et ajouterai un commentaire `bd comments add` sur chaque ticket concerné
> avec les instructions pour invoquer l'UI Designer ultérieurement.
```

**Si l'utilisateur choisit l'Option A ("invoquer UI") :**

Annoncer puis invoquer directement, composant par composant :
> "J'invoque l'agent **designer** pour [NomComposant]."

Transmettre au sous-agent pour chaque composant :
```
Composant : [NomDuComposant.vue]
Feature : [nom de la feature]
Comportement attendu : [description fonctionnelle du composant]
Design system en place : [DSFR / autre]
Spec UX associée : [user flow si déjà produit]

Demande : Spec UI pour [NomComposant]
```

Attendre la réponse de **designer** au format `## SPEC UI — [NomComposant]` puis reprendre directement avec la section "Reprise après spec UI" ci-dessous.

**Reprise après spec UI** — quand l'utilisateur rapporte la spec UI :

1. Identifier le(s) ticket(s) concerné(s) par cette spec
2. Intégrer la spec dans le template `--design` du/des ticket(s) concerné(s)
3. Compléter l'acceptance avec les critères visuels issus de la spec (états, contrastes, ARIA)
4. Annoncer : "J'ai intégré la spec UI pour [composant]. Je continue vers Phase 2."

---

### Si "continuer sans UX/UI"

Appliquer la stratégie de traçabilité en Phase 5 : pour chaque ticket concerné, ajouter un `bd comments add` avec les instructions d'invocation précises (voir Phase 5 — Tickets sans spec design).

---

### Récap de fin de Phase 1.5

```markdown
## [Phase 1.5] Délégation design terminée

**Specs UX produites :**
- <feature ou parcours concerné> — spec reçue de designer
- (aucune si skip)

**Specs UI produites :**
- <composant 1> — spec reçue de designer
- <composant 2> — spec reçue de designer
- (aucune si skip)

**Intégration dans la planification :**
- <élément 1 — ex : ajout de 2 tickets pour gérer les états d'erreur identifiés dans la spec UX>
- <élément 2 — ex : champ --design des tickets frontend pré-rempli avec la spec UI>

**Specs manquantes (si skip) :**
- <composant ou parcours> — sera tracé via bd comments add en Phase 5
```

### Question de validation obligatoire

⚠️ **AUTOCONTRÔLE** : Le récap Phase 1.5 (ci-dessus — specs UX/UI reçues ou skippées, intégration dans la planification) **doit être affiché en texte** avant ce checkpoint. Si ce n'est pas fait → produire le récap MAINTENANT.

**Si CONTEXTE = standalone :**
```
question({
  questions: [{
    header: "Questions complémentaires",
    question: "[Planner — Phase 1.5 complétée | Feature : <nom>]\nSpecs design intégrées. Passer aux questions complémentaires (Phase 2) ?",
    options: [
      { label: "Passer à Phase 2 (Recommandé)", description: "Poser les questions de clarification identifiées" },
      { label: "Revenir à Phase 1", description: "Explorer à nouveau avec les specs design reçues" }
    ]
  }]
})
```

**Si CONTEXTE = orchestrator_feature :**
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

**Selon la réponse (dans tous les contextes) :**
- **Phase 2** → Phase 2 (questions complémentaires)
- **Revenir à Phase 1** → Phase 1 (les specs modifient le périmètre d'exploration)
