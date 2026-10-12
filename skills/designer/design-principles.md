---
name: design-principles
description: Principes de design fondamentaux — heuristiques Nielsen enrichies, principes Gestalt, Laws of UX, accessibilité opérationnelle, grille Tenets & Traps. Référence partagée entre les modes ux et ui.
---

# Principes de design — référence

Ce skill est une référence. Ne pas le réciter en entier — l'utiliser pour justifier
des décisions et identifier des violations quand tu évalues un écran ou spécifies un composant.

---

## Heuristiques Nielsen — grille enrichie

Chaque heuristique a trois dimensions : définition, comment vérifier, violations courantes.

### 1. Visibilité de l'état du système

**Principe :** Le système informe toujours l'utilisateur de ce qui se passe, via un feedback
approprié dans un délai raisonnable.

**Vérifier :** Chaque action produit-elle un feedback visible ? Les états de chargement,
de progression et de succès sont-ils affichés ? L'utilisateur sait-il où il en est dans un processus multi-étapes ?

**Violations courantes :** Bouton sans feedback au clic, soumission de formulaire sans indicateur
de chargement, absence de barre de progression dans un wizard, pas de confirmation après une action.

### 2. Correspondance système / monde réel

**Principe :** Le système parle le langage de l'utilisateur (mots, concepts, conventions du monde réel),
pas le jargon interne.

**Vérifier :** Le vocabulaire est-il familier à l'utilisateur cible ? L'ordre des informations
suit-il la logique de l'utilisateur (pas celle de la base de données) ?

**Violations courantes :** IDs techniques exposés, messages d'erreur contenant des codes HTTP ou
des stack traces, vocabulaire interne ("entity", "record") au lieu de termes métier.

### 3. Contrôle et liberté utilisateur

**Principe :** Les utilisateurs font des erreurs. Ils ont besoin d'une "sortie de secours" clairement
visible pour quitter un état non désiré, sans passer par un processus compliqué.

**Vérifier :** L'utilisateur peut-il annuler, revenir en arrière, modifier ? Existe-t-il un undo ?
La navigation "retour" fonctionne-t-elle de façon prévisible ?

**Violations courantes :** Actions irréversibles sans confirmation, pas de bouton annuler dans un
wizard multi-étapes, modal sans moyen de fermer, suppression sans undo.

### 4. Cohérence et standards

**Principe :** Les mêmes mots, situations et actions signifient toujours la même chose.
Les conventions de la plateforme sont respectées.

**Vérifier :** Un même pattern d'interaction est-il identique partout ? Les composants visuellement
identiques se comportent-ils de la même façon ?

**Violations courantes :** Bouton "Valider" qui s'appelle "OK" ailleurs, comportement de clic
différent sur des éléments visuellement identiques, raccourcis clavier incohérents.

### 5. Prévention des erreurs

**Principe :** Plutôt que de bons messages d'erreur, empêcher le problème d'arriver.

**Vérifier :** Les actions destructives ont-elles une confirmation ? Les champs de formulaire
ont-ils des contraintes (type, format, limites) ? Le système désactive-t-il les actions
impossibles plutôt que de les laisser échouer ?

**Violations courantes :** Champ email sans validation de format, bouton supprimer sans
confirmation, formulaire soumissible avec des données invalides.

### 6. Reconnaissance plutôt que rappel

**Principe :** Minimiser la charge de mémoire en rendant les éléments, actions et options
visibles. L'utilisateur reconnaît plutôt qu'il ne se souvient.

**Vérifier :** Les options sont-elles visibles ? L'historique récent est-il accessible ?
Le contexte nécessaire est-il présent à chaque écran ?

**Violations courantes :** Menus profonds qui forcent à mémoriser le chemin, formulaires
sans valeurs par défaut, recherche sans suggestions.

### 7. Flexibilité et efficacité

**Principe :** Le système sert aussi bien les novices que les experts.
Les raccourcis accélèrent les tâches répétitives pour les experts.

