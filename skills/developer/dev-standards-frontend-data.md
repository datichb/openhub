---
name: dev-standards-frontend-data
description: Gestion des données côté frontend — choisir entre état local, Context Provider, Store centralisé, Queries, Cookies, Web Storage, IndexedDB ou Query String selon les caractéristiques de la donnée. Utiliser quand on parle de state management, cache, persistance, props drilling, fetch de données, localStorage, sessionStorage, cookies, IndexedDB, TanStack Query, Redux, Pinia, Vuex, Context API.
---

# Skill — Gestion des Données dans le Front

## Rôle

Ce skill guide les décisions de gestion de données côté frontend.
Il complète `dev-standards-universal.md` et `dev-standards-frontend.md`.

La donnée est une information pouvant être analysée ou interprétée au sein d'un système.
Il est essentiel de la catégoriser selon sa finalité, sa portée et sa durée de vie
avant de choisir comment la gérer.

---

## 🔒 Règle absolue — Validation obligatoire

Toute décision de gestion de données côté frontend est soumise au processus
de validation défini dans `dev-standards-universal.md`.

**Rappel du processus :**
1. Détecter le besoin lié aux données
2. Présenter 2 à 3 options adaptées au contexte, avec avantages et inconvénients
3. Attendre une validation **explicite** de l'utilisateur
4. N'implémenter qu'après confirmation claire

❌ Pas de choix par défaut imposé (ex. : "j'utilise Redux" sans discussion)
❌ Pas d'implémentation sans validation
✅ Poser les questions de caractérisation avant de proposer

---

## Caractériser sa donnée — Les 5 questions clés

Avant tout choix technique, poser ces questions sur la donnée concernée :

| # | Question | Impact sur le choix |
|---|---|---|
| 1 | **Accessibilité** — Doit-elle être accessible à tout le code ou limitée à un composant ? | Global → Store / Context. Local → État local |
| 2 | **Modifiabilité** — Est-elle modifiable ? Les modifications doivent-elles notifier les dépendances ? | Réactivité requise → State manager. Lecture seule → Cookie / Storage |
| 3 | **Persistance** — Doit-elle survivre à un rechargement ? À la fermeture du navigateur ? | Session → SessionStorage. Long terme → LocalStorage / Cookie. Serveur → API |
| 4 | **Portée** — Doit-elle être restreinte au composant, à la page ou à l'application entière ? | Composant → État local. Application → Store / Context |
| 5 | **Compatibilité SSR** — Le rendu serveur doit-il y accéder ? | Oui → Cookie ou props. Non → WebStorage / IndexedDB |

**Règle d'or : poser ces questions avant d'écrire la moindre ligne.**
L'écueil le plus fréquent est d'appliquer systématiquement une seule approche
pour tous les cas d'usage sans se poser ces questions.

---

## Tableau de décision

Matrice des mécanismes selon leurs caractéristiques :

| Mécanisme | Accessible à tout le code | Restreint au composant | Éditable | Notifie ses dépendances | Persisté sans client | Compatible SSR |
|---|:---:|:---:|:---:|:---:|:---:|:---:|
| **État local** | — | ✅ | ✅ | Partiel (props/events) | ❌ | ✅ |
| **Context Provider** | ✅ (sous-arbre) | — | ✅ | ✅ | ❌ | ✅ |
| **Store centralisé** | ✅ | — | ✅ | ✅ | ❌ | Partiel |
| **Queries (TanStack)** | ✅ | — | ✅ (mutations) | ✅ | ❌ (cache mémoire) | Partiel |
| **Cookies** | ✅ | — | ✅ | ❌ | ✅ | ✅ |
| **LocalStorage** | ✅ | — | ✅ | ❌ | ✅ (même navigateur) | ❌ |
| **SessionStorage** | ✅ | — | ✅ | ❌ | ❌ (onglet seulement) | ❌ |
| **IndexedDB** | ✅ | — | ✅ | ❌ | ✅ (même navigateur) | ❌ |
| **Query String** | ✅ | — | ✅ | ❌ | Partiel (URL) | ✅ |

---

## Fiches par mécanisme

### État local du composant

**Ce que c'est :**
L'état interne du composant. Restreint à son usage propre et à ses enfants via les props.

**Quand l'utiliser :**
- Toggle, champ de formulaire, état UI localisé (ouvert/fermé, tab actif)
- Données qui n'ont de sens que dans ce composant précis

**Trade-offs :**
- ✅ Simple, sans dépendance externe, facile à tester
- ✅ Pas d'impact sur le reste de l'application
- ❌ Non partageable sans props — attention au **props drilling** au-delà de 2 niveaux
- ❌ Perdu à chaque démontage du composant

**Points d'attention :**
- **Props drilling** : faire descendre de la donnée sur plusieurs couches de descendants → signe que la donnée doit monter vers un Context ou un Store
- **Event bubbling** : faire remonter un événement sur plusieurs couches de parents → même signal

---

### Context Provider

