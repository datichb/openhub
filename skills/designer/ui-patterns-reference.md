---
name: ui-patterns-reference
description: Référence de patterns UI par type de composant — navigation, dashboard, états (vide, erreur, chargement), formulaires, modals, menus. Chaque pattern inclut description, quand utiliser, checklist et anti-patterns. Inclut aussi les checklists de validation UXLoom et les niveaux Atomic Design.
---

# Patterns UI — référence

Référence de patterns par type. Utiliser pour :
- Proposer des solutions éprouvées lors d'une spec
- Vérifier la complétude d'un composant (checklist)
- Identifier les états manquants

---

## Navigation

### Sidebar (navigation latérale)

Navigation verticale à gauche. Standard pour les dashboards et panels admin.
Peut être repliable/dépliable. Supporte les groupes avec headers et les icônes.

**Quand utiliser :** Application avec 5+ sections de navigation. Contenu principal qui
bénéficie de la largeur complète de l'écran.

**Checklist :**
- [ ] État actif visible sur l'item courant
- [ ] Repliable sur petits écrans (icônes seules ou hamburger)
- [ ] Groupement logique des items (sections avec headers)
- [ ] Raccourci clavier pour basculer (replié/déplié)
- [ ] Navigation clavier complète (Tab + flèches)

### Command Palette (Cmd+K)

Overlay de recherche déclenchable au clavier qui permet de naviguer, lancer des commandes
ou chercher du contenu. Puissant pour les utilisateurs avancés.

**Quand utiliser :** Tout produit avec 20+ pages/actions. Complément d'une navigation
classique — pas un remplacement.

**Checklist :**
- [ ] Ouverture via raccourci clavier standard (Cmd+K / Ctrl+K)
- [ ] Recherche fuzzy sur les noms d'actions et de pages
- [ ] Catégories de résultats (pages, actions, recherche)
- [ ] Navigation clavier dans les résultats (flèches + Enter)
- [ ] Fermeture via Escape
- [ ] Résultats récents / fréquents en priorité

### Breadcrumbs

Fil d'Ariane horizontal montrant la position dans la hiérarchie. Utile pour les contenus profonds.

**Quand utiliser :** Hiérarchie de contenu à 3+ niveaux. E-commerce, documentation, admin.

**Checklist :**
- [ ] Chaque segment est cliquable (sauf le dernier = courant)
- [ ] Troncature gracieuse si trop de niveaux (ellipsis au milieu)
- [ ] Dernier élément = page courante (non cliquable, style différent)

### Tab Bar / Onglets

Navigation horizontale entre vues de même niveau. Le contenu change sans changer de page.

**Quand utiliser :** 2-7 vues de même niveau. Le contenu de chaque onglet est indépendant.

**Checklist :**
- [ ] Onglet actif visuellement distinct
- [ ] Navigation clavier (flèches gauche/droite entre onglets)
- [ ] Pas plus de 7 onglets (sinon : overflow avec scroll ou "More")
- [ ] Le contenu change sans rechargement de page

---

## Dashboard

### KPI Cards

3-5 métriques clés en haut du dashboard, en cards scannables. Les chiffres que
l'utilisateur vérifie à chaque visite.

**Quand utiliser :** Tout dashboard. C'est la première chose que l'œil cherche.

**Règles :**
- Maximum 5 KPIs visibles (Loi de Miller)
- Chaque card : valeur + label + tendance (flèche haut/bas ou sparkline)
- Taille de police large pour la valeur (hero number)
- Couleur sémantique pour la tendance (vert/rouge ou neutre)

### Activity Feed

Flux chronologique d'événements récents. Donne le "pouls" de ce qui se passe.

**Quand utiliser :** Produits collaboratifs, monitoring, admin.

**Règles :**
- Ordre chronologique inverse (récent en haut)
- Chaque entrée : avatar/icône + description + timestamp
- Regroupement par jour si volume élevé
- Lien vers le détail de chaque événement

### Data Density Management

Équilibrer densité d'information et clarté.

**Règles :**
- Expert users : densité élevée acceptable (terminal Bloomberg)
- Nouveaux utilisateurs : espacement généreux, progressive disclosure
- Adapter la densité au contexte — dashboard quotidien vs rapport détaillé
- Proposer des vues (compact / comfortable / spacious) quand c'est possible

---

## Empty States (états vides)

### First-use (premier usage)

L'utilisateur voit la feature pour la première fois, pas encore de données.

**Structure :**
1. Illustration ou icône (optionnel mais efficace)
2. Titre explicatif : ce que fait cette zone
3. Description : pourquoi c'est vide et quel bénéfice une fois rempli
4. **CTA primaire** : l'action pour commencer (ex : "Créer votre premier projet")

**Anti-patterns :** Page blanche sans explication. Message "No data" sans action.
Jargon technique ("Aucun enregistrement trouvé").

### No-results (recherche sans résultat)

La recherche ou le filtre ne retourne rien.

