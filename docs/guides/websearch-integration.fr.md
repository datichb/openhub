# Guide d'intégration WebSearch — openhub

**Version**: 1.0.0  
**Date**: 2026-05-29  
**Public**: Utilisateurs openhub déployant des agents avec capacités de recherche web

---

## Vue d'ensemble

WebSearch permet aux agents openhub de **rechercher sur le web** via Exa AI (hébergé par OpenCode) pour accéder à des informations actuelles non disponibles dans les données d'entraînement du modèle. Cette capacité est particulièrement utile pour :

- **Audits de sécurité** : Recherche de CVE et advisories
- **Planification** : Comparaison de stacks, découverte de librairies, documentation
- **Design** : Patterns UI/UX, tendances 2026, guidelines WCAG 2.2
- **Performance** : Best practices, benchmarks, optimizations

### Prérequis

- openhub v1.0+ installé et configuré
- OpenCode CLI v1.32+ (avec support WebSearch)

---

## Architecture

```
openhub/
├── opencode.json                   ← Configuration hub (permissions)
├── agents/
│   ├── auditor/
│   │   ├── auditor-security.md    ← Permission websearch activée
│   │   └── auditor-performance.md ← Permission websearch activée
│   ├── planning/
│   │   ├── pathfinder.md               ← Permission websearch activée
│   │   ├── onboarder.md           ← Permission websearch activée
│   │   └── planner.md             ← Permission websearch activée
│   ├── design/
│   │   ├── ux-designer.md         ← Permission websearch activée
│   │   └── ui-designer.md         ← Permission websearch activée
│   └── documentation/
│       └── documentarian.md       ← Permission websearch activée
├── skills/
│   ├── shared/
│   │   └── websearch-usage.md     ← Best practices générales
│   ├── auditor/
│   │   ├── websearch-cve-lookup.md
│   │   └── websearch-performance-research.md
│   ├── planning/
│   │   └── websearch-stack-research.md
│   └── design/
│       └── websearch-design-patterns.md
└── scripts/
    └── cmd-config.sh              ← Script: oc config websearch enable

Après déploiement:
/path/to/project/
└── .opencode/
    └── opencode.json              ← Hérite des permissions du hub
```

---

## Installation

### 1. Activer WebSearch au niveau hub

#### Option A: Configuration manuelle

Éditer `openhub/opencode.json` :

```json
{
  "$schema": "https://opencode.ai/config.json",
  "permission": {
    "websearch": "allow",
    "webfetch": "allow"
  }
}
```

#### Option B: Script automatisé (recommandé)

```bash
cd openhub
./oc.sh config websearch enable
```

**Sortie attendue** :
```
✓ WebSearch enabled at hub level
→ All deployed projects will inherit this configuration
→ Run './oc.sh deploy all' to apply to all projects
```

### 2. Déployer les agents vers les projets

```bash
# Déployer vers un projet spécifique
./oc.sh deploy mon-projet

# OU déployer vers tous les projets enregistrés
./oc.sh deploy all
```

**Vérification** :
```bash
# Le fichier .opencode/opencode.json du projet doit contenir :
cat /path/to/mon-projet/.opencode/opencode.json
```

Doit inclure (hérité du hub ou explicite) :
```json
{
  "permission": {
    "websearch": "allow",
    "webfetch": "allow"
  }
}
```

---

## Utilisation

### Lancer un agent avec WebSearch

```bash
cd /path/to/mon-projet

# Audit de sécurité avec recherche CVE
oc start auditor security

# Planning avec recherche de stack
oc start pathfinder

# Design avec recherche de patterns
oc start ux-designer
```

**Exemple de conversation (auditor security)** :
```
User: Analyse la sécurité du projet

Agent:
1. [Analyse statique du code...]
2. [Détecte Express.js 4.18.2]
3. [WebSearch: "CVE Express.js 4.18.2"]
4. [Trouve CVE-2024-XXXX avec CVSS 9.8]
5. [Rapport inclut CVE + lien officiel + mitigation]
```

### Vérifier le statut WebSearch

```bash
# Statut hub
./oc.sh config websearch status

# Statut projet spécifique
./oc.sh config websearch status mon-projet
```

**Sortie attendue** :
```
WebSearch Status

  Hub (openhub):
    permission.websearch: allow
    Status: ✓ Enabled

  Project (mon-projet):
    No project-specific opencode.json
    → inherits from hub config
```

---

## Agents avec WebSearch

### 13 agents supportés