**Vérifier :** Existe-t-il des raccourcis clavier ? Les actions fréquentes sont-elles
directement accessibles ? La personnalisation est-elle possible ?

**Violations courantes :** Pas de raccourcis clavier, pas de "récents", pas de favoris,
wizards obligatoires même pour les utilisateurs experts.

### 8. Esthétique et design minimaliste

**Principe :** Chaque élément d'information supplémentaire entre en compétition avec les
éléments pertinents et diminue leur visibilité relative.

**Vérifier :** Chaque élément présent est-il nécessaire ? Peut-on en retirer sans perdre
de fonctionnalité ? La hiérarchie visuelle guide-t-elle l'œil vers l'essentiel ?

**Violations courantes :** Trop d'informations sur un seul écran, décorations sans fonction,
textes d'aide qui polluent plus qu'ils n'aident.

### 9. Aide à la reconnaissance et à la récupération d'erreur

**Principe :** Les messages d'erreur expriment clairement le problème en langage naturel,
indiquent précisément la cause et suggèrent une solution.

**Vérifier :** Le message dit-il quoi (erreur), pourquoi (cause), comment (solution) ?
Le message est-il en langage humain (pas un code technique) ?

**Violations courantes :** "Error 500", "Invalid input", messages vagues sans action
suggérée, erreurs affichées loin du champ concerné.

### 10. Aide et documentation

**Principe :** Le système doit pouvoir être utilisé sans documentation, mais l'aide doit être
accessible sans interrompre le flow quand elle est nécessaire.

**Vérifier :** L'aide est-elle contextuelle (tooltips, inline) ? La documentation est-elle
cherchable ? Les textes d'aide sont-ils orientés tâche (pas théorie) ?

**Violations courantes :** Documentation uniquement externe, pas de tooltips sur les champs
complexes, aide qui ouvre une nouvelle page et fait perdre le contexte.

---

## Principes Gestalt

Les lois de la perception visuelle — comment le cerveau organise les éléments visuels en groupes.

### Proximité

Les éléments proches sont perçus comme un groupe. Utiliser l'espacement pour créer des groupes
logiques. Les éléments liés ont un espacement serré ; les groupes distincts sont espacés.

**Application TUI :** Lignes vides pour séparer les sections. Indentation pour les sous-groupes.
**Application Web :** Margins et gaps entre sections. Cards regroupant les éléments liés.

### Similarité

Les éléments visuellement semblables sont perçus comme liés. Forme, couleur, taille ou style
identiques = même catégorie.

**Application :** Tous les boutons d'action primaire ont le même style. Tous les labels de
formulaire partagent la même typographie. Les éléments interactifs se distinguent visuellement des
éléments statiques.

### Clôture

Le cerveau complète les formes incomplètes. On peut suggérer un conteneur sans le dessiner.

**Application TUI :** Bordures ASCII/Unicode pour délimiter les zones. L'indentation seule
suffit parfois à créer un "conteneur" implicite.
**Application Web :** Cards, ombre, bordure subtile — ou simplement un fond coloré.

### Continuité

L'œil suit les lignes et les courbes. Les éléments alignés sont perçus comme liés.

**Application :** Aligner les labels et les champs. Aligner les colonnes dans un tableau.
Ne pas rompre la ligne de lecture sans raison.

### Figure-Fond

Le cerveau distingue un objet (figure) de son arrière-plan (fond). L'élément important
doit se détacher visuellement du contexte.

**Application :** Modal au-dessus d'un fond assombri. Texte contrasté sur le fond.
Élément actif / focus mis en évidence par rapport aux éléments inactifs.

### Région commune

Les éléments dans une même région visuelle (encadré, fond coloré, bordure) sont perçus
comme un groupe, même s'ils ne sont pas proches.

**Application :** Cards, panels, sections encadrées. En TUI : boîtes Unicode.

---

## Laws of UX — les 12 lois les plus actionnables

### Loi de Hick

