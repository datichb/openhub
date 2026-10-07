> [Read in English](cli-workflows.en.md)

# Référence CLI — Workflows

Les workflows déclaratifs (`apiVersion: oh/v1`) décrivent un cas d'usage : agent d'entrée, agents, checkpoints, entrées, ressources et environnements d'exécution autorisés. Ils sont lus par couches (hub, puis équipe et projet du team-state, voir [Workflows d'équipe](../guides/team-workflows.fr.md)) ; la couche la plus spécifique étend celle du dessous.

## oh run

```
oh run [workflow] [-i clé=valeur]… [--tickets a,b] [--one-session] [--mode <mode>] [--runtime local|container|remote]
                  [--location base|new|<worktree>] [--attach <ouverture>] [--recap] [--draft] [-a <agent>]
                  [--headless [--output <fichier>] [--timeout <durée>]] [--parent <session>] [-p <projet>] [-P <fournisseur>]
```

| Flag | Court | Type | Défaut | Description |
|------|-------|------|--------|-------------|
| `--input` | `-i` | string (répétable) | | Entrée du workflow `clé=valeur` |
| `--tickets` | | liste (virgules) | | Tickets Beads (une session par ticket si le workflow le permet) |
| `--one-session` | | bool | `false` | Tous les tickets dans une seule session |
| `--mode` | | string | mode par défaut du workflow | `manuel`, `semi-auto` ou `auto` |
| `--runtime` | | string | voir ci-dessous | `local`, `container` ou `remote` |
| `--location` | | string | `base` | `base`, `new` (nouveau worktree) ou chemin d'un worktree existant |
| `--attach` | | string | préférence des Réglages | Ouverture : `auto`, `iterm`, `terminal`, `tmux`, `browser`, `suspend`, `none` |
| `--recap` | | bool | `false` | Récapitulatif et confirmation avant le lancement |
| `--draft` | | bool | `false` | Lancer votre brouillon du workflow (local uniquement) |
| `--agent` | `-a` | string | | Agent d'entrée (workflows à entrée au choix, ex. `libre`) |
| `--headless` | | bool | `false` | Sans interface : attendre la fin du tour, afficher la réponse, arrêter la session |
| `--output` | | string | sortie standard | Avec `--headless` : fichier de la réponse |
| `--timeout` | | durée | `30m` | Avec `--headless` : attente maximale (`0` = aucune) |
| `--parent` | | string | | Session précédente (enchaînement) |
| `--project` | `-p` | string | projet du dossier courant | Projet |
| `--provider` | `-P` | string | fournisseur du projet | Fournisseur LLM |

Lance un workflow : résolution des couches et validation, paquet de session, plan des sessions, rendu du prompt initial, puis démarrage par le RunService (un serveur par groupe : version du paquet, projet, environnement).

- **Entrées** (`-i`, répétable) : valeurs des `inputs` du workflow (`true`, `3`, `a,b` sont convertis selon le type). Les valeurs par défaut peuvent dépendre d'autres entrées (`branch: feat/{{ .ticket }}`). Dans le gabarit de prompt, les entrées texte sont encadrées par la fonction `data` (balises `<oh:data name="…">…</oh:data>`) et tronquées (`max_length`, sinon 20 000 caractères) ; voir [Schéma des workflows](workflow-schema.fr.md).
- **Tickets** (`--tickets`) : remplissent la première entrée `beads-id` ; avec `picker.multi`, **une session par ticket**, toutes dans le même groupe de serveur, chacune dans son worktree si le workflow écrit. Une entrée `beads-ids` reçoit la liste dans une seule session.
- **Emplacement** (`--location`) : `base` (défaut, dossier du projet), `new` (un nouveau worktree par session, branche = entrée `branch` ou `oh/<workflow>-<ticket>`), ou le chemin d'un worktree existant. Une session qui écrit (`risk` autre que `read`) reçoit **automatiquement un worktree** si une autre session qui écrit est active dans le même dossier. Remplace `--worktree`.
- **Workflow par défaut** : sans argument, `oh run` lance le workflow par défaut du projet (Config projet › Exécution).
- **Exécution** (`--runtime`) : `local`, `container` ou `remote` (doit figurer dans `runtime.allowed` ; refusé avec la raison si le moteur de conteneurs est indisponible ou si la cible distante n'est pas configurée) ; sans `--runtime`, runtime par défaut du projet, puis des Réglages, puis du workflow, s'il est autorisé. Voir [Conteneur](../guides/container.fr.md) et [Exécution distante](../guides/remote-runners.fr.md) (`oh remote setup`, puis `oh session fetch` / `oh session resolve` au retour).
- **Récapitulatif** (`--recap`) : agents, budget du premier tour, isolation, sessions et emplacements, avertissements (modifications non commitées, worktree automatique, décisions en attente), puis confirmation.
- **Une seule session** (`--one-session`) : tous les tickets dans la même session, au lieu d'une session par ticket.
- **MCP** : sans champ `mcp:` dans le workflow, la session reçoit les serveurs MCP du projet ; avec `mcp:` (même vide), seulement ceux listés (un serveur listé mais absent du projet est signalé).
- **Préconditions** : une précondition bloquante refuse le lancement ; une suggestion (ex. pas de wiki → `onboarding`) propose de lancer d'abord le workflow suggéré. Avec `resume: true`, le lancement initial est mémorisé et reproposé à la fin de cette session (« Enchaîner avec… » dans la TUI).
- **Sans interface** (`--headless [--output <fichier>] [--timeout 30m]`) : aucune fenêtre n'est ouverte ; oh attend la fin du tour, écrit la réponse (sortie standard ou fichier, un fichier par session : `<fichier>.<ticket>`) puis arrête la session. Refusé si un checkpoint attend une validation dans le mode choisi. Une session qui demande une décision (permission, question) reste ouverte : `oh session inbox`, `oh session approve`. Au-delà de `--timeout`, la session est arrêtée et oh sort en erreur (code 1). `oh takeover-brief enrich` passe par `oh run brief-enrich --headless`.
- **Enchaînement** (`--parent <session>`) : rattache la nouvelle session à la précédente (« Enchaîner avec… » dans la TUI).
- Deux lancements simultanés dans le même dossier sont refusés (« lancement en cours »).
- **Brouillon** (`--draft`) : lance la version en cours d'édition (votre brouillon, couche équipe ou projet) au lieu de la version publiée. Refusé en exécution distante et si le brouillon **élargit** la version publiée (risque, checkpoints, environnements, Beads, budget…) : publiez-le pour appliquer ces changements. Voir [Workflows d'équipe](../guides/team-workflows.fr.md).
- **Agent d'entrée** (`--agent`, `-a`) : pour un workflow à entrée au choix (`libre`), l'agent de départ ; les membres sont cet agent et ceux qu'il peut appeler. Refusé sur les autres workflows.
- Exige opencode V2 (version minimale dans `oh doctor`).

