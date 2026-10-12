---
name: websearch-stack-research
description: "Protocole de recherche WebSearch spécialisé pour les agents de planification — découverte et évaluation de stacks technologiques, librairies, frameworks et patterns architecturaux. Utiliser quand le pathfinder détecte une stack incomplète ou non documentée, quand le planner doit choisir entre plusieurs librairies, ou quand l'onboarder identifie des lacunes technologiques. Couvre : comparaisons de librairies, critères d'évaluation (popularité, maintenance, TypeScript, bundle size, licence), patterns de requêtes ciblées, gestion des résultats biaisés ou obsolètes. Mots-clés : library comparison, framework selection, migration planning, tech stack, npm downloads, GitHub stars, bundle size, ecosystem trends."
---

# Recherche de stack par WebSearch — protocole des agents de planification

Complète le skill `websearch-usage` (règles générales, sécurité, format de restitution).

## Objectif

Utiliser WebSearch pour découvrir et évaluer **stacks technologiques**, **librairies**, **frameworks** et **patterns architecturaux** pendant la reconnaissance et la planification, afin de recommander des technologies sur des données à jour.

## Quand faire une recherche de stack

### Déclencheurs
1. **Initialisation du projet** : le pathfinder détecte une stack incomplète ou non documentée
2. **Choix technologique** : plusieurs librairies/frameworks en concurrence
3. **Planification de migration** : alternatives à une technologie legacy
4. **Implémentation d'une feature** : on ne sait pas comment répondre au besoin
5. **Découverte de bonnes pratiques** : technologie peu familière

### Selon l'agent
```
Pathfinder :
→ découvrir la stack existante
→ WebSearch : documentation manquante, guides de migration

Onboarder :
→ identifier les lacunes de connaissance
→ WebSearch : documentation officielle, guides d'installation, tutoriels

Planner :
→ concevoir des features à l'implémentation incertaine
→ WebSearch : comparaisons de librairies, patterns d'architecture
```

## Patterns de requête

### 1. Comparaison de librairies
```
✅ "React state management comparison Zustand Redux Jotai 2026"
✅ "Next.js authentication library comparison NextAuth Clerk Auth.js"
✅ "Node.js ORM comparison Prisma TypeORM Drizzle"
✅ "headless CMS comparison Sanity Contentful Strapi 2026"
```

### 2. Documentation
```
✅ "Tailwind CSS official documentation"
✅ "Prisma schema reference guide"
✅ "Next.js 15 App Router migration guide"
✅ "TypeScript 5.4 documentation"
```

### 3. Tendances de l'écosystème
```
✅ "React ecosystem trends 2026"
✅ "serverless framework comparison AWS Lambda 2026"
✅ "Jamstack hosting comparison Vercel Netlify Cloudflare Pages"
✅ "monorepo tools comparison Turborepo Nx pnpm workspaces"
```

### 4. Bonnes pratiques et patterns
```
✅ "Next.js project structure best practices 2026"
✅ "microservices architecture Node.js patterns"
✅ "GraphQL API design best practices"
✅ "React component design patterns 2026"
```

### 5. Intégration et installation
```
✅ "integrate Stripe Next.js App Router"
✅ "setup Tailwind CSS with Next.js 14"
✅ "configure ESLint Prettier TypeScript"
✅ "deploy Next.js Vercel environment variables"
```

## Critères d'évaluation

