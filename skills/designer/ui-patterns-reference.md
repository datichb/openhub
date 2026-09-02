---
name: ui-patterns-reference
description: Reference de patterns UI par type de composant -- navigation, dashboard, etats (vide, erreur, chargement), formulaires, modals, menus. Chaque pattern inclut description, quand utiliser, checklist et anti-patterns. Inclut aussi les checklists de validation UXLoom et les niveaux Atomic Design.
---

# Patterns UI -- reference

Reference de patterns par type. Utiliser pour :
- Proposer des solutions eprouvees lors d'une spec
- Verifier la completude d'un composant (checklist)
- Identifier les etats manquants

---

## Navigation

### Sidebar (navigation laterale)

Navigation verticale a gauche. Standard pour les dashboards et panels admin.
Peut etre repliable/depliable. Supporte les groupes avec headers et les icones.

**Quand utiliser :** Application avec 5+ sections de navigation. Contenu principal qui
beneficie de la largeur complete de l'ecran.

**Checklist :**
- [ ] Etat actif visible sur l'item courant
- [ ] Repliable sur petits ecrans (icones seules ou hamburger)
- [ ] Groupement logique des items (sections avec headers)
- [ ] Raccourci clavier pour basculer (replie/deplie)
- [ ] Navigation clavier complete (Tab + fleches)

### Command Palette (Cmd+K)

Overlay de recherche declenchable au clavier qui permet de naviguer, lancer des commandes
ou chercher du contenu. Puissant pour les utilisateurs avances.

**Quand utiliser :** Tout produit avec 20+ pages/actions. Complement d'une navigation
classique -- pas un remplacement.

**Checklist :**
- [ ] Ouverture via raccourci clavier standard (Cmd+K / Ctrl+K)
- [ ] Recherche fuzzy sur les noms d'actions et de pages
- [ ] Categories de resultats (pages, actions, recherche)
- [ ] Navigation clavier dans les resultats (fleches + Enter)
- [ ] Fermeture via Escape
- [ ] Resultats recents / frequents en priorite

### Breadcrumbs

Trail horizontal montrant la position dans la hierarchie. Utile pour les contenus profonds.

**Quand utiliser :** Hierarchie de contenu a 3+ niveaux. E-commerce, documentation, admin.

**Checklist :**
- [ ] Chaque segment est cliquable (sauf le dernier = courant)
- [ ] Troncation gracieuse si trop de niveaux (ellipsis au milieu)
- [ ] Dernier element = page courante (non cliquable, style different)

### Tab Bar / Onglets

Navigation horizontale entre vues de meme niveau. Contenu change sans changer de page.

**Quand utiliser :** 2-7 vues de meme niveau. Le contenu de chaque onglet est independant.

**Checklist :**
- [ ] Onglet actif visuellement distinct
- [ ] Navigation clavier (fleches gauche/droite entre onglets)
- [ ] Pas plus de 7 onglets (sinon : overflow avec scroll ou "More")
- [ ] Le contenu change sans rechargement de page

---

## Dashboard

### KPI Cards

3-5 metriques cles en haut du dashboard, en cards scannables. Les chiffres que
l'utilisateur verifie a chaque visite.

**Quand utiliser :** Tout dashboard. C'est la premiere chose que l'oeil cherche.

**Regles :**
- Maximum 5 KPIs visibles (Loi de Miller)
- Chaque card : valeur + label + tendance (fleche haut/bas ou sparkline)
- Taille de police large pour la valeur (hero number)
- Couleur semantique pour la tendance (vert/rouge ou neutre)

### Activity Feed

Flux chronologique d'evenements recents. Donne le "pouls" de ce qui se passe.

**Quand utiliser :** Produits collaboratifs, monitoring, admin.

**Regles :**
- Ordre chronologique inverse (recent en haut)
- Chaque entree : avatar/icone + description + timestamp
- Regroupement par jour si volume eleve
- Lien vers le detail de chaque evenement

### Data Density Management

Equilibrer densite d'information et clarte.

**Regles :**
- Expert users : densite elevee acceptable (terminal Bloomberg)
- Nouveaux utilisateurs : espacement genereux, progressive disclosure
- Adapter la densite au contexte -- dashboard quotidien vs rapport detaille
- Proposer des vues (compact / comfortable / spacious) quand possible

---

## Empty States (etats vides)

### First-use (premier usage)

L'utilisateur voit la feature pour la premiere fois, pas encore de donnees.

**Structure :**
1. Illustration ou icone (optionnel mais efficace)
2. Titre explicatif : ce que fait cette zone
3. Description : pourquoi c'est vide et quel benefice une fois rempli
4. **CTA primaire** : l'action pour commencer (ex: "Creer votre premier projet")

**Anti-patterns :** Page blanche sans explication. Message "No data" sans action.
Jargon technique ("Aucun enregistrement trouve").

### No-results (recherche sans resultat)

La recherche ou le filtre ne retourne rien.

