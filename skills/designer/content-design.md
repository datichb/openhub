---
name: content-design
description: Principes d'UX writing et content design -- redaction des messages d'interface, copy patterns par contexte (erreurs, etats vides, notifications, formulaires), voice & tone, voice systems de reference.
---

# Content Design -- reference

Le wording d'une interface n'est pas une decoration : c'est du design.
Un bon message d'erreur economise un ticket de support. Un bon empty state transforme
un nouvel utilisateur en utilisateur actif.

---

## 11 principes d'UX writing

### 1. Clarte avant tout

Chaque ligne d'interface doit etre comprise instantanement. Si l'utilisateur marque
une pause pour comprendre, le texte a echoue. Preferer un mot simple a un mot elegant.

### 2. Front-load le sens

Le mot le plus important en premier. Les utilisateurs scannent -- ils lisent rarement
jusqu'au bout d'une phrase. "Enregistrer les modifications" plutot que "Cliquez ici pour
enregistrer vos modifications".

### 3. Voix active

La voix active est plus courte, plus claire et nomme l'acteur. La voix passive masque
la responsabilite et allonge la phrase.
- Actif : "Vous avez supprime le fichier" (clair)
- Passif : "Le fichier a ete supprime" (par qui ?)

### 4. Etre specifique

Remplacer les mots generiques par des mots concrets. Le specifique construit la confiance ;
le generique cree le doute.
- Generique : "Une erreur est survenue"
- Specifique : "Le serveur de paiement ne repond pas"

### 5. Reconnaitre l'utilisateur

Ecrire a "vous" -- une vraie personne avec un objectif, une contrainte de temps et des
emotions. Ne pas ecrire pour un systeme ou un admin.

### 6. Utiliser les mots de l'utilisateur

Utiliser le vocabulaire que l'utilisateur connait deja. Les noms internes du produit,
le jargon d'equipe et les termes techniques n'ont pas leur place dans l'interface.

### 7. Terminologie coherente

Un concept = un mot -- utilise partout. "Projet", "espace de travail", "workspace" : choisir
un seul terme. L'inconsistance fait douter l'utilisateur que ce sont des choses differentes.

### 8. Anatomie d'un message d'erreur

Un bon message d'erreur a trois parties :
1. **Quoi** : ce qui s'est passe ("Le paiement a echoue")
2. **Pourquoi** : la cause probable ("Votre carte a ete refusee")
3. **Comment** : l'action a prendre ("Verifiez les informations ou essayez une autre carte")

Jamais de message qui se contente de dire quoi sans pourquoi ni comment.

### 9. Structure scannable

Les utilisateurs scannent avant de lire. Structurer le texte pour que le sens soit
extractible en deux secondes : titres, listes a puces, gras sur les mots cles.

### 10. Langage inclusif

Ecrire pour tous les lecteurs. Eviter le langage qui exclut, stereotypet ou suppose.
Pas de references culturelles implicites. Pas de jargon genere.

### 11. Voix vs Ton

La **voix** d'un produit ne change pas (formelle, decontractee, technique).
Le **ton** s'adapte a la situation :
- Erreur : empathique, orient solution ("On va corriger ca")
- Succes : celebratoire mais sobre ("Bien joue, c'est enregistre")
- Neutre : informatif et direct ("3 taches restantes")
- Danger : serieux et clair ("Cette action est irreversible")

---

## Copy patterns par contexte

### Messages d'erreur

**Structure :** Quoi + Pourquoi + Comment

| Situation | Mauvais | Bon |
|-----------|---------|-----|
| Champ invalide | "Erreur de validation" | "L'email doit contenir un @ (ex: nom@example.com)" |
| Serveur down | "Error 500" | "Le serveur ne repond pas. Reessayez dans quelques instants." |
| Permission refusee | "Forbidden" | "Vous n'avez pas acces a cette page. Contactez votre admin." |
| Timeout | "Request timed out" | "L'operation prend plus de temps que prevu. Reessayez ou verifiez votre connexion." |

**Regles :**
- Jamais de code HTTP ou d'identifiant technique
- Jamais de blame ("Vous avez fait une erreur")
- Toujours une action suggeree
- Ton empathique : "Nous comprenons que c'est frustrant"

### Empty states (textes d'accompagnement)

