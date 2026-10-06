> [Read in English](cli-workflows.en.md)

# Référence CLI — Workflows

Les workflows déclaratifs (`apiVersion: oh/v1`) décrivent un cas d'usage : agent d'entrée, agents, checkpoints, entrées, ressources et environnements d'exécution autorisés. Ils sont lus par couches (hub, puis équipe et projet en phase 2) ; la couche la plus spécifique étend celle du dessous.

## oh workflow list

Liste les workflows du catalogue, un par identifiant (sa couche la plus spécifique), avec version, risque, environnements d'exécution et validité (`✓` valide, `!` avertissements, `✗` erreurs).

| Flag | Type | Description |
|------|------|-------------|
| `--json` | bool | Sortie JSON (tableau de résumés : id, couche, chaîne, risque, entrée, modes, runtimes, entrée Beads, diagnostics) |

## oh workflow show

```
oh workflow show <id>|<couche>:<id> [--origin] [--json]
```

Affiche un workflow après résolution des `extends` : en-tête (chaîne, version, risque, isolation, agent d'entrée, modes, exécution, Code Mode), entrées, agents, checkpoints (comportement par mode), ressources (skills, MCP, Beads, plugins, sorties, modèles) et diagnostics. La commande échoue si le workflow est invalide.

| Flag | Type | Description |
|------|------|-------------|
| `--origin` | bool | Affiche la couche qui a posé chaque valeur (`défaut` quand la valeur n'est écrite nulle part) |
| `--json` | bool | Sortie JSON (`spec` résolue, `origins` par chemin de champ, `diagnostics`) |

## oh workflow validate

```
oh workflow validate <fichier>|<id>|<couche>:<id> [--layer hub|team|project] [--json]
oh workflow validate --all [--json]
```

Valide un fichier ou un workflow du catalogue : lecture stricte, résolution des `extends`, règles de sécurité, références au catalogue des briques. Sortie non nulle en cas d'erreur.

## oh bundle build / show

```
oh bundle build <workflow> [-p <projet>] [-P <fournisseur>] [--json]
oh bundle show <workflow>|<hash> [-p <projet>] [--budget] [--json]
```

Compile (de façon idempotente, par hash) le paquet de session d'un workflow dans `~/.oh/bundles/<hash>/` et l'affiche : agents (l'agent d'entrée en premier), délégations, skills à la demande, MCP, plugins, modèle par défaut, profondeur de délégation, isolation (`strict` si le workflow l'exige), permissions globales et coût estimé du premier tour (agent d'entrée + catalogue des skills). Sans `-p`, le projet est celui du dossier courant (sinon paquet du hub seul, sans instructions ni MCP de projet).

| Flag | Type | Description |
|------|------|-------------|
| `--budget` | bool | Détaille le budget estimé : tokens par agent et par skill (≈ 4 caractères par token) |
| `--json` | bool | Sortie JSON (`bundle.Report`) |

`oh skill budget <workflow>` est un alias déprécié de `oh bundle show <workflow> --budget` ; sans workflow, l'ancien calcul par agent reste disponible avec un avertissement.

## Variables d'environnement

| Variable | Effet |
|----------|-------|
| `OH_WORKFLOWS_DIR` | Remplace `~/.oh/hub/workflows` comme couche hub (workflows en développement, tests) |
