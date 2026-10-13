---
name: audit-accessibility
description: "Utiliser quand l'audit porte sur l'accessibilité : grille WCAG 2.1 AA / RGAA 4.1 pour HTML, CSS, JS et composants d'interface."
---

# Skill — Audit Accessibilité

## Référentiels couverts

- **WCAG 2.1** (Web Content Accessibility Guidelines) — niveaux A et AA obligatoires
- **RGAA 4.1** (Référentiel Général d'Amélioration de l'Accessibilité) — obligation légale en France (loi du 11 février 2005 + décret 2019-768)
- **ARIA 1.2** (Accessible Rich Internet Applications) — WAI-ARIA patterns
- **EN 301 549** — norme européenne d'accessibilité numérique

> Le RGAA 4.1 reprend les WCAG 2.1 AA avec des tests et critères adaptés au contexte français.
> La conformité RGAA implique la conformité WCAG 2.1 AA.

---

## Niveaux de conformité WCAG

| Niveau | Obligation | Description |
|--------|-----------|-------------|
| **A** | Obligatoire | Barrières majeures d'accès |
| **AA** | Obligatoire (loi française) | Critères standard — cible de conformité |
| **AAA** | Recommandé | Critères avancés — best effort |

---

## Checklist — Principe 1 : Perceptible

### 1.1 — Alternatives textuelles

- [ ] **[A]** Toutes les images informatives ont un attribut `alt` descriptif
- [ ] **[A]** Les images décoratives ont `alt=""` (vide, pas absent)
- [ ] **[A]** Les images de boutons ou liens ont un `alt` décrivant l'action (pas le visuel)
- [ ] **[A]** Les CAPTCHA ont une alternative (audio ou autre)
- [ ] Les graphiques SVG complexes ont une description via `aria-labelledby` ou `<title>` + `<desc>`

### 1.2 — Médias temporels

- [ ] **[A]** Les vidéos avec dialogue ont des sous-titres synchronisés
- [ ] **[AA]** Les vidéos ont une audiodescription (ou une alternative textuelle complète)
- [ ] **[A]** Les médias audio en direct ont une alternative textuelle
- [ ] Les contrôles de lecture sont accessibles au clavier

### 1.3 — Adaptabilité

- [ ] **[A]** La structure sémantique HTML est utilisée correctement
  - `<h1>`–`<h6>` pour la hiérarchie de titres (pas de sauts)
  - `<nav>`, `<main>`, `<header>`, `<footer>`, `<aside>` pour les régions
  - `<ul>`, `<ol>`, `<dl>` pour les listes
- [ ] **[A]** Les tableaux de données ont `<th>` avec `scope` approprié
- [ ] **[A]** Les formulaires ont des `<label>` explicitement associés à chaque champ (`for`/`id`)
- [ ] **[AA]** L'ordre de lecture dans le DOM correspond à l'ordre visuel attendu
- [ ] **[AA]** L'orientation de la page n'est pas verrouillée (portrait/paysage libres)
- [ ] **[AA]** L'identification des champs de formulaire ne repose pas uniquement sur des indices sensoriels (couleur, position)

### 1.4 — Distinguable

- [ ] **[AA]** Rapport de contraste texte normal : ≥ 4.5:1
- [ ] **[AA]** Rapport de contraste grand texte (≥ 18pt ou 14pt gras) : ≥ 3:1
- [ ] **[AA]** Rapport de contraste composants d'interface (bordures de champs, icônes) : ≥ 3:1
- [ ] **[A]** L'information n'est jamais transmise par la couleur seule
- [ ] **[AA]** Le texte peut être redimensionné à 200% sans perte de contenu ni de fonctionnalité
- [ ] **[AA]** Les textes ne sont pas présentés sous forme d'image (sauf logo)
- [ ] **[AA]** L'espacement du texte peut être modifié sans perte de contenu (ligne ×1.5, paragraphe ×2, lettres ×0.12em, mots ×0.16em)
- [ ] **[AAA]** Rapport de contraste texte normal : ≥ 7:1

---

## Checklist — Principe 2 : Utilisable

### 2.1 — Accessibilité clavier

- [ ] **[A]** Toutes les fonctionnalités sont accessibles au clavier (Tab, Shift+Tab, Entrée, Espace, flèches)
- [ ] **[A]** Aucun piège clavier (il est possible de sortir de tout composant avec le clavier seul)
- [ ] **[AAA]** Tous les raccourcis clavier à une touche peuvent être désactivés ou remappés

### 2.2 — Délais suffisants

- [ ] **[A]** Les sessions avec délai d'expiration avertissent l'utilisateur et permettent une extension
- [ ] **[A]** Les contenus qui bougent automatiquement peuvent être mis en pause, arrêtés ou cachés
- [ ] **[AA]** Aucune limite de temps n'est imposée sauf exception documentée

### 2.3 — Convulsions et réactions physiques

- [ ] **[A]** Aucun contenu ne clignote plus de 3 fois par seconde
- [ ] **[AAA]** Seuil de flash réduit (< 3 flashes, < seuil de zone)

### 2.4 — Navigation