**Ce que c'est :**
Mécanisme natif des frameworks pour partager des données à travers l'arborescence
sans passer par les props. Bon compromis entre état local et Store centralisé.

**Quand l'utiliser :**
- Données partagées dans un sous-arbre de composants (thème, langue, auth locale)
- Éviter le props drilling sans avoir besoin d'un Store global

**Trade-offs :**
- ✅ Pas de librairie externe — natif dans tous les frameworks majeurs
- ✅ Source de vérité unique pour le sous-arbre concerné
- ❌ Peut devenir difficile à suivre si l'application jongle avec beaucoup de contextes
- ❌ Pas adapté à des données très volumineuses ou de complexité élevée

**Spécificités frameworks (informatives) :**
- **React** — Context API. Réactif par défaut : toute modification re-rend tous les consommateurs du contexte → surveiller les performances
- **Vue.js** — `provide` / `inject`. Non réactif par défaut → la réactivité doit être gérée explicitement (plus fin mais plus contraignant)
- **Angular** — Dependency Injection (DI) : injection de services dans la hiérarchie des composants
- **Svelte** — Context API sur le même principe que React

---

### Store centralisé

**Ce que c'est :**
Conteneur d'état global partagé par toute l'application. Exemples : Vuex, Pinia, Redux, NgRx.
L'état ne peut pas être altéré directement — les modifications passent par des **mutations** et des **actions**.

**Quand l'utiliser :**
- Données partagées à grande échelle, accessibles partout et à tout moment
- Flux de données complexes nécessitant traçabilité et contrôle strict
- Authentification, panier, données métier critiques partagées entre pages

**Trade-offs :**
- ✅ Source de vérité unique pour toute l'application
- ✅ Contrôle total sur les modifications (mutations/reducers)
- ✅ Outils de débogage puissants (Redux DevTools, Pinia DevTools)
- ❌ **Impact performance** : instance à part entière, coûteuse si mal dimensionnée
- ❌ **Complexité technique** : actions + mutations + reducers = code plus lourd pour de simples besoins
- ❌ **Effet "fourre-tout"** : le Store est pratique et disponible → risque de tout y mettre sans réflexion

**⚠️ Piège principal — l'effet "fourre-tout" :**
Le Store est pratique, on peut y retrouver la donnée facilement partout.
C'est exactement ce qui pousse à tout stocker sans se poser les questions
de l'utilisation et de la consommation de la donnée.
Sans rigueur, le Store devient vite ingérable.

---

### Queries (TanStack Query / React Query)

**Ce que c'est :**
Méthodes émergentes placées entre l'API et le rendu. Simplifient la gestion
des appels API asynchrones avec cache intégré et gestion de la validité des données.

**Quand l'utiliser :**
- Données provenant d'une API externe (fetch, REST, GraphQL)
- Besoin de cache, de revalidation automatique, de gestion de stale data
- États asynchrones : loading, error, success à gérer proprement

**Trade-offs :**
- ✅ Gestion directe et performante des appels API asynchrones
- ✅ Cache intégré avec gestion de la validité (`staleTime`, `gcTime`)
- ✅ Évite de dupliquer l'état serveur dans un Store global
- ✅ États asynchrones (loading, error, success) gérés nativement
- ❌ Données non persistées côté client (cache en mémoire uniquement)
- ❌ Ne remplace pas un Store pour les données purement locales et non serveur

**Règle :**
L'état serveur n'est pas dupliqué dans un Store global quand les Queries sont disponibles.
Les Queries sont la source de vérité pour les données provenant du serveur.

---

### Cookies

**Ce que c'est :**
Données stockées par le navigateur sur le terminal client, liées au domaine,
envoyées automatiquement dans chaque requête HTTP entre client et serveur.

**Quand l'utiliser :**
- Tokens de session utilisateur (avec `HttpOnly` + `Secure` + `SameSite`)
- Données nécessaires côté serveur à chaque requête
- Préférences de personnalisation du parcours client

**Trade-offs :**
- ✅ Accessible côté client ET côté serveur (compatible SSR)
- ✅ Configurable : domaine, protocole, durée de vie (`max-age`, `expires`)
- ❌ **Limité à 4 ko** par cookie
- ❌ Envoyé automatiquement dans chaque requête HTTP → vecteur d'attaque (XSS, CSRF)
- ❌ Les sites tiers autorisés peuvent déposer leurs propres cookies (tracking, profiling)

**⚠️ Ne jamais stocker de données sensibles** dont la divulgation pourrait causer préjudice.
Utiliser `HttpOnly` pour les tokens afin d'empêcher l'accès via JavaScript.

---

### Web Storage — LocalStorage & SessionStorage

**Ce que c'est :**
API JavaScript de stockage clé/valeur (chaînes de caractères) dans le navigateur.
Liées au couple site/protocole — une donnée HTTP n'est pas accessible en HTTPS.

**LocalStorage :**
- Persiste jusqu'à suppression manuelle
- Partagée entre tous les onglets du même domaine