**Structure :**
1. Confirmer la requete ("Aucun resultat pour \<requete\>")
2. Suggestions : verifier l'orthographe, essayer des termes plus larges
3. Actions alternatives : reinitialiser les filtres, voir tout, contacter le support

**Anti-patterns :** Page vide sans explication. Pas de moyen de corriger la recherche.

### Cleared/Completed (donnees supprimees ou tache terminee)

L'utilisateur a volontairement vide la zone ou termine toutes les taches.

**Structure :**
1. Confirmation positive ("Tout est traite !" ou "Corbeille videe")
2. Action suivante si pertinente ("Revenir a la liste")
3. Pas de ton condescendant

---

## Error States (etats d'erreur)

### Inline field errors

Messages d'erreur sous le champ concerne. Le pattern le plus efficace.

**Regles :**
- Message directement sous le champ en erreur
- Couleur semantique error + icone (pas couleur seule)
- Texte : quoi est faux + comment corriger (ex: "L'email doit contenir un @")
- Apparait au blur (quitter le champ), pas pendant la frappe
- Le champ en erreur a une bordure/outline distincte

### Error pages (404, 500)

Ecrans d'erreur pleine page pour les liens casses ou erreurs serveur.

**Structure :**
1. Explication claire en langage humain (pas "Error 500")
2. Ce que l'utilisateur peut faire : retour a l'accueil, contacter le support, reessayer
3. **Toujours un CTA** -- jamais de dead-end

**Anti-patterns :** Page technique avec stack trace. Pas de lien pour sortir.
Ton accusateur ("Vous avez fait une erreur").

### Connection / Network error

Perte de connexion internet ou serveur injoignable.

**Structure :**
1. Message clair : "Connexion perdue" ou "Serveur indisponible"
2. Indication si temporaire : "Reconnexion automatique..."
3. Bouton "Reessayer" visible
4. Preserver le travail non sauvegarde (draft local)

### Partial failure (degradation gracieuse)

Une partie de la page echoue mais le reste fonctionne.

**Regles :**
- Afficher le contenu qui fonctionne
- Indicateur d'erreur localise sur la zone en echec (pas d'erreur pleine page)
- Bouton "Reessayer" sur la zone en echec
- Log l'erreur mais ne pas la montrer a l'utilisateur

---

## Loading States (etats de chargement)

### Skeleton screens

Placeholder qui imite le layout du contenu a venir avec des formes grises.

**Quand utiliser :** Chargement initial (< 3 secondes). Layout previsible.

**Regles :**
- Les formes imitent la structure reelle (pas un spinner generique)
- Animation subtile (pulse ou shimmer) pour indiquer l'activite
- Passer au contenu des qu'il est disponible (pas d'attente artificielle)

### Optimistic UI

Afficher le resultat de l'action de l'utilisateur immediatement, avant la confirmation serveur.
Rollback uniquement en cas d'echec.

**Quand utiliser :** Actions rapides (like, save, toggle, delete). Taux d'echec faible.

**Regles :**
- Feedback visuel instantane (< 50ms)
- Rollback avec message d'erreur si le serveur rejette
- Ne pas utiliser pour les actions irreversibles critiques (paiement)

### Progressive loading

Charger et afficher le contenu incrementalement, au fur et a mesure.

**Quand utiliser :** Listes longues, flux de contenu, resultats de recherche.

### Loading par duree

Adapter l'indicateur de chargement a la duree attendue :

