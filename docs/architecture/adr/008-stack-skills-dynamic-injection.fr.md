> 🇬🇧 [Read in English](008-stack-skills-dynamic-injection.en.md)

# ADR-008 — Injection dynamique des skills spécifiques aux stacks au déploiement

## Statut

Accepté — **Évolué par [ADR-010](./010-hybrid-skills-architecture.fr.md)**

Le mécanisme de détection de stack (`detect_stack()`, `config/stack-skills.json`) reste valide. L'ADR-010 change le chemin de déploiement : les stack skills ne sont plus assemblées inline dans les system prompts des agents. Elles sont désormais déployées vers `.opencode/skills/` par `deploy_native_skills()` et chargées à la demande à l'inférence via l'outil `skill` (Bucket B). Cela complète l'intention de l'ADR-008 — les agents ne reçoivent le contexte de stack que quand la tâche le requiert.

## Contexte

Les agents developer avaient des skills spécifiques à leur stack codés en dur dans leur frontmatter (ex : `dev-standards-vuejs` toujours injecté dans `developer-frontend`, quelle que soit la stack du projet). Cela forçait tous les projets à recevoir des conventions Vue.js même pour des codebases React ou Angular.

Avec l'introduction de 38 skills spécifiques aux stacks couvrant 9 catégories (langages, frontend, backend, ORMs, outils de test, mobile, data/ML, DevOps, platform), maintenir des variantes d'agents par stack n'était pas viable. Une approche `developer-frontend-vue`, `developer-frontend-react`, etc. aurait créé N × M combinaisons non maintenables et incompatibles avec la matrice de routing de l'agent orchestrator.

Les skills génériques (`dev-standards-universal`, `dev-standards-backend`, etc.) contenaient également des références framework-spécifiques qui s'étaient accumulées au fil du temps, en faisant des doublons partiels des skills spécifiques.

## Décision

- Créer un dossier `skills/developer/stacks/` pour les skills atomiques spécifiques aux stacks (un fichier par stack, une responsabilité par fichier).
- Déclarer le mapping entre les stacks détectées et les skills à injecter dans `config/stack-skills.json`. Chaque type d'agent a un scope défini (`_agent_scope`) qui limite les catégories de stack skills qu'il reçoit.
- Détecter la stack du projet à chaque `oc deploy` via `detect_stack(project_path)` : lecture de `package.json`, `pyproject.toml`, `requirements.txt`, `Gemfile`, `build.gradle`, `pom.xml`, `pubspec.yaml` et des fichiers d'infrastructure (`Dockerfile`, `.github/workflows/`, `*.tf`, `Chart.yaml`, manifests ArgoCD, etc.).
- Injecter les skills correspondants dynamiquement via `resolve_stack_skills(agent_id, stacks, config)`, qui filtre par scope d'agent et déduplique les skills déjà déclarés dans le frontmatter.
- Les stack skills sont **additifs** : ils s'ajoutent après les skills statiques déclarés dans le frontmatter de l'agent — jamais en remplacement.
- Purger toutes les références framework-spécifiques des skills génériques (`dev-standards-universal`, `dev-standards-testing`, `dev-standards-api`, `dev-standards-security`, `dev-standards-devops`) pour qu'ils restent véritablement agnostiques des outils.
- Étendre `oc deploy --check` pour re-détecter la stack et vérifier les mtimes des stack skills injectés dynamiquement, de sorte que les modifications de `skills/developer/stacks/*.md` déclenchent correctement une obsolescence pour les agents concernés.

## Conséquences

### Positives

- Un seul agent `developer-frontend` couvre Vue, React, Angular, Next.js, Nuxt.js sans duplication : chaque projet reçoit uniquement les conventions qui correspondent à sa stack réelle.
- Ajouter une nouvelle stack nécessite uniquement un fichier skill + une entrée dans `config/stack-skills.json` — aucun changement de frontmatter d'agent requis.
- Les standards sont au plus précis pour la stack réelle du projet.
- Les skills génériques sont épurés et véritablement agnostiques : ils se concentrent sur les principes, pas les outils.
- `oc deploy --check` détecte correctement l'obsolescence causée par des changements de stack skills.

### Négatives / trade-offs

- `jq` est une dépendance runtime pour `resolve_stack_skills`. Ce prérequis existait déjà pour d'autres fonctions du hub — il n'introduit pas de nouvelle contrainte.
- Les agents déployés au niveau hub (sans `PROJECT_ID`) ne bénéficient pas de l'injection de stack skills — le chemin de déploiement manque d'un contexte de projet pour la détection.
- Les heuristiques de détection dans `detect_stack()` sont basées sur la présence de fichiers et peuvent occasionnellement produire des faux positifs (ex : détecter `docker` si un `Dockerfile` est présent pour une raison non liée). Ce sont des faux positifs à faible risque qui ajoutent du contexte sans impact négatif.

## Alternatives rejetées

**Variantes d'agents par stack** (`developer-frontend-vue`, `developer-frontend-react`, etc.) : rejeté car N agents × M stacks est non maintenable, casse la matrice de routing de l'agent orchestrator, et duplique la logique du corps des agents.

**Stack déclarée dans `hub.json`** : déclarer la stack du projet dans la config du hub et générer les frontmatters correspondants. Rejeté car cela crée un couplage fort entre la config hub et les agents canoniques, nécessite une maintenance manuelle à chaque changement de stack, et ne supporte pas la détection automatique.

**Skills déclarés par projet dans `projects.md`** : étendre le format `projects.md` avec un champ `skills:` de surcharge. Rejeté car cela déplace les décisions de configuration du hub (qui gère les agents canoniques) vers le fichier d'enregistrement des projets, créant une double source de vérité.
