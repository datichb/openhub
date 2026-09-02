---
name: design-principles
description: Principes de design fondamentaux -- heuristiques Nielsen enrichies, principes Gestalt, Laws of UX, accessibilite operationnelle, grille Tenets & Traps. Reference partagee entre les modes ux et ui.
---

# Principes de design -- reference

Ce skill est une reference. Ne pas le reciter en entier -- l'utiliser pour justifier
des decisions et identifier des violations quand tu evalues un ecran ou specifies un composant.

---

## Heuristiques Nielsen -- grille enrichie

Chaque heuristique a trois dimensions : definition, comment verifier, violations courantes.

### 1. Visibilite de l'etat du systeme

**Principe :** Le systeme informe toujours l'utilisateur de ce qui se passe, via un feedback
appropriate dans un delai raisonnable.

**Verifier :** Chaque action produit-elle un feedback visible ? Les etats de chargement,
progres et succes sont-ils affiches ? L'utilisateur sait-il ou il en est dans un processus multi-etapes ?

**Violations courantes :** Bouton sans feedback au clic, soumission de formulaire sans indicateur
de chargement, absence de barre de progres dans un wizard, pas de confirmation apres une action.

### 2. Correspondance systeme / monde reel

**Principe :** Le systeme parle le langage de l'utilisateur (mots, concepts, conventions du monde reel),
pas le jargon interne.

**Verifier :** Le vocabulaire est-il familier a l'utilisateur cible ? L'ordre des informations
suit-il la logique de l'utilisateur (pas celle de la base de donnees) ?

**Violations courantes :** IDs techniques exposes, messages d'erreur contenant des codes HTTP ou
des stack traces, vocabulaire interne ("entity", "record") au lieu de termes metier.

### 3. Controle et liberte utilisateur

**Principe :** Les utilisateurs font des erreurs. Ils ont besoin d'une "sortie de secours" clairement
visible pour quitter un etat non desire, sans passer par un processus complique.

**Verifier :** L'utilisateur peut-il annuler, revenir en arriere, modifier ? Existe-t-il un undo ?
La navigation "retour" fonctionne-t-elle de facon previsible ?

**Violations courantes :** Actions irreversibles sans confirmation, pas de bouton annuler dans un
wizard multi-etapes, modal sans moyen de fermer, suppression sans undo.

### 4. Coherence et standards

**Principe :** Les memes mots, situations et actions signifient toujours la meme chose.
Les conventions de la plateforme sont respectees.

**Verifier :** Un meme pattern d'interaction est-il identique partout ? Les composants visuellement
identiques se comportent-ils de la meme facon ?

**Violations courantes :** Bouton "Valider" qui s'appelle "OK" ailleurs, comportement de clic
different sur des elements visuellement identiques, raccourcis clavier inconsistants.

### 5. Prevention des erreurs

**Principe :** Plutot que de bons messages d'erreur, empecher le probleme d'arriver.

**Verifier :** Les actions destructives ont-elles une confirmation ? Les champs de formulaire
ont-ils des contraintes (type, format, limites) ? Le systeme desactive-t-il les actions
impossibles plutot que de les laisser echouer ?

**Violations courantes :** Champ email sans validation de format, bouton supprimer sans
confirmation, formulaire soumissible avec des donnees invalides.

### 6. Reconnaissance plutot que rappel

**Principe :** Minimiser la charge de memoire en rendant les elements, actions et options
visibles. L'utilisateur reconnait plutot qu'il ne se souvient.

**Verifier :** Les options sont-elles visibles ? L'historique recent est-il accessible ?
Le contexte necessaire est-il present a chaque ecran ?

**Violations courantes :** Menus profonds qui forcent a memoriser le chemin, formulaires
sans valeurs par defaut, recherche sans suggestions.

### 7. Flexibilite et efficacite

**Principe :** Le systeme sert aussi bien les novices que les experts.
Les raccourcis accelerent les taches repetitives pour les experts.

