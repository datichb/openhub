# Guide de contribution

Ce guide explique comment ajouter un agent, un skill ou un adapter au hub,
et comment contribuer via une PR.

---

## Ajouter un agent

### 1. Créer le fichier agent

```bash
touch agents/<famille>/<id>.md
```

Respecter la convention de nommage :
- `<domaine>-<spécialité>.md` pour les sous-agents (ex: `auditor-security.md`)
- `<rôle>.md` pour les agents principaux (ex: `orchestrator.md`)

### 2. Structure minimale du frontmatter

```markdown
---
id: <identifiant-unique>
label: <NomAffiché>
description: <Description courte en une phrase — visible dans les listes d'agents>
permission:
  skill: allow        # allow pour les agents utilisant des skills natives ; deny pour les coordinateurs
skills: [chemin/vers/skill, ...]          # Bucket A — skills inline toujours actives
native_skills: [chemin/vers/skill, ...]   # Bucket B — skills natives à la demande (optionnel)
---
```

**Règles :**
- `id` : slug unique, minuscules, tirets autorisés, pas d'espaces
- `label` : PascalCase, affiché dans l'outil IA
- `description` : une phrase, commence par un verbe ou un nom de rôle
- `skills` : chemins Bucket A relatifs à `skills/` — protocoles de workflow, formats de handoff, principes universels (toujours actifs)
- `native_skills` : chemins Bucket B relatifs à `skills/` — standards de domaine, checklists, skills contextuelles (chargées à la demande via l'outil `skill`)
- `permission.skill` : `allow` si l'agent utilise des skills natives ; `deny` pour les coordinateurs/orchestrateurs

Voir [ADR-010](../architecture/adr/010-hybrid-skills-architecture.fr.md) pour le raisonnement Bucket A / B.

### 3. Corps de l'agent

Structure recommandée (voir `agents/auditor/auditor.md` comme référence pour les coordinateurs,
`agents/developer/developer-frontend.md` pour les agents implémenteurs) :

```markdown
# <NomAffiché>

<Phrase d'identité : qui tu es et ce que tu fais en 2-3 lignes>

## Ce que tu fais

- <Action 1>
- <Action 2>

## Ce que tu NE fais PAS

- <Contrainte 1>
- <Contrainte 2>

## Workflow

<Workflow condensé en 4-6 étapes>

## Exemples d'invocation (optionnel)

| Demande | Action |
|---------|--------|
| "..." | ... |
```

### 4. Créer ou référencer les skills

Si l'agent nécessite un protocole dédié, créer le skill correspondant
(voir section "Ajouter un skill" ci-dessous) avant de le référencer dans le frontmatter.

### 5. Déployer et tester

```bash
oc deploy opencode
# Vérifier que l'agent apparaît
oc agent list
oc agent info <id>
```

---

## Ajouter un skill

### 1. Choisir le bon dossier

Les skills sont organisés par domaine dans `skills/` :

| Dossier | Usage |
|---------|-------|
| `skills/developer/` | Standards de développement (partagés entre developers et reviewer) |
| `skills/auditor/` | Protocoles d'audit |
| `skills/orchestrator/` | Protocoles de coordination |
| `skills/planning/` | Protocoles de planification |
| `skills/qa/` | Protocoles qualité |
| `skills/debugger/` | Protocoles de diagnostic |
| `skills/reviewer/` | Protocoles de review |
| `skills/documentarian/` | Protocoles de documentation |
| `skills/designer/` | Protocoles de design (ux-designer, ui-designer) |
| `skills/design/` | Contrats de handoff design |
| `skills/quality/` | Contrats de handoff qualité (debugger et agents hors `qa/` et `reviewer/`) |
| `skills/posture/` | Posture et comportement transversal (`expert-posture`, `tool-question`) |

Pour un nouveau domaine, créer un nouveau sous-dossier.

### 2. Structure minimale du frontmatter

```markdown
---
name: <nom-du-skill>
description: <Description courte — visible dans oc agent edit et oc skills list>
---
```

> La clé `name` est documentaire. Les scripts lisent uniquement `description`.
> Le chemin du fichier est la référence utilisée dans le frontmatter des agents.

### 3. Contenu du skill

Un bon skill contient :

- **Rôle** : rappel de l'identité de l'agent qui utilise ce skill
- **Règles absolues** : ❌/✅ — les contraintes non négociables
- **Protocole / workflow** : les étapes détaillées
- **Formats de sortie** : les structures exactes des rapports, avec exemples
- **Checklists** : les vérifications systématiques
- **Ce que tu ne fais PAS** : les anti-patterns explicites

Voir `skills/reviewer/review-protocol.md` ou `skills/qa/qa-protocol.md` comme exemples.

### 4. Référencer le skill dans un agent

Déterminer si le skill est Bucket A ou Bucket B (voir [ADR-010](../architecture/adr/010-hybrid-skills-architecture.fr.md)) :

**Bucket A** — ajouter dans `skills:` du frontmatter de l'agent :
```markdown
---
skills: [chemin/vers/mon-skill]
---
```

**Bucket B** — ajouter dans `native_skills:` du frontmatter de l'agent, et ajouter `permission: skill: allow` :
```markdown
---
permission:
  skill: allow
native_skills: [chemin/vers/mon-skill]
---
```
Ajouter également une ligne dans la section guide "## Skills disponibles" du corps de l'agent avec le déclencheur de chargement.

**Skills de handoff :** si votre skill définit un format de retour structuré entre deux agents (un bloc `## Retour vers ...`), il est toujours **Bucket A** — l'injecter dans **les deux** agents — l'agent producteur (celui qui produit le bloc) et l'agent consommateur (celui qui lit le bloc). Cela garantit que les deux agents partagent le même contrat. Voir `skills/reviewer/reviewer-handoff-format.md` ou `skills/auditor/audit-handoff-format.md` comme exemples.

---

## Ajouter un adapter

Un adapter traduit les agents du format hub vers le format d'un outil cible.

Le contrat complet (8 fonctions obligatoires, paramètres, fonctions utilitaires
disponibles et exemple minimal) est documenté dans
[docs/architecture/adapters.fr.md](../architecture/adapters.fr.md).

### Étapes rapides

1. Créer `scripts/adapters/<cible>.adapter.sh` avec les 8 fonctions du contrat
2. Ajouter la cible dans `config/hub.json`
3. Tester avec `oc deploy <cible>` puis `oc agent list`

---

## Conventions de contribution

### Commits

Format **Conventional Commits** obligatoire :

```
feat: ajouter l'agent <nom>
fix: corriger <problème> dans <fichier>
docs: mettre à jour <section>
chore: <maintenance>
refactor: <restructuration>
```

### Nommage des fichiers

| Type | Convention | Exemple |
|------|-----------|---------|
| Agent | `<domaine>[-<spécialité>].md` | `developer-frontend.md` |
| Skill (dans un sous-dossier) | `<domaine>-<sujet>.md` | `audit-security.md` |
| Script shell | `cmd-<commande>.sh` | `cmd-deploy.sh` |
| Adapter | `<cible>.adapter.sh` | `opencode.adapter.sh` |

### Scripts shell

Règles obligatoires pour tous les scripts shell :

```bash
#!/bin/bash
set -euo pipefail

# ✅ Les variables locales sont déclarées dans des fonctions
my_function() {
  local my_var="value"
}

# ❌ Jamais de 'local' hors d'une fonction — undefined behavior avec set -euo pipefail
# ❌ Jamais de "$var" && commande — toujours if [ "$var" = "true" ]
```

### Documentation interne

Le dossier `docs/` est organisé comme suit (pour référence lors d'ajouts de documentation) :

- `docs/architecture/` — décisions architecturales (ADR), schémas, référence agents et skills
- `docs/guides/` — guides pratiques (contribution, workflows, authoring)
- `docs/reference/` — référence CLI et configuration
- `docs/dev/` — notes techniques internes (gotchas, patterns shell)

### ADR

Toute décision architecturale significative doit être documentée dans un ADR :

```bash
touch docs/architecture/adr/<NNN>-<titre-kebab-case>.md
```

Format : voir [ADR-001](../architecture/adr/001-agent-skill-separation.fr.md) comme modèle.

### PR

Avant de soumettre une PR :

```bash
# Vérifier que les agents déploient correctement
oc deploy opencode
oc deploy --check opencode

# Lister les agents pour vérifier la cohérence
oc agent list
```

---

## Checklist avant PR

- [ ] Le fichier agent respecte la structure minimale (frontmatter + corps)
- [ ] Le skill a un frontmatter avec `name` et `description`
- [ ] Les skills Bucket A sont dans `skills:`, les skills Bucket B sont dans `native_skills:` — raisonnement documenté dans [ADR-010](../architecture/adr/010-hybrid-skills-architecture.fr.md)
- [ ] Si l'agent utilise `native_skills:`, `permission: skill: allow` est défini ; si c'est un coordinateur, `skill: deny` est défini
- [ ] Si l'agent a des skills natives, le corps de l'agent a une section guide "## Skills disponibles" les listant avec les déclencheurs de chargement
- [ ] L'agent est référencé dans `README.md` et `docs/architecture/agents.fr.md`
- [ ] Le skill est référencé dans `docs/architecture/skills.fr.md` avec son marqueur de bucket (A) ou (B)
- [ ] Si le skill définit un format de retour structuré : injecté dans l'agent producteur ET l'agent consommateur (toujours Bucket A)
- [ ] Si décision architecturale : un ADR est créé dans `docs/architecture/adr/`
- [ ] Le commit respecte les Conventional Commits
- [ ] `oc deploy opencode` et `oc deploy --check opencode` passent sans erreur
- [ ] `oc deploy --diff opencode` ne montre aucune divergence inattendue

---

## Créer une release

> Réservé aux mainteneurs disposant d'un accès en écriture sur `main`.

### Prérequis

- Être sur la branche `main` avec un working tree propre
- `jq` installé (`brew install jq` ou `apt install jq`)
- Accès push au dépôt distant

### Préparation du CHANGELOG

Avant de lancer le script, rédiger le contenu de la release sous `## [Unreleased]` dans `CHANGELOG.md` :

```markdown
## [Unreleased]

### Ajouté
- ...

### Corrigé
- ...
```

Le script insérera automatiquement l'en-tête `## [X.Y.Z] — YYYY-MM-DD` après `[Unreleased]`.

### Lancer la release

```bash
# Prévisualiser sans rien modifier (dry-run)
bash scripts/release.sh 1.2.0 --dry-run

# Créer la release
bash scripts/release.sh 1.2.0
```

Le script effectue dans l'ordre :
1. Valide le format `X.Y.Z`
2. Vérifie branche `main` + working tree propre + tag inexistant
3. Met à jour `config/hub.json` (local, non tracké par git) → `.version`
4. Met à jour `config/hub.json.example` (tracké) → `.version`
5. Insère `## [X.Y.Z] — date` dans `CHANGELOG.md`
6. Crée le commit `chore(release): vX.Y.Z`
7. Crée le tag annoté `vX.Y.Z`
8. Propose de pusher (`git push && git push --tags`)

### Convention des fichiers de version

| Fichier | Tracké git | Rôle |
|---------|-----------|------|
| `config/hub.json` | Non (local) | Config active de l'instance — jamais committé |
| `config/hub.json.example` | Oui | Source de vérité de la version — committé à chaque release |

Un nouvel utilisateur qui clone le dépôt exécute `install.sh` qui copie `hub.json.example` → `hub.json`.

### Convention des tags

Les tags suivent le format `vX.Y.Z` (annoté) :

```bash
git tag -a v1.2.0 -m "Release v1.2.0"
```

### One-liner d'installation post-release

Après le push, le one-liner pour installer cette version est :

```bash
curl -fsSL https://raw.githubusercontent.com/datichb/opencode-hub/main/install.sh | VERSION=v1.2.0 bash
```