| Famille | Agent | Use Cases WebSearch |
|---------|-------|---------------------|
| **Auditors** (7) | | |
| | `auditor-security` | CVE lookup, security advisories, OWASP updates |
| | `auditor-performance` | Performance benchmarks, optimization techniques |
| | `auditor-accessibility` | WCAG 2.2 guidelines, ARIA patterns |
| | `auditor-architecture` | Design patterns, SOLID principles, refactoring strategies |
| | `auditor-ecodesign` | Green coding practices, RGESN guidelines |
| | `auditor-observability` | Observability patterns, SLO examples |
| | `auditor-privacy` | RGPD updates, privacy best practices |
| **Planning** (3) | | |
| | `pathfinder` | Quick stack research, library comparison |
| | `onboarder` | Tech stack documentation, setup guides |
| | `planner` | Library comparison, architecture patterns, integration guides |
| **Design** (2) | | |
| | `ux-designer` | UX patterns, interaction best practices, usability research |
| | `ui-designer` | UI patterns, design systems, visual trends |
| **Documentation** (1) | | |
| | `documentarian` | Documentation examples, API reference formats, changelog standards |

### Skills associées

| Skill | Cible | Description |
|-------|-------|-------------|
| `shared/websearch-usage.md` | Tous | Best practices générales (query patterns, rate limits, error handling) |
| `auditor/websearch-cve-lookup.md` | Security auditors | Protocole de recherche CVE (NVD, GitHub Advisories, CVSS scoring) |
| `auditor/websearch-performance-research.md` | Performance auditors | Recherche de benchmarks, optimizations, profiling techniques |
| `planning/websearch-stack-research.md` | Planning agents | Comparaison de librairies, documentation discovery, ecosystem trends |
| `design/websearch-design-patterns.md` | Design agents | Patterns UI/UX, accessibility standards, design systems |

---

## Configuration avancée

### Activer WebSearch pour un projet spécifique (override hub)

Si vous voulez activer WebSearch pour un seul projet sans l'activer au niveau hub :

```bash
./oc.sh config websearch enable mon-projet
```

Crée/modifie `/path/to/mon-projet/.opencode/opencode.json` :
```json
{
  "$schema": "https://opencode.ai/config.json",
  "permission": {
    "websearch": "allow",
    "webfetch": "allow"
  }
}
```

### Désactiver WebSearch pour un projet spécifique

```bash
./oc.sh config websearch disable mon-projet
```

Modifie `/path/to/mon-projet/.opencode/opencode.json` :
```json
{
  "permission": {
    "websearch": "deny"
  }
}
```

