> [Read in English](cli-workflows.en.md)

# Référence CLI — Workflows

Les workflows déclaratifs (`apiVersion: oh/v1`) décrivent un cas d'usage : agent d'entrée, agents, checkpoints, entrées, ressources et environnements d'exécution autorisés. Ils sont lus par couches (hub, puis équipe et projet en phase 2) ; la couche la plus spécifique étend celle du dessous.

## oh run

```
oh run <workflow> [-i clé=valeur]… [--tickets a,b] [--mode <mode>] [--runtime local|container]
                  [--location base|new|<worktree>] [--attach <ouverture>] [--recap] [-p <projet>] [-P <fournisseur>]
```

Lance un workflow : résolution des couches et validation, paquet de session, plan des sessions, rendu du prompt initial, puis démarrage par le RunService (un serveur par groupe : version du paquet, projet, environnement).

- **Entrées** (`-i`, répétable) : valeurs des `inputs` du workflow (`true`, `3`, `a,b` sont convertis selon le type). Les valeurs par défaut peuvent dépendre d'autres entrées (`branch: feat/{{ .ticket }}`). Le texte libre est injecté entre balises `<oh:input name="…">…</oh:input>` et tronqué (`max_length`, sinon 1 000 caractères, 8 000 pour `text`).
- **Tickets** (`--tickets`) : remplissent la première entrée `beads-id` ; avec `picker.multi`, **une session par ticket**, toutes dans le même groupe de serveur, chacune dans son worktree si le workflow écrit. Une entrée `beads-ids` reçoit la liste dans une seule session.
- **Emplacement** (`--location`) : `base` (défaut, dossier du projet), `new` (un nouveau worktree par session, branche = entrée `branch` ou `oh/<workflow>-<ticket>`), ou le chemin d'un worktree existant. Une session qui écrit (`risk` autre que `read`) reçoit **automatiquement un worktree** si une autre session qui écrit est active dans le même dossier. Remplace `--worktree`.
- **Exécution** (`--runtime`) : `local` ou `container` (doit figurer dans `runtime.allowed` ; refusé avec la raison si le moteur de conteneurs est indisponible).
- **Récapitulatif** (`--recap`) : agents, budget du premier tour, isolation, sessions et emplacements, avertissements (modifications non commitées, worktree automatique, décisions en attente), puis confirmation.
- **Une seule session** (`--one-session`) : tous les tickets dans la même session, au lieu d'une session par ticket.
- **MCP** : sans champ `mcp:` dans le workflow, la session reçoit les serveurs MCP du projet ; avec `mcp:` (même vide), seulement ceux listés (un serveur listé mais absent du projet est signalé).
- **Préconditions** : une précondition bloquante refuse le lancement ; une suggestion (ex. pas de wiki → `onboarding`) propose de lancer d'abord le workflow suggéré. Avec `resume: true`, le lancement initial est mémorisé et reproposé à la fin de cette session (« Enchaîner avec… » dans la TUI).
- Deux lancements simultanés dans le même dossier sont refusés (« lancement en cours »).
- `--draft` (brouillons) arrive en phase 2. Exige opencode V2.

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