**Verifier :** Existe-t-il des raccourcis clavier ? Les actions frequentes sont-elles
directement accessibles ? La personnalisation est-elle possible ?

**Violations courantes :** Pas de raccourcis clavier, pas de "recents", pas de favoris,
wizards obligatoires meme pour les utilisateurs experts.

### 8. Esthetique et design minimaliste

**Principe :** Chaque element d'information supplementaire entre en competition avec les
elements pertinents et diminue leur visibilite relative.

**Verifier :** Chaque element present est-il necessaire ? Peut-on en retirer sans perdre
de fonctionnalite ? La hierarchie visuelle guide-t-elle l'oeil vers l'essentiel ?

**Violations courantes :** Trop d'informations sur un seul ecran, decorations sans fonction,
textes d'aide qui polluent plus qu'ils n'aident.

### 9. Aide a la reconnaissance et recuperation d'erreur

**Principe :** Les messages d'erreur expriment clairement le probleme en langage naturel,
indiquent precisement la cause et suggerent une solution.

**Verifier :** Le message dit-il quoi (erreur), pourquoi (cause), comment (solution) ?
Le message est-il en langage humain (pas un code technique) ?

**Violations courantes :** "Error 500", "Invalid input", messages vagues sans action
suggeree, erreurs affichees loin du champ concerne.

### 10. Aide et documentation

**Principe :** Le systeme doit pouvoir etre utilise sans documentation, mais l'aide doit etre
accessible sans interrompre le flow quand elle est necessaire.

**Verifier :** L'aide est-elle contextuelle (tooltips, inline) ? La documentation est-elle
cherchable ? Les textes d'aide sont-ils orientes tache (pas theorie) ?

**Violations courantes :** Documentation uniquement externe, pas de tooltips sur les champs
complexes, aide qui ouvre une nouvelle page et fait perdre le contexte.

---

## Principes Gestalt

Les lois de la perception visuelle -- comment le cerveau organise les elements visuels en groupes.

### Proximite

Les elements proches sont percus comme un groupe. Utiliser l'espacement pour creer des groupes
logiques. Les elements lies ont un espacement serre ; les groupes distincts sont espaces.

**Application TUI :** Lignes vides pour separer les sections. Indentation pour les sous-groupes.
**Application Web :** Margins et gaps entre sections. Cards regroupant les elements lies.

### Similarite

Les elements visuellement semblables sont percus comme lies. Forme, couleur, taille ou style
identiques = meme categorie.

**Application :** Tous les boutons d'action primaire ont le meme style. Tous les labels de
formulaire partagent la meme typographie. Les elements interactifs se distinguent visuellement des
elements statiques.

### Cloture

Le cerveau complete les formes incompletes. On peut suggerer un conteneur sans le dessiner.

**Application TUI :** Bordures ASCII/Unicode pour delimiter les zones. Indentation seule
suffit parfois a creer un "conteneur" implicite.
**Application Web :** Cards, ombre, bordure subtile -- ou simplement un fond colore.

### Continuite

L'oeil suit les lignes et les courbes. Les elements alignes sont percus comme lies.

**Application :** Aligner les labels et les champs. Aligner les colonnes dans un tableau.
Ne pas rompre la ligne de lecture sans raison.

### Figure-Fond

Le cerveau distingue un objet (figure) de son arriere-plan (fond). L'element important
doit se detacher visuellement du contexte.

**Application :** Modal au-dessus d'un fond assombri. Texte contrastraste sur le fond.
Element actif / focus mis en evidence par rapport aux elements inactifs.

### Region commune

Les elements dans une meme region visuelle (encadre, fond colore, bordure) sont percus
comme un groupe, meme s'ils ne sont pas proches.

**Application :** Cards, panels, sections encadrees. En TUI : boites Unicode.

---

## Laws of UX -- les 12 lois les plus actionnables

### Loi de Hick

