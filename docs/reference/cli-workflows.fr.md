> [Read in English](cli-workflows.en.md)

# Référence CLI — Workflows

Les workflows déclaratifs (`apiVersion: oh/v1`) décrivent un cas d'usage : agent d'entrée, agents, checkpoints, entrées, ressources et environnements d'exécution autorisés. Ils sont lus par couches (hub, puis équipe et projet du team-state, voir [Workflows d'équipe](../guides/team-workflows.fr.md)) ; la couche la plus spécifique étend celle du dessous.

## oh run

```
oh run [workflow] [-i clé=valeur]… [--tickets a,b] [--mode <mode>] [--runtime local|container]
                  [--location base|new|<worktree>] [--attach <ouverture>] [--recap] [-p <projet>] [-P <fournisseur>]
```

Lance un workflow : résolution des couches et validation, paquet de session, plan des sessions, rendu du prompt initial, puis démarrage par le RunService (un serveur par groupe : version du paquet, projet, environnement).

- **Entrées** (`-i`, répétable) : valeurs des `inputs` du workflow (`true`, `3`, `a,b` sont convertis selon le type). Les valeurs par défaut peuvent dépendre d'autres entrées (`branch: feat/{{ .ticket }}`). Le texte libre est injecté entre balises `<oh:input name="…">…</oh:input>` et tronqué (`max_length`, sinon 1 000 caractères, 8 000 pour `text`).
- **Tickets** (`--tickets`) : remplissent la première entrée `beads-id` ; avec `picker.multi`, **une session par ticket**, toutes dans le même groupe de serveur, chacune dans son worktree si le workflow écrit. Une entrée `beads-ids` reçoit la liste dans une seule session.
- **Emplacement** (`--location`) : `base` (défaut, dossier du projet), `new` (un nouveau worktree par session, branche = entrée `branch` ou `oh/<workflow>-<ticket>`), ou le chemin d'un worktree existant. Une session qui écrit (`risk` autre que `read`) reçoit **automatiquement un worktree** si une autre session qui écrit est active dans le même dossier. Remplace `--worktree`.
- **Workflow par défaut** : sans argument, `oh run` lance le workflow par défaut du projet (Config projet › Exécution).
- **Exécution** (`--runtime`) : `local` ou `container` (doit figurer dans `runtime.allowed` ; refusé avec la raison si le moteur de conteneurs est indisponible) ; sans `--runtime`, runtime par défaut du projet, puis des Réglages, puis du workflow, s'il est autorisé. Voir [Conteneur](../guides/container.fr.md).
- **Récapitulatif** (`--recap`) : agents, budget du premier tour, isolation, sessions et emplacements, avertissements (modifications non commitées, worktree automatique, décisions en attente), puis confirmation.
- **Une seule session** (`--one-session`) : tous les tickets dans la même session, au lieu d'une session par ticket.
- **MCP** : sans champ `mcp:` dans le workflow, la session reçoit les serveurs MCP du projet ; avec `mcp:` (même vide), seulement ceux listés (un serveur listé mais absent du projet est signalé).
- **Préconditions** : une précondition bloquante refuse le lancement ; une suggestion (ex. pas de wiki → `onboarding`) propose de lancer d'abord le workflow suggéré. Avec `resume: true`, le lancement initial est mémorisé et reproposé à la fin de cette session (« Enchaîner avec… » dans la TUI).
- **Sans interface** (`--headless [--output <fichier>] [--timeout 30m]`) : aucune fenêtre n'est ouverte ; oh attend la fin du tour, écrit la réponse (sortie standard ou fichier, un fichier par session : `<fichier>.<ticket>`) puis arrête la session. Refusé si un checkpoint attend une validation dans le mode choisi. Une session qui demande une décision (permission, question) reste ouverte : `oh session inbox`, `oh session approve`. `oh takeover-brief enrich` passe par `oh run brief-enrich --headless`.
- Deux lancements simultanés dans le même dossier sont refusés (« lancement en cours »).
- **Brouillon** (`--draft`) : lance la version en cours d'édition (votre brouillon, couche équipe ou projet) au lieu de la version publiée. Refusé en exécution distante et si le brouillon **élargit** la version publiée (risque, checkpoints, environnements, Beads, budget…) : publiez-le pour appliquer ces changements. Voir [Workflows d'équipe](../guides/team-workflows.fr.md).
- Exige opencode V2.

## oh workflow list

