# Guide — Onboarding sur un projet existant

Ce guide couvre l'utilisation de l'agent `onboarder` pour découvrir rapidement
un projet existant : stack, architecture, risques, et agents à prioriser.

---

## Quand invoquer l'onboarder

| Situation | Recommandation |
|-----------|---------------|
| Tu arrives sur un projet que tu ne connais pas | Invoquer l'onboarder en premier |
| Tu reprends un projet après une longue absence | Invoquer l'onboarder pour remettre à jour le contexte |
| Tu vas démarrer une mission importante (feature, refactoring) | Invoquer avant de confier quoi que ce soit à l'orchestrator |
| Tu veux savoir quels agents du hub sont pertinents pour ce projet | L'onboarder produit la carte des agents recommandés |
| L'orchestrator détecte un projet inconnu (Mode C) | L'orchestrator te propose d'invoquer l'onboarder — tu peux accepter ou skipper |

L'onboarder est en **lecture seule**. Il n'écrit aucun code, ne modifie aucun
fichier du projet (sauf `docs/wiki/`, `ONBOARDING.md` minimaliste et `projects.md` — uniquement après confirmation explicite).

Lorsque `docs/wiki/index.md` **existe déjà** (enrichi par d'autres agents),
l'onboarder propose par défaut un **enrichissement incrémental** plutôt qu'une réécriture complète.
Il applique le skill `living-docs-enrichment` et délègue au `documentarian`.
La réécriture complète reste disponible avec un avertissement explicite sur la perte des enrichissements accumulés.

---

## Invoquer l'onboarder

```
"Onboarde-toi sur ce projet"
"Découvre ce projet et donne-moi un état des lieux"
"Avant de commencer, explore le projet"
"Qu'est-ce que ce projet ?"
```

L'onboarder annonce ce qu'il va lire avant de le lire, puis produit un rapport
de contexte structuré.

---

## Exemple de session

### 1. Invocation

```
> "Onboarde-toi sur ce projet"
```

### 2. L'onboarder annonce l'exploration

```
Je vais lire les fichiers de configuration à la racine pour identifier la stack.

Je lis : package.json, .nvmrc, docker-compose.yml, .github/workflows/ci.yml

Stack détectée : Node.js 20 + Vue 3 + PostgreSQL — profil fullstack.

Je vais maintenant explorer les fichiers structurants :
- src/router/index.ts — routes frontend
- src/stores/ — état global Pinia
- src/services/ — appels API
- server/routes/ — endpoints backend
- server/models/ — modèles de données
- migrations/ — migrations en attente
- .env.example — variables d'environnement
```

### 3. Rapport de contexte

```markdown
## Rapport de contexte — mon-app — 2026-03-30

### Stack

| Catégorie | Technologies détectées |
|-----------|----------------------|
| Langage(s) | TypeScript 5.3 |
| Framework(s) | Vue 3.4 + Nuxt 3, Fastify 4 |
| Base(s) de données | PostgreSQL 15 (Prisma ORM), Redis 7 |
| Infrastructure | Docker, GitHub Actions |
| Tests | Vitest, Playwright (E2E) |

### Architecture

Monorepo fullstack — frontend Nuxt en SSR, backend Fastify exposant
une API REST. Communication frontend/backend via fetch natif (pas de
client généré). Redis utilisé pour les sessions et le cache.

### Patterns dominants

- Composition API + `<script setup>` systématique côté Vue
- Repository pattern côté backend (services → repositories → Prisma)
- Conventional Commits respectés dans le git log

### Points d'attention

🔴 **Critiques**
- `server/config/db.ts` ligne 12 : mot de passe de base de données
  en dur dans le fichier (non lu depuis les variables d'environnement)

🟠 **Importants**
- 3 migrations en attente non appliquées (`migrations/20260318_*`,
  `20260322_*`, `20260325_*`) — risque de désynchronisation schema/code
- Aucun fichier de test dans `server/services/` (logique métier non testée)

🟡 **Améliorations**
- Pas d'attributs ARIA sur les formulaires (`src/components/forms/`)
- Bundle non splitté — un seul chunk JS de 1.2MB détecté dans `nuxt.config.ts`

### Zones d'ombre

- Logique d'authentification OAuth déléguée à un service externe —
  pas de code visible dans ce dépôt
- Pas de README — le setup n'est pas documenté

### Questions de clarification

1. Les 3 migrations en attente sont-elles intentionnellement non appliquées
   (en cours de validation) ou oubliées ?
2. Le service d'authentification OAuth externe — a-t-il un dépôt séparé
   ou une documentation disponible ?
3. La clé de base de données en dur dans `db.ts` — est-ce un fichier
   de dev local ou ce fichier est-il commité dans le dépôt ?

### Agents recommandés

#### Prioritaires — zones à risque détectées

| Agent | Pourquoi | Invocation suggérée |
|-------|----------|---------------------|
| `auditor` (security) | Secret en dur dans `server/config/db.ts:12` | `"Audite la sécurité de ce projet"` |
| `developer-security` | À invoquer après l'audit pour corriger les failles | `"Implémente le hardening suite à l'audit sécurité"` |

#### Recommandés — stack détectée

| Agent | Pourquoi | Invocation suggérée |
|-------|----------|---------------------|
| `developer-fullstack` | Vue 3 + Fastify dans le même dépôt | `"Implémente [feature]"` |
| `developer-api` | API REST Fastify exposée | `"Implémente l'endpoint [X]"` |

#### Optionnels — selon les ambitions du projet

| Agent | Pourquoi | Invocation suggérée |
|-------|----------|---------------------|
| `auditor` (accessibility) | Formulaires sans ARIA dans `src/components/forms/` | `"Audite l'accessibilité"` |
| `auditor` (performance) | Bundle JS de 1.2MB non splitté | `"Audite les performances"` |
| `documentarian` | Pas de README — setup non documenté | `"Documente le setup et l'architecture de ce projet"` |

---

> Ces invocations sont des suggestions — c'est à toi de décider quand et si tu les lances.
```

### 4. Q&A — réponses aux questions de clarification

L'onboarder pose ses questions et **attend les réponses avant d'écrire quoi que ce soit**.
Tu peux répondre, ou dire `"passe"` / `"skip"` pour ignorer les questions.

```
1. Les migrations sont en attente depuis la semaine dernière —
   elles attendent une validation métier, pas un oubli.
2. Le service OAuth est dans un dépôt privé : github.com/mon-org/auth-service
3. db.ts est un fichier de dev local non commité — .gitignore le couvre.
```

### 5. Rapport mis à jour

L'onboarder intègre les réponses et réaffiche uniquement les sections impactées :

```markdown
### Zones d'ombre — mises à jour

- ~~Logique d'authentification OAuth~~ → dépôt séparé : `github.com/mon-org/auth-service`
- Pas de README — le setup n'est pas documenté

### Points d'attention — mis à jour

🔴 **Critiques** — inchangés
🟠 **Importants**
- 3 migrations en attente intentionnelles (validation métier) — à appliquer après décision
- ~~Clé de base de données en dur~~ → fichier de dev local non commité, non critique
- Aucun fichier de test dans `server/services/` (logique métier non testée)
```

### 6. Confirmation avant génération

```
Tout est clair — je suis prêt à générer les fichiers de contexte.
Je génère ? (Générer / Annuler)
```

```
→ Générer
```

### 7. Génération du wiki documentaire

L'onboarder produit les fichiers suivants et les ajoute au `.git/info/exclude` (exclusion locale, invisible pour les autres devs) :

- **`docs/wiki/index.md`** — carte globale : stack critique, architecture, god nodes (concepts les plus connectés), carte des domaines métier, points critiques actifs, zones d'ombre
- **`docs/wiki/technical/architecture.md`** — architecture détaillée, découpage, décisions structurantes
- **`docs/wiki/technical/stack.md`** — stack complète, versions, librairies clés, variables d'environnement
- **`docs/wiki/technical/tests.md`** — stratégie de test, frameworks, couverture, conventions
- **`docs/wiki/technical/conventions.md`** — linting, nommage, git, config, patterns équipe
- **`docs/wiki/business/index.md`** — carte des domaines métier
- **`docs/wiki/business/<domaine>.md`** — un fichier par domaine métier détecté : règles de gestion, flux, entités, risques
- **`ONBOARDING.md`** — résumé minimaliste à la racine (15-25 lignes), redirige vers `docs/wiki/index.md`

> Le wiki remplace les anciens fichiers `CONVENTIONS.md`, `docs/context/technical.md` et `docs/context/business/`. L'`index.md` identifie les "god nodes" — les concepts clés apparaissant dans plusieurs pages — pour guider les agents vers l'essentiel.
> Chaque enrichissement porte un tag de confiance : `` `CONFIRMÉ` `` (observation directe dans le code), `` `DÉDUIT` `` (raisonnement contextuel) ou `` `INCERTAIN` `` (hypothèse à valider).

### 8. Proposition de mise à jour `projects.md`

Si le champ `Stack` est absent ou générique dans `projects.md` :

```
J'ai détecté la stack suivante : TypeScript 5.3, Vue 3 + Nuxt 3, Fastify 4,
PostgreSQL 15, Redis 7.

Souhaites-tu que je mette à jour le champ `Stack` dans projects.md ? (oui / non)
```

---

## Interpréter le rapport

### Les niveaux de points d'attention

| Niveau | Signification | Action recommandée |
|--------|--------------|-------------------|
| 🔴 Critique | Impact direct sur la sécurité, la stabilité ou les données | Traiter avant toute nouvelle feature |
| 🟠 Important | Dette technique notable, risque à moyen terme | Planifier dans les prochains sprints |
| 🟡 Amélioration | Opportunité de qualité, performance, accessibilité | Prioriser selon les ambitions du projet |

### La carte des agents

- **Prioritaires** — directement activés par les 🔴/🟠 détectés. À traiter en premier.
- **Recommandés** — déterminés par la stack. Ce sont les agents que tu utiliseras au quotidien.
- **Optionnels** — pertinents selon les objectifs du projet. À activer quand le moment est venu.

Les invocations suggérées sont des points de départ — adapte-les au contexte réel.

### Les zones d'ombre

Les zones d'ombre sont ce que l'onboarder **ne peut pas déterminer** depuis la
codebase. Ce n'est pas un échec — c'est une information utile. Les questions de
clarification qui suivent t'aident à combler ces lacunes.

Après avoir reçu tes réponses, l'onboarder met à jour le rapport (sections
impactées uniquement), puis demande une confirmation explicite avant d'écrire
les fichiers de contexte. Ils reflètent ainsi l'analyse enrichie, pas le premier jet.

