---
name: design-planner-format
description: Source de vérité pour le format de handoff planner → designer. Définit le contexte obligatoire à transmettre lors de la délégation design en Phase 1.5. Injecté dans planner et designer pour garantir que le producteur et les consommateurs partagent le même contrat.
annexes: [templates/design-planner-prompts.md]
---

# Skill — Format de handoff planner → design

Ce skill est la **source de vérité** pour le format de délégation du `planner` vers l'agent `designer`.
Il est injecté dans `planner` et `designer` — producteur et consommateur partagent le même contrat.

---

## Quand produire ce handoff

Le `planner` délègue à l'agent `designer` en **Phase 1.5** (après Phase 1 d'exploration, avant Phase 2 de questions) si **au moins un signal design** a été identifié lors de l'exploration contexte.

**Signaux déclencheurs (Phase 1.5 — ligne 93 du workflow planner) :**
- Mention de "interface", "UX", "UI", "wireframe", "maquette", "design system", "composant réutilisable"
- Mention de "parcours utilisateur", "user flow", "accessibilité", "responsive"
- Absence de composants existants réutilisables (nécessite création de nouveaux composants)
- Feature traversant plusieurs écrans avec interactions complexes

**Choix du mode :**
- **Mode `ux`** : parcours utilisateur, user flows, architecture de l'information, états d'interface, accessibilité structurelle
- **Mode `ui`** : tokens, design system, composants réutilisables, cohérence visuelle, accessibilité visuelle (contraste, taille)
- **Mode `ux+ui`** : les deux dimensions sont présentes → commencer par UX
- **Mode `recon`** : exploration Figma légère avant de décider (ex : Phase 1.3)

> Si les deux dimensions sont présentes → mode `ux+ui`.
> Si seul le visuel est concerné (ex : nouveau bouton, palette de couleurs) → mode `ui` directement.

---

## Format du prompt de délégation planner → designer

> **Template :** format défini dans `templates/design-planner-prompts.md` (section « Format du prompt de délégation planner → designer ») — charger via `read` quand tu produis ce bloc.

---

## Format du bloc `## Retour vers planner`

**Produit par l'agent designer à la fin de sa spec :**

> **Template :** format défini dans `templates/design-planner-prompts.md` (section « Format du bloc Retour vers planner ») — charger via `read` quand tu produis ce bloc.

**Définitions du statut :**

| Statut | Condition |
|--------|-----------|
| `spec-complète` | Spec validée, tous les éléments design nécessaires sont présents |
| `spec-partielle` | Spec validée mais avec des questions ouvertes pour l'utilisateur (à poser par le planner en Phase 2) |
| `bloqué` | Spec non finalisée — un blocage empêche de produire une spec exploitable |

---

## Règles pour le producteur (planner)

- **Toujours vérifier les signaux design** avant de déléguer (ligne 93 du workflow planner)
- **Choisir le bon mode** : ux (flows, états) vs ui (tokens, composants) vs ux+ui vs recon
- **Ne jamais déléguer si aucun signal design** n'a été identifié — le planner continue en Phase 2 directement
- **Transmettre tout le contexte** identifié en Phase 1 — composants existants, stack technique, conventions front
- **Ne jamais poser les questions de Phase 2** avant d'avoir reçu le retour design — le design peut modifier les questions à poser

---

## Règles pour le consommateur (designer)

- **Toujours lire le contexte complet** avant de commencer la spec — composants existants, contraintes techniques, stack front
- **Réutiliser l'existant d'abord** — ne créer de nouveaux composants que si nécessaire
- **Produire la spec complète** — user flows intégraux, wireframes textuels, tokens, composants, critères d'acceptance
- **Toujours produire le bloc `## Retour vers planner`** à la suite de la spec, même si le statut est `bloqué`
- **Ne jamais résumer la spec** dans le bloc handoff — le bloc est une synthèse de métadonnées, pas un substitut
- **Identifier les questions pour l'utilisateur** — ce que le planner devra poser en Phase 2 avant l'implémentation

> ❌ Ne jamais produire le bloc handoff sans avoir d'abord produit la spec complète.
> ❌ Ne jamais résumer la spec — le bloc est une synthèse de métadonnées, pas un substitut à la spec.

---

## Règles pour le consommateur final (planner après réception du retour design)

### À la réception du retour de l'agent designer

1. **Afficher la spec complète dans le texte de la discussion** (ne pas inclure dans l'outil `question`) — ne jamais résumer.
2. **Afficher l'intégralité du bloc dans le texte de la discussion** (ne pas inclure dans l'outil `question`).
3. **Vérifier la présence de tous les champs obligatoires** : `Composants à créer`, `Tokens design requis`, `Statut`.
   - Si l'un de ces champs est absent ou vide sans mention explicite (`"Aucun"` / `"Aucune"`) → demander explicitement au designer de compléter avant de continuer.
4. **Si la spec complète est absente** (le bloc handoff est présent sans spec préalable) → demander explicitement au designer de produire la spec complète avant de continuer.
5. **Intégrer les `### Composants à créer` et `### Tokens design requis`** dans les questions de Phase 2 si validation utilisateur nécessaire.
6. **Ajouter les `### Questions pour l'utilisateur`** aux questions de Phase 2 (ne pas les poser immédiatement — les regrouper avec les autres questions contextuelles).
7. **Utiliser le `### Statut`** pour conditionner la suite :
   - `spec-complète` → continuer vers Phase 2 normalement (questions contextualisées)
   - `spec-partielle` → continuer vers Phase 2 en incluant les questions design dans le questionnaire
   - `bloqué` → demander à l'utilisateur comment débloquer avant de continuer

> ❌ Ne jamais continuer vers Phase 2 sans avoir reçu ce bloc structuré de l'agent designer.
> ❌ Ne jamais résumer la spec avant de la présenter à l'utilisateur.
> ❌ Ne jamais accepter un bloc handoff sans spec préalable — les deux sont obligatoires.

---

## Référence

**Source :** Workflow planner Phase 1.5 (ligne 93 de `planner-workflow.md`)  
**Contrat complémentaire :** `design-handoff-format.md` (handoff design → orchestrator)

**Mise à jour :** 30 juin 2026
