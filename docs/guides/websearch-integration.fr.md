> [Read in English](websearch-integration.en.md)

# Guide d'intégration WebSearch — openhub

**Version**: 1.0.0  
**Date**: 2026-05-29  
**Public**: Utilisateurs openhub dont les agents ont besoin de recherche web

---

## Vue d'ensemble

WebSearch permet aux agents openhub de **rechercher sur le web** via Exa AI (hébergé par OpenCode) pour accéder à des informations actuelles non disponibles dans les données d'entraînement du modèle. Cette capacité est particulièrement utile pour :

- **Audits de sécurité** : Recherche de CVE et advisories
- **Planification** : Comparaison de stacks, découverte de librairies, documentation
- **Design** : Patterns UI/UX, tendances 2026, guidelines WCAG 2.2
- **Performance** : Best practices, benchmarks, optimizations

### Prérequis

- oh v5 installé et configuré
- opencode V2 (≥ 2.0.0) : opencode V1 n'est plus pris en charge (voir le [guide de migration v5](migration-v5.fr.md))

---

## Architecture

```
openhub/
├── agents/
│   ├── auditor/
│   │   └── auditor-subagent.md    ← Permission websearch activée
│   ├── planning/
│   │   ├── pathfinder.md               ← Permission websearch activée
│   │   ├── onboarder.md           ← Permission websearch activée
│   │   └── planner.md             ← Permission websearch activée
│   ├── design/
│   │   └── designer.md            ← Permission websearch activée
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
└── cli/
    └── cmd/config.go                ← Commande: oh config websearch enable

~/.oh/hub.toml                       ← [websearch] enabled = true|false

Au lancement (oh v5, rien n'est déployé dans le projet) :
~/.oh/bundles/<hash>/              ← Paquet de session : reçoit les permissions websearch/webfetch si activé
```

---

## Installation

### 1. Activer WebSearch au niveau hub

```bash
oh config websearch enable
```

**Sortie attendue** :
```
WebSearch activé — les agents peuvent effectuer des recherches web
  Pris en compte au prochain lancement d'une session (paquet reconstruit)
```

La commande écrit `[websearch] enabled = true` dans `~/.oh/hub.toml` (équivalent : `oh config set websearch.enabled true`). Le réglage vaut pour tous les projets du hub.

### 2. Lancer une session

Aucun redéploiement n'est nécessaire (`oh deploy` supprimé en v5) : les permissions sont
placées dans le paquet de session au prochain lancement (`oh run <workflow>`).

**Vérification** :
```bash
# Inspecter le paquet de session d'un workflow
oh bundle show <workflow> -p mon-projet
```

Quand WebSearch est activé, le paquet autorise les outils `websearch` et `webfetch`.

---

## Utilisation

### Lancer un agent avec WebSearch

```bash
cd /path/to/mon-projet

# Audit de sécurité avec recherche CVE
oh run audit -i type=security

# Cadrage avec recherche de stack (pathfinder, planner, designer)
oh run cadrage -i request="Ajouter l'export PDF"

# Session avec l'agent de ton choix
oh run libre --agent designer
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
oh config websearch status
```

**Sortie attendue** :
```
WebSearch : activé
```

---

## Agents avec WebSearch

### 13 agents supportés

| Famille | Agent | Use Cases WebSearch |
|---------|-------|---------------------|
| **Auditors** (1) | | |
| | `auditor-subagent` | CVE lookup, security advisories, OWASP updates, performance benchmarks, WCAG guidelines, design patterns, green coding, observability patterns, RGPD updates |
| **Planning** (3) | | |
| | `pathfinder` | Quick stack research, library comparison |
| | `onboarder` | Tech stack documentation, setup guides |
| | `planner` | Library comparison, architecture patterns, integration guides |
| **Design** (1) | | |
| | `designer` | UX patterns, interaction best practices, usability research, UI patterns, design systems, visual trends |
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

### Pas de réglage par projet

En v5, `oh config websearch` ne prend pas de projet : le réglage est global au hub. Le `.opencode/opencode.json` d'un projet n'est plus lu par les sessions (monde fermé : seules les permissions du paquet comptent). Les permissions par agent se règlent dans le frontmatter des agents du hub (`permission:`), qui sont compilées dans le paquet.

### Désactiver WebSearch

```bash
oh config websearch disable
```

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
1. Vérifier que WebSearch est activé
   ```bash
   oh config websearch status
   ```
2. Relancer la session (le paquet est reconstruit au lancement)
   ```bash
   oh run <workflow> -p mon-projet
   ```
3. Vérifier la version d'opencode (V2 requise) et l'état du runtime
   ```bash
   oh doctor
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

### Statistiques

Le plugin global RTK (`oh plugin`) a été retiré en v5 (voir [Plugin RTK — retiré en v5](rtk-plugin-installation.fr.md)) : oh ne fournit plus de statistiques WebSearch dédiées. Le flux d'une session (`oh session follow <id>`, ou `t` dans la vue Sessions) montre les appels d'outils, dont `websearch` et `webfetch`.

---

## Migration

### Désactiver WebSearch globalement

Si vous voulez désactiver WebSearch pour tous les projets :

1. Lancer :
   ```bash
   oh config websearch disable
   ```

2. Aucun redéploiement nécessaire : le changement s'applique au prochain lancement de session (paquet reconstruit). Les sessions déjà lancées gardent leur paquet.

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
- Activation WebSearch pour 7 agents (1 auditor-subagent, 3 planning, 2 design, 1 documentarian)
- 4 skills spécialisées créées (CVE lookup, performance research, stack research, design patterns)
- Script `oh config websearch enable|disable|status`
- Documentation complète (intégration + exemples)

---

**Contributeurs** : openhub team  
**License** : MIT