---

## Structure du wiki documentaire produit

L'onboarder génère une structure wiki à deux niveaux optimisant le chargement de contexte en session :

```
docs/wiki/
├── index.md                    ← toujours lu en premier — 40-80 lignes
├── technical/
│   ├── architecture.md         ← chargé si la tâche concerne l'architecture
│   ├── stack.md                ← chargé si la tâche concerne les dépendances
│   ├── tests.md                ← chargé si la tâche concerne les tests
│   └── conventions.md          ← chargé si la tâche concerne le code
└── business/
    ├── index.md                ← carte des domaines
    └── <domaine>.md            ← chargé si la tâche concerne ce domaine
ONBOARDING.md                   ← minimaliste, redirige vers docs/wiki/index.md
```

**`docs/wiki/index.md`** contient : stack critique (3-5 lignes), architecture (2-3 lignes),
tableau des god nodes (concepts critiques apparaissant dans plusieurs pages), carte des
domaines métier, points critiques actifs, zones d'ombre.

**`docs/wiki/technical/conventions.md`** contient : linting, langage & typage, nommage,
conventions Git, config & secrets, patterns équipe, à ne pas utiliser.

Les autres pages `technical/` et `business/<domaine>.md` sont chargées à la demande par
les agents selon leur tâche courante — jamais toutes en même temps.

