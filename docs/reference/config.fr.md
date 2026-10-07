> 🇬🇧 [Read in English](config.en.md)

# Référence de configuration

Fichiers de configuration d'oh v5 : `hub.toml` (votre machine), la base des projets (`oh.db`), le `config.toml` du team-state (équipe) et les workflows (`oh/v1`, voir [schéma](workflow-schema.fr.md)).

---

## Fichier de configuration du hub

**Emplacement :** `~/.oh/hub.toml` (ou `$OH_HOME/hub.toml`)  
**Créé par :** `oh init`  
**Afficher le chemin :** `oh config path`

### Structure TOML complète

```toml
name = "OpenHub"                   # nom affiché dans le titre de la TUI

[cli]
language = "fr"                    # "fr" ou "en" (défaut : "en")

[opencode]
default_provider = "bedrock"       # bedrock | anthropic | openrouter | github-copilot

[provider.bedrock]
aws_profile = "default"            # profil AWS (bedrock uniquement)
aws_region = "eu-west-1"           # région AWS (bedrock uniquement)
auth_mode = "bearer"               # "bearer" | "profile" | "env" (bedrock uniquement)

[provider.anthropic]
# Utilise la clé API stockée dans le trousseau (pas de champ)

[provider.openrouter]
# Utilise la clé API stockée dans le trousseau (pas de champ)

[models]
default = "claude-sonnet-4-20250514"  # modèle par défaut du hub

[models.families]
quality = "claude-opus-4-20250514"    # par famille d'agents

[models.agents]
reviewer = "claude-opus-4-20250514"   # par agent

[[teams]]
id = "acme"                        # identifiant court local
name = "Équipe ACME"               # nom affiché
enabled = true
state_repo = "git@gitlab.com:acme/team-state.git"
member_id = "alice"

[mcp.figma]
enabled = true                     # activer le serveur MCP Figma
token_key = "openhub.mcp.figma.token"   # nom de la clé dans le trousseau (PAS le token)

[mcp.gitlab]
enabled = true
token_key = "openhub.mcp.gitlab.token"
write_enabled = true
url = ""                           # vide = URL du team-state ou défaut

[mcp.jira]
enabled = false

[mcp.gslides]
enabled = false
token_key = "openhub.mcp.gslides.token"

[worktree]
auto_cleanup = true                # supprimer les worktrees mergés au démarrage
base_branch = ""                   # vide = détection auto (main/master)
branch_pattern = "oh/%s"           # nommage des branches (%s = nom du worktree)

[deploy]
instruction_files = []             # fichiers du projet ajoutés aux instructions des agents (en plus d'ONBOARDING.md, CONVENTIONS.md, .claude/CLAUDE.md)

[tracker]                          # surcharges locales de la sync tracker de l'équipe
# enabled = false                  # décommenter pour désactiver la sync localement
# push_labels = false              # décommenter pour désactiver le push de labels

[websearch]
enabled = false                    # WebSearch / WebFetch (Exa AI) pour les agents

[session]
attach = "auto"                    # auto | iterm | terminal | tmux | browser | suspend
iterm_style = "tab"                # tab | split | window (iTerm2)
idle_sleep_minutes = 5             # mise en veille d'un serveur inactif
notify = "on"                      # notifications système : on | off

[execution]
runtime = ""                       # local | container ; vide = défaut du workflow
engine = "auto"                    # auto | colima | podman | docker
keep_images = 2                    # images conservées par projet et par rôle
opencode_version = ""              # version d'opencode des images ; vide = celle de la machine
strict_isolation = false           # masquer aussi la config personnelle d'opencode en local

[remote]
[[remote.targets]]                 # projets oh-runner (un par instance GitLab et groupe)
name = "acme"
url = "https://gitlab.com"
group = "acme"
# runner_project = "acme/oh-runner"   # défaut : <group>/oh-runner
# tag = "oh"  builder = "kaniko"  arch = "amd64"  timeout = "3h"
[remote.projects]                  # projet oh → cible (sinon choix automatique)
# web-app = "acme"

[limits]                           # restrictions I6 du hub (désactivées par défaut)
# max_active_sessions = 4
# session_budget_usd = 5
# daily_budget_usd = 30
# memory_mb = 4096
# models = ["eu.anthropic.claude-*"]
```

