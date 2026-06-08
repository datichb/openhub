> 🇬🇧 [Read in English](010-hybrid-skills-architecture.en.md)

# ADR-010 — Architecture hybride des skills : Inline (Bucket A) vs Natif (Bucket B)

## Statut

Accepté

## Contexte

Au fur et à mesure que le hub a atteint 40+ skills, toutes les skills étaient assemblées inline dans les agents au moment du déploiement. Chaque system prompt d'agent contenait le texte complet de chaque skill déclarée dans son frontmatter — y compris les standards spécifiques à un domaine (conventions Vue.js, durcissement sécurité, checklists WCAG) qui ne sont pertinents que pour un sous-ensemble de tâches.

Cela créait trois problèmes :

1. **Gonflement de tokens** : Les agents développeur avaient des system prompts dépassant 8 000 tokens avant le premier message utilisateur. Les skills spécifiques aux stacks (TypeScript, React, NestJS, Prisma…) étaient toujours incluses, même pour des tâches sans rapport avec ces stacks.
2. **Bruit cognitif** : Le LLM reçoit tout le contexte dès le départ, indépendamment de la tâche réelle. Un agent `developer-frontend` travaillant sur un bug CSS pur reçoit les standards NestJS, Prisma et OpenAPI complets sans bénéfice.
3. **Stack skills partiellement redondantes avec l'ADR-008** : L'injection dynamique de stack (ADR-008) était censée ajouter _uniquement_ les skills du stack du projet. Mais le modèle inline embarquait quand même ces skills en permanence dans le system prompt une fois injectées, sans possibilité de les charger conditionnellement à l'inférence.

OpenCode a introduit un outil natif `skill` qui permet aux agents de charger des fichiers de skills à la demande au moment de l'inférence, depuis `.opencode/skills/<name>/SKILL.md`. Cela permet un modèle de chargement sélectif et orienté tâche.

## Décision

Diviser les skills en deux buckets selon leur exigence de garantie de chargement :

### Bucket A — Inline (obligatoire, toujours actif)

Skills qui **doivent être actives dès le premier token** parce qu'elles définissent le comportement fondamental de l'agent, les contrats de sortie, ou la structure du workflow.

Déclarées dans le frontmatter de l'agent sous `skills: [...]`. Assemblées inline au déploiement par `prompt-builder.sh`. Le LLM ne peut pas les ignorer.

**Le Bucket A comprend :**
- Protocoles de workflow (`*-protocol`, `*-workflow`)
- Formats de handoff (`*-handoff-format`) — contrats partagés entre agents producteur et consommateur
- Skills d'exécution de base (`beads-plan`, `beads-dev`, `quick-fix`)
- Principes universels (`dev-standards-universal`, `dev-standards-simplicity`)
- Skills de posture (`expert-posture`, `tool-question`, `coordination-only`, `retranscription-coordinateur`)
- Skills de documentation vivante (`shared/living-docs-enrichment` — tous les agents produisant une analyse ou une implémentation)

### Bucket B — Natif (contextuel, à la demande)

Skills qui **fournissent un contexte spécifique à un domaine** et ne sont pertinentes que pour un sous-ensemble de tâches. Chargées par le LLM à la demande via l'outil `skill` quand la tâche le requiert.

Déclarées dans le frontmatter de l'agent sous `native_skills: [...]`. Déployées par `deploy_native_skills()` dans `opencode.adapter.sh` vers `.opencode/skills/<name>/SKILL.md`. Le guide dans le body de l'agent liste les skills natives disponibles et quand les charger.

**Le Bucket B comprend :**
- Standards de domaine (`dev-standards-security`, `dev-standards-backend`, `dev-standards-frontend`, `dev-standards-testing`, `dev-standards-git`, etc.)
- Skills spécifiques aux stacks (`developer/stacks/*`)
- Checklists de domaine d'audit (`audit-security`, `audit-performance`, `audit-accessibility`, etc.)
- Skills de type documentaire (`doc-standards`, `doc-adr`, `doc-api`, `doc-changelog`, `doc-slides`)
- Skills de recherche contextuelle (`websearch-stack-research`, `websearch-design-patterns`, `websearch-cve-lookup`, `websearch-performance-research`)

### Modèle de permission

- Agents qui utilisent des skills natives : `permission: skill: allow` dans le frontmatter
- Agents coordinateurs/orchestrateurs (pas de skills natives nécessaires) : `permission: skill: deny`

### Mécanisme de déploiement

`deploy_native_skills()` dans `opencode.adapter.sh` :
1. Collecte toutes les entrées `native_skills` de tous les frontmatters d'agents
2. Collecte les stack skills de `config/stack-skills.json` pour le stack du projet détecté
3. Déduplique par basename
4. Efface entièrement `.opencode/skills/`, puis le recrée avec un `SKILL.md` par skill
5. Chaque `SKILL.md` généré a un frontmatter opencode valide (`name:`, `description:`)

L'approche effacement-et-recréation garantit que les skills obsolètes des déploiements précédents ne sont jamais laissées derrière.

## Conséquences

### Positives

- Taille du system prompt réduite significativement pour tous les agents — les standards de domaine ne sont plus injectés inconditionnellement.
- Le LLM reçoit le contexte de domaine exactement quand il est pertinent pour la tâche, pas toujours.
- Ajouter une nouvelle skill de standard de domaine n'augmente pas la taille du system prompt de base.
- Les stack skills (ADR-008) suivent désormais le même chemin natif, complétant l'intention de l'ADR-008.
- La distinction obligatoire/optionnelle est explicite dans le frontmatter, rendant l'intention de l'auteur de l'agent claire.

### Négatives / compromis

- Si un agent oublie de charger une skill native avant de produire sa sortie, les standards de domaine sont absents. Atténué par la section guide des skills dans le body de l'agent qui liste les skills disponibles et leurs déclencheurs de chargement.
- L'outil `skill` doit être disponible (`permission: skill: allow`) — les coordinateurs qui n'ont jamais besoin de skills contextuelles définissent explicitement `skill: deny`.
- Le répertoire `.opencode/skills/` est entièrement régénéré à chaque déploiement (effacement + recréation). C'est intentionnel pour la cohérence mais signifie que les skills ne peuvent pas être patché manuellement dans le projet cible.

## Relation avec les autres ADRs

- **ADR-001** (Séparation Agent/Skill) : évolué — la séparation a maintenant deux chemins de déploiement (inline vs natif) plutôt qu'un assemblage unique au déploiement.
- **ADR-008** (Injection dynamique des stack skills) : évolué — les stack skills se déploient maintenant via `deploy_native_skills()` vers `.opencode/skills/` et sont chargées nativement à l'inférence, plutôt qu'être assemblées inline.