| Critère | Requêtes | Seuils / signaux |
|---------|----------|------------------|
| **Popularité** | `"most popular {catégorie} 2026"`, `"npm downloads {lib}"`, `"GitHub stars {lib}"` | ✅ Haute : > 1M téléchargements npm/semaine, > 20K stars — ✅ Moyenne : 100K-1M, 5K-20K — ⚠ Faible : < 100K, < 5K (risque plus élevé) |
| **Maintenance et communauté** | `"{lib} active development 2026"`, `"{lib} community support"` | ❌ Dernier commit > 6 mois, nombreuses issues critiques ouvertes, annonce d'épuisement des mainteneurs |
| **Documentation** | `"{lib} documentation"`, `"{lib} getting started guide"` | ✅ Docs officielles avec recherche, exemples TypeScript, guides de migration, référence d'API |
| **TypeScript** | `"{lib} TypeScript support"`, `"{lib} type definitions"` | ✅ Excellent : écrit en TS — ✅ Bon : types DefinitelyTyped maintenus — ⚠ Faible : pas de types ou types communautaires incomplets |
| **Bundle size** | `"{lib} bundle size"`, `"bundlephobia {lib}"` | Gzippé : ✅ < 10KB, ✅ 10-50KB, ⚠ > 50KB (à justifier) |
| **Performance** | `"{lib} performance benchmark 2026"`, `"{lib} vs {alt} speed comparison"` | Benchmarks indépendants (pas publiés par l'éditeur), scénarios réels, données récentes |
| **Licence** | `"{lib} license"` | ✅ MIT, Apache 2.0, ISC, BSD — ⚠ GPL (revue juridique) — ❌ Propriétaire (achat) |

## Exemples de déroulé

### Pathfinder
```
1. PATHFINDER : analyse du code existant
   → Détecté : Next.js 13, React 18, gestion d'état inconnue
2. WEBSEARCH : "Next.js 13 state management detection patterns"
   → Chercher les imports Redux, Zustand, Jotai
3. PATHFINDER : nouveau scan → Zustand utilisé
4. WEBSEARCH : "Zustand best practices 2026" → documentation officielle pour le handoff
5. HANDOFF : carte complète de la stack
   → État : Zustand (docs : https://zustand-demo.pmnd.rs/)
```

### Onboarder
```
1. ONBOARDER : revue des besoins du projet → lacune : « Comment gérer l'authentification ? »
2. WEBSEARCH : "Next.js authentication library comparison 2026"
   - NextAuth.js (le plus populaire, complexe)
   - Clerk (le plus simple, offres payantes)
   - Auth.js (NextAuth v5, bêta)
3. WEBSEARCH : "NextAuth.js Next.js 14 App Router setup" → guide de migration officiel
4. ONBOARDER : documenter l'installation (provider, sessions, routes protégées)
```

### Planner
```
1. PLANNER : feature « Notifications temps réel » → implémentation incertaine
2. WEBSEARCH : "real-time notifications Next.js 2026 WebSocket vs SSE"
   - Pusher (hébergé, simple) — Ably (hébergé, complet)
   - Socket.io (auto-hébergé, complexe) — Server-Sent Events (natif, simple)
3. WEBSEARCH : "Server-Sent Events Next.js App Router example" → tutoriel avec code
4. PLANNER : dans le plan d'implémentation
   → Approche : SSE avec l'API EventSource
   → Justification : support natif des navigateurs, plus simple que WebSocket pour des notifications à sens unique
   → Référence : [URL du guide SSE]
```

## Gabarit de comparaison

```markdown
## Comparaison de librairies : {Catégorie}

**Cas d'usage** : {besoin précis}
**Date** : {AAAA-MM-JJ}
**Requête** : "{requête}"

### Options évaluées

#### Option 1 : {Librairie A}
- **Popularité** : {téléchargements/stars}
- **Bundle size** : {taille} KB gzippé
- **TypeScript** : ✅ Complet / ⚠ Partiel / ❌ Aucun
- **Documentation** : [URL]
- **Dernière mise à jour** : {date}
- **Pour** : {…}
- **Contre** : {…}
- **Idéal pour** : {scénario}

#### Option 2 : {Librairie B}
...

### Recommandation

**Choix** : {Librairie X}

**Justification** :
1. {raison 1, chiffrée}
2. {raison 2, chiffrée}

**Compromis acceptés** :
- {compromis}

**Notes d'implémentation** :
- Installation : `npm install {librairie}` <!-- capability: user -->
- Mise en place : {aperçu}
- Documentation : [URL]

**Sources** :
- Article de comparaison : [URL]
- Documentation officielle : [URL]
- Benchmark : [URL]
```

## Recherches courantes par écosystème

| Écosystème | Requêtes |
|------------|----------|
| **React** | `"React UI component library comparison 2026"` (MUI, Chakra, Radix, shadcn/ui), `"React form library comparison 2026"` (React Hook Form, Formik, Final Form), `"React animation library comparison"` (Framer Motion, React Spring, GSAP), `"React testing library best practices"` |
| **Next.js** | `"Next.js database integration 2026"` (Prisma, Drizzle, Supabase), `"Next.js deployment options comparison"` (Vercel, Netlify, AWS Amplify), `"Next.js CMS integration"` (Sanity, Contentful, Strapi, Payload), `"Next.js monitoring tools"` (Sentry, LogRocket, Datadog) |
| **Node.js backend** | `"Node.js framework comparison 2026"` (Express, Fastify, Hapi, NestJS), `"Node.js validation library"` (Zod, Yup, Joi, AJV), `"Node.js job queue library"` (Bull, BullMQ, Bee-Queue), `"Node.js logging library"` (Winston, Pino, Bunyan) |
| **Base de données** | `"database comparison SQL vs NoSQL 2026"`, `"PostgreSQL vs MySQL vs MongoDB use cases"`, `"serverless database comparison PlanetScale Neon Supabase"`, `"database migration tool comparison Prisma Drizzle TypeORM"` |

## Résultats biaisés, obsolètes ou incomplets

| Problème | Solutions |
|----------|-----------|
| **Biais éditeur** (pages marketing uniquement) | Ajouter « independent comparison » (`"independent headless CMS comparison 2026"`), chercher les discussions communautaires (`"Reddit {A} vs {B}"`), des sources académiques |
| **Information obsolète** (résultats 2020-2022) | Ajouter l'année (`"React state management 2025-2026"`), « latest » / « current », consulter les changelogs officiels |
| **Contexte manquant** (comparaison générique) | Ajouter le besoin précis (`"React form library large multi-step forms"`), l'échelle (`"database choice small startup vs enterprise"`), les contraintes techniques (`"real-time library serverless AWS Lambda"`) |

## Limiter le nombre de recherches

### Affinage progressif
```
Tour 1 : vue d'ensemble
✅ "Next.js full-stack architecture 2026"

Tour 2 : approfondissements ciblés (2-3 candidats par catégorie)
✅ "Next.js authentication NextAuth vs Clerk"
✅ "Next.js database Prisma vs Drizzle"

Tour 3 : détails d'implémentation (WebFetch)
✅ webfetch("https://next-auth.js.org/getting-started")
```

### Regrouper les catégories
```
✅ "Next.js tech stack recommendations 2026 database authentication deployment"
→ vue d'ensemble de toutes les catégories en une recherche, puis requêtes ciblées seulement si nécessaire
```

### Ressources connues
```
✅ webfetch("https://2024.stateofjs.com/en-US/libraries/")
✅ webfetch("https://risingstars.js.org/2025/en")
✅ webfetch("https://npmtrends.com/{lib1}-vs-{lib2}")
→ les enquêtes annuelles et rapports de tendances couvrent de nombreuses comparaisons
```

## Cas difficiles

| Situation | Action |
|-----------|--------|
| **Pas de gagnant net** (compromis similaires) | Choisir le plus populaire (moins de risque, plus de ressources) ; documenter « pas de différence significative, {X} choisi pour la taille de sa communauté » ; noter qu'une migration reste possible |
| **Toutes les options ont un défaut rédhibitoire** | Réévaluer les besoins ; chercher des solutions hybrides (`"{feature} custom implementation"`) ; envisager une solution maison minimale ; documenter les compromis pour décision |
| **Recommandations contradictoires** | Comparer les dates (la plus récente l'emporte), l'autorité (officiel > blog), le contexte (échelle, contraintes) ; restituer « avis partagés, {X} recommandé parce que {raison} » |

## Restitution dans le handoff

Après la recherche, inclure dans le document de handoff :

```markdown
## Stack technique

### Gestion d'état
- **Zustand 4.5**
  - Docs : https://zustand-demo.pmnd.rs/
  - Choisi pour : simplicité, support TypeScript, bundle réduit (3KB)
  - Alternative envisagée : Redux (écartée : boilerplate)

### Base de données
- **Prisma 5** + PostgreSQL 16
  - Docs : https://www.prisma.io/docs
  - Choisi pour : ORM typé, bonne DX, outillage de migration
  - Alternative envisagée : Drizzle (écartée : communauté plus réduite)

### Sources de la recherche
- Comparaison de stack : [URL]
- Recommandations communautaires : [URL]
- Benchmark : [URL]
```

## Bonnes pratiques

**À FAIRE** :
- ✅ Rechercher avant de s'engager sur une librairie (éviter les migrations coûteuses)
- ✅ Évaluer plusieurs options (2-3 candidats minimum)
- ✅ Vérifier les dates de publication (sources 2024-2026)
- ✅ Confirmer avec la documentation officielle
- ✅ Prendre en compte bundle size, support TypeScript, maintenance
- ✅ Documenter la justification des choix et inclure les sources dans le handoff

**À NE PAS FAIRE** :
- ❌ Choisir sur le seul effet de mode
- ❌ Ignorer le bundle size (la performance frontend compte)
- ❌ Retenir une librairie non maintenue (risque de sécurité futur)
- ❌ Sur-concevoir (YAGNI)
- ❌ Croire sans recul les comparaisons d'éditeurs
- ❌ Oublier de documenter les compromis

---

## Checklist avant recommandation

- [ ] 2-3 options candidates identifiées
- [ ] Popularité vérifiée (téléchargements npm, stars GitHub)
- [ ] Maintenance active vérifiée (commits récents, réponse aux issues)
- [ ] Qualité de la documentation évaluée
- [ ] Support TypeScript confirmé (si applicable)
- [ ] Bundle size vérifié (< 50KB de préférence)
- [ ] Benchmarks trouvés (si la performance est critique)
- [ ] Compatibilité de licence vérifiée
- [ ] Retours communautaires lus (Reddit, discussions GitHub)
- [ ] Justification et compromis documentés
- [ ] Liens vers la documentation officielle inclus dans le handoff

Toutes les cases cochées → recommander en confiance.