**Structure :**
1. Confirmer la requête ("Aucun résultat pour \<requête\>")
2. Suggestions : vérifier l'orthographe, essayer des termes plus larges
3. Actions alternatives : réinitialiser les filtres, voir tout, contacter le support

**Anti-patterns :** Page vide sans explication. Pas de moyen de corriger la recherche.

### Cleared/Completed (données supprimées ou tâche terminée)

L'utilisateur a volontairement vidé la zone ou terminé toutes les tâches.

**Structure :**
1. Confirmation positive ("Tout est traité !" ou "Corbeille vidée")
2. Action suivante si pertinente ("Revenir à la liste")
3. Pas de ton condescendant

---

## Error States (états d'erreur)

### Inline field errors

Messages d'erreur sous le champ concerné. Le pattern le plus efficace.

**Règles :**
- Message directement sous le champ en erreur
- Couleur sémantique error + icône (pas la couleur seule)
- Texte : ce qui est faux + comment corriger (ex : "L'email doit contenir un @")
- Apparaît au blur (quitter le champ), pas pendant la frappe
- Le champ en erreur a une bordure/outline distincte

### Error pages (404, 500)

Écrans d'erreur pleine page pour les liens cassés ou erreurs serveur.

**Structure :**
1. Explication claire en langage humain (pas "Error 500")
2. Ce que l'utilisateur peut faire : retour à l'accueil, contacter le support, réessayer
3. **Toujours un CTA** — jamais de dead-end

**Anti-patterns :** Page technique avec stack trace. Pas de lien pour sortir.
Ton accusateur ("Vous avez fait une erreur").

### Connection / Network error

Perte de connexion internet ou serveur injoignable.

**Structure :**
1. Message clair : "Connexion perdue" ou "Serveur indisponible"
2. Indication si temporaire : "Reconnexion automatique..."
3. Bouton "Réessayer" visible
4. Préserver le travail non sauvegardé (draft local)

### Partial failure (dégradation gracieuse)

Une partie de la page échoue mais le reste fonctionne.

**Règles :**
- Afficher le contenu qui fonctionne
- Indicateur d'erreur localisé sur la zone en échec (pas d'erreur pleine page)
- Bouton "Réessayer" sur la zone en échec
- Logger l'erreur mais ne pas la montrer à l'utilisateur

---

## Loading States (états de chargement)

### Skeleton screens

Placeholder qui imite le layout du contenu à venir avec des formes grises.

**Quand utiliser :** Chargement initial (< 3 secondes). Layout prévisible.

**Règles :**
- Les formes imitent la structure réelle (pas un spinner générique)
- Animation subtile (pulse ou shimmer) pour indiquer l'activité
- Passer au contenu dès qu'il est disponible (pas d'attente artificielle)

### Optimistic UI

Afficher le résultat de l'action de l'utilisateur immédiatement, avant la confirmation serveur.
Rollback uniquement en cas d'échec.

**Quand utiliser :** Actions rapides (like, save, toggle, delete). Taux d'échec faible.

**Règles :**
- Feedback visuel instantané (< 50ms)
- Rollback avec message d'erreur si le serveur rejette
- Ne pas utiliser pour les actions irréversibles critiques (paiement)

### Progressive loading

Charger et afficher le contenu de manière incrémentale, au fur et à mesure.

**Quand utiliser :** Listes longues, flux de contenu, résultats de recherche.

### Loading par durée

Adapter l'indicateur de chargement à la durée attendue :