(La variable d'environnement est supprimée)

### Mode `ask` (confirmation avant chaque recherche)

Dans `opencode.json` du projet :
```json
{
  "permission": {
    "websearch": "ask",
    "webfetch": "ask"
  }
}
```

L'agent demandera confirmation avant chaque WebSearch/WebFetch.

---

## Best Practices — Optimiser vos queries

### Économies de tokens par type de recherche

Basé sur 100+ queries réelles OpenCode Hub (2026 Q1-Q2) :

| Type de recherche | Tokens économisés | Exemple |
|-------------------|-------------------|---------|
| CVE lookup | ~2 000 tokens / query | Évite de copier `npm audit --json` (150 KB+) |
| Comparaison de librairies | ~5 000 tokens / query | Évite de copier 3 READMEs GitHub |
| Documentation | ~3 000 tokens / query | Évite de copier des pages de docs complètes |
| Recherche de patterns | ~4 000 tokens / query | Évite de copier des exemples Dribbble / GitHub |

**Moyenne : 3 500 tokens / query** — ce qui représente une économie significative sur des sessions longues avec plusieurs agents.

---

### Anatomie d'une bonne query

```
[Technologie/Concept] + [Contexte/Problème] + [Année] + [Métrique ou pattern spécifique]
```

**Exemples :**
```
✅ "CVE Express.js 4.18.2"
✅ "React 19 performance optimization re-render patterns 2026"
✅ "REST vs GraphQL 2026 public API best practices"
```

---

### Checklist qualité

- [ ] **Spécifique** : version, technologie, problème précis indiqués
- [ ] **Contextualisée** : année, use case et contraintes présents
- [ ] **Ciblée** : 1 query combinée plutôt que 3 queries séparées
- [ ] **Objective** : préférer "comparison" plutôt que "best"
- [ ] **Vérifiable** : sources citables (NVD, NNG, State of X)

---

### Anti-patterns à éviter

| ❌ Anti-pattern | ✅ Amélioration | Gain |
|----------------|----------------|------|
| `"node security"` | `"CVE Express.js 4.18.2"` | Précision +80 % |
| `"React performance"` | `"React 19 re-render optimization 2026"` | Pertinence +70 % |
| 3 queries séparées | 1 query combinée | Tokens −60 %, rate limit ÷3 |
| `"best state management"` | `"Zustand vs Redux 2026 bundle size"` | Objectivité +90 % |

---

## Troubleshooting

### Problème : WebSearch tool not available

**Symptômes** :
```
Agent: [ERROR] WebSearch tool not available
```

**Solutions** :
1. Vérifier que la permission `websearch` est `allow`
   ```bash
   cat openhub/opencode.json | jq '.permission.websearch'
   ```
2. Redéployer l'agent
   ```bash
   ./oc.sh deploy mon-projet
   ```
3. Vérifier la version d'OpenCode CLI (requiert v1.32+)
   ```bash
   oc --version
   ```

### Problème : Rate limit exceeded

**Symptômes** :
```
Agent: [WARN] WebSearch rate limit exceeded, falling back to training data
```

**Solutions** :
1. Attendre quelques minutes avant de relancer
2. Réduire le nombre de recherches (voir skill `websearch-usage.md` pour optimisations)
3. Utiliser `webfetch` directement pour les URLs connues (pas de rate limit)
4. Batch les recherches (1 query large > 5 queries étroites)

### Problème : No results found

**Symptômes** :
```
Agent: WebSearch returned no results for "..."
```

**Solutions** :
1. Élargir la query (ex: "React performance" au lieu de "React 18.3.1 performance useMemo")
2. Retirer les contraintes de version trop strictes
3. Essayer des termes alternatifs (ex: "security vulnerability" vs "CVE")
4. Ajouter l'année actuelle : "React patterns 2026"

### Problème : Outdated results

**Symptômes** :
```
Agent: Found article from 2021, may be outdated
```

**Solutions** :
1. Ajouter l'année à la query : "Next.js best practices 2026"
2. Chercher "latest" ou "recent" : "latest React optimization techniques"
3. Utiliser `webfetch` sur les sites officiels qui sont toujours à jour
   ```
   webfetch("https://react.dev/learn")
   ```

---

## Sécurité et confidentialité

### Données transmises à Exa AI
- **Query string uniquement** : La recherche web envoie uniquement le texte de la query
- **Pas de code source** : Le code du projet n'est jamais transmis
- **Pas de secrets** : Les clés API, tokens, etc. restent locaux
- **Anonyme** : Aucune identification utilisateur transmise

### Recommandations
❌ **Ne jamais rechercher** :
- Secrets, clés API, tokens
- Données utilisateur (PII, emails, noms)
- Propriétés intellectuelles (code propriétaire, architecture interne)
- Informations confidentielles client

✅ **Recherches appropriées** :
- Noms de packages publics (npm, PyPI)
- CVE IDs publics
- Concepts techniques génériques ("React performance", "PostgreSQL indexing")
- Documentation publique

---

## Monitoring et métriques

### Logs WebSearch

Les logs OpenCode incluent les requêtes WebSearch :
```
[INFO] WebSearch: "CVE Express.js 4.18.2" → 5 results
[INFO] WebFetch: https://nvd.nist.gov/vuln/detail/CVE-2024-12345
[WARN] WebSearch rate limited, retrying in 60s
```

### Statistiques (RTK plugin)

Si RTK est installé, les stats incluent les WebSearch calls :
```bash
rtk report
```

Sortie :
```
WebSearch Stats (30 days):
  Total queries: 47
  Avg queries/audit: 3.2
  Most common: CVE lookup (35%), library comparison (28%)
  Rate limits: 2 occurrences
```

---

## Migration

### Désactiver WebSearch globalement

Si vous voulez désactiver WebSearch pour tous les projets :

1. Éditer `openhub/opencode.json` :
   ```json
   {
     "permission": {
       "websearch": "deny"
     }
   }
   ```

2. Redéployer tous les projets :
   ```bash
   ./oc.sh deploy all
   ```

### Rollback

En cas de problème, revenir à l'état antérieur :
```bash
cd openhub
git checkout opencode.json
./oc.sh deploy all
```

---

## Ressources

### Documentation OpenCode
- WebSearch tool: https://opencode.ai/docs/tools/#websearch
- Permissions: https://opencode.ai/docs/permissions/
- Environment variables: https://opencode.ai/docs/config/

### Skills openhub
- `skills/shared/websearch-usage.md` — Best practices WebSearch
- `skills/auditor/websearch-cve-lookup.md` — Protocole CVE lookup
- `skills/planning/websearch-stack-research.md` — Recherche de stack
- `skills/design/websearch-design-patterns.md` — Patterns design

### Exemples d'usage
- `docs/guides/websearch-usage-examples.fr.md` — Cas d'usage réels

### Support
- Issues openhub: https://github.com/anomalyco/opencode/issues
- Discord OpenCode: https://opencode.ai/discord

---

## Changelog

### v1.0.0 (2026-05-29)
- Activation WebSearch pour 13 agents (7 auditors, 3 planning, 2 design, 1 documentarian)
- 4 skills spécialisées créées (CVE lookup, performance research, stack research, design patterns)
- Script `oc config websearch enable|disable|status`
- Documentation complète (intégration + exemples)

---

**Contributeurs** : openhub team  
**License** : MIT