Le temps de decision augmente avec le nombre d'options.
**Action :** Reduire les choix visibles. Masquer les options avancees. Hierarchiser.

### Loi de Fitts

Le temps d'acquisition d'une cible depend de sa taille et de sa distance.
**Action :** Actions principales grandes et accessibles. Actions destructives petites et
eloignees. Boutons de validation larges.

### Peak-End Rule

Les utilisateurs jugent une experience principalement sur son pic (moment le plus intense)
et sa fin -- pas sur la moyenne de l'experience.
**Action :** Soigner le premier ecran (premiere impression) et le dernier ecran
(confirmation, succes). Celebrer les accomplissements.

### Goal-Gradient Effect

La motivation augmente quand on approche du but.
**Action :** Afficher la progression (barre, etapes restantes). Montrer "Plus que 2 etapes".

### Aesthetic-Usability Effect

Les utilisateurs percoivent les interfaces esthetiques comme plus faciles a utiliser.
**Action :** L'esthetique n'est pas du luxe -- elle ameliore la perception d'utilisabilite.
Investir dans la coherence visuelle.

### Doherty Threshold

La productivite explose quand le systeme repond en < 400ms.
**Action :** Feedback immediat (< 100ms pour les interactions directes, < 400ms pour les
resultats). Skeleton screens, optimistic UI.

### Loi de Miller

La memoire de travail retient 7 +/- 2 elements.
**Action :** Limiter les groupes a 5-9 elements. Chunkifier les listes longues. Regrouper.

### Loi de Jakob

Les utilisateurs passent la plupart de leur temps sur d'autres sites/apps.
Ils preferent que votre produit fonctionne comme ce qu'ils connaissent deja.
**Action :** Suivre les conventions de la plateforme. Pas d'innovation dans les patterns
d'interaction de base (navigation, formulaires, modals).

### Serial Position Effect

Les utilisateurs retiennent mieux le premier et le dernier element d'une liste.
**Action :** Placer les actions/informations importantes en debut et fin de liste ou de
navigation.

### Pareto Principle (80/20)

80% des effets viennent de 20% des causes.
**Action :** Identifier les 20% de features les plus utilisees et les optimiser. Ne pas
distribuer l'attention equitablement entre toutes les fonctions.

### Loi de Tesler (Conservation de la complexite)

Toute application a une complexite irreductible. La question est : qui la porte ?
L'utilisateur ou le systeme ?
**Action :** Le systeme doit absorber la complexite (smart defaults, auto-detection,
simplification) plutot que de la deleguer a l'utilisateur.

### Von Restorff Effect (Isolation Effect)

Un element qui se distingue visuellement des autres est mieux retenu.
**Action :** Utiliser la differenciation visuelle pour les elements critiques
(CTA primaire, alerte, element nouveau). Mais avec parcimonie : si tout est mis en
evidence, rien ne l'est.

---

## Accessibilite -- guide operationnel

### Contraste couleur

Texte normal : ratio minimum **4.5:1** (WCAG AA).
Texte large (>= 18px, ou >= 14px bold) : ratio minimum **3:1**.
Composants UI et focus : ratio minimum **3:1**.
**Ne jamais utiliser la couleur seule** pour transmettre une information. Toujours doubler
avec icone, texte ou pattern.

### Navigation clavier

Tous les elements interactifs sont atteignables via Tab.
L'ordre de tabulation suit l'ordre visuel logique.
Le focus est toujours visible (outline, couleur distincte).
Les raccourcis clavier ne chevauchent pas ceux de la plateforme.

### Lecteurs d'ecran

Les images ont un alt texte (ou aria-hidden si decoratives).
Les composants interactifs ont un label accessible.
La structure semantique (headings, landmarks, listes) est correcte.
Les changements dynamiques sont annonces (aria-live).

### Taille des cibles tactiles