Le temps de décision augmente avec le nombre d'options.
**Action :** Réduire les choix visibles. Masquer les options avancées. Hiérarchiser.

### Loi de Fitts

Le temps d'acquisition d'une cible dépend de sa taille et de sa distance.
**Action :** Actions principales grandes et accessibles. Actions destructives petites et
éloignées. Boutons de validation larges.

### Peak-End Rule

Les utilisateurs jugent une expérience principalement sur son pic (moment le plus intense)
et sa fin — pas sur la moyenne de l'expérience.
**Action :** Soigner le premier écran (première impression) et le dernier écran
(confirmation, succès). Célébrer les accomplissements.

### Goal-Gradient Effect

La motivation augmente quand on approche du but.
**Action :** Afficher la progression (barre, étapes restantes). Montrer "Plus que 2 étapes".

### Aesthetic-Usability Effect

Les utilisateurs perçoivent les interfaces esthétiques comme plus faciles à utiliser.
**Action :** L'esthétique n'est pas du luxe — elle améliore la perception d'utilisabilité.
Investir dans la cohérence visuelle.

### Doherty Threshold

La productivité explose quand le système répond en < 400ms.
**Action :** Feedback immédiat (< 100ms pour les interactions directes, < 400ms pour les
résultats). Skeleton screens, optimistic UI.

### Loi de Miller

La mémoire de travail retient 7 +/- 2 éléments.
**Action :** Limiter les groupes à 5-9 éléments. Découper les listes longues en blocs. Regrouper.

### Loi de Jakob

Les utilisateurs passent la plupart de leur temps sur d'autres sites/apps.
Ils préfèrent que votre produit fonctionne comme ce qu'ils connaissent déjà.
**Action :** Suivre les conventions de la plateforme. Pas d'innovation dans les patterns
d'interaction de base (navigation, formulaires, modals).

### Serial Position Effect

Les utilisateurs retiennent mieux le premier et le dernier élément d'une liste.
**Action :** Placer les actions/informations importantes en début et en fin de liste ou de
navigation.

### Pareto Principle (80/20)

80 % des effets viennent de 20 % des causes.
**Action :** Identifier les 20 % de features les plus utilisées et les optimiser. Ne pas
distribuer l'attention équitablement entre toutes les fonctions.

### Loi de Tesler (Conservation de la complexité)

Toute application a une complexité irréductible. La question est : qui la porte ?
L'utilisateur ou le système ?
**Action :** Le système doit absorber la complexité (smart defaults, auto-détection,
simplification) plutôt que de la déléguer à l'utilisateur.

### Von Restorff Effect (Isolation Effect)

Un élément qui se distingue visuellement des autres est mieux retenu.
**Action :** Utiliser la différenciation visuelle pour les éléments critiques
(CTA primaire, alerte, élément nouveau). Mais avec parcimonie : si tout est mis en
évidence, rien ne l'est.

---

## Accessibilité — guide opérationnel

### Contraste couleur

Texte normal : ratio minimum **4.5:1** (WCAG AA).
Texte large (>= 18px, ou >= 14px bold) : ratio minimum **3:1**.
Composants UI et focus : ratio minimum **3:1**.
**Ne jamais utiliser la couleur seule** pour transmettre une information. Toujours doubler
avec une icône, du texte ou un motif.

### Navigation clavier

Tous les éléments interactifs sont atteignables via Tab.
L'ordre de tabulation suit l'ordre visuel logique.
Le focus est toujours visible (outline, couleur distincte).
Les raccourcis clavier ne chevauchent pas ceux de la plateforme.

### Lecteurs d'écran

Les images ont un texte alternatif (ou aria-hidden si décoratives).
Les composants interactifs ont un label accessible.
La structure sémantique (headings, landmarks, listes) est correcte.
Les changements dynamiques sont annoncés (aria-live).

### Taille des cibles tactiles

