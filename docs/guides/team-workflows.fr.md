# Workflows d'équipe (team-state)

> v5, phase 2. Les workflows déclaratifs (`apiVersion: oh/v1`) d'une équipe et de ses projets sont rangés dans le dépôt team-state. Ce guide décrit leur emplacement, le contrôle d'intégrité et les verrous.

## Couches

Un workflow est résolu de la couche la plus générale à la plus spécifique :

| Couche | Emplacement |
|---|---|
| `hub` | workflows livrés avec oh (`~/.oh/hub/workflows/`) |
| `team` | `team-state/workflows/published/<id>.yaml` |
| `project` | `team-state/projects/<projet>/workflows/published/<id>.yaml` |

Un workflow d'une couche qui porte le même identifiant qu'un workflow d'une couche inférieure **doit l'étendre** (`extends`), sinon il est refusé. Les champs de sécurité ne peuvent que se durcir.

## Arborescence

```
team-state/
├── workflows/
│   ├── published/<id>.yaml            # versions publiées (chargées)
│   ├── drafts/<membre>/<id>.yaml      # brouillons (jamais chargés pour les autres)
│   ├── prompts/<id>.md.tmpl           # gabarits de prompt
│   └── history/<id>/<version>.yaml    # versions précédentes (+ <version>.prompt.md.tmpl)
├── projects/<projet>/workflows/…      # même structure pour un projet
├── catalog/{agents,skills}/           # briques d'équipe
└── workflows.lock                     # intégrité des fichiers publiés
```

Le chemin d'un gabarit (`prompt.template: prompts/<id>.md.tmpl`) est relatif au dossier `workflows/` de sa portée, quel que soit le sous-dossier du document (publié, brouillon, historique). Il ne peut pas en sortir.

## `workflows.lock` et intégrité

Chaque publication enregistre la version et l'empreinte du fichier publié (et de son gabarit de prompt) :

```toml
[team.ticket-hotfix]
version = 2
hash = "sha256:…"
prompt_hash = "sha256:…"
published_by = "alice"
published_at = 2026-10-06T10:00:00Z
message = "Checkpoint de revue obligatoire"

[projects.web.ticket]
…
```

Au chargement, oh **ignore avec un avertissement** :

- un fichier publié absent de `workflows.lock` (jamais publié depuis oh) ;
- un fichier ou un gabarit modifié à la main (empreinte différente) ;
- une entrée du lock dont le fichier a disparu ;
- tous les workflows de l'équipe si `workflows.lock` est illisible.