**SessionStorage :**
- Durée de vie de l'onglet — effacée à sa fermeture
- Isolée par onglet

**Quand les utiliser :**
- Petits objets, données non complexes
- Données n'ayant pas vocation à être synchronisées immédiatement avec le serveur
- Préférences UI, filtres, états de navigation pour une expérience fluide
- Mode **offline-first** (avec IndexedDB pour les données volumineuses)

**Trade-offs :**
- ✅ Rapide et simple d'accès (synchrone)
- ✅ Pas d'expiration automatique pour LocalStorage
- ❌ **Non compatible SSR** — le navigateur n'existe pas côté serveur
- ❌ Stockage synchrone → peut bloquer le thread principal si mal utilisé
- ❌ Pas de notification des dépendances → pas de réactivité native
- ❌ Limité aux chaînes de caractères (sérialisation JSON nécessaire)

**⚠️ Ne jamais stocker de données sensibles** (tokens d'auth, données personnelles critiques).

---

### IndexedDB

**Ce que c'est :**
Base de données transactionnelle orientée objet dans le navigateur.
Stockage clé/valeur pour données volumineuses (fichiers, Blobs, objets complexes).

**Quand l'utiliser :**
- Données volumineuses : fichiers, images, objets complexes
- Applications devant fonctionner **hors connexion** (synchronisation différée)
- Cache local de données pour réduire les appels réseau
- Jeux web, applications offline-first

**Trade-offs :**
- ✅ Supporte des données volumineuses et complexes (contrairement à WebStorage)
- ✅ API asynchrone — ne bloque pas le thread principal
- ✅ Pilier du **Local-First** : fluidité même sans réseau
- ❌ API verbeux et complexe — utiliser une librairie d'abstraction (Dexie.js, idb)
- ❌ **Non compatible SSR**
- ❌ Pas de réactivité native

**⚠️ Ne jamais stocker de données sensibles** dont la divulgation pourrait causer préjudice.

---

### Query String

**Ce que c'est :**
Paramètres positionnés dans l'URL après un `?` : `https://site.fr/?cle=valeur&cle2=valeur2`

**Quand l'utiliser :**
- Paramètres de navigation partageables via URL (filtres, pagination, recherche)
- Méthodes GET d'API REST
- Navigation entre pages où l'état doit survivre à un copier-coller d'URL

**Trade-offs :**
- ✅ Partage facile d'un état via l'URL (bookmark, partage de lien)
- ✅ Compatible SSR
- ❌ **Impact SEO** : des paramètres incohérents ou trop nombreux nuisent au référencement
- ❌ Lisibilité dégradée pour l'utilisateur avec trop de paramètres
- ❌ Pas adapté aux données sensibles — visible dans l'URL

---

## Guide de choix rapide

```
La donnée est locale à un composant ?
  → État local

La donnée est partagée dans un sous-arbre sans aller chercher de store ?
  → Context Provider

La donnée vient d'une API ?
  → Queries (TanStack Query)

La donnée est un état global métier critique partagé partout ?
  → Store centralisé (avec rigueur)

La donnée doit survivre entre pages et sessions, accessible côté serveur ?
  → Cookie

La donnée doit persister côté navigateur, pas besoin de SSR ?
  → LocalStorage (persistant) ou SessionStorage (temporaire)

La donnée est volumineuse ou l'app doit fonctionner offline ?
  → IndexedDB

La donnée structure la navigation et doit être partageable via URL ?
  → Query String
```

---

## Règle d'or — Élaguer sa donnée

> L'un des secrets d'une bonne gestion des données, c'est de ne pas en avoir.

**Ne transmets que l'essentiel :**
Avant d'envoyer une donnée au front, comprendre son rôle, sa portée et sa durée de vie.
Chaque information a un coût : performance, maintenance, sécurité.

**Voyager léger :**
Envoyer toutes les données, c'est transporter un sac plein "au cas où" — la plupart
ne servent jamais, mais ralentissent tout et augmentent les risques d'erreur.
Ne garder que ce qui est vraiment nécessaire.

**Contrats d'interface précis :**
Pour séparer clairement front et back, définir exactement quelles données circulent
et comment elles doivent être utilisées (qu'elles viennent d'une API ou du rendu serveur).
Ces contrats évitent le couplage inutile.

**Limiter les risques en réduisant le volume de données :**
- Limite la surface d'attaque
- Augmente la robustesse et la stabilité
- Facilite les évolutions et la maintenance
- Réduit le volume de données qui transitent sur le réseau

---

## Ce que tu ne fais PAS

- Choisir un mécanisme sans avoir posé les 5 questions de caractérisation
- Mettre toutes les données dans un Store "parce que c'est pratique"
- Dupliquer l'état serveur dans un Store global quand les Queries sont disponibles
- Stocker des données sensibles dans LocalStorage, SessionStorage ou IndexedDB
- Implémenter une décision de gestion de données sans validation explicite de l'utilisateur
- Appliquer la même approche pour tous les cas d'usage sans réfléchir au contexte
