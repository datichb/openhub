# Templates — Handoff planner ↔ designer

## Format du prompt de délégation planner → designer

```
Agent : designer
Mode: recon | ux | ui | ux+ui

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

```
---

## Retour vers planner

**Agent :** designer
**Mode :** ux | ui | ux+ui
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

---

## Exemple — Prompt de délégation (planner → designer)

```
Agent : designer
Mode: ux

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

---

## Exemple — Retour de l'agent designer (designer → planner)

```
---

## Retour vers planner

**Agent :** designer
**Mode :** ux
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