| Contexte | Structure |
|----------|-----------|
| Premier usage | Titre + 1 phrase de benefice + CTA : "Aucun projet pour l'instant. Creez votre premier projet pour commencer a organiser votre travail." + [Creer un projet] |
| Recherche vide | Confirmation de la requete + suggestion : "Aucun resultat pour \<requete\>. Verifiez l'orthographe ou essayez un terme plus general." |
| Liste videe | Confirmation positive + action suivante : "Toutes les taches sont terminees ! Revenez demain ou creez un nouveau projet." |
| Filtre vide | Description + reset : "Aucun element ne correspond a vos filtres. Essayez de retirer un filtre ou [voir tous les elements]." |

**Regles :**
- Jamais de page blanche sans texte
- Toujours un CTA ou une action suivante
- Ton positif et guidant, pas accusateur

### Notifications

| Type | Ton | Exemple |
|------|-----|---------|
| Succes | Celebratoire sobre | "Fichier enregistre." / "Modifications publiees." |
| Info | Neutre | "3 nouvelles mises a jour disponibles." |
| Warning | Preventif | "Votre abonnement expire dans 5 jours." |
| Error | Empathique + action | "Impossible d'enregistrer. Verifiez votre connexion et reessayez." |

**Regles :**
- Court (1 phrase max pour les toasts)
- Commencer par le resultat, pas par l'action
- "Fichier enregistre" (pas "Le fichier a ete enregistre avec succes")

### Validation de formulaires

| Timing | Message |
|--------|---------|
| Hint (avant saisie) | Texte d'aide sous le champ : "Minimum 8 caracteres, une majuscule et un chiffre" |
| Succes (au blur) | Checkmark vert -- pas de texte necessaire |
| Erreur (au blur) | Texte rouge sous le champ : specification precise de ce qui est attendu |
| Summary (a la soumission) | "2 champs a corriger" en haut du formulaire + scroll vers le premier champ en erreur |

---

## Labels et microcopy

### Boutons

Le texte d'un bouton est un verbe d'action. Il decrit ce qui va se passer au clic.

| Mauvais | Bon |
|---------|-----|
| "Soumettre" | "Enregistrer les modifications" |
| "OK" | "Supprimer le projet" |
| "Oui" | "Confirmer la suppression" |
| "Cliquez ici" | "Telecharger le rapport" |

### Titres de page

Le titre de page repond a "Ou suis-je ?". Court, descriptif, unique.

### Placeholders

Le placeholder n'est **pas** un label. Il disparait a la saisie.
Utiliser pour montrer le format attendu ("nom@example.com") ou un exemple ("Rechercher...").
Jamais d'instruction critique en placeholder (elle disparait).

---

## Voice systems de reference

Quatre modeles de ton/voix issus de design systems reconnus :

### Conversational Product Voice (standard SaaS)

Registre amical, direct, concret, chaleureux sans en faire trop, calme en cas d'erreur.
C'est le ton que la plupart des produits SaaS visent. Source-neutre, synthetise des
standards de langage clair.

### GOV.UK (standard gouvernemental)

Langage clair absolu, zero marketing, age de lecture 9 ans. Le gold standard pour l'ecriture
qui doit fonctionner pour tout le monde. Pas de jargon, pas de metaphores, pas de superlatifs.

### Shopify Polaris (commerce)

Ton confiant, habilitant, oriente action. Ecrit pour des marchands qui veulent avancer.
Encourage sans condescendre.

### Atlassian (collaboration)

Ton audacieux, optimiste, pratique. Ecrit pour des equipes. Collaboratif dans le ton,
jamais condescendant. Permet l'humour subtil mais jamais aux depens de la clarte.

---

## Anti-patterns de content design

| Anti-pattern | Probleme | Correction |
|--------------|----------|------------|
| **Jargon technique expose** | "NullPointerException" dans l'UI | Traduire en langage humain |
| **Messages vagues** | "Une erreur est survenue" | Specifier quoi, pourquoi, comment |
| **Double negation** | "Ne voulez-vous pas ne pas supprimer ?" | Phrase affirmative directe |
| **Blame utilisateur** | "Vous avez entre un email invalide" | "L'email doit contenir un @" |
| **Wall of text** | Paragraphe de 10 lignes dans une modal | Listes a puces, titres, gras |
| **Ton inconsistant** | Formel dans les erreurs, decontracte dans les succes | Definir et respecter une voix |
| **Placeholder comme label** | Label invisible quand le champ est rempli | Label permanent au-dessus du champ |
| **Humour en erreur** | "Oups ! On dirait que quelque chose a plante :)" | Empathie sans minimiser |
