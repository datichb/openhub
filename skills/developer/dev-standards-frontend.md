---
name: dev-standards-frontend
description: Bonnes pratiques frontend agnostiques du framework — architecture, composants, CSS, performance, gestion des erreurs.
---

# Skill — Standards Frontend (Agnostique)

## Rôle
Ce skill définit les bonnes pratiques frontend indépendantes du framework.
Il complète `dev-standards-universal.md` et s'applique à tout projet frontend.

---

## 🔒 Gestion de données — Règle héritée

Toute décision liée à la gestion de données côté frontend est soumise
à validation explicite de l'utilisateur. Voir `dev-standards-universal.md`.

**Spécifique frontend :**
- Choix de la stratégie de state management
- Organisation et découpage des stores
- Stratégie de cache des requêtes API
- Gestion des états asynchrones (loading, error, success)

Pour le guide complet de décision (état local, Context Provider, Store, Queries, Cookies,
Web Storage, IndexedDB, Query String), voir `dev-standards-frontend-data`.

---

## Architecture & Organisation

- Séparation stricte entre logique métier et présentation
- La logique métier ne réside jamais dans les composants UI
- Les composants UI sont sans état métier — ils reçoivent et émettent
- Découpage des composants selon le principe de responsabilité unique

---

## Composants

- Un composant = une responsabilité claire
- Props en entrée, événements en sortie — flux unidirectionnel
- Pas de side effects directs dans les composants de présentation
- Composants réutilisables extraits dès la deuxième occurrence

---

## CSS & Styles

- Pas de styles globaux non justifiés
- Variables CSS pour les valeurs répétées (couleurs, espacements, typographies)
- Mobile-first systématiquement
- Pas de valeurs magiques — toute valeur doit être une variable ou justifiée

---

## Performance

- Lazy loading des routes et composants lourds
- Pas d'optimisation prématurée — mesurer avant d'optimiser
- Images optimisées et dimensionnées correctement
- Pas de re-renders inutiles

---

## Gestion des erreurs

- Tout appel asynchrone a un état d'erreur géré explicitement
- Pas de `console.log` laissé en production
- Messages d'erreur utilisateur clairs et non techniques