Liste les workflows du catalogue, un par identifiant (sa couche la plus spécifique), avec version, risque, environnements d'exécution et validité (`✓` valide, `!` avertissements, `✗` erreurs). Avec un team-state : puis **vos brouillons** (`✎`, nombre d'erreurs, « nouvelle brique »), les fichiers ignorés par le contrôle d'intégrité et les publications en attente du réseau (`⏳`). Un fichier d'équipe illisible est listé invalide dans sa couche (équipe ou projet).

| Flag | Type | Description |
|------|------|-------------|
| `-p, --project` | string | Team-state et couche du projet (même contexte que les commandes d'édition) |
| `--team` | string | Équipe ou espace solo, sans projet |
| `--json` | bool | Sortie JSON (tableau de résumés : id, couche, chaîne, risque, entrée, modes, runtimes, entrée Beads, diagnostics ; les brouillons y figurent avec `"draft": true`, et `queued`, `team_bricks`, `new_bricks` le cas échéant) |

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
oh workflow validate <fichier>|<id>|<couche>:<id> [--layer hub|team|project] [--project <projet>] [--json]
oh workflow validate --all [--json]
```

Valide un fichier ou un workflow du catalogue : lecture stricte, résolution des `extends`, règles de sécurité, références au catalogue des briques. Sortie non nulle en cas d'erreur.

Avec `--project <projet>`, les couches équipe et projet de son team-state sont chargées (sinon l'équipe active) ; les fichiers publiés modifiés hors publication sont ignorés avec un avertissement. Les références aux briques sont vérifiées contre le catalogue **fusionné** avec les briques d'équipe (`catalog/`), comme au lancement.

## Édition des workflows d'équipe et de projet

Commandes du cycle brouillon → publication (voir [Workflows d'équipe](../guides/team-workflows.fr.md)). Toutes acceptent `-p, --project <projet>` : team-state et couche du projet ; par défaut, le projet du dossier courant (s'il a une équipe ou un espace solo), sinon l'équipe active (ou, sans équipe, votre unique espace solo) ; `--team <id>` choisit une équipe ou un espace solo sans projet. Un identifiant nu désigne la couche projet si elle contient le workflow, sinon la couche équipe ; `team:<id>` / `project:<id>` la fixent.

### oh workflow new

```
oh workflow new <id> [--layer team|project] [--extends <ref> | --copy <ref>] [--file <fichier>|-] [--no-edit]
```

Crée votre brouillon : squelette vide (valide), patch d'un workflow (`--extends hub:ticket`) ou copie d'un document sous le nouvel identifiant (`--copy hub:review`, gabarit de prompt compris). Ouvre `$VISUAL`/`$EDITOR` (sinon `vi`), puis valide : en cas d'erreur, l'éditeur peut être rouvert, sinon le fichier modifié est conservé.

### oh workflow edit

```
oh workflow edit <id> [--layer team|project] [--prompt] [--file <fichier>] [--prompt-file <fichier>]
```

Modifie votre brouillon (sans brouillon : une copie de la version publiée devient votre brouillon). `--prompt` ouvre le gabarit de prompt propre au brouillon ; `--file` / `--prompt-file` remplacent le contenu sans éditeur.

### oh workflow diff

```
oh workflow diff <id> [--against published|<version>] [--json]
```

Diff du document et du gabarit de prompt entre votre brouillon et la version publiée (ou une version de l'historique), puis **résumé d'impact** (⚠ = élargissement : risque, agents qui écrivent, distant, checkpoints, Beads, budget, MCP…) et briques d'équipe nouvelles.

### oh workflow publish

```
oh workflow publish <id> -m "<message>" [--yes]
oh workflow publish --retry
```

Affiche la version suivante et l'impact, demande confirmation si le workflow est élargi (sauf `--yes` ou sans terminal), puis publie (synchronisation, revalidation, version + 1, historique, `workflows.lock`, commit + push, refait si un autre membre a publié entre-temps). Hors ligne : mise en file d'attente ; `--retry` rejoue les publications en attente (la TUI les rejoue aussi automatiquement à chaque synchronisation du team-state). Réservé aux membres de l'équipe (`[governance] publish`).

### oh workflow history

```
oh workflow history <id> [--json]
```

Versions publiées, de la plus récente à la plus ancienne : version, date, auteur, message.

### oh workflow restore

```
oh workflow restore <id> <version> [--yes]
```

Republie le contenu d'une version (document et gabarit) comme **nouvelle** version, après revalidation.

### oh workflow archive

```
oh workflow archive <id> [-m "<raison>"] [--yes]
```

Retire le workflow publié ; sa dernière version reste dans l'historique (restaurable).

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
