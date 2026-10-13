---
name: content-design
description: "Utiliser pour rédiger les textes d'interface : erreurs, états vides, notifications, formulaires, voice & tone."
---

# Content Design — référence

Le wording d'une interface n'est pas une décoration : c'est du design.
Un bon message d'erreur économise un ticket de support. Un bon empty state transforme
un nouvel utilisateur en utilisateur actif.

---

## 11 principes d'UX writing

### 1. Clarté avant tout

Chaque ligne d'interface doit être comprise instantanément. Si l'utilisateur marque
une pause pour comprendre, le texte a échoué. Préférer un mot simple à un mot élégant.

### 2. Front-load le sens

Le mot le plus important en premier. Les utilisateurs scannent — ils lisent rarement
jusqu'au bout d'une phrase. "Enregistrer les modifications" plutôt que "Cliquez ici pour
enregistrer vos modifications".

### 3. Voix active

La voix active est plus courte, plus claire et nomme l'acteur. La voix passive masque
la responsabilité et allonge la phrase.
- Actif : "Vous avez supprimé le fichier" (clair)
- Passif : "Le fichier a été supprimé" (par qui ?)

### 4. Être spécifique

Remplacer les mots génériques par des mots concrets. Le spécifique construit la confiance ;
le générique crée le doute.
- Générique : "Une erreur est survenue"
- Spécifique : "Le serveur de paiement ne répond pas"

### 5. Reconnaître l'utilisateur

Écrire à "vous" — une vraie personne avec un objectif, une contrainte de temps et des
émotions. Ne pas écrire pour un système ou un admin.

### 6. Utiliser les mots de l'utilisateur

Utiliser le vocabulaire que l'utilisateur connaît déjà. Les noms internes du produit,
le jargon d'équipe et les termes techniques n'ont pas leur place dans l'interface.

### 7. Terminologie cohérente

Un concept = un mot — utilisé partout. "Projet", "espace de travail", "workspace" : choisir
un seul terme. L'incohérence fait croire à l'utilisateur que ce sont des choses différentes.

### 8. Anatomie d'un message d'erreur

Un bon message d'erreur a trois parties :
1. **Quoi** : ce qui s'est passé ("Le paiement a échoué")
2. **Pourquoi** : la cause probable ("Votre carte a été refusée")
3. **Comment** : l'action à prendre ("Vérifiez les informations ou essayez une autre carte")

Jamais de message qui se contente de dire quoi sans pourquoi ni comment.

### 9. Structure scannable

Les utilisateurs scannent avant de lire. Structurer le texte pour que le sens soit
extractible en deux secondes : titres, listes à puces, gras sur les mots clés.

### 10. Langage inclusif

Écrire pour tous les lecteurs. Éviter le langage qui exclut, stéréotype ou suppose.
Pas de références culturelles implicites. Pas de jargon genré.

### 11. Voix vs Ton

La **voix** d'un produit ne change pas (formelle, décontractée, technique).
Le **ton** s'adapte à la situation :
- Erreur : empathique, orienté solution ("On va corriger ça")
- Succès : célébratoire mais sobre ("Bien joué, c'est enregistré")
- Neutre : informatif et direct ("3 tâches restantes")
- Danger : sérieux et clair ("Cette action est irréversible")

---

## Copy patterns par contexte

### Messages d'erreur

**Structure :** Quoi + Pourquoi + Comment

| Situation | Mauvais | Bon |
|-----------|---------|-----|
| Champ invalide | "Erreur de validation" | "L'email doit contenir un @ (ex : nom@example.com)" |
| Serveur down | "Error 500" | "Le serveur ne répond pas. Réessayez dans quelques instants." |
| Permission refusée | "Forbidden" | "Vous n'avez pas accès à cette page. Contactez votre admin." |
| Timeout | "Request timed out" | "L'opération prend plus de temps que prévu. Réessayez ou vérifiez votre connexion." |

**Règles :**
- Jamais de code HTTP ou d'identifiant technique
- Jamais de blame ("Vous avez fait une erreur")
- Toujours une action suggérée
- Ton empathique : "Nous comprenons que c'est frustrant"

### Empty states (textes d'accompagnement)