- [ ] **[A]** Un mécanisme de saut vers le contenu principal est présent ("Aller au contenu" en lien en tête de page)
- [ ] **[A]** Les pages ont un `<title>` descriptif et unique
- [ ] **[A]** L'ordre de focus est logique et cohérent avec la présentation visuelle
- [ ] **[A]** Le focus clavier est toujours visible (outline non supprimé sans alternative)
- [ ] **[AA]** Les pages ont des titres et des libellés descriptifs pour faciliter la navigation
- [ ] **[AA]** La navigation multiple est disponible (menu, fil d'Ariane, plan du site)
- [ ] **[AA]** Le focus est visible avec un rapport de contraste ≥ 3:1 sur la zone de focus

### 2.5 — Modalités d'entrée

- [ ] **[AA]** Les gestes complexes (pinch, swipe multi-doigts) ont une alternative simple
- [ ] **[AA]** Les fonctionnalités activées au pointeur peuvent être annulées (mouseup vs mousedown)
- [ ] **[AA]** Les libellés visuels des champs correspondent à leur nom accessible (`aria-label`)
- [ ] **[AA]** L'authentification ne repose pas sur un test cognitif sans alternative

---

## Checklist — Principe 3 : Compréhensible

### 3.1 — Lisibilité

- [ ] **[A]** La langue principale de la page est déclarée (`<html lang="fr">`)
- [ ] **[AA]** Les changements de langue dans le contenu sont indiqués (`lang="en"` sur les passages en anglais)

### 3.2 — Prévisibilité

- [ ] **[A]** La réception du focus ne déclenche pas de changement de contexte automatique
- [ ] **[A]** Le changement de valeur d'un champ ne déclenche pas de navigation automatique sans avertissement
- [ ] **[AA]** La navigation est cohérente sur toutes les pages
- [ ] **[AA]** Les éléments identiques ont des libellés identiques sur toutes les pages

### 3.3 — Assistance à la saisie

- [ ] **[A]** Les erreurs de formulaire sont décrites textuellement (pas uniquement par couleur)
- [ ] **[A]** Les champs obligatoires sont indiqués
- [ ] **[AA]** Des suggestions de correction sont fournies quand possible
- [ ] **[AA]** Les soumissions importantes (paiement, suppression) sont vérifiables ou annulables
- [ ] L'attribut `autocomplete` est renseigné sur les champs standards (nom, email, adresse...)

---

## Checklist — Principe 4 : Robuste

### 4.1 — Compatibilité

- [ ] **[A]** Le HTML est valide (pas d'éléments mal fermés, doublons d'`id`)
- [ ] **[A]** Les composants ARIA ont les rôles, états et propriétés requis
  - Un bouton custom a `role="button"` + `tabindex="0"` + `onkeydown` (Enter/Space)
  - Un modal a `role="dialog"` + `aria-modal="true"` + `aria-labelledby`
  - Un menu a `role="menu"` + items en `role="menuitem"`
- [ ] **[A]** Les messages de statut dynamiques utilisent `aria-live` ou `role="status"`
- [ ] **[AA]** Tous les composants interactifs ont un nom accessible (texte visible, `aria-label`, ou `aria-labelledby`)

---

## Checklist RGAA 4.1 — Points spécifiques

### Thématique 8 — Éléments obligatoires

- [ ] La page possède un `<title>` non vide
- [ ] La page possède une déclaration de type de document (`<!DOCTYPE html>`)
- [ ] Les jeux de caractères sont déclarés (`<meta charset="UTF-8">`)

### Thématique 9 — Structuration de l'information

- [ ] La page possède au moins un titre de niveau 1 (`<h1>`)
- [ ] La hiérarchie des titres est strictement croissante (pas de saut de h1 à h3)
- [ ] Les listes utilisent les balises sémantiques appropriées (`<ul>`, `<ol>`, `<dl>`)

### Thématique 12 — Navigation

- [ ] Un lien d'évitement vers le contenu principal est présent et fonctionnel
- [ ] Les ensembles de pages ont une navigation cohérente (identique en structure)
- [ ] Le fil d'Ariane est présent sur les sites complexes

### Déclaration d'accessibilité (obligation légale)

- [ ] Une déclaration d'accessibilité est publiée sur le site
- [ ] La déclaration mentionne le niveau de conformité (total, partiel, non conforme)
- [ ] La déclaration liste les non-conformités avec les critères RGAA concernés
- [ ] Un moyen de contact pour les problèmes d'accessibilité est fourni

---

> Les outils de vérification (axe-core CLI, Lighthouse accessibility, html-validate, etc.)
> sont référencés dans `docs/reference/audit-tools.md` pour usage humain.
> Les outils automatisés couvrent environ 30-40% des critères WCAG — les tests manuels restent indispensables.

## Ce que tu ne fais PAS dans ce domaine

- Tester avec un lecteur d'écran réel (pas d'accès à un environnement d'exécution)
- Valider le rendu visuel réel (contraste, positionnement) — uniquement analyse du code source
- Certifier la conformité RGAA — seul un audit par un expert habilité peut certifier
- Ignorer les critères de niveau A pour se concentrer sur AA — les niveaux sont cumulatifs
