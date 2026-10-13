---
name: audit-performance
description: "Utiliser quand l'audit porte sur la performance : Web Vitals, N+1, bundle, cache, lazy loading, requêtes."
---

# Skill — Audit Performance

## Référentiels couverts

- **Core Web Vitals** (Google) — LCP, INP, CLS
- **RAIL Model** — Response, Animation, Idle, Load
- **HTTP Archive / Web Almanac** — benchmarks de l'industrie
- **Lighthouse** — outil de mesure Google
- **WebPageTest** — analyse réseau détaillée

---

## Métriques cibles

### Core Web Vitals (seuils Google)

| Métrique | Bon | À améliorer | Mauvais |
|---------|-----|-------------|---------|
| **LCP** (Largest Contentful Paint) | ≤ 2.5s | 2.5–4s | > 4s |
| **INP** (Interaction to Next Paint) | ≤ 200ms | 200–500ms | > 500ms |
| **CLS** (Cumulative Layout Shift) | ≤ 0.1 | 0.1–0.25 | > 0.25 |

### Métriques complémentaires

| Métrique | Cible recommandée |
|---------|------------------|
| **TTFB** (Time to First Byte) | ≤ 600ms |
| **FCP** (First Contentful Paint) | ≤ 1.8s |
| **TTI** (Time to Interactive) | ≤ 3.8s |
| **TBT** (Total Blocking Time) | ≤ 200ms |
| **Bundle JS total** (gzippé) | ≤ 200KB (initial load) |
| **Bundle CSS total** (gzippé) | ≤ 50KB |
| **Nombre de requêtes HTTP** | ≤ 50 (page initiale) |

---

## Checklist — Requêtes base de données

### Problème N+1

- [ ] Les relations sont chargées avec eager loading (pas de requête par item dans une boucle)
  - ORM Eloquent : `with()`, `load()`
  - ORM Doctrine : `fetch: EAGER` ou `JOIN FETCH`
  - TypeORM : `relations: [...]` dans le find
  - Prisma : `include: { ... }`
- [ ] Les boucles sur des collections n'exécutent pas de requêtes SQL individuelles
- [ ] Les comptages utilisent `COUNT()` SQL plutôt que `.length` sur une collection chargée

### Indexation

- [ ] Les colonnes utilisées dans les clauses `WHERE` fréquentes ont un index
- [ ] Les colonnes de jointure (`JOIN ON`) ont un index sur les deux tables
- [ ] Les colonnes de tri (`ORDER BY`) fréquentes ont un index
- [ ] Les index composites sont dans le bon ordre (colonne la plus sélective en premier)
- [ ] Absence d'index inutilisés sur des tables à fort volume d'écriture

### Pagination

- [ ] Les listes paginées utilisent `LIMIT/OFFSET` ou curseur (keyset pagination)
- [ ] Absence de chargement de toute une table pour filtrer côté application
- [ ] Les APIs retournant des listes ont une limite maximale de résultats par requête

### Requêtes

- [ ] Les colonnes inutiles ne sont pas sélectionnées (`SELECT *` évité sur les grandes tables)
- [ ] Les sous-requêtes peuvent être remplacées par des `JOIN` plus performants
- [ ] Les transactions regroupent les opérations multiples (pas de commit par item dans une boucle)

---

## Checklist — Bundle et ressources front-end

### JavaScript

- [ ] Le code est minifié et gzippé/brotli en production
- [ ] Le bundle est analysé (webpack-bundle-analyzer, source-map-explorer)
- [ ] Le code splitting est implémenté — les routes chargent leur code à la demande
- [ ] Les dépendances volumineuses ont des alternatives légères vérifiées
  - `moment.js` (67KB) → `date-fns` ou `dayjs`
  - `lodash` (full) → `lodash-es` avec tree-shaking ou fonctions natives
- [ ] Les polyfills ne sont inclus que pour les cibles réelles (pas IE11 si non supporté)
- [ ] Les chunks de vendor sont séparés du code applicatif (meilleur cache)

### CSS

- [ ] Le CSS inutilisé est purgé en production (PurgeCSS, Tailwind purge)
- [ ] Les fonts web utilisent `font-display: swap` pour éviter le blocage de rendu
- [ ] Les fonts sont auto-hébergées ou utilisent un CDN avec préconnexion (`<link rel="preconnect">`)