| Contexte | Structure |
|----------|-----------|
| Premier usage | Titre + 1 phrase de bénéfice + CTA : "Aucun projet pour l'instant. Créez votre premier projet pour commencer à organiser votre travail." + [Créer un projet] |
| Recherche vide | Confirmation de la requête + suggestion : "Aucun résultat pour \<requête\>. Vérifiez l'orthographe ou essayez un terme plus général." |
| Liste vidée | Confirmation positive + action suivante : "Toutes les tâches sont terminées ! Revenez demain ou créez un nouveau projet." |
| Filtre vide | Description + reset : "Aucun élément ne correspond à vos filtres. Essayez de retirer un filtre ou [voir tous les éléments]." |

**Règles :**
- Jamais de page blanche sans texte
- Toujours un CTA ou une action suivante
- Ton positif et guidant, pas accusateur

### Notifications

| Type | Ton | Exemple |
|------|-----|---------|
| Succès | Célébratoire sobre | "Fichier enregistré." / "Modifications publiées." |
| Info | Neutre | "3 nouvelles mises à jour disponibles." |
| Warning | Préventif | "Votre abonnement expire dans 5 jours." |
| Error | Empathique + action | "Impossible d'enregistrer. Vérifiez votre connexion et réessayez." |

**Règles :**
- Court (1 phrase max pour les toasts)
- Commencer par le résultat, pas par l'action
- "Fichier enregistré" (pas "Le fichier a été enregistré avec succès")

### Validation de formulaires

| Timing | Message |
|--------|---------|
| Hint (avant saisie) | Texte d'aide sous le champ : "Minimum 8 caractères, une majuscule et un chiffre" |
| Succès (au blur) | Checkmark vert — pas de texte nécessaire |
| Erreur (au blur) | Texte rouge sous le champ : spécification précise de ce qui est attendu |
| Summary (à la soumission) | "2 champs à corriger" en haut du formulaire + scroll vers le premier champ en erreur |

---

## Labels et microcopy

### Boutons

Le texte d'un bouton est un verbe d'action. Il décrit ce qui va se passer au clic.

| Mauvais | Bon |
|---------|-----|
| "Soumettre" | "Enregistrer les modifications" |
| "OK" | "Supprimer le projet" |
| "Oui" | "Confirmer la suppression" |
| "Cliquez ici" | "Télécharger le rapport" |

### Titres de page

Le titre de page répond à "Où suis-je ?". Court, descriptif, unique.

### Placeholders

Le placeholder n'est **pas** un label. Il disparaît à la saisie.
L'utiliser pour montrer le format attendu ("nom@example.com") ou un exemple ("Rechercher...").
Jamais d'instruction critique en placeholder (elle disparaît).

---

## Voice systems de référence

Quatre modèles de ton/voix issus de design systems reconnus :

### Conversational Product Voice (standard SaaS)

Registre amical, direct, concret, chaleureux sans en faire trop, calme en cas d'erreur.
C'est le ton que la plupart des produits SaaS visent. Neutre quant à la source, il synthétise des
standards de langage clair.

### GOV.UK (standard gouvernemental)

Langage clair absolu, zéro marketing, âge de lecture 9 ans. Le gold standard pour l'écriture
qui doit fonctionner pour tout le monde. Pas de jargon, pas de métaphores, pas de superlatifs.

### Shopify Polaris (commerce)

Ton confiant, habilitant, orienté action. Écrit pour des marchands qui veulent avancer.
Encourage sans condescendre.

### Atlassian (collaboration)

Ton audacieux, optimiste, pratique. Écrit pour des équipes. Collaboratif dans le ton,
jamais condescendant. Permet l'humour subtil mais jamais aux dépens de la clarté.

---

## Anti-patterns de content design

| Anti-pattern | Problème | Correction |
|--------------|----------|------------|
| **Jargon technique exposé** | "NullPointerException" dans l'UI | Traduire en langage humain |
| **Messages vagues** | "Une erreur est survenue" | Spécifier quoi, pourquoi, comment |
| **Double négation** | "Ne voulez-vous pas ne pas supprimer ?" | Phrase affirmative directe |
| **Blame utilisateur** | "Vous avez entré un email invalide" | "L'email doit contenir un @" |
| **Wall of text** | Paragraphe de 10 lignes dans une modal | Listes à puces, titres, gras |
| **Ton incohérent** | Formel dans les erreurs, décontracté dans les succès | Définir et respecter une voix |
| **Placeholder comme label** | Label invisible quand le champ est rempli | Label permanent au-dessus du champ |
| **Humour en erreur** | "Oups ! On dirait que quelque chose a planté :)" | Empathie sans minimiser |