---

## Intégration dans un workflow orchestrator

L'orchestrator peut proposer d'invoquer l'onboarder automatiquement en **Mode C**
quand il détecte un projet inconnu. Ce mode est toujours optionnel.

### Workflow complet avec Mode C

```
1. Tu demandes à l'orchestrator : "Implémente la feature d'authentification JWT"
2. L'orchestrator détecte que le projet n'a pas été exploré dans cette session
3. L'orchestrator propose :
   "Projet inconnu. Invoquer l'onboarder en premier ? (oui / non — skip si tu connais déjà le projet)"
4. Tu réponds "oui"
5. L'onboarder explore le projet et produit le rapport
6. [CP-onboard] L'orchestrator présente le résumé du rapport et demande :
   "Contexte suffisant pour démarrer la feature ? (oui / non — questions ?)"
7. Tu valides → l'orchestrator continue en Mode A (planner → routing)
```

### Skipper le Mode C

Si tu connais déjà le projet ou que tu n'as pas besoin du rapport :

```
> "Non, skip — je connais le projet"
```

L'orchestrator passe directement au Mode A ou B.

---

## Cas d'usage avancés

### Onboarder + planner en séquence

L'onboarder identifie des zones de dette ou des risques → tu peux ensuite demander
au planner de créer des tickets pour les traiter :