### Images

- [ ] Les images sont servies en formats modernes (WebP, AVIF) avec fallback
- [ ] Les images ont des dimensions explicites (`width`/`height`) pour éviter le CLS
- [ ] Les images hors viewport utilisent le lazy loading (`loading="lazy"`)
- [ ] Les images LCP (above the fold) utilisent `loading="eager"` et `fetchpriority="high"`
- [ ] Un CDN image avec redimensionnement automatique est utilisé si pertinent

### Ressources critiques

- [ ] Les ressources critiques (polices, CSS au-dessus de la ligne de flottaison) sont préchargées (`<link rel="preload">`)
- [ ] Les scripts non critiques ont l'attribut `async` ou `defer`
- [ ] Le HTML inline est minimal (pas de CSS/JS volumineux inline)

---

## Checklist — Cache et CDN

### Cache navigateur

- [ ] Les ressources statiques ont des headers `Cache-Control` avec TTL long + fingerprint dans l'URL
  - Exemple : `Cache-Control: public, max-age=31536000, immutable`
- [ ] Les pages HTML ont une politique de cache adaptée (souvent `no-cache` ou TTL court)
- [ ] Les APIs REST utilisent les headers `ETag` et/ou `Last-Modified` pour la revalidation

### Cache applicatif

- [ ] Les requêtes coûteuses en base de données sont mises en cache (Redis, Memcached)
- [ ] Le cache a une stratégie d'invalidation documentée (TTL, event-driven, ou LRU)
- [ ] Le cache de session est dimensionné pour la charge prévue
- [ ] Les calculs déterministes coûteux sont mémoïsés (côté serveur et/ou côté client)

### CDN

- [ ] Les ressources statiques sont servies via un CDN (pas depuis le serveur applicatif)
- [ ] Les origines CDN ont des règles de cache cohérentes avec les headers applicatifs
- [ ] Le CDN est configuré pour compresser automatiquement (gzip/brotli)

---

## Checklist — Rendu et expérience utilisateur

### Rendu côté serveur / côté client

- [ ] Les pages avec contenu critique pour le SEO utilisent SSR ou SSG (pas CSR pur)
- [ ] L'hydratation (SSR→CSR) est progressive et ne bloque pas l'interactivité
- [ ] Les états de chargement sont gérés (skeleton screens plutôt que spinners bloquants)

### Animations et interactions

- [ ] Les animations utilisent `transform` et `opacity` uniquement (pas de propriétés qui déclenchent un reflow)
- [ ] Les handlers d'événements fréquents (scroll, resize) sont débouncés ou throttlés
- [ ] Pas de `requestAnimationFrame` ou `setInterval` actifs sur des pages inactives

### Web Workers / tâches longues

- [ ] Les traitements lourds (parsing, calcul) sont déplacés dans un Web Worker si possible
- [ ] Les tâches synchrones bloquantes (> 50ms) dans le thread principal sont identifiées et fragmentées

---

## Checklist — APIs et réseau

- [ ] Les appels API sont groupés quand possible (éviter les waterfalls de requêtes séquentielles)
- [ ] Les données nécessaires à la page initiale sont préfetchées (SSR ou prefetch)
- [ ] Les réponses JSON ne contiennent pas de champs inutiles pour le client (over-fetching)
  - GraphQL : sélection des champs
  - REST : paramètre `fields` si disponible
- [ ] Les websockets ou SSE remplacent le polling long si applicable
- [ ] Les retry automatiques utilisent un backoff exponentiel avec jitter

---

> Les outils de mesure (Lighthouse CLI, WebPageTest, webpack-bundle-analyzer, EXPLAIN ANALYZE, etc.)
> sont référencés dans `docs/reference/audit-tools.md` pour usage humain.

## Ce que tu ne fais PAS dans ce domaine

- Mesurer des performances réelles (pas d'accès à un environnement live)
- Comparer avec des métriques de production sans données fournies par l'utilisateur
- Garantir un score Lighthouse spécifique — l'analyse statique a des limites
- Optimiser prématurément des chemins non critiques au détriment de la lisibilité
