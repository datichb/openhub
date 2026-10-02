> [Read in English](sweep-mode.en.md)

# Mode sweep — Guide

## Vue d'ensemble

Le mode sweep est un modele d'execution oriente objectif : vous decrivez un objectif de haut niveau, et openhub le decompose en sous-taches independantes, les execute en parallele, et verifie optionnellement les resultats.

```
Objectif -> Decomposer -> Executer (parallele) -> Collecter -> Fusionner -> Verifier
```

---

## Demarrage rapide

```bash
oh start --sweep "Migrate all deprecated API calls to v2" --sweep-strategy llm
```

---

## Strategies de decomposition

| Strategie | Description | Ideal pour |
|-----------|-------------|------------|
| `manual` | Liste de taches fournie par l'utilisateur via `--sweep-tasks` | Sous-taches connues et bien definies |
| `by-file` | Regroupe les fichiers correspondant a des patterns glob | Transformations au niveau fichier (en-tetes, formatage) |
| `by-package` | Une tache par package Go (via `go list`) | Refactoring au niveau package |
| `llm` | Un planificateur LLM decompose l'objectif en taches | Objectifs complexes necessitant une analyse |

---

## Reference des commandes

```bash
oh start --sweep "objectif" --sweep-strategy <strategie> [options]
```

| Flag | Description | Defaut |
|------|-------------|--------|
| `--sweep` | Description de l'objectif de haut niveau (requis) | — |
| `--sweep-strategy` | Strategie de decomposition (requis) | — |
| `--sweep-tasks` | Liste manuelle de taches, separees par des virgules (necessite la strategie `manual`) | — |
| `--sweep-include` | Patterns glob a inclure | — |
| `--sweep-exclude` | Patterns glob a exclure | — |
| `--sweep-verify` | Verification post-sweep : `none`, `tests`, `lint`, `build`, `all`, `custom` | `none` |
| `--sweep-verify-cmd` | Commande de verification personnalisee (necessite `custom` pour verify) | — |
| `--sweep-dry-run` | Afficher le plan des taches sans executer | false |
| `--sweep-branch-prefix` | Prefixe du nom de branche | `sweep/` |
| `--max-sessions` | Nombre maximal de sessions simultanees | Defaut de la config |
| `--project` / `-p` | Identifiant du projet | Detection automatique |

---

## Pipeline

### 1. Decomposition (Split)

Le decomposeur analyse l'objectif et cree une liste de sous-taches independantes. Chaque tache comprend :
- Une description de ce qu'il faut faire
- Un perimetre (liste de fichiers ou packages a modifier)
- Une contrainte d'isolation (l'agent est instruit de ne modifier que les fichiers dans son perimetre assigne)

### 2. Chemin rapide mono-tache

Lorsque la decomposition produit exactement **1 tache**, le mode sweep contourne l'infrastructure parallele complete et s'execute directement en mode headless. Cela economise environ 5 secondes de surcharge.

### 3. Execution multi-taches

Pour 2 taches ou plus, un coordinateur parallele complet est cree :
- Un worktree par tache
- Meme moniteur TUI que le mode parallele (naviguer, se connecter, rafraichir)
- Meme systeme de detection et de notification des conflits

### 4. Collecte et fusion

Apres l'execution, les resultats sont collectes et les branches sont fusionnees :
- Auto-merge propose pour les fusions sans conflit
- Confirmation manuelle pour les fusions avec conflits
- Politique de fusion configurable

### 5. Verification

La verification post-sweep s'execute sur le resultat fusionne :

| Strategie | Commande | Arret en cas d'echec |
|-----------|----------|:---:|
| `none` | (ignore) | — |
| `tests` | `go test ./...` | Oui |
| `lint` | `golangci-lint run ./...` | Oui |
| `build` | `go build ./...` | Oui |
| `all` | build + tests + lint (dans l'ordre) | Oui |
| `custom` | Commande fournie par l'utilisateur | Oui |

---

## Execution a blanc (Dry Run)

Previsualiser le plan de decomposition sans executer :

```bash
oh start --sweep "Add error handling to all HTTP handlers" \
  --sweep-strategy by-package \
  --sweep-include "cli/internal/mcp/*" \
  --sweep-dry-run
```

Cela affiche la liste des taches, les assignations de fichiers et le perimetre estime par tache.

---

## Exemples

### Par fichier — Mettre a jour les en-tetes de copyright

```bash
oh start --sweep "Update copyright year to 2026 in all Go files" \
  --sweep-strategy by-file \
  --sweep-include "**/*.go" \
  --sweep-verify build
```

### Par package — Ajouter de l'observabilite

```bash
oh start --sweep "Add structured logging to all MCP servers" \
  --sweep-strategy by-package \
  --sweep-include "cli/internal/mcp/*" \
  --sweep-verify tests
```

### Planificateur LLM — Migration complexe

```bash
oh start --sweep "Migrate from log.Printf to slog structured logging" \
  --sweep-strategy llm \
  --sweep-verify all
```

### Manuel — Taches connues

```bash
oh start --sweep "Fix lint warnings" \
  --sweep-strategy manual \
  --sweep-tasks "fix-bodyclose,fix-nilerr,fix-misspell" \
  --sweep-verify lint
```

---

## Isolation du perimetre

Chaque agent de sous-tache recoit un prompt qui impose les limites du perimetre :
- L'agent est informe des fichiers qu'il est autorise a modifier
- Les modifications en dehors du perimetre assigne sont signalees comme des violations
- Cela empeche les changements chevauchants entre les sessions paralleles

---

## Depannage

### Aucune tache generee

```
Sweep decomposition produced 0 tasks
```

- Pour `by-file` : verifiez que le glob `--sweep-include` correspond a des fichiers existants
- Pour `by-package` : verifiez que `go list` peut trouver des packages correspondant au pattern d'inclusion
- Pour `llm` : reformulez l'objectif avec des instructions plus specifiques

### Echec de la verification

```
Sweep verification failed: tests
```

Le resultat fusionne presente des echecs de tests. Corrigez manuellement ou lancez un autre sweep ciblant les tests en echec.

### Conflits de fusion entre les taches

Deux sous-taches ont modifie le meme fichier. La vue de fusion affichera le conflit. Envisagez :
- Reduire la granularite des taches (moins de taches, plus larges)
- Utiliser `--sweep-exclude` pour eviter les chevauchements
- Passer a la strategie `manual` avec des assignations de fichiers explicites

---

## Ressources

- [Guide du mode parallele](parallel-mode.fr.md)
- [Reference CLI](../reference/cli.fr.md)