**Exemples :**

```bash
oh run                                              # workflow par défaut du projet
oh run feature -i request="Ajoute un endpoint /health"
oh run ticket --tickets bd-42,bd-43                 # une session par ticket
oh run ticket --tickets bd-42,bd-43 --one-session
oh run libre --agent debugger -i request="Le test X échoue"
oh run audit -i type=security --runtime container
oh run review --headless --output review.md --timeout 20m
oh run ticket --tickets bd-42 --runtime remote
```

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

| Flag | Type | Défaut | Description |
|------|------|--------|-------------|
| `--all` | bool | `false` | Valide tous les workflows du hub |
| `--layer` | string | `hub` | Couche du fichier validé : `hub`, `team` ou `project` |
| `--project` | string | | Projet (id ou nom) dont la couche est chargée avec sa couche équipe (pas de forme courte) |
| `--team` | string | | Équipe ou espace solo, sans projet |
| `--json` | bool | `false` | Sortie JSON |

Avec `--project <projet>`, les couches équipe et projet de son team-state sont chargées (sinon l'équipe active) ; les fichiers publiés modifiés hors publication sont ignorés avec un avertissement. Les références aux briques sont vérifiées contre le catalogue **fusionné** avec les briques d'équipe (`catalog/`), comme au lancement.

## Édition des workflows d'équipe et de projet

Commandes du cycle brouillon → publication (voir [Workflows d'équipe](../guides/team-workflows.fr.md)). Toutes acceptent `-p, --project <projet>` : team-state et couche du projet ; par défaut, le projet du dossier courant (s'il a une équipe ou un espace solo), sinon l'équipe active (ou, sans équipe, votre unique espace solo) ; `--team <id>` choisit une équipe ou un espace solo sans projet. Un identifiant nu désigne la couche projet si elle contient le workflow, sinon la couche équipe ; `team:<id>` / `project:<id>` la fixent. Les flags `-p, --project` et `--team` ne sont pas répétés dans les tableaux ci-dessous.

### oh workflow new

```
oh workflow new <id> [--layer team|project] [--extends <ref> | --copy <ref>] [--file <fichier>|-] [--no-edit]
```

Crée votre brouillon : squelette vide (valide), patch d'un workflow (`--extends hub:ticket`) ou copie d'un document sous le nouvel identifiant (`--copy hub:review`, gabarit de prompt compris). Ouvre `$VISUAL`/`$EDITOR` (sinon `vi`), puis valide : en cas d'erreur, l'éditeur peut être rouvert, sinon le fichier modifié est conservé.

| Flag | Type | Défaut | Description |
|------|------|--------|-------------|
| `--layer` | string | `team` | Couche du brouillon : `team` ou `project` |
| `--extends` | string | | Workflow à étendre (ex. `hub:ticket`) : le brouillon est un patch |
| `--copy` | string | | Document à dupliquer (ex. `hub:review`) sous le nouvel identifiant |
| `--file` | string | | Lire le document depuis ce fichier (`-` : entrée standard) au lieu d'ouvrir l'éditeur |
| `--no-edit` | bool | `false` | Enregistrer le brouillon initial sans ouvrir l'éditeur |

### oh workflow edit

```
oh workflow edit <id> [--layer team|project] [--prompt] [--file <fichier>] [--prompt-file <fichier>]
```

Modifie votre brouillon (sans brouillon : une copie de la version publiée devient votre brouillon). `--prompt` ouvre le gabarit de prompt propre au brouillon ; `--file` / `--prompt-file` remplacent le contenu sans éditeur.

| Flag | Type | Défaut | Description |
|------|------|--------|-------------|
| `--layer` | string | projet s'il contient le workflow, sinon équipe | `team` ou `project` |
| `--prompt` | bool | `false` | Éditer le gabarit de prompt au lieu du document |
| `--file` | string | | Remplacer le document par ce fichier (`-` : entrée standard) |
| `--prompt-file` | string | | Remplacer le gabarit de prompt du brouillon par ce fichier |

### oh workflow diff

```
oh workflow diff <id> [--against published|<version>] [--json]
```

Diff du document et du gabarit de prompt entre votre brouillon et la version publiée (ou une version de l'historique), puis **résumé d'impact** (⚠ = élargissement : risque, agents qui écrivent, distant, checkpoints, Beads, budget, MCP…) et briques d'équipe nouvelles.

| Flag | Type | Défaut | Description |
|------|------|--------|-------------|
| `--against` | string | `published` | `published` ou un numéro de version |
| `--layer` | string | projet s'il contient le workflow, sinon équipe | `team` ou `project` |
| `--json` | bool | `false` | Sortie JSON (diff, impact, version suivante) |

### oh workflow publish

```
oh workflow publish <id> -m "<message>" [--yes]
oh workflow publish --retry
```

Affiche la version suivante et l'impact, demande confirmation si le workflow est élargi (sauf `--yes` ou sans terminal), puis publie (synchronisation, revalidation, version + 1, historique, `workflows.lock`, commit + push, refait si un autre membre a publié entre-temps). Hors ligne : mise en file d'attente ; `--retry` rejoue les publications en attente (la TUI les rejoue aussi automatiquement à chaque synchronisation du team-state). Réservé aux membres de l'équipe (`[governance] publish`).

| Flag | Court | Type | Description |
|------|-------|------|-------------|
| `--message` | `-m` | string | Message de publication (obligatoire) |
| `--retry` | | bool | Rejouer les publications en attente (hors ligne) |
| `--yes` | `-y` | bool | Ne pas demander de confirmation |

### oh workflow history

```
oh workflow history <id> [--json]
```

Versions publiées, de la plus récente à la plus ancienne : version, date, auteur, message.

| Flag | Type | Description |
|------|------|-------------|
| `--json` | bool | Sortie JSON |

### oh workflow restore

```
oh workflow restore <id> <version> [--yes]
```

Republie le contenu d'une version (document et gabarit) comme **nouvelle** version, après revalidation.

| Flag | Court | Type | Description |
|------|-------|------|-------------|
| `--yes` | `-y` | bool | Ne pas demander de confirmation |

### oh workflow archive

```
oh workflow archive <id> [-m "<raison>"] [--yes]
```

Retire le workflow publié ; sa dernière version reste dans l'historique (restaurable).

| Flag | Court | Type | Description |
|------|-------|------|-------------|
| `--message` | `-m` | string | Raison de l'archivage |
| `--yes` | `-y` | bool | Ne pas demander de confirmation |

## oh bundle build / show

```
oh bundle build <workflow> [-p <projet>] [-P <fournisseur>] [--json]
oh bundle show <workflow>|<hash> [-p <projet>] [-P <fournisseur>] [--budget] [--json]
```

Compile (de façon idempotente, par hash) le paquet de session d'un workflow dans `~/.oh/bundles/<hash>/` et l'affiche : agents (l'agent d'entrée en premier), délégations, skills à la demande, MCP, plugins, modèle par défaut, profondeur de délégation, isolation (`strict` si le workflow l'exige), permissions globales et coût estimé du premier tour (agent d'entrée + catalogue des skills). Sans `-p`, le projet est celui du dossier courant (sinon paquet du hub seul, sans instructions ni MCP de projet).

| Flag | Court | Type | Description |
|------|-------|------|-------------|
| `--project` | `-p` | string | Projet (instructions, modèles, MCP) ; défaut : détecté depuis le dossier courant |
| `--provider` | `-P` | string | Fournisseur LLM (normalisation des modèles) |
| `--budget` | | bool | `show` seulement : détaille le budget estimé, tokens par agent et par skill (≈ 4 caractères par token) |
| `--json` | | bool | Sortie JSON (`bundle.Report`) |

`oh bundle show` accepte un nom de workflow ou le hash d'un paquet déjà construit.

```bash
oh bundle build ticket -p mon-app
oh bundle show ticket --budget
oh bundle show 3f9c2a… --json
```

`oh skill budget <workflow>` est un alias déprécié de `oh bundle show <workflow> --budget` ; sans workflow, l'ancien calcul par agent reste disponible avec un avertissement.

## Variables d'environnement

| Variable | Effet |
|----------|-------|
| `OH_WORKFLOWS_DIR` | Remplace `~/.oh/hub/workflows` comme couche hub (workflows en développement, tests) |