Minimum **44x44px** (WCAG 2.5.5) pour les cibles tactiles.
Espacement minimum de **8px** entre les cibles.
Les actions destructives ne sont PAS adjacentes aux actions constructives.

### Mouvement et animation

Respecter `prefers-reduced-motion`. Pas d'animation auto-play sans controle utilisateur.
Les animations transmettent du sens (transition d'etat, feedback), pas de la decoration.

### Lisibilite du texte

Taille minimale corps : **16px** (14px max pour texte secondaire).
Interligne >= **1.5** pour le corps.
Longueur de ligne : **45-75 caracteres**.
Texte redimensionnable jusqu'a 200% sans perte de fonctionnalite.

---

## Tenets & Traps -- grille d'evaluation duale

Concept inspire de Memi : evaluer une interface sur ses **qualites a proteger** (tenets)
et ses **pieges a eviter** (traps).

### Tenets (qualites a proteger)

| Tenet | Description | Question de verification |
|-------|-------------|------------------------|
| **Clarte** | L'utilisateur comprend instantanement ce qu'il peut faire | Un nouvel utilisateur sait-il quoi faire en < 5 secondes ? |
| **Feedback** | Chaque action produit une reponse perceptible | Y a-t-il un retour visible pour chaque interaction ? |
| **Coherence** | Memes patterns, memes mots, memes comportements partout | Un element identique se comporte-t-il toujours pareil ? |
| **Reversibilite** | L'utilisateur peut defaire ses actions | Chaque action destructive a-t-elle un undo ou une confirmation ? |
| **Accessibilite** | L'interface fonctionne pour tous (clavier, lecteur d'ecran, contraste) | L'interface est-elle utilisable sans souris et avec un contraste suffisant ? |
| **Progression** | L'utilisateur sait ou il en est et ce qui reste | Les indicateurs de progression sont-ils presents dans les processus multi-etapes ? |

### Traps (pieges a eviter)

| Trap | Description | Signal d'alerte |
|------|-------------|----------------|
| **Mystery Meat Navigation** | Elements cliquables non identifiables comme tels | L'utilisateur doit-il deviner ce qui est interactif ? |
| **Information Overload** | Trop d'informations sur un seul ecran | Plus de 7 groupes d'information distincts visibles ? |
| **Invisible System State** | L'utilisateur ne sait pas ce qui se passe | Actions sans feedback, chargement sans indicateur ? |
| **Dead End** | Aucune action possible apres une erreur ou un etat vide | L'ecran d'erreur ou l'etat vide a-t-il un CTA ? |
| **Forced Recall** | L'utilisateur doit se souvenir d'informations vues precedemment | Des informations critiques disparaissent-elles entre les etapes ? |
| **Premature Commitment** | Demander un choix irreversible trop tot | L'utilisateur peut-il revenir sur ses choix facilement ? |

---

## D4D -- Design for Delight (cadre d'empathie)

Framework en trois niveaux pour valider les decisions produit :

1. **Fonctionnel** -- Le produit fait-il ce qu'il est cense faire ? (baseline)
2. **Fiable** -- Le produit est-il previsible, rapide, sans erreur ? (confiance)
3. **Delightful** -- Le produit depasse-t-il les attentes ? (differenciation)

**Usage :** Toujours assurer le niveau 1 et 2 avant de travailler le niveau 3.
Un produit "delightful" mais dysfonctionnel detruit la confiance.

---

## Comment utiliser cette reference

- **Pendant un audit UX** : parcourir les 10 heuristiques Nielsen et les 6 traps pour identifier les violations
- **Pendant une spec UX** : verifier que les principes Gestalt sont respectes dans le layout propose
- **Pendant une spec composant** : verifier accessibilite (contraste, clavier, cibles tactiles)
- **Pour justifier une decision** : citer le principe par son nom (ex: "Loi de Hick -- reduire les options")
- **Pour un audit rapide** : utiliser la grille Tenets (6 qualites a verifier) puis Traps (6 pieges a detecter)