```
1. "Onboarde-toi sur ce projet"
   → Rapport : migrations en attente, tests manquants sur les services

2. "Crée des tickets pour les points d'attention identifiés"
   → Le planner crée les tickets de dette avec les bonnes priorités
```

### Onboarder + auditor en séquence

L'onboarder signale un risque sécurité prioritaire → tu lances l'audit ciblé :

```
1. "Onboarde-toi sur ce projet"
   → Rapport : secret en dur, CORS absent

2. "Audite la sécurité de ce projet"
   → L'auditor approfondit l'analyse (domaine security)

3. "Implémente le hardening suite à l'audit sécurité"
   → Le developer-security corrige les failles
```

### Re-onboarding après une longue absence

L'onboarder peut être invoqué à tout moment — pas seulement la première fois.
Si tu reviens sur un projet après plusieurs semaines :

```
"Onboarde-toi sur ce projet — j'ai été absent 3 semaines"
```

L'onboarder lira l'état actuel de la codebase, les tickets récemment clos,
et te donnera un état des lieux à jour.

### Re-onboarding — mode enrichissement incrémental

Lorsque `docs/wiki/index.md` existe déjà (généré lors d'un onboarding précédent
et progressivement enrichi par d'autres agents : `developer-*`, `reviewer`, `auditor`, etc.),
l'onboarder le détecte en Phase 5 et propose trois options :

1. **Enrichissement incrémental (Recommandé)** — les pages wiki concernées sont enrichies
   via le skill `living-docs-enrichment` qui délègue au `documentarian` ; les enrichissements
   existants (avec leurs tags de confiance) sont préservés.
2. **Réécriture complète** — avec un avertissement explicite sur la perte des enrichissements accumulés.
3. **Conserver l'existant** — aucune modification.

Si un **nouveau domaine métier** est découvert lors du re-onboarding, l'onboarder propose
la création d'une nouvelle page `docs/wiki/business/<nouveau-domaine>.md` et la mise à
jour de `docs/wiki/business/index.md`.

Cela préserve la boucle d'amélioration continue : le wiki accumule les connaissances de
tous les agents tout au long du cycle de vie du projet, chaque enrichissement étant traçable
avec son agent source, sa date et son niveau de confiance.