Supprimés en v5 (ignorés s'ils restent dans le fichier) : `[opencode] version`, `channel`, `auto_update`, `install_dir` (opencode V2 s'installe à part, voir [migration v5](../guides/migration-v5.fr.md)), `[deploy] disable_native_agents` (le monde fermé désactive toujours les agents natifs), `[workflow.overrides]` (migré dans `~/.oh/migrated/`, voir [workflows d'équipe](../guides/team-workflows.fr.md#migration-des-anciennes-surcharges-de-workflow-v5)). L'ancienne section `[team]` est migrée en `[[teams]]` au chargement.

### Sections v5

| Section | Clé | Défaut | Rôle |
|---|---|---|---|
| `[session]` | `attach` | `auto` | Ouverture d'une session (`OH_SESSION_ATTACH` et `--attach` passent avant) |
| | `iterm_style` | `tab` | Onglet, panneau ou fenêtre iTerm2 |
| | `idle_sleep_minutes` | `5` | Veille d'un serveur sans activité ni décision en attente |
| | `notify` | `on` | Notifications système du démon (décision en attente, fin de tour) |
| `[execution]` | `runtime` | vide | Environnement préféré (Réglages › Exécution), utilisé si le workflow l'autorise |
| | `engine` | `auto` | Moteur de conteneurs |
| | `keep_images` | `2` | Images conservées par projet et par rôle (base, dev) |
| | `opencode_version` | vide | Version d'opencode des images ; si elle diffère du client de la machine, le lancement en conteneur est refusé |
| | `strict_isolation` | `false` | Masque aussi la configuration personnelle d'opencode (`XDG_CONFIG_HOME`) des serveurs locaux |
| `[remote]` | `targets[]` | — | Cibles `oh-runner` (`oh remote setup`) : `name`, `url`, `group`, `runner_project`, `token_key`, `trigger_key`, `tag`, `builder` (`kaniko`\|`dind`), `arch` (`amd64`\|`arm64`), `timeout` |
| | `projects` | — | Projet oh → nom de cible |
| `[limits]` | `max_active_sessions`, `session_budget_usd`, `daily_budget_usd`, `memory_mb`, `models` | non définies | Restrictions I6 du hub (`oh budget set`) |
| `[deploy]` | `instruction_files` | `[]` | Fichiers du projet ajoutés aux instructions des agents du paquet |
| `[websearch]` | `enabled` | `false` | Permissions WebSearch / WebFetch |

Voir [conteneur](../guides/container.fr.md), [exécution distante](../guides/remote-runners.fr.md) et [sessions v5](../guides/sessions-v5.fr.md#restrictions).

---

## Commandes de configuration

| Commande | Description |
|----------|-------------|
| `oh config list [--json]` | Afficher toutes les valeurs |
| `oh config get <clé>` | Obtenir une valeur (notation pointée : `opencode.default_provider`) |
| `oh config set <clé> <valeur>` | Définir une valeur |
| `oh config unset <clé>` | Supprimer une clé |
| `oh config path` | Afficher le chemin du fichier |
| `oh config language [fr\|en]` | Obtenir ou définir la langue d'affichage |
| `oh config websearch [enable\|disable\|status]` | Gérer WebSearch pour les agents |
| `oh config model agent\|family\|default\|show\|unset` | Cascade des modèles (voir [résolution des modèles](model-resolution.fr.md)) |
| `oh budget show\|set\|unset\|raise` | Restrictions I6 du hub ou d'un projet |
| `oh remote setup\|status` | Cibles d'exécution distante |

---

## Stockage des projets

Les projets sont stockés dans une **base SQLite** : `~/.oh/oh.db`.

### Champs d'un projet

| Champ | Description |
|-------|-------------|
| ID | Slug auto-généré + préfixe UUID |
| Name | Nom lisible du projet |
| Path | Chemin absolu |
| Language | go, typescript, python, rust, java, etc. |
| Provider | Surcharge du fournisseur, ou vide pour le défaut du hub |
| Model | Modèle du projet, ou vide pour le défaut du hub |
| ModelOverrides | Modèles par famille et par agent du projet |
| MCPConfig | Configuration MCP du projet (surcharge le hub) |
| ProviderConfig | Configuration du fournisseur du projet (surcharge le hub) |
| TrackerConfig | Surcharges tracker du projet (projet et motif du tracker) |
| ExecConfig | Configuration Exécution (voir ci-dessous) |
| TeamID | Équipe du projet (`[[teams]].id`), vide = projet sans équipe |
| Status | active, archived |
| CreatedAt / UpdatedAt | Horodatages |

### Configuration Exécution du projet

Réglée dans la TUI (Config projet › Exécution) :

| Champ | Défaut | Rôle |
|---|---|---|
| Dockerfile | détecté : `Dockerfile.dev`, `dev.Dockerfile`, `.devcontainer/Dockerfile`, `Dockerfile` | Dockerfile de dev de l'image du projet (conteneur) |
| Arguments de build | — | `build_args` de l'image de dev |
| Volumes | — | Volumes de cache persistants (chemin absolu dans le conteneur, ou relatif à chaque emplacement monté, ex. `node_modules`) |
| Workflow par défaut | aucun | Lancé par `oh run` sans argument, premier du bloc « Démarrer » |
| Runtime par défaut | vide | `local`, `container` ou `remote`, utilisé si le workflow l'autorise |

Ordre du runtime : `--runtime` > projet > Réglages (`[execution] runtime`) > `runtime.default` du workflow.

### Commandes projet

| Commande | Description |
|----------|-------------|
| `oh project list` | Lister les projets |
| `oh project add` | Enregistrer un projet |
| `oh project remove` | Désenregistrer un projet |
| `oh project rename` | Renommer un projet |
| `oh project move` | Mettre à jour le chemin d'un projet |
| `oh project configure` | Modifier les paramètres d'un projet (fournisseur, modèle, tracker) |

---

## Configuration des notifications

Les notifications d'équipe sont configurées dans le `config.toml` du **team-state** (pas dans `hub.toml`). Elles sont gérées via `oh team init` ou en éditant la configuration du team-state. Les notifications **système** des sessions se règlent dans `[session] notify` de `hub.toml`.

Voir le [guide de configuration d'équipe](../guides/team-setup.fr.md) et le [guide de configuration](../guides/configuration-guide.fr.md#5-configuration-equipe).

```toml
# config.toml du team-state (PAS hub.toml)
[notification]
enabled = true

[[notification.destinations]]
type = "slack"           # mattermost | slack | discord | teams
webhook_url = "https://hooks.slack.com/services/..."
bot_name = "OpenHub"

[[notification.destinations]]
type = "discord"
webhook_url = "https://discord.com/api/webhooks/..."
```

| Champ | Type | Défaut | Description |
|-------|------|--------|-------------|
| `enabled` | bool | false | Activer les notifications |
| `destinations[].type` | string | — | Backend : mattermost, slack, discord, teams |
| `destinations[].webhook_url` | string | — | URL du webhook entrant |
| `destinations[].channel` | string | — | Nom du canal (Mattermost uniquement) |
| `destinations[].bot_name` | string | "OpenHub" | Nom d'affichage du bot |

---

## Configuration multi-équipes (ADR-029)

Un utilisateur peut appartenir à **plusieurs équipes**. Chaque équipe est une entrée du tableau `[[teams]]` ; un projet référence une équipe par son `id`.

### Niveau hub : `[[teams]]`

```toml
[[teams]]
id = "acme"                              # identifiant court local
name = "Équipe ACME"                     # nom affiché (facultatif, sinon l'id)
enabled = true                           # activer les fonctions d'équipe
state_repo = "git@gitlab.com:acme/team-state.git"
state_path = ""                          # vide = ~/.oh/team-states/<hôte>/<dépôt>
member_id = "alice"                      # votre identité dans cette équipe

[[teams]]
id = "beta"
name = "Équipe Beta"
state_repo = "git@github.com:beta/oh-team.git"
member_id = "alice.dupont"
```

| Champ | Type | Obligatoire | Description |
|-------|------|-------------|-------------|
| `id` | string | Oui | Identifiant court local, utilisé par les projets |
| `name` | string | Non | Nom affiché (sinon `id`) |
| `enabled` | bool | Non | Activer / désactiver cette équipe |
| `state_repo` | string | Oui (sauf solo) | URL Git du dépôt team-state |
| `state_path` | string | Non | Chemin du clone local (déduit de `state_repo`) |
| `member_id` | string | Oui | Votre identité dans `members.toml` |
| `solo` | bool | Non | Espace solo (voir ci-dessous) |

### Espace solo (`solo = true`)

`oh team init --solo` ajoute une équipe locale, sans `state_repo` :

```toml
[[teams]]
id = "solo"
enabled = true
solo = true
state_path = "~/.oh/teams/solo"
member_id = "alice"
```

Un espace solo ne contient que des workflows : les fonctions d'équipe restent désactivées et il n'est jamais l'équipe active. `oh team promote --remote <url>` retire `solo` et renseigne `state_repo`.

### Niveau projet : `team_id`

| Valeur | Signification |
|--------|---------------|
| vide | Projet sans équipe |
| `"acme"` | Le projet appartient à l'équipe « acme » |

### Cascade de résolution

```
project.TeamID vide          →  projet sans équipe, fonctions d'équipe désactivées
project.TeamID == "acme"     →  recherche dans teams[] par id, config de cette équipe
project.TeamID inconnu       →  dégradation douce (équipe désactivée)
```

### Migration depuis `[team]` (ancien format)

L'ancienne section unique `[team]` est migrée en `[[teams]]` au premier chargement :
- l'ID est déduit de l'URL `state_repo` (dernier segment, sans `.git`) ;
- une sauvegarde de `hub.toml` est faite avant l'écriture ;
- projets en `Mode: "inherit"` → `TeamID = <id déduit>` ;
- projets en `Mode: "disabled"` → `TeamID` vide.

### Commandes CLI

| Commande | Description |
|----------|-------------|
| `oh teams list` | Lister les équipes configurées |
| `oh teams add --repo <url> --member-id <id>` | Ajouter une équipe |
| `oh teams remove <team-id>` | Retirer une équipe (les projets deviennent sans équipe) |

---

## Configuration recommandée ou imposée (ADR-030)

Un réglage du team-state est soit **recommandé** (modifiable), soit **imposé** (verrouillé).

### Cascade de résolution d'un réglage

```
1. Imposé par l'équipe ?        → OUI : valeur de l'équipe (verrouillée)
                                → NON : continuer
2. Surcharge explicite du projet ? → OUI : valeur du projet
                                   → NON : continuer
3. Valeur dans le hub ?         → OUI : valeur du hub
                                → NON : continuer
4. Recommandé par l'équipe ?    → OUI : recommandation de l'équipe
                                → NON : défaut du système
```

Les restrictions I6 (`[limits.*]`) suivent leur propre règle : la valeur la plus précise l'emporte, une valeur imposée par l'équipe est un plafond (voir [`[limits]`](#section-limits)).

### Schéma TOML dans le `config.toml` du team-state

```toml
[mcp.gitlab]
enabled = true
enabled_enforced = true    # les membres DOIVENT avoir GitLab activé
url = "https://gitlab.company.com"
url_enforced = true        # l'URL ne peut pas être modifiée localement
write_recommended = true   # indicatif : l'écriture reste réglée par write_enabled du hub

[mcp.jira]
enabled = true             # recommandé (sans _enforced = modifiable)
```

### Affichage dans la TUI

- 🔒 **Imposé** : icône cadenas, champ non éditable, toast en cas de tentative
- `[team: recommended]` : origine quand la valeur vient de l'équipe
- `[hub]`, `[project]` : origine des valeurs locales

---

## Configuration d'équipe par projet (ancien format)

> **Déprécié :** `ProjectTeamConfig` (champ `mode`) est remplacé par `project.TeamID`. Les projets à l'ancien format restent pris en charge.

| Champ | Type | Description |
|-------|------|-------------|
| `mode` | string | `"inherit"` (défaut) · `"custom"` · `"disabled"` |
| `state_repo` | string | URL Git du team-state _(custom uniquement)_ |
| `state_path` | string | Chemin du clone local, déduit de `state_repo` si vide _(custom uniquement)_ |
| `member_id` | string | Identité, sinon `member_id` du hub _(custom uniquement)_ |

### Paquet de session : `OH_TEAM_ID` (anciennement `.opencode/team.json`)

Au lancement, oh résout la config effective et déclare le serveur MCP `team` dans le paquet de session avec `OH_TEAM_ID` et `OH_PROJECT_ID` dans son environnement ; le serveur les lit pour trouver l'équipe du projet de la session. `.opencode/team.json` n'est plus écrit ni lu (`oh deploy` supprimé en v5 ; les restes sont supprimés par `oh migrate deploy-cleanup`).

Quand l'équipe est désactivée pour le projet, le serveur MCP `team` n'est pas placé dans le paquet.

### Déduction automatique du chemin du clone

Quand `state_path` est vide, il est déduit de `state_repo` :

```
git@gitlab.com:acme/other-team.git  →  ~/.oh/team-states/gitlab.com/other-team
https://github.com/acme/my-team.git →  ~/.oh/team-states/github.com/my-team
```

Un ancien clone `~/.oh/team-states/<dépôt>` (sans hôte) est réutilisé s'il existe.

### Définir l'équipe d'un projet

- **À la création :** l'assistant `oh project add` contient une étape Équipe.
- **Après :** `team configure` dans l'omnibar de la TUI ; pris en compte au prochain lancement (paquet reconstruit).

---

## Configuration du team-state (`config.toml`)

**Emplacement :** racine du clone du team-state (`~/.oh/team-states/<hôte>/<dépôt>/config.toml`, espace solo : `~/.oh/teams/<id>/config.toml`)  
**Objet :** réglages partagés de l'équipe (notifications, claims, tracker, board, MCP, modèles, gouvernance des workflows, restrictions).  
Ce fichier est versionné dans le dépôt team-state — **pas** dans `hub.toml`.

> **Les identifiants ne sont pas stockés ici.** Les tokens sont lus dans le trousseau de chaque membre (`[mcp.gitlab]`, `[mcp.jira]` de `hub.toml`, ou `tracker_token_key`).

### Section `[claim]`

```toml
[claim]
done_retention_days = 7
```

| Champ | Type | Défaut | Description |
|-------|------|--------|-------------|
| `done_retention_days` | int | `7` | Jours pendant lesquels un claim reste `done` avant d'être purgé (`CleanupDoneClaims`) |

### Section `[tracker]`

Relie le team-state à un tracker externe (GitLab ou Jira).

```toml
[tracker]
enabled = true
type = "gitlab"              # "gitlab" ou "jira"
tracker_url = ""             # instance du tracker ; vide = URL du MCP
tracker_project = "group/app"   # projet GitLab (ID ou chemin) ou clé Jira
ticket_pattern = "APP-(\\d+)"   # regex à un groupe de capture → IID externe
auto_sync = true             # synchroniser à l'ouverture des vues d'équipe
sync_interval_minutes = 5    # intervalle quand le board est ouvert
auto_plan_assigned = true    # claims "planned" pour les issues assignées
max_auto_plan_per_member = 5
auto_plan_unassigned = false # claims de pool pour les issues non assignées
push_labels = false          # renvoyer les labels vers le tracker
write_enabled = false        # écritures tracker (labels, assignation)

[tracker.label_status_mapping]
"DEV DOING" = "in_progress"
"TO REVIEW" = "review"
```

| Champ | Type | Défaut | Description |
|-------|------|--------|-------------|
| `enabled` | bool | `false` | Activer l'intégration |
| `type` | string | — | `"gitlab"` ou `"jira"` (`type_enforced` pour l'imposer) |
| `tracker_url` | string | vide | Instance du tracker, indépendante du MCP |
| `tracker_token_key` | string | `openhub.tracker.<type>.token` | Clé du token dans le trousseau (sinon token du MCP) |
| `tracker_project` | string | vide | Projet du tracker par défaut (surchargeable par projet) |
| `ticket_pattern` | string | vide | Regex à **un groupe de capture** extrayant l'IID |
| `auto_sync` | bool | `false` | Synchroniser à l'ouverture des vues d'équipe |
| `sync_interval_minutes` | int | `5` si `auto_sync`, sinon `0` | Intervalle de sync quand le board est ouvert (`0` = désactivé) |
| `auto_plan_assigned` | bool | `false` | Claims `planned` pour les issues assignées aux membres |
| `max_auto_plan_per_member` | int | `5` | Plafond de claims auto-planifiés par membre |
| `auto_plan_unassigned` | bool | `false` | Claims de pool (à prendre avec `c`) pour les issues non assignées |
| `unassigned_labels` | liste | `[]` | Labels exigés pour les issues non assignées |
| `max_unassigned_issues` | int | `20` | Plafond d'issues non assignées par projet |
| `push_labels` | bool | `false` | Renvoyer les labels de claim vers le tracker (`push_labels_enforced` pour l'imposer) |
| `write_enabled` | bool | `false` | Autoriser les écritures du tracker |
| `status_mapping` | map | `{}` | Statut du tracker → statut de claim |
| `label_status_mapping` | map | `{}` | Label → statut de claim (le premier label trouvé gagne) |
| `ticket_patterns`, `[tracker.projects]` | map | `{}` | Ancien format par projet (déprécié, encore lu) |

**Notes :**

- Jira : les issues fermées sont détectées via `statusCategory.key == "done"`.
- Statuts de claim valides : `planned`, `in_progress`, `review`, `validation`, `blocked`, `done`.
- `hub.toml` `[tracker]` permet à un membre de désactiver localement `enabled`, `auto_sync`, `push_labels`, `auto_plan_assigned`, etc.

### Section `[board]`

Disposition des colonnes du board d'équipe. Sans cette section : 6 colonnes (TODO, IN PROGRESS, REVIEW, VALIDATION, DONE, BLOCKED).

| Rôle | Fonction |
|------|----------|
| `initial` | Colonne d'entrée : les nouveaux tickets arrivent ici (claims de pool, option `--planned`) |
| `active` | Travail en cours : compté dans les badges et résumés |
| `terminal` | Fin : déclenche le nettoyage des claims terminés |
| `blocked` | Blocage : compté à part dans les résumés |

```toml
[board]
columns = [
    { id = "todo",        name = "TODO",         role = "initial" },
    { id = "in_progress", name = "IN PROGRESS",  role = "active" },
    { id = "review",      name = "CODE REVIEW",  role = "active" },
    { id = "testing",     name = "TESTING",      role = "active",  color = "cyan" },
    { id = "done",        name = "DONE",         role = "terminal" },
    { id = "blocked",     name = "BLOCKED",      role = "blocked" },
]
```

| Champ | Type | Défaut | Description |
|-------|------|--------|-------------|
| `columns` | tableau | 6 colonnes | Colonnes ordonnées |
| `columns[].id` | string | — | Identifiant interne, minuscules, sans espace, `..` ni `/` ; valeur de statut des claims |
| `columns[].name` | string | — | Libellé de l'en-tête |
| `columns[].color` | string | selon le rôle | `orange`, `blue`, `gray`, `cyan`, `green`, `red`, `purple`, `yellow` |
| `columns[].role` | string | `"active"` | `initial`, `active`, `terminal`, `blocked` |

> **Astuce :** la commande `Discover Tracker` (omnibar ou touche `y` dans la config d'équipe) configure les colonnes depuis les labels du tracker. Sans `[board]`, les claims `"planned"` existants vont dans la première colonne.

### Section `[mcp.<service>]`

Recommandations ou obligations sur les serveurs MCP (voir [recommandé ou imposé](#configuration-recommandée-ou-imposée-adr-030)) : `enabled`, `enabled_enforced`, `url`, `url_enforced`, `write_recommended`.

### Section `[models]`

```toml
[models]
default = "claude-sonnet-4-20250514"
[models.families]
quality = "claude-opus-4-20250514"
[models.agents]
reviewer = "claude-opus-4-20250514"
```

Recommandations de modèles de l'équipe, éditées dans la TUI (Équipe › Modèles). **Non appliquées au lancement en v5** : la cascade d'une session ne contient pas de niveau équipe (voir [résolution des modèles](model-resolution.fr.md)).

### Section `[governance]`

```toml
[governance]
publish = "any_member"   # valeur par défaut, seule prise en charge
```

Qui peut publier les workflows de l'équipe. Une valeur inconnue bloque la publication. Voir [workflows d'équipe](../guides/team-workflows.fr.md#gouvernance).

### Section `[limits]`

```toml
[limits.recommended]     # valeur par défaut, modifiable par le hub, le projet ou le workflow
session_budget_usd = 5

[limits.enforced]        # plafond qu'aucun autre niveau ne peut assouplir
daily_budget_usd = 50
models = ["eu.anthropic.claude-*"]
```

Clés : `max_active_sessions`, `session_budget_usd`, `daily_budget_usd`, `memory_mb`, `models`. Cascade : hub → équipe (recommandé / imposé) → projet → workflow (`limits:`). Voir [ADR-044](../architecture/adr/044-credential-proxy-session-limits.fr.md) et `oh budget show`.

### Sections `[takeover]` et `[parallel]`

| Champ | Défaut | Description |
|---|---|---|
| `takeover.stale_days` | `3` | Jours d'inactivité avant qu'un claim soit considéré comme abandonné (briefs de reprise) |
| `parallel.max_sessions` | `5` | Hérité de l'ancien mode parallèle (supprimé en v5) ; encore affiché dans le détail d'équipe, sans effet sur `oh run --tickets` |

---

## Vue Notifications

La TUI garde chaque toast dans un store en mémoire et rend l'historique complet accessible dans une vue dédiée.

### Accéder à la vue

| Commande (omnibar) | Description |
|----------|-------------|
| `notifications` | Ouvrir l'historique des notifications |
| `notif` / `logs` / `messages` / `toasts` / `erreurs` | Alias |

### Ce qu'elle affiche

```
14:32:05  ✗  Erreur setup team : git clone https://gitlab.com/...: fatal: repository not found
14:31:58  ✓  Équipe configurée : custom — appliqué au prochain lancement d'une session
14:31:52  →  Initialisation team pour ce projet...
```

Chaque entrée contient :
- **Horodatage** (`HH:MM:SS`)
- **Icône de niveau** : `✓` succès · `✗` erreur · `!` avertissement · `→` info
- **Message complet** : les toasts à l'écran sont tronqués (80 caractères pour les erreurs et avertissements, 50 pour les autres) ; le store garde le texte complet

La vue défile avec `j` / `k`.

### Comportement du store

- **Capacité :** 50 entrées en mémoire (FIFO).
- **Persistance :** ajout dans `~/.oh/notifications.jsonl` après chaque toast. Rotation au démarrage de la TUI (500 lignes max, entrées de plus de 7 jours supprimées). La vue charge les 50 dernières entrées à chaque ouverture.
- **Journal stderr :** les erreurs sont aussi écrites sur stderr (`[ERROR] <message>`).

---

## Sélection de texte

La TUI permet de sélectionner du texte à la souris en dehors des widgets interactifs (omnibar, fenêtres modales), sans dépendance externe.

### Utilisation

1. **Cliquer-glisser** pour sélectionner (vidéo inverse)
2. **Relâcher** : le texte est copié dans le presse-papiers
3. **Double-clic** : sélectionne le mot
4. **Triple-clic** : sélectionne la ligne
5. **Échap** : efface la sélection

### Presse-papiers

| Stratégie | Plateformes |
|-----------|-------------|
| Séquence OSC 52 via `/dev/tty` | Terminaux modernes (iTerm2, WezTerm, Alacritty, kitty, tmux avec `set-clipboard on`), fonctionne en SSH |
| `pbcopy` | Repli macOS |
| `wl-copy` | Repli Linux/Wayland |
| `xclip` / `xsel` | Repli Linux/X11 |
| `clip.exe` | Repli Windows |

Les deux stratégies sont tentées ; il suffit qu'une réussisse.

### Zones interactives

Les événements souris de ces zones passent à tview sans déclencher de sélection :

- omnibar (conteneur, champ de saisie, suggestions) ;
- toute fenêtre modale (invites, listes de choix).

### Toasts

Les toasts sont **sélectionnables** : un clic dessus démarre une sélection.

---

## Secrets / clés API

Les secrets sont stockés dans le **trousseau du système** (macOS Keychain, Linux secret-service, Windows Credential Manager) ; `~/.oh/secrets-index.json` liste les clés connues (sans valeur).

### Repli

Si le trousseau n'est pas disponible, les secrets sont stockés dans `~/.oh/secrets.enc` (AES-256-GCM, dérivation Argon2id).

**Source de la phrase de passe :**

1. Variable d'environnement `OH_PASSPHRASE`
2. Saisie dans le terminal (confirmation à la première utilisation, 8 caractères minimum)

### Commandes

| Commande | Description |
|----------|-------------|
| `oh mcp setup` | Enregistrer un token de service MCP |
| `oh provider` | Configurer les identifiants d'un fournisseur |
| `oh secrets` | Lister et gérer les secrets |

### Clés connues

| Clé | Usage |
|-----|-------|
| `openhub.provider.bedrock.token` | Token Bearer AWS pour Bedrock |
| `openhub.provider.anthropic.token` | Clé API Anthropic |
| `openhub.provider.openrouter.token` | Clé API OpenRouter |
| `openhub.provider.<fournisseur>.token.<projet>` | Identifiants propres à un projet |
| `openhub.team.<équipe>.provider.<fournisseur>.token` | Identifiants fournis par une équipe |
| `openhub.mcp.<service>.token` | Token d'un serveur MCP (`figma`, `gitlab`, `jira`, `gslides`…) |
| `openhub.team.<équipe>.gitlab.token` | Token GitLab propre à une équipe |
| `openhub.tracker.<type>.token` | Token du tracker de l'équipe |
| `openhub.remote.<cible>.token` / `.trigger` | Token API et token de déclenchement d'une cible distante |
| `openhub.daemon.capability` | Capacité locale du démon `ohd` |

Les anciens noms (`bedrock-token-default`, `gitlab-token`, `figma-token`…) sont renommés automatiquement au chargement.

Les vraies clés des fournisseurs ne sont jamais données aux sessions : le **proxy d'identifiants** du démon remet à chaque groupe de serveur un jeton `ohs_…` ([ADR-044](../architecture/adr/044-credential-proxy-session-limits.fr.md)).

---

## Configuration de session (paquet de session)

Le `opencode.json` du projet n'est plus généré (`oh deploy` supprimé en v5). À chaque lancement, oh construit un **paquet de session** depuis le workflow (`~/.oh/bundles/<hash>/`, immuable) ; l'adaptateur en tire la configuration opencode de la session (agents du paquet seulement, permissions, MCP, plugins du workflow, modèle) et la passe au serveur `opencode serve`.

> **Note :** l'inspecter avec `oh bundle show <workflow>` (`--json`, `--budget`). Dans un projet déployé par une version précédente, `oh migrate deploy-cleanup` ne retire du `opencode.json` que les clés écrites par oh et inchangées depuis le dernier déploiement (vos propres clés sont conservées).

---

## Variables d'environnement

| Variable | Usage |
|----------|-------|
| `OH_HOME` | Déplace le dossier `~/.oh` (tests, environnements isolés) |
| `OH_PASSPHRASE` | Phrase de passe du stockage chiffré de secrets (repli) |
| `OH_SESSION_ATTACH` | Ouverture des sessions (passe avant `[session] attach`) |
| `OH_RICH_TUI` | `0` désactive la TUI (invites en ligne seulement) |
| `OH_DAEMON_INPROCESS` | `1` fait tourner le démon dans le processus oh (toujours le cas sous Windows) |
| `CI` | Non vide : pas de TUI |
| `TERM` | `dumb` : pas de TUI |
| `TMUX`, `TERM_PROGRAM` | Détection de tmux et d'iTerm2 pour l'ouverture des sessions |
| `OH_SESSION_ID`, `OH_TEAM_ID`, `OH_PROJECT_ID` | Posées par oh dans l'environnement des sessions et des serveurs MCP (ne pas définir à la main) |
| `AWS_PROFILE`, `AWS_REGION`, `AWS_DEFAULT_REGION`, `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, `AWS_BEARER_TOKEN_BEDROCK` | Détection des identifiants Bedrock (`oh init`, `oh doctor`) |
| `ANTHROPIC_API_KEY`, `OPENROUTER_API_KEY` | Détection des identifiants Anthropic et OpenRouter |
| `FIGMA_TOKEN` | Token API Figma (serveur MCP) |
| `GITLAB_TOKEN`, `GITLAB_URL`, `GITLAB_WRITE_ENABLED` | Serveur MCP GitLab (URL par défaut : `https://gitlab.com`) |
| `GOOGLE_ACCESS_TOKEN` | Token OAuth Google (serveur MCP) |
| `GITHUB_TOKEN` (alias `GH_TOKEN`), `GITHUB_WRITE_ENABLED` | Serveur MCP GitHub |
| `JIRA_URL`, `JIRA_TOKEN` (ou `JIRA_USER` + `JIRA_API_TOKEN`), `JIRA_WRITE_ENABLED` | Serveur MCP Jira |
| `LINEAR_API_KEY`, `LINEAR_WRITE_ENABLED` | Serveur MCP Linear |
| `SSL_CERT_FILE` | Certificats transmis au serveur des sessions distantes (sur le runner) |
| `PAGER` | Pager de l'aide (défaut : `less`) |

Les variables d'environnement des serveurs MCP passent avant le trousseau (voir [serveurs MCP](services.fr.md#variables-denvironnement-au-runtime)).

---

## Résolution du fournisseur

Au démarrage d'une session, le fournisseur LLM est résolu dans cet ordre :

1. Option `--provider` / `-P`
2. `project.Provider` dans la base
3. `opencode.default_provider` dans `hub.toml`
4. `"bedrock"` (repli en dur)

---

## Récapitulatif des emplacements

| Chemin | Usage |
|--------|-------|
| `~/.oh/` | Dossier de configuration (`OH_HOME`) |
| `~/.oh/hub.toml` | Configuration du hub |
| `~/.oh/hub/` | Contenu du hub extrait (agents, skills, workflows) |
| `~/.oh/oh.db` | Base SQLite (projets, sessions, décisions) |
| `~/.oh/secrets.enc` | Secrets chiffrés (repli) |
| `~/.oh/secrets-index.json` | Index des clés du trousseau |
| `~/.oh/bundles/<hash>/` | Paquets de session construits au lancement — plus rien n'est déployé dans `<projet>/.opencode/` ni `<projet>/opencode.json` |
| `~/.oh/run/` | Démon `ohd` (socket, journaux) |
| `~/.oh/servers/`, `~/.oh/sessions/<id>/` | État des serveurs et des sessions (dont `limits.json`) |
| `~/.oh/cache/` | Cache (images de conteneur…) |
| `~/.oh/team-states/<hôte>/<dépôt>/` | Clones des team-states |
| `~/.oh/teams/<id>/` | Espaces solo |
| `~/.oh/migrated/` | Anciennes surcharges de workflow archivées |
| `~/.oh/notifications.jsonl` | Historique des notifications de la TUI |
| `~/.oh/mcp/<nom>/manifest.json` | Manifeste d'un serveur MCP personnalisé |

> **Intégrité de la base :** `oh repair --check-only` vérifie la base SQLite ; `oh repair` affiche la version du schéma.
>
> **Tableau de bord web :** `oh serve` écoute sur `127.0.0.1` uniquement, jamais sur le réseau.