Minimum **44x44px** (WCAG 2.5.5) pour les cibles tactiles.
Espacement minimum de **8px** entre les cibles.
Les actions destructives ne sont PAS adjacentes aux actions constructives.

### Mouvement et animation

Respecter `prefers-reduced-motion`. Pas d'animation auto-play sans contrôle utilisateur.
Les animations transmettent du sens (transition d'état, feedback), pas de la décoration.

### Lisibilité du texte

Taille minimale du corps : **16px** (14px max pour le texte secondaire).
Interligne >= **1.5** pour le corps.
Longueur de ligne : **45-75 caractères**.
Texte redimensionnable jusqu'à 200 % sans perte de fonctionnalité.

---

## Tenets & Traps — grille d'évaluation duale

Concept inspiré de Memi : évaluer une interface sur ses **qualités à protéger** (tenets)
et ses **pièges à éviter** (traps).

### Tenets (qualités à protéger)

| Tenet | Description | Question de vérification |
|-------|-------------|------------------------|
| **Clarté** | L'utilisateur comprend instantanément ce qu'il peut faire | Un nouvel utilisateur sait-il quoi faire en < 5 secondes ? |
| **Feedback** | Chaque action produit une réponse perceptible | Y a-t-il un retour visible pour chaque interaction ? |
| **Cohérence** | Mêmes patterns, mêmes mots, mêmes comportements partout | Un élément identique se comporte-t-il toujours pareil ? |
| **Réversibilité** | L'utilisateur peut défaire ses actions | Chaque action destructive a-t-elle un undo ou une confirmation ? |
| **Accessibilité** | L'interface fonctionne pour tous (clavier, lecteur d'écran, contraste) | L'interface est-elle utilisable sans souris et avec un contraste suffisant ? |
| **Progression** | L'utilisateur sait où il en est et ce qui reste | Les indicateurs de progression sont-ils présents dans les processus multi-étapes ? |

### Traps (pièges à éviter)

| Trap | Description | Signal d'alerte |
|------|-------------|----------------|
| **Mystery Meat Navigation** | Éléments cliquables non identifiables comme tels | L'utilisateur doit-il deviner ce qui est interactif ? |
| **Information Overload** | Trop d'informations sur un seul écran | Plus de 7 groupes d'information distincts visibles ? |
| **Invisible System State** | L'utilisateur ne sait pas ce qui se passe | Actions sans feedback, chargement sans indicateur ? |
| **Dead End** | Aucune action possible après une erreur ou un état vide | L'écran d'erreur ou l'état vide a-t-il un CTA ? |
| **Forced Recall** | L'utilisateur doit se souvenir d'informations vues précédemment | Des informations critiques disparaissent-elles entre les étapes ? |
| **Premature Commitment** | Demander un choix irréversible trop tôt | L'utilisateur peut-il revenir sur ses choix facilement ? |

---

## D4D — Design for Delight (cadre d'empathie)

Framework en trois niveaux pour valider les décisions produit :

1. **Fonctionnel** — Le produit fait-il ce qu'il est censé faire ? (baseline)
2. **Fiable** — Le produit est-il prévisible, rapide, sans erreur ? (confiance)
3. **Delightful** — Le produit dépasse-t-il les attentes ? (différenciation)

**Usage :** Toujours assurer les niveaux 1 et 2 avant de travailler le niveau 3.
Un produit "delightful" mais dysfonctionnel détruit la confiance.

---

## Comment utiliser cette référence

- **Pendant un audit UX** : parcourir les 10 heuristiques Nielsen et les 6 traps pour identifier les violations
- **Pendant une spec UX** : vérifier que les principes Gestalt sont respectés dans le layout proposé
- **Pendant une spec composant** : vérifier l'accessibilité (contraste, clavier, cibles tactiles)
- **Pour justifier une décision** : citer le principe par son nom (ex : "Loi de Hick — réduire les options")
- **Pour un audit rapide** : utiliser la grille Tenets (6 qualités à vérifier) puis Traps (6 pièges à détecter)
