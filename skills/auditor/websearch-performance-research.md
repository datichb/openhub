---
name: websearch-performance-research
description: Protocole de recherche web pour les audits de performance — Core Web Vitals, benchmarks, patterns d'optimisation.
---

# Recherche Performance via websearch

## Quand chercher

Utiliser websearch pour la performance quand :

1. **Bottleneck identifié** — le profiling local montre un chemin lent mais la solution optimale n'est pas évidente
2. **Optimisation framework/lib** — besoin des best practices spécifiques au stack (React, Next.js, Node.js, etc.)
3. **Comparaison de benchmarks** — valider si les métriques observées sont acceptables vs l'industrie
4. **Alternatives** — trouver des bibliothèques ou patterns plus performants
5. **Techniques émergentes** — stratégies d'optimisation récentes (2024-2026)

**Pré-requis** : toujours profiler localement AVANT de chercher (baseline obligatoire).

## Sources de référence

| Domaine | Sources fiables |
|---------|----------------|
| Web Vitals / Frontend | web.dev, MDN, Chrome DevTools docs |
| React / Next.js | react.dev, nextjs.org/docs |
| Node.js | nodejs.org/docs, clinic.js docs |
| Bases de données | Documentation officielle (PG, Mongo, Redis) |
| Bundles | Vite/Webpack docs, bundlephobia.com |
| Benchmarks | GitHub repos avec méthodologie reproductible |

**Priorité** : documentation officielle > articles techniques récents > blog posts.
**Fraîcheur** : privilégier 2024-2026 pour les frameworks à évolution rapide.

## Stratégie de recherche

### Principes

- **Spécifique** — inclure framework + version + problème précis + année
- **Groupée** — combiner termes connexes en une requête plutôt que plusieurs recherches atomiques
- **Orientée solution** — chercher technique + benchmark + trade-offs ensemble
- **Webfetch ciblé** — pour les sources connues (docs officielles), aller directement à l'URL

### Domaines de recherche par priorité

| Priorité | Cible | Exemples |
|----------|-------|----------|
| HIGH | Chemin critique (>1000 appels/s), render-blocking, bundles >50KB, queries >200ms | Virtualisation, code splitting, indexation |
| MEDIUM | Fréquence modérée (100-1000/s), contenu hors viewport, bundles 10-50KB | Lazy loading, memoization |
| LOW | Opérations rares (<100/s), tâches background, utilitaires <10KB | Micro-optimisations |

## Format de synthèse attendu

Chaque finding de recherche doit contenir :

```markdown
### [Composant/Feature] — Optimisation Performance

**Problème** : [métrique actuelle + seuil dépassé]
**Localisation** : [fichier:ligne]
**Impact** : HIGH | MEDIUM | LOW

**Recherche** :
- Sources consultées : [liste avec dates]
- Technique recommandée : [nom + description courte]
- Amélioration attendue : [% ou valeur absolue, citée depuis source]
- Complexité : Low | Medium | High
- Trade-offs : [mémoire, maintenance, compatibilité]

**Validation** :
- Baseline mesurée : [valeur avant]
- Post-optimisation : [valeur après]
- Résultat vs attendu : [confirmation ou écart + explication]
```

### En cas de résultats contradictoires

Quand les sources divergent : vérifier dates, autorité (docs officielles > blogs), contexte (scale comparable), et recommander une approche guidée par le profiling local.

## Seuils de référence

### Frontend — Core Web Vitals

| Métrique | Bon | Acceptable | Mauvais |
|----------|-----|------------|---------|
| FCP (First Contentful Paint) | <1.8s | <3.0s | ≥3.0s |
| LCP (Largest Contentful Paint) | <2.5s | <4.0s | ≥4.0s |
| TTI (Time to Interactive) | <3.8s | <7.3s | ≥7.3s |
| CLS (Cumulative Layout Shift) | <0.1 | <0.25 | ≥0.25 |
| INP (Interaction to Next Paint) | <200ms | <500ms | ≥500ms |
| Bundle JS initial (gzip) | <100KB | <250KB | ≥250KB |
| Lighthouse Score | >90 | >50 | ≤50 |

### Backend — Latence & Ressources

| Métrique | Bon | Acceptable | Mauvais |
|----------|-----|------------|---------|
| Latence P50 | <100ms | <300ms | ≥300ms |
| Latence P95 | <500ms | <1s | ≥1s |
| Latence P99 | <1s | <3s | ≥3s |
| Query DB simple | <50ms | <200ms | ≥200ms |
| Query DB complexe | <200ms | <500ms | ≥500ms |
| Mémoire (small app) | <512MB | <1GB | ≥1GB |
| CPU moyen | <50% | <70% | ≥70% |

## Checklist avant recommandation

- [ ] Baseline locale mesurée (profiling avant optimisation)
- [ ] Recherche effectuée sur sources fiables et récentes
- [ ] Amélioration quantifiée avec source citée
- [ ] Complexité d'implémentation évaluée (effort vs gain)
- [ ] Trade-offs documentés (mémoire, maintenance, compatibilité)
- [ ] Validation locale planifiée (mesurer avant/après)
- [ ] Impact utilisateur clair (métrique user-facing identifiée)

**Règle d'or** : l'optimisation est data-driven — websearch fournit techniques et benchmarks, le profiling local valide l'impact réel.
