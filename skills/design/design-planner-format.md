---
name: design-planner-format
description: Source de vérité pour le format de handoff planner → ux-designer / ui-designer. Définit le contexte obligatoire à transmettre lors de la délégation design en Phase 1.5. Injecté dans planner, ux-designer et ui-designer pour garantir que le producteur et les consommateurs partagent le même contrat.
---

# Skill — Format de handoff planner → design

Ce skill est la **source de vérité** pour le format de délégation du `planner` vers les agents design (`ux-designer` / `ui-designer`).
Il est injecté dans `planner`, `ux-designer` et `ui-designer` — producteur et consommateurs partagent le même contrat.

---

## Quand produire ce handoff

Le `planner` délègue à un agent design en **Phase 1.5** (après Phase 1 d'exploration, avant Phase 2 de questions) si **au moins un signal design** a été identifié lors de l'exploration contexte.

**Signaux déclencheurs (Phase 1.5 — ligne 93 du workflow planner) :**
- Mention de "interface", "UX", "UI", "wireframe", "maquette", "design system", "composant réutilisable"
- Mention de "parcours utilisateur", "user flow", "accessibilité", "responsive"
- Absence de composants existants réutilisables (nécessite création de nouveaux composants)
- Feature traversant plusieurs écrans avec interactions complexes

**Choix d'agent :**
- **ux-designer** : parcours utilisateur, user flows, architecture de l'information, états d'interface, accessibilité structurelle
- **ui-designer** : tokens, design system, composants réutilisables, cohérence visuelle, accessibilité visuelle (contraste, taille)

> Si les deux dimensions sont présentes → commencer par **ux-designer**.
> Si seul le visuel est concerné (ex : nouveau bouton, palette de couleurs) → **ui-designer** directement.

---

## Format du prompt de délégation planner → design

```
Agent : ux-designer | ui-designer

Objectif : <résumé en 1-2 phrases de ce que l'agent design doit produire>

---

### Feature demandée
<Description complète de la feature — copier la demande utilisateur telle quelle>

### Contexte projet
<Éléments découverts en Phase 1 : stack technique, conventions front, frameworks UI, design system existant si présent>

### Composants existants identifiés
<Liste des composants réutilisables identifiés lors de l'exploration>
<"Aucun composant réutilisable identifié" si vide>

### Signaux design détectés
<Liste des signaux qui ont déclenché la délégation design — parcours multi-écrans, mention de wireframe, interactions complexes, etc.>

### Contraintes techniques anticipées
<Contraintes identifiées lors de Phase 1 qui peuvent impacter le design : responsive obligatoire, contraintes de performance, limitations du framework, etc.>
<"Aucune contrainte technique anticipée" si vide>

### Questions ouvertes (à trancher par design ou à clarifier avec l'utilisateur)
<Questions contextuelles identifiées mais non posées car nécessitent une décision design avant — ex : "Faut-il créer un nouveau layout ou réutiliser l'existant ?">
<"Aucune" si toutes les questions ont été posées en Phase 2>

---

**Livrables attendus :**
- <livrable 1 — ex : user flow complet avec tous les états, wireframes textuels, critères d'acceptance UX>
- <livrable 2>

**Format de retour :** Utiliser le skill `design-planner-format` (bloc `## Retour vers planner` défini ci-dessous).
```

---

## Format du bloc `## Retour vers planner`

**Produit par l'agent design (ux-designer / ui-designer) à la fin de sa spec :**

```
---

## Retour vers planner

**Agent :** ux-designer | ui-designer
**Feature :** <titre de la feature>

### Spec produite
Voir spec complète ci-dessus — jamais résumée ni reproduite ici.

### Composants à créer
- `<nom composant 1>` : <rôle, props principaux>
- `<nom composant 2>` : <rôle, props principaux>
<"Aucun nouveau composant nécessaire" si seuls des composants existants sont utilisés>

### Tokens design requis
- `<token 1>` : <valeur ou plage si non défini>
- `<token 2>` : <valeur ou plage si non défini>
<"Tous les tokens nécessaires existent déjà" si aucun nouveau token requis>

### Dépendances design
<Autres composants ou écrans qui doivent être conçus avant ou en parallèle, ou impacts sur le design system global>
<"Aucune" si la spec est autonome>

### Questions pour l'utilisateur (à poser par le planner en Phase 2)
- <question 1 — ce qui nécessite une décision métier ou utilisateur avant implémentation>
- <question 2>
<"Aucune" si tous les éléments ont été tranchés>

### Statut
`spec-complète` | `spec-partielle` | `bloqué`
```

**Définitions du statut :**

| Statut | Condition |
|--------|-----------|
| `spec-complète` | Spec validée, tous les éléments design nécessaires sont présents |
| `spec-partielle` | Spec validée mais avec des questions ouvertes pour l'utilisateur (à poser par le planner en Phase 2) |
| `bloqué` | Spec non finalisée — un blocage empêche de produire une spec exploitable |

---

## Règles pour le producteur (planner)

- **Toujours vérifier les signaux design** avant de déléguer (ligne 93 du workflow planner)
- **Choisir le bon agent** : ux-designer (flows, états) vs ui-designer (tokens, composants)
- **Ne jamais déléguer si aucun signal design** n'a été identifié — le planner continue en Phase 2 directement
- **Transmettre tout le contexte** identifié en Phase 1 — composants existants, stack technique, conventions front
- **Ne jamais poser les questions de Phase 2** avant d'avoir reçu le retour design — le design peut modifier les questions à poser

---

## Règles pour les consommateurs (ux-designer / ui-designer)

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

### À la réception du retour d'un agent design

1. **Afficher la spec complète dans le texte de la discussion** (ne pas inclure dans l'outil `question`) — ne jamais résumer.
2. **Afficher l'intégralité du bloc dans le texte de la discussion** (ne pas inclure dans l'outil `question`).
3. **Vérifier la présence de tous les champs obligatoires** : `Composants à créer`, `Tokens design requis`, `Statut`.
   - Si l'un de ces champs est absent ou vide sans mention explicite (`"Aucun"` / `"Aucune"`) → demander explicitement à l'agent design de compléter avant de continuer.
4. **Si la spec complète est absente** (le bloc handoff est présent sans spec préalable) → demander explicitement à l'agent design de produire la spec complète avant de continuer.
5. **Intégrer les `### Composants à créer` et `### Tokens design requis`** dans les questions de Phase 2 si validation utilisateur nécessaire.
6. **Ajouter les `### Questions pour l'utilisateur`** aux questions de Phase 2 (ne pas les poser immédiatement — les regrouper avec les autres questions contextuelles).
7. **Utiliser le `### Statut`** pour conditionner la suite :
   - `spec-complète` → continuer vers Phase 2 normalement (questions contextualisées)
   - `spec-partielle` → continuer vers Phase 2 en incluant les questions design dans le questionnaire
   - `bloqué` → demander à l'utilisateur comment débloquer avant de continuer

> ❌ Ne jamais continuer vers Phase 2 sans avoir reçu ce bloc structuré de l'agent design.
> ❌ Ne jamais résumer la spec avant de la présenter à l'utilisateur.
> ❌ Ne jamais accepter un bloc handoff sans spec préalable — les deux sont obligatoires.

---

## Exemple complet

### Prompt de délégation (planner → ux-designer)

```
Agent : ux-designer

Objectif : Définir le parcours utilisateur complet pour la gestion des favoris dans l'interface de recherche.

---

### Feature demandée
Permettre aux utilisateurs de sauvegarder leurs recherches favorites et de les retrouver rapidement dans un menu dédié.

### Contexte projet
- Stack : Vue 3 (Composition API), TypeScript, Tailwind CSS
- Design system existant : composants Button, Card, Input, Dropdown déjà présents dans `/src/components/ui`
- Convention : tous les composants réutilisables dans `/src/components`, pages dans `/src/views`

### Composants existants identifiés
- `Button.vue` : bouton standard avec variants (primary, secondary, ghost)
- `Dropdown.vue` : menu déroulant générique
- `Card.vue` : carte de contenu avec slot header/body

### Signaux design détectés
- Parcours multi-écrans : page de recherche + menu favoris + modal de confirmation
- Mention de "retrouver rapidement" → nécessite une réflexion sur l'accès au menu (header, sidebar, modal ?)
- Interaction complexe : sauvegarder depuis la recherche + gérer la liste des favoris (édition, suppression)

### Contraintes techniques anticipées
- Responsive obligatoire (mobile-first)
- Performance : liste de favoris peut contenir jusqu'à 100 entrées (virtualisation si nécessaire)

### Questions ouvertes (à trancher par design ou à clarifier avec l'utilisateur)
- Où placer le bouton d'accès au menu favoris ? (header global, sidebar, floating button)
- Limite du nombre de favoris par utilisateur ?
- Synchronisation des favoris entre devices ?

---

**Livrables attendus :**
- User flow complet : sauvegarder une recherche, accéder au menu favoris, éditer/supprimer un favori
- Wireframes textuels pour chaque écran (page recherche, menu favoris, modal confirmation)
- États d'interface : vide, chargement, erreur, limite atteinte
- Critères d'acceptance UX : délai de feedback < 200ms, confirmation avant suppression, undo possible

**Format de retour :** Utiliser le skill `design-planner-format` (bloc `## Retour vers planner` défini ci-dessous).
```

### Retour de l'agent design (ux-designer → planner)

```
---

## Retour vers planner

**Agent :** ux-designer
**Feature :** Gestion des favoris dans l'interface de recherche

### Spec produite
Voir spec complète ci-dessus (user flows, wireframes, états, critères UX).

### Composants à créer
- `FavoriteButton.vue` : bouton toggle pour sauvegarder/retirer des favoris (props : `isFavorite`, `onToggle`)
- `FavoritesMenu.vue` : menu déroulant affichant la liste des favoris (props : `favorites`, `onSelect`, `onDelete`)
- `ConfirmDeleteModal.vue` : modal de confirmation avant suppression (réutilisable, props : `title`, `message`, `onConfirm`, `onCancel`)

### Tokens design requis
- Tous les tokens nécessaires existent déjà (couleurs, espacements, typographie définis dans le design system)

### Dépendances design
- Aucune — la spec est autonome et réutilise les composants existants (Button, Card, Dropdown)

### Questions pour l'utilisateur (à poser par le planner en Phase 2)
- Où souhaitez-vous placer le bouton d'accès au menu favoris ? (Options : header global à droite, sidebar gauche, floating button en bas à droite)
- Souhaitez-vous limiter le nombre de favoris par utilisateur ? Si oui, quelle limite ?
- Les favoris doivent-ils être synchronisés entre devices (compte utilisateur) ou stockés localement (localStorage) ?

### Statut
`spec-partielle`

**Justification :** Spec UX complète mais 3 questions métier nécessitent une décision utilisateur avant implémentation (placement du menu, limite, synchronisation).
```

---

## Référence

**Source :** Workflow planner Phase 1.5 (ligne 93 de `planner-workflow.md`)  
**Contrat complémentaire :** `design-handoff-format.md` (handoff design → orchestrator)

**Mise à jour :** 28 mai 2026
