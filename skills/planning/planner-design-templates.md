---
name: planner-design-templates
description: "Utiliser quand le planner détecte des signaux design en phase 1 : délégation au designer, prototype, reprise des specs."
---

# Skill — Planner : Délégation design (Phase 1.5)

## Phase 1.5 — Délégation design (optionnelle)

**Déclenchée si :** signal UX ou UI détecté en Phase 1.

Cette phase se place **avant** Phase 2 car les specs UX/UI influencent directement le découpage en tickets.
<!-- oh:when role=entry -->
Elle se traite en sessions séparées — le planner ne continue pas tant que l'utilisateur n'a pas rapporté les specs (ou explicitement décidé de les ignorer).

### Proposer la délégation

Pour chaque spec recommandée (UX : parcours multi-étapes, changement d'interaction, formulaire complexe, flow critique ; UI : nouveau composant, composant profondément modifié, variantes à spécifier ; les deux dans le même message si besoin), présente en texte :

```markdown
## ⚠️ Spec <UX|UI> recommandée avant planification

Cette feature [raison concrète]. Planifier sans spec <UX|UI> risque [tickets découpés selon la logique technique plutôt qu'utilisateur | champ `--design` incomplet, décisions visuelles laissées au développeur].

**Option A — Je l'invoque directement** *(recommandé)* : tapez "invoquer <UX|UI>" ; j'invoque l'agent **designer** maintenant (UI : composant par composant) et j'intègre sa spec.
**Option B — Vous l'invoquez vous-même** : ouvrez une session avec l'agent **designer** et le contexte ci-dessous, demandez "Spec <UX|UI> pour [feature | composant]", puis revenez avec la spec.
> ---
> [contexte du prompt de délégation ci-dessous]
> ---
**Option C — Continuer sans spec** : tapez "continuer sans <UX|UI>" ; les critères UX ou le champ `--design` seront tracés ticket par ticket en Phase 5.
```
<!-- /oh:when -->
<!-- oh:when role=subagent -->
Sous-agent : n'expose pas d'options ; invoque directement le designer (une spec UX pour le parcours, une spec UI par composant) et intègre ses specs.
<!-- /oh:when -->

## Prompt de délégation au designer

Déléguer seulement avec au moins un signal design ; mode `ux` (parcours, états, accessibilité structurelle), `ui` (tokens, composants, cohérence visuelle), `ux+ui` (les deux, UX d'abord) ou `recon` (exploration Figma légère). Les questions de phase 2 attendent le retour du designer : sa spec peut les changer.

```
Agent : designer
Mode: <recon | ux | ui | ux+ui>
Objectif : <ce que le designer doit produire, en 1 ou 2 phrases>

### Feature demandée
<la demande de l'utilisateur, telle quelle>
### Contexte projet
<stack, conventions front, frameworks UI, design system existant (phase 1)>
### Composants existants identifiés
<UI : le composant visé (NomDuComposant.vue) et son comportement attendu>
### Signaux design détectés
<UX : utilisateurs concernés (rôles, personas), interaction ou parcours à analyser, tickets existants liés>
### Contraintes techniques anticipées
### Questions ouvertes
<à trancher par le design ou à clarifier avec l'utilisateur>

Livrables attendus : <user flow, wireframes textuels, tokens, composants, critères d'acceptance>
```

### Réception du retour du designer

Son enveloppe porte un statut (`spec-complète`, `spec-partielle`, `choix-requis`, `prototype-livré`, `bloqué`) et l'identifiant de son rapport. Affiche la spec complète (lue avec `workflow_report_get`, jamais résumée) avant toute question ; reprends `### Composants à créer` et `### Tokens design requis` dans les questions de phase 2 si une validation est utile, et ajoute ses `### Questions pour l'utilisateur` à celles de la phase 2. `spec-partielle` : les questions design rejoignent le questionnaire ; `bloqué` : demande à l'utilisateur comment débloquer avant de continuer. Sans spec (enveloppe seule, sans rapport) : demande au designer de la produire.

### Reprise après la spec

- **UX** : lire le flow nominal et les flows alternatifs ; ajouter les tickets qu'imposent des étapes ou cas d'erreur non prévus ; intégrer les critères UX dans `## Comportement fonctionnel` des tickets concernés ; noter dans leurs notes `User flow : [flow nominal en 1-2 phrases]`.
- **UI** : identifier les tickets concernés ; intégrer la spec dans leur `--design` ; compléter leur acceptance avec les critères visuels (états, contrastes, ARIA).

Sans spec (« continuer sans UX/UI ») : en Phase 5, chaque ticket concerné reçoit un `bd comments add` avec les instructions d'invocation (template « spec UI non disponible » de `planner-phase-5-6`).

---

### Prototype rapide (signal ambigu)

Si l'utilisateur choisit **"Prototype rapide"** dans la question de délégation Phase 1 :

1. **Identifier la question précise** que le prototype doit résoudre (ex : "Comment organiser les 3 zones d'information sur l'écran principal ?")

2. **Invoquer le designer** avec le skill prototype injecté :
   ```
   task(
     subagent_type: "designer",
     prompt: "Mode: ux [SKILL:designer/prototype-protocol] [QUESTION: <question précise>]\n\nFeature : [nom]\nContexte : [résumé du besoin]\nSignal ambigu détecté : [description du signal]"
   )
   ```

3. **À la réception du retour prototype** (`### Réponse à la question`, `### Recommandation`) : `spec-complete-necessaire` → délégation design complète ; `resolu` → réponse intégrée au contexte de la phase 2 ; `clarification` → nouvelle question en phase 2.

---

### Récap de fin de Phase 1.5

```markdown
## [Phase 1.5] Délégation design terminée

**Specs UX produites :** <parcours> — spec reçue de designer (aucune si skip)
**Specs UI produites :** <composant> — spec reçue de designer (aucune si skip)
**Prototypes produits :** <question> — réponse : <résumé 1 ligne> (recommandation : <suite>)
**Intégration dans la planification :** <ex : 2 tickets ajoutés pour les états d'erreur de la spec UX>
**Specs manquantes (si skip) :** <composant ou parcours> — tracé via bd comments add en Phase 5
```

### Question de validation obligatoire
<!-- oh:when role=entry -->

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

**Selon la réponse :** Phase 2 ; revenir à Phase 1 (les specs modifient le périmètre d'exploration).
<!-- /oh:when -->
<!-- oh:when role=subagent -->

**En mode sous-agent :** pas de validation de phase : intègre les specs reçues (ou leur absence) dans ton rapport et enchaîne sur la phase 2 (protocole d'exécution).
<!-- /oh:when -->