| Durée | Pattern |
|-------|---------|
| < 200ms | Pas d'indicateur (imperceptible) |
| 200ms - 1s | Feedback subtil (spinner inline, changement d'opacité) |
| 1s - 3s | Skeleton screen ou spinner avec message |
| 3s - 10s | Barre de progression déterminée |
| > 10s | Progression + estimation du temps restant + option d'annulation |

---

## Forms (formulaires)

### Layout single-column

Tous les champs en colonne unique verticale. Les utilisateurs traitent les formulaires
de haut en bas.

**Règle :** Ne jamais placer deux champs côte à côte sauf s'ils sont sémantiquement liés
(ex : Prénom / Nom, ou Ville / Code postal).

### Inline validation

Valider les champs au fur et à mesure (blur), afficher succès ou erreur en temps réel.

**Règles :**
- Valider au blur (quitter le champ), pas pendant la frappe
- Afficher le succès (checkmark vert) et l'erreur (message + icône)
- Ne pas valider un champ vide au premier blur (seulement si l'utilisateur a commencé à taper)

### Smart defaults et auto-detection

Pré-remplir les champs autant que possible.

**Règles :**
- Auto-détecter pays, timezone, devise via le navigateur
- Activer l'autocomplete du navigateur (attributs `autocomplete`)
- Proposer des valeurs par défaut sensées (date du jour, devise locale)

### Progressive disclosure

Afficher uniquement les champs pertinents selon les choix précédents.

**Règles :**
- Champs conditionnels : apparaissent/disparaissent selon la sélection
- "Options avancées" pour masquer la complexité par défaut
- Formulaire en étapes si > 8 champs

---

## Modals et Dialogs

### Confirmation dialog

Avant une action destructive ou irréversible.

**Structure :** Titre clair (pas "Êtes-vous sûr ?") + description de la conséquence +
bouton de confirmation avec le verbe de l'action ("Supprimer", pas "OK") +
bouton annuler visible.

### Form modal

Formulaire dans une modal. Pour les saisies rapides (renommer, commenter).

**Règles :**
- Maximum 3-5 champs (au-delà : page dédiée)
- Focus automatique sur le premier champ
- Fermeture par Escape
- Validation du formulaire par Enter (si un seul champ)

### Toast / Snackbar (non-modal)

Notification temporaire, non bloquante. Pour les confirmations de succès et les
informations transitoires.

**Règles :**
- Disparaît automatiquement (3-5 secondes)
- Ne bloque pas l'interaction
- Action undo si applicable
- Maximum 1 toast visible à la fois
- Position cohérente (toujours en bas ou toujours en haut)

### Overlay management

**Règle cruciale :** Ne jamais empiler les modals. Une modal qui ouvre une modal = red flag.
Si le flow est assez complexe pour nécessiter des modals imbriquées, il mérite sa propre page.

---

## Dropdown / Menus

### Select menu (choix de valeur)

Pour choisir une valeur parmi une liste.

**Règles :**
- Moins de 5 options : radio buttons ou toggle (pas de dropdown)
- 5-15 options : dropdown classique
- Plus de 15 options : dropdown avec recherche
- Option par défaut si une valeur est recommandée

### Action menu (commandes)

Menu contextuel avec des actions (éditer, supprimer, partager).

**Règles :**
- Actions destructives en bas, séparées visuellement (rouge ou séparateur)
- Raccourcis clavier affichés à droite de chaque action
- Groupement logique avec séparateurs
- Icônes pour les actions fréquentes

### Keyboard interaction

**Règles pour tous les menus :**
- Flèches haut/bas pour naviguer
- Enter pour sélectionner
- Escape pour fermer
- Type-ahead (taper une lettre = sauter au premier item correspondant)
- Tab pour sortir du menu (pas pour naviguer à l'intérieur)

---

## Structure composants — niveaux Atomic Design

Hiérarchie de composition pour les composants UI :

| Niveau | Exemples | Composition | État | Fetch data |
|--------|----------|-------------|------|------------|
| **Atom** | Bouton, Input, Label, Icon | Rien | Non | Non |
| **Molecule** | SearchBar (Input + Button), FormField (Label + Input + Error) | 2-5 atomes | Optionnel (ouvert/fermé) | Non |
| **Organism** | Header, Sidebar, Card, FormSection | Molécules + atomes | Oui | Oui |
| **Template** | PageLayout, DashboardLayout | Organismes | Non | Non |
| **Page** | HomePage, SettingsPage | Templates | Non | Oui (données réelles) |

**Règles :**
- Un atome ne compose jamais d'autres atomes
- Une molécule ne fetch pas de données
- Un template utilise uniquement du contenu placeholder
- Une page gère **tous les états de données** : loading, empty, error, populated

---

## Checklist de validation par pattern (inspirée UXLoom)

Avant de valider une spec, vérifier ces 5 dimensions :

### 1. Complétude journey

- [ ] Chaque état du user flow est atteignable depuis l'état précédent
- [ ] Pas de dead-end (écran sans action suivante)
- [ ] Pas d'état orphelin (écran inatteignable)
- [ ] Les flows de retour arrière sont définis

### 2. Couverture d'états

Pour chaque écran, vérifier que ces états sont couverts (ou explicitement exemptés) :

- [ ] **État nominal** (données présentes, tout fonctionne)
- [ ] **État vide** (premier usage ou données supprimées)
- [ ] **État chargement** (données en cours de chargement)
- [ ] **État erreur** (échec de chargement, erreur serveur)
- [ ] **État partiel** (certaines données présentes, d'autres en erreur)

Si un état ne s'applique pas, documenter pourquoi (exemption justifiée).

### 3. Contraste WCAG

- [ ] Texte normal : ratio >= 4.5:1
- [ ] Texte large : ratio >= 3:1
- [ ] Composants interactifs et focus : ratio >= 3:1
- [ ] Information transmise autrement que par la couleur seule

### 4. Cibles interactives

- [ ] Taille minimum 44x44px pour les cibles tactiles
- [ ] Espacement minimum 8px entre les cibles
- [ ] Actions destructives éloignées des actions constructives

### 5. Expansion texte (i18n)

- [ ] Labels et boutons tolèrent +40% d'expansion texte (traductions)
- [ ] Pas de texte tronqué sans indication (ellipsis + tooltip)
- [ ] Les layouts ne cassent pas avec du texte plus long