| Duree | Pattern |
|-------|---------|
| < 200ms | Pas d'indicateur (imperceptible) |
| 200ms - 1s | Feedback subtil (spinner inline, changement d'opacite) |
| 1s - 3s | Skeleton screen ou spinner avec message |
| 3s - 10s | Barre de progression determinee |
| > 10s | Progression + estimation du temps restant + option d'annulation |

---

## Forms (formulaires)

### Layout single-column

Tous les champs en colonne unique verticale. Les utilisateurs traitent les formulaires
de haut en bas.

**Regle :** Ne jamais placer deux champs cote a cote sauf si semantiquement lies
(ex: Prenom / Nom, ou Ville / Code postal).

### Inline validation

Valider les champs au fur et a mesure (blur), afficher succes ou erreur en temps reel.

**Regles :**
- Valider au blur (quitter le champ), pas pendant la frappe
- Afficher le succes (checkmark vert) et l'erreur (message + icone)
- Ne pas valider un champ vide au premier blur (seulement si l'utilisateur a commence a taper)

### Smart defaults et auto-detection

Pre-remplir les champs autant que possible.

**Regles :**
- Auto-detecter pays, timezone, devise via le navigateur
- Activer l'autocomplete du navigateur (attributs `autocomplete`)
- Proposer des valeurs par defaut sensees (date du jour, devise locale)

### Progressive disclosure

Afficher uniquement les champs pertinents selon les choix precedents.

**Regles :**
- Champs conditionnels : apparaissent/disparaissent selon la selection
- "Options avancees" pour masquer la complexite par defaut
- Formulaire en etapes si > 8 champs

---

## Modals et Dialogs

### Confirmation dialog

Avant une action destructive ou irreversible.

**Structure :** Titre clair (pas "Etes-vous sur ?") + description de la consequence +
bouton de confirmation avec le verbe de l'action ("Supprimer", pas "OK") +
bouton annuler visible.

### Form modal

Formulaire dans une modal. Pour les saisies rapides (renommer, commenter).

**Regles :**
- Maximum 3-5 champs (au-dela : page dediee)
- Focus automatique sur le premier champ
- Fermeture par Escape
- Validation du formulaire par Enter (si un seul champ)

### Toast / Snackbar (non-modal)

Notification temporaire, non bloquante. Pour les confirmations de succes et les
informations transitoires.

**Regles :**
- Disparait automatiquement (3-5 secondes)
- Ne bloque pas l'interaction
- Action undo si applicable
- Maximum 1 toast visible a la fois
- Position coherente (toujours en bas ou toujours en haut)

### Overlay management

**Regle cruciale :** Ne jamais empiler les modals. Une modal qui ouvre une modal = red flag.
Si le flow est assez complexe pour necessiter des modals imbriquees, il merite sa propre page.

---

## Dropdown / Menus

### Select menu (choix de valeur)

Pour choisir une valeur parmi une liste.

**Regles :**
- Moins de 5 options : radio buttons ou toggle (pas de dropdown)
- 5-15 options : dropdown classique
- Plus de 15 options : dropdown avec recherche
- Option par defaut si une valeur est recommandee

### Action menu (commandes)

Menu contextuel avec des actions (editer, supprimer, partager).

**Regles :**
- Actions destructives en bas, separees visuellement (rouge ou separateur)
- Raccourcis clavier affiches a droite de chaque action
- Groupement logique avec separateurs
- Icones pour les actions frequentes

### Keyboard interaction

**Regles pour tous les menus :**
- Fleches haut/bas pour naviguer
- Enter pour selectionner
- Escape pour fermer
- Type-ahead (taper une lettre = sauter au premier item correspondant)
- Tab pour sortir du menu (pas pour naviguer a l'interieur)

---

## Structure composants -- niveaux Atomic Design

Hierarchie de composition pour les composants UI :

| Niveau | Exemples | Composition | Etat | Fetch data |
|--------|----------|-------------|------|------------|
| **Atom** | Bouton, Input, Label, Icon | Rien | Non | Non |
| **Molecule** | SearchBar (Input + Button), FormField (Label + Input + Error) | 2-5 atomes | Optionnel (ouvert/ferme) | Non |
| **Organism** | Header, Sidebar, Card, FormSection | Molecules + atomes | Oui | Oui |
| **Template** | PageLayout, DashboardLayout | Organismes | Non | Non |
| **Page** | HomePage, SettingsPage | Templates | Non | Oui (donnees reelles) |

**Regles :**
- Un atome ne compose jamais d'autres atomes
- Une molecule ne fetch pas de donnees
- Un template utilise uniquement du contenu placeholder
- Une page gere **tous les etats de donnees** : loading, empty, error, populated

---

## Checklist de validation par pattern (inspiree UXLoom)

Avant de valider une spec, verifier ces 5 dimensions :

### 1. Completude journey

- [ ] Chaque etat du user flow est atteignable depuis l'etat precedent
- [ ] Pas de dead-end (ecran sans action suivante)
- [ ] Pas d'etat orphelin (ecran inatteignable)
- [ ] Les flows de retour arriere sont definis

### 2. Couverture d'etats

Pour chaque ecran, verifier que ces etats sont couverts (ou explicitement exempts) :

- [ ] **Etat nominal** (donnees presentes, tout fonctionne)
- [ ] **Etat vide** (premier usage ou donnees supprimees)
- [ ] **Etat chargement** (donnees en cours de chargement)
- [ ] **Etat erreur** (echec de chargement, erreur serveur)
- [ ] **Etat partial** (certaines donnees presentes, d'autres en erreur)

Si un etat ne s'applique pas, documenter pourquoi (exemption justifiee).

### 3. Contraste WCAG

- [ ] Texte normal : ratio >= 4.5:1
- [ ] Texte large : ratio >= 3:1
- [ ] Composants interactifs et focus : ratio >= 3:1
- [ ] Information transmise autrement que par la couleur seule

### 4. Cibles interactives

- [ ] Taille minimum 44x44px pour les cibles tactiles
- [ ] Espacement minimum 8px entre les cibles
- [ ] Actions destructives eloignees des actions constructives

### 5. Expansion texte (i18n)

- [ ] Labels et boutons tolerent +40% d'expansion texte (traductions)
- [ ] Pas de texte tronque sans indication (ellipsis + tooltip)
- [ ] Les layouts ne cassent pas avec du texte plus long