Ces avertissements apparaissent dans `oh workflow list`, `oh workflow validate`, le catalogue de la TUI et `oh doctor` (« Workflows d'équipe »). Un fichier publié qui ne se lit pas (erreur de syntaxe) est listé comme invalide **dans sa couche** (équipe ou projet). Pour corriger : republier le workflow depuis oh ou restaurer le fichier (`git checkout`).

## Vérifier les workflows d'équipe

```bash
oh workflow validate team:ticket-hotfix            # équipe active
oh workflow validate project:ticket --project web  # couche du projet + son équipe
oh workflow validate --all --project web           # hub, équipe et projet
oh workflow validate ./hotfix.yaml --layer team    # fichier, avec les briques d'équipe
```

La validation utilise le **catalogue de briques fusionné** (hub + `catalog/` de l'équipe), comme `oh run` et la publication : un workflow qui utilise un agent d'équipe est valide.

`oh workflow list [-p <projet>|--team <id>]` affiche aussi **vos brouillons** (✎, nombre d'erreurs, « nouvelle brique »), les fichiers ignorés par le contrôle d'intégrité et les publications en attente du réseau (⏳). En JSON, les brouillons sont des entrées du même tableau avec `"draft": true`.

## Verrous (`enforce`)

Un workflow peut verrouiller des champs pour les couches plus spécifiques :

```yaml
apiVersion: oh/v1
kind: Workflow
id: ticket
extends: hub:ticket
enforce: [checkpoints, modes]   # ou ["*"] pour tout le document
```

- Valeurs possibles : les champs de premier niveau du document (`checkpoints`, `modes`, `agents`, `models`, `runtime`…) ou `"*"`.
- Un document qui étend un workflow verrouillé et écrit un champ verrouillé est **refusé** (`enforced_field`). Avec `"*"`, seuls `id`, `version`, `extends` et `enforce` restent permis.
- Les verrous s'additionnent le long de la chaîne `extends` ; une couche plus spécifique ne peut pas les retirer.
- Les options de lancement (mode, environnement, entrées) restent choisies dans les limites du workflow.

## Brouillons, publication, historique

Le cycle de vie d'un workflow d'équipe ou de projet passe par le WorkflowService (CLI : `oh workflow new|edit|diff|publish|history|restore|archive`, voir la [référence](../reference/cli-workflows.fr.md#édition-des-workflows-déquipe-et-de-projet)) :

1. **Brouillon** : `workflows/drafts/<membre>/<id>.yaml` (et son propre gabarit `<id>.prompt.md.tmpl` s'il en a un). Il est **validé à l'enregistrement** (refusé avec ses erreurs), poussé avec le team-state, mais n'est jamais chargé pour les autres membres.
2. **Test** : `oh run <id> --draft` lance le brouillon, en local seulement, et le refuse s'il élargit la version publiée (un brouillon ne peut pas assouplir la sécurité).
3. **Publication** : pull du team-state → **revalidation** du brouillon contre l'état à jour → version + 1 → version précédente copiée dans `history/<id>/` (document, gabarit et entrée du lock) → fichier publié et `workflows.lock` → commit + push. Si un autre membre a publié entre-temps, tout est **refait** au-dessus de sa publication (nouvelle revalidation, version suivante). Le brouillon est consommé ; un événement `workflow.published` est ajouté.
4. **Hors ligne** : la publication est mise en **file d'attente** (dans le clone, non versionnée) et rejouée plus tard avec le même cycle ; le brouillon est conservé. Le rejeu est **automatique à chaque synchronisation du team-state dans la TUI** (notification du résultat) ; en CLI : `oh workflow publish --retry` (pas de rejeu au démarrage de la CLI).

### Résumé d'impact

Chaque publication compare la nouvelle version à la précédente : risque relevé, nouveaux agents (et ceux qui écrivent), exécution distante ou nouveaux environnements autorisés, checkpoints retirés, rendus facultatifs ou assouplis, commandes Beads ajoutées, budget relevé, nouveaux MCP ou plugins, Code Mode, entrées et prompt modifiés. Les changements qui **élargissent** le workflow sont signalés. Les briques du catalogue d'équipe utilisées pour la première fois portent le badge « nouvelle brique ».

### Historique, restauration, archivage

- L'historique liste la version publiée puis les précédentes (auteur, date, message).
- **Restaurer** une version la republie comme **nouvelle** version (événement `workflow.restored`).
- **Archiver** retire le workflow publié (déplacé dans l'historique, entrée du lock supprimée, événement `workflow.archived`) ; il peut être restauré.
- Les workflows publiés qui étendent celui-ci et deviendraient invalides sont signalés à la publication.

Les événements des workflows d'équipe sont rangés dans `projects/_team/events/`, ceux d'un projet dans `projects/<projet>/events/`.

## Catalogue de briques d'équipe

Une équipe peut fournir ses propres agents et skills, au même format que ceux du hub :

```
team-state/catalog/
├── agents/<famille>/<id>.md
└── skills/<chemin>.md          # + skills/templates/… (annexes)
```

- Les workflows de l'équipe et des projets les utilisent comme les briques du hub (validation et paquet de session).
- Un identifiant déjà utilisé par le hub (id d'agent, chemin ou nom de skill) est **refusé**, sauf si la brique déclare explicitement `extends: hub:<id>` (agent) ou `extends: hub:<chemin>` (skill) dans son frontmatter : elle remplace alors la brique du hub. Une brique refusée est ignorée avec un avertissement (`oh doctor`).
- Une annexe ne peut pas remplacer un fichier du hub.

## Gouvernance

La publication se règle dans `config.toml` du team-state :

```toml
[governance]
publish = "any_member"   # valeur par défaut, seule prise en charge
```

- `any_member` : tout membre inscrit dans `members.toml` peut publier ; la validation du workflow reste obligatoire.
- Une valeur inconnue de cette version d'oh **bloque la publication** (sans gêner le reste de la configuration) : mettez oh à jour ou revenez à `any_member`.

## Projet sans équipe : espace solo

Un projet sans équipe range ses workflows dans un **espace solo** : un team-state local, sans remote, dans `~/.oh/teams/<id>/`.

```bash
oh team init --solo --project web-app     # crée l'espace et rattache le projet
oh team promote --remote <url-vide>       # plus tard : le partager avec une équipe
```

Les publications y sont des commits locaux. `oh team promote` pousse tout l'historique vers le remote, sans rien perdre (voir [CLI équipe](../reference/cli-team.fr.md#oh-team-promote)).

Dans la TUI :

- **premier lancement** : bouton « Espace solo (workflows locaux) » à l'étape Équipe (le mode « solo » de l'accueil l'active d'office) ; le projet de l'assistant y est rattaché ;
- **ajout d'un projet** : choix « Créer un espace solo » / « Espace solo <id> » à l'étape Équipe (l'espace existant est réutilisé) ;
- **catalogue des workflows** : `n` sans team-state propose de créer l'espace solo (identifiant, membre, rattachement du projet actif) ;
- **détail d'équipe** d'un espace solo : ligne « Espace » et action **« Passer en équipe »** (URL d'un dépôt distant vide, confirmation, puis l'URL à transmettre aux membres, qui lancent `oh team init`). Le détail d'équipe affiche aussi la **gouvernance** des workflows en lecture (« Publication : tout membre »).

## Migration des anciennes surcharges de workflow (v5)

Les surcharges de l'ancien workflow unique (onglet Workflow de la TUI) sont migrées **automatiquement** au premier lancement d'oh v5 (migration v38), vers des workflows qui étendent `hub:feature` (mêmes identifiants de checkpoints et d'agents) :

| Ancienne configuration | Devient |
|---|---|
| `config.toml` du team-state, section `[workflow]` | brouillon puis workflow d'équipe `feature` (`extends: hub:feature` ; `enforced = true` → `enforce: ["*"]`), publié si l'équipe n'en a pas encore ; section retirée de `config.toml` |
| configuration de workflow d'un projet (base oh) | brouillon puis workflow de projet `feature` (étend `team:feature` si l'équipe en a un, sinon `hub:feature`) ; un **espace solo** est créé pour un projet sans équipe |
| `hub.toml`, section `[workflow.overrides]` (développement du hub) | fichiers `~/.oh/migrated/` (non chargés) ; section retirée de `hub.toml` |

- **Rien n'est perdu** : la configuration d'origine est archivée (`workflows/migrated/…` du team-state, `~/.oh/migrated/`) ; ce qui n'a pas d'équivalent (`cp-routing`, agents d'un checkpoint, `insert_after`, `task_permissions`) est listé en commentaire en tête du workflow migré.
- Un workflow migré invalide, ou une portée qui a déjà un `feature` publié, reste **votre brouillon** : `oh workflow diff feature`, `oh workflow edit feature`, puis `oh workflow publish`.
- Si l'équipe verrouillait son workflow, les surcharges de projet (ignorées auparavant) restent un brouillon : la publication est refusée par le verrou.
- Hors ligne, la migration d'une équipe est simplement retentée au lancement suivant.
- L'ancienne vue Workflow de la TUI et `oh deploy` ont été supprimés en v5 : les workflows se consultent dans le catalogue (`workflows`) et s'éditent dans l'éditeur.
