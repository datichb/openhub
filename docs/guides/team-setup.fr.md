> [Read in English](team-setup.en.md)

# Guide de Setup Équipe

## Prérequis

- CLI `oh` installé et configuré (`oh init` fait)
- Un repo Git créé sur GitLab/GitHub pour le team-state (vide ou avec un README)
- Tous les membres de l'équipe ont les droits push sur ce repo

## 1. Créer le repo team-state

Un membre de l'équipe crée le repo sur GitLab/GitHub :

```bash
# Sur GitLab/GitHub, créer un nouveau repo nommé "team-state"
# (ou le nom de votre choix)
# Visibilité : Internal ou Private (accessible à l'équipe)
```

Le repo sera peuplé automatiquement par `oh team init`.

## 2. Initialiser les fonctions d'équipe

Chaque membre exécute :

```bash
oh team init
```

Ou depuis le TUI, tapez `team init` dans l'omnibar (`Ctrl+P`).

### Mode CLI (terminal)

Le wizard s'exécute en plein écran avec 5 étapes adaptatives et un panneau d'info latéral.

### Mode TUI (wizard inline)

Le wizard tourne directement dans le shell TUI sans quitter l'interface. L'omnibar
et les toasts restent accessibles. Raccourcis : `Ctrl+S` valider, `Ctrl+B` (ou `Alt+←`, `PgUp`) retour,
`Esc` pour « Quitter l'assistant ? » (quitter, passer une étape facultative ou continuer).

### Phase 1 — Connexion au repo (séquentiel)

Un formulaire demande l'URL du repo team-state puis :
1. Clone le repo dans `~/.oh/team-state/` (ou pull si déjà cloné)
2. Crée la structure de répertoires (si premier membre)
3. Détecte les clones dupliqués pour éviter les états divergents

Si le repo est en HTTPS, une étape supplémentaire propose de configurer les credentials
(token, ou skip si déjà configuré). Elle n'est **proposée que si le dépôt ne répond pas sans
token** : une fois le dépôt cloné, `oh` lance `git ls-remote` sans aucune invite ; si git lit
le dépôt avec ce qu'il a déjà (assistant d'identifiants, `netrc`, dépôt public), l'étape est
sautée, comme pour une URL SSH. Sur l'option par défaut « Fournir un token », un champ vide
est refusé avec un message qui dit quoi faire (saisir le token, ou choisir « Déjà configuré »
ou « Accès public »).

### Phase 2 — Configuration (wizard interactif)

Le wizard détecte automatiquement l'état du repo et adapte le comportement :

| Étape | Si repo vide (1er membre) | Si repo déjà configuré |
|-------|---------------------------|------------------------|
| **Config globale** | Création `config.toml` (stale_days, sessions) | Mise à jour si modifié |
| **Identité** | Enregistrement (6 champs) + commit-and-push | Mise à jour + commit-and-push |
| **Notifications** | Configuration webhook Mattermost | Mise à jour si modifié |
| **Policies** | Multi-select de policies recommandées | Merge avec existantes |

#### Champs du profil (étape Identité)

| Champ | Description | Exemple |
|-------|-------------|---------|
| Identifiant | Clé unique dans members.toml | `benjamin` |
| Nom d'affichage | Comment ton nom apparaît dans les notifications | `Benjamin` |
| Username GitLab | Pour l'intégration GitLab | `bdatiche` |
| Username Mattermost | Pour les mentions dans les notifications | `benjamin.datiche` |
| Username tracker | Ton nom sur le tracker (`tracker_username`). **Indispensable quand le tracker est sur un autre serveur que le dépôt d'équipe** (voir ci-dessous) | `benjamin.datiche` |
| Rôle | Ton rôle dans l'équipe | `lead`, `dev`, ou `reviewer` |

#### Ton nom sur le tracker (`tracker_username`)

La synchro du tracker attribue les issues aux membres **par nom d'utilisateur** : le `tracker_username` de la fiche du membre, à défaut son `gitlab_username`. Quand le tracker est sur un autre GitLab (ou Jira) que le dépôt d'équipe, ton nom n'y est pas forcément le même (`benjamin.datiche` d'un côté, `benjamin.datiche1` de l'autre) : sans `tracker_username`, aucune issue du tracker ne t'est attribuée. Il s'écrit dans ta fiche du dépôt d'équipe (`members.toml`, un commit) et se règle à ces endroits :

| Où | Comment |
|----|---------|
| `oh team init` → étape Identité | champ « Username tracker » (CLI et TUI) |
| `oh team rejoin` | option `--tracker-username <nom>`, ou la question posée quand le tracker de l'équipe est sur un autre serveur que le dépôt et que ta fiche n'a pas de nom |
| Assistant `oh init` (groupe équipe, « rejoindre ») et TUI `team rejoin` | étape « Nom sur le tracker », mêmes conditions |
| `oh team profile` | `oh team profile --tracker-username <nom>` ; sans option, affiche ta fiche ; une option donnée vide efface le champ |
| TUI, vue `team-detail` | section « Ma fiche de membre » : nom affiché, noms GitLab, Mattermost et tracker, enregistrés tout de suite (un commit), sans passer par `w` |

`oh doctor` le contrôle : ✗ si le tracker est sur un autre serveur que le dépôt d'équipe et que ton `tracker_username` est vide ; ⚠ (en ligne) si le nom que le tracker renvoie pour ton jeton ne correspond à aucun membre. La synchro (`oh team sync-tracker`) le dit aussi dans son résumé quand des issues sont assignées à des noms inconnus de l'équipe et que ta fiche n'a pas de `tracker_username`. Voir [`oh doctor`](../reference/cli-infra.fr.md#oh-doctor).

#### Policies recommandées (étape Policies)

Le wizard propose un set recommandé à multi-sélectionner :
- **Branch naming** — `feat/fix/chore/.../[a-z0-9-]+` (enforcement: refuse)
- **Commit format** — Conventional Commits (enforcement: warn)
- **Max WIP** — 2 tickets par membre (enforcement: warn)
- **Review required** — Review obligatoire avant merge (enforcement: refuse)

### Résultat

À la fin du wizard :
1. `config.toml` est créé/mis à jour dans le repo team-state
2. Le membre est enregistré dans `members.toml` et commité + pushé
3. Les notifications sont configurées (optionnel)
4. Les policies sont créées dans `policies.toml` et commitées + pushées (optionnel)
5. `hub.toml` est mis à jour avec la configuration `[[teams]]`

En mode TUI, un écran résumé s'affiche avec les valeurs configurées et propose
de naviguer vers la vue de détail équipe (`Enter`) ou de retourner à l'accueil (`Esc`).

## 3. Configurer les notifications (référence)

> **Note:** Les notifications sont proposées automatiquement lors de `oh team init`.
> Cette section décrit comment les modifier manuellement après le setup initial.

Édite `config.toml` dans le repo team-state (ou `hub.toml` pour la config hub) :

### Mattermost

```toml
[notification]
enabled = true
type = "mattermost"
mattermost_webhook = "https://mattermost.example.com/hooks/..."
channel = "#dev-ai"
bot_name = "OpenHub"
```

Pour obtenir l'URL du webhook : Mattermost > Intégrations > Webhooks entrants > Ajouter.

### Slack

```toml
[notification]
enabled = true
type = "slack"
webhook_url = "https://hooks.slack.com/services/T.../B.../..."
bot_name = "OpenHub"
```

Pour obtenir l'URL du webhook : Slack > Ton app > Incoming Webhooks > Add New Webhook to Workspace.

### Discord

```toml
[notification]
enabled = true
type = "discord"
webhook_url = "https://discord.com/api/webhooks/.../..."
bot_name = "OpenHub"
```

Pour obtenir l'URL du webhook : Paramètres du salon Discord > Intégrations > Webhooks > Nouveau webhook.

### Microsoft Teams

```toml
[notification]
enabled = true
type = "teams"
webhook_url = "https://outlook.office.com/webhook/..."
```

Pour obtenir l'URL du webhook : Canal Teams > Connecteurs > Incoming Webhook > Configurer.

### Multi-destination (notifier plusieurs plateformes simultanément)

```toml
[[notification.destinations]]
type = "slack"
webhook_url = "https://hooks.slack.com/services/T.../B.../..."

[[notification.destinations]]
type = "discord"
webhook_url = "https://discord.com/api/webhooks/.../..."
```

Commite et push :

```bash
cd ~/.oh/team-state
git add config.toml
git commit -m "config: activation des notifications"
git push
```

## 4. Utiliser la team dans les sessions

Aucun redéploiement n'est nécessaire (`oh deploy` / `oh sync` supprimés en v5) : dès le prochain lancement (`oh run <workflow>`), le serveur MCP `team` est placé dans le paquet de session, rendant les outils d'équipe accessibles aux agents IA.

## 4b. Configuration de la team par projet

Par défaut, chaque projet **hérite** de la configuration team du hub (le bloc `[team]` dans
`hub.toml`). Chaque projet peut
choisir son mode team indépendamment.

### Les trois modes

| Mode | Description |
|------|-------------|
| `inherit` | Utilise le team-state repo et le member ID du hub _(défaut)_ |
| `custom` | Utilise un repo team-state différent — member ID fall back sur le hub si non défini |
| `disabled` | Désactive explicitement la team — MCP team non placé dans le paquet de session |

### Comment le mode est appliqué

Au lancement, oh résout la config team effective
(override projet → fallback hub) et déclare le serveur MCP `team` dans le paquet de
session avec `OH_TEAM_ID` dans son environnement. Le serveur lit `OH_TEAM_ID` pour
trouver la team du projet de la session (plus de `.opencode/team.json`).

Quand le mode est `disabled`, le serveur MCP `team` n'est pas placé dans le paquet de session.

Hors session (pas de `OH_TEAM_ID`), le serveur MCP team fall back sur la team active
de `hub.toml`.

### Choisir le mode à la création du projet

`oh project add` (ou le wizard `oh init` pour le premier projet) inclut une étape
**Team** qui présente le choix explicitement :

```
Team hub : git@gitlab.com:acme/team-state.git (member : alice)

Mode team :
  ▶ Utiliser la team du hub (alice)   ← défaut si le hub a une team
    Configuration spécifique
    Pas de team pour ce projet
```

Si le hub n'a **pas de team configurée**, l'étape propose par défaut *"Pas de team pour
ce projet"* et offre un opt-in vers une configuration custom.

### Changer le mode après la création

**Depuis l'omnibar TUI** (disponible à tout moment, quel que soit l'état team du hub) :

```
team configure
```

Cela ouvre un modal avec les trois choix. Le changement est pris en compte au
prochain lancement de session (paquet reconstruit) — aucun redéploiement nécessaire.

### Team custom : résolution du state path

Pour le mode `custom`, le `StatePath` est déduit automatiquement de l'URL du remote
si non défini explicitement :

```
git@gitlab.com:acme/other-team.git  →  ~/.oh/team-states/other-team
https://github.com/acme/my-team.git →  ~/.oh/team-states/my-team
```

Chaque remote team-state distinct obtient son propre répertoire de clone local sous
`~/.oh/team-states/` pour éviter les collisions.

### Résolution du Member ID

Pour le mode `custom`, si `MemberID` est laissé vide, il fall back sur le `member_id`
du hub. C'est l'approche recommandée quand un développeur participe à plusieurs équipes
avec la même identité.

```toml
# hub.toml — identité globale
[team]
enabled    = true
state_repo = "git@gitlab.com:acme/main-team.git"
member_id  = "alice"           # ← utilisé comme fallback pour tous les projets custom

# override par projet stocké en SQLite (colonne team_config) :
# { "mode": "custom", "state_repo": "git@github.com:beta/other.git" }
# member_id vide → résolu en "alice" depuis le hub au runtime
```

### Troubleshooting

**Le clone échoue avec "terminal prompts disabled" :**

Le remote requiert une authentification (HTTP basic ou token) et aucun credential
n'est configuré. `oh` positionne `GIT_TERMINAL_PROMPT=0` pour ne pas bloquer sur
un prompt — l'erreur est intentionnelle.

Solutions :
- **HTTPS + credential helper :** `git config --global credential.helper osxkeychain`
  (macOS) ou `git config --global credential.helper store`
- **SSH plutôt que HTTPS :** utiliser `git@gitlab.com:org/team-state.git`
- **Clé SSH non chargée :** `ssh-add ~/.ssh/id_ed25519` ou `eval "$(ssh-agent)"`

**Le clone timeout après 30 secondes :**

Le remote est injoignable (réseau, pare-feu, ou URL incorrecte). Le setup affichera
un toast d'erreur après expiration du délai.

**Le toast d'erreur est tronqué :**

Ouvrir le message complet via l'omnibar : taper `notifications` (ou `notif`, `logs`).
La vue Notifications charge les 50 dernières entrées depuis `~/.oh/notifications.jsonl`
(persistant entre les sessions) — le message complet est toujours disponible.

**Le TUI semble gelé pendant le setup :**

Cela ne devrait plus se produire depuis le fix deadlock Phase 3. Si c'est le cas,
appuyer sur `Ctrl+C` pour quitter et vérifier `~/.oh/team-states/` pour voir si le
clone a été tenté. L'erreur apparaîtra dans la vue Notifications au prochain démarrage
(`notifications` dans l'omnibar).

## Usage Quotidien

### Claims — Réservation de tickets

Les claims suivent un cycle de vie en 6 statuts :

```
oh team claim --planned → planned → (démarrage du ticket) → in_progress → review → (validation) → done
                                                              ↘ blocked
```

| Statut | Colonne | Description |
|--------|---------|-------------|
| `planned` | TODO | Réservé mais pas encore commencé |
| `in_progress` | IN PROGRESS | En cours de traitement (défaut au claim) |
| `review` | REVIEW | En attente de review |
| `validation` | VALIDATION | Revue passée, en validation (test, QA, pré-production) |
| `blocked` | BLOCKED | Bloqué |
| `done` | DONE | Terminé (conservé jusqu'à expiration de la rétention) |

```bash
# Réserver un ticket et démarrer immédiatement (statut : in_progress)
oh team claim SRU-142

# Réserver pour plus tard sans démarrer (statut : planned, colonne TODO)
oh team claim SRU-142 --planned

# Avec branche associée
oh team claim SRU-142 --worktree feat/SRU-142-user-auth

# Libérer quand c'est terminé
oh team release SRU-142

# Transférer à un autre membre
oh team claim transfer SRU-142 --to alice
```

Le tableau des statuts, ce qui met un ticket en REVIEW, le bail d'une réservation et ce que devient la réservation à la fin d'une session sont décrits en fin de guide : [Modèle de statut](#modèle-de-statut) et [Bail et fin de session](#bail-et-fin-de-session).

> **Note :** Démarrer une session sur un ticket déjà claimé en `planned` avec `oh run ticket --tickets <id>` le fait passer automatiquement en `in_progress`. Sinon, change le statut depuis le board d'équipe.

### Statut d'équipe

```bash
# Qui travaille sur quoi
oh team status

# Avec le détail des sous-tickets
oh team status --detail

# Board kanban interactif plein écran
oh team board

# Activité récente
oh team activity          # dernières 24h
oh team activity --today  # aujourd'hui seulement
oh team activity --week   # 7 derniers jours
oh team activity --member alice  # filtrer par membre
```

Le board affiche 5 colonnes : **TODO** · **IN PROGRESS** · **REVIEW** · **BLOCKED** · **DONE**.

Raccourcis clavier dans le board (et les autres vues d'équipe) :

| Touche | Action |
|--------|--------|
| `c` | Créer un claim |
| `x` | Libérer le claim sélectionné |
| `t` | Transférer le claim sélectionné |
| `s` | Sync tracker |
| `r` | Rafraîchir |

> **Pull automatique :** toutes les vues d'équipe (board, status, activity, etc.) effectuent un `git pull` automatique à l'ouverture et sur la touche `r`. Un toast n'est affiché que si l'opération prend plus d'une seconde.

### Synchronisation avec le tracker externe

```bash
oh team sync-tracker   # sync les claims avec le tracker externe (GitLab/Jira)
```

La commande tire les états des issues depuis le tracker et met à jour les statuts des claims :
- Issue fermée → claim passé en `done`
- Issue réouverte → claim repassé en `in_progress`
- Les labels sont mirrorés
- Avec `auto_plan_assigned = true`, des claims `planned` sont créés automatiquement pour les issues assignées
- Si des issues sont assignées à des noms qui ne sont ceux d'aucun membre alors que votre fiche n'a pas de `tracker_username`, le résumé de la synchro l'avertit et donne la commande (`oh team profile --tracker-username <nom>`)

### Gestion du wiki

```bash
# Valider les propositions wiki des agents IA
oh team wiki review

# Lister les pages wiki
oh team wiki list

# Lire une page
oh team wiki read decisions
```

## Structure du repo team-state

Après setup, le repo ressemble à :

```
team-state/
├── members.toml          # Registre de l'équipe
├── config.toml           # Config notifications + takeover + [governance]
├── workflows.lock        # Intégrité des workflows publiés
├── workflows/            # Workflows v5 (voir team-workflows.fr.md)
├── catalog/              # Briques d'équipe (agents, skills), commité avec la structure
├── policies.toml         # Règles d'équipe (enforcement configurable)
├── projects/
│   └── T-SRU/
│       ├── claims/
│       │   └── SRU-142.toml
│       ├── events/
│       │   └── 2026-07.jsonl
│       ├── takeover-briefs/      # Briefs de reprise de tickets
│       │   ├── bd-42_2026-07-13.toml
│       │   └── bd-42_2026-07-13.md
│       ├── workflows/            # Workflows du projet
│       └── policies-override.toml  # Overrides par projet (optionnel)
├── wiki/
│   ├── .pending/         # Propositions en attente
│   ├── decisions.md      # Décisions architecturales
│   └── patterns.md       # Patterns récurrents
└── reports/
```

`oh team init` crée les dossiers de cette structure (chacun avec un `.gitkeep`) et les **commite** en un commit « team: init structure », poussé comme les autres écritures de l'équipe ; `catalog/` (le catalogue partagé des briques d'équipe) en fait partie et n'est donc pas laissé non suivi dans votre clone. Rien n'est commité quand tout est déjà suivi, et un commit impossible (hors ligne) est repris à la commande suivante.

Le guide [Workflows d'équipe](team-workflows.fr.md) détaille `workflows/`, `workflows.lock` et la gouvernance (`[governance]` de `config.toml`).

## Comment les agents IA utilisent les données d'équipe

Quand les fonctions d'équipe sont activées, tous les agents ont accès en lecture aux outils team :

| Tool | Ce que les agents voient |
|------|--------------------------|
| `team_members` | Registre de l'équipe, rôles |
| `team_claims` | Qui travaille sur quoi |
| `team_wiki_read` | Connaissances partagées |
| `team_events` | Activité récente |
| `team_policies` | Règles d'équipe (conventions enforceables) |
| `team_takeover_brief` | Brief de reprise d'un ticket transféré |

L'agent `documentarian` a en plus `team_wiki_write` pour proposer des entrées wiki (toujours en attente de validation humaine).

## 5. Configurer les policies d'équipe

Les policies permettent d'enforcer les conventions automatiquement (bloquant ou warning).

Créer `policies.toml` dans le repo team-state :

```bash
# Interactif — ajoute une policy custom
oh policies add

# Ou éditer directement
cd ~/.oh/team-state
vim policies.toml
git add policies.toml && git commit -m "policies: initial setup" && git push
```

Voir le [guide des conventions](./team-conventions.fr.md#team-policies--enforcement-configurable) pour le format complet et les exemples.

```bash
# Vérifier les policies
oh policies list                  # Afficher les policies actives
oh policies check                 # Vérifier l'état courant
oh policies check --branch main   # Vérifier un nom de branche
```

## 6. Briefs de reprise (Takeover)

Quand un ticket est transféré d'un membre à un autre, un brief de reprise est
généré automatiquement. Il contient le contexte nécessaire pour reprendre le
travail sans perte d'information.

### Génération automatique

```bash
# Le brief est généré automatiquement au transfert
oh team claim transfer SRU-142 --to alice
# → Brief de reprise généré. oh takeover-brief show SRU-142
```

Si la réservation d'un autre membre a expiré (son bail est dépassé, voir
`lease_hours` ; sans bail, `stale_days` jours d'inactivité dans `config.toml`),
`oh team claim` propose de la reprendre : après confirmation (`--yes` sans
terminal), un brief de reprise est généré et la réservation vous revient avec
un nouveau bail :

```bash
oh team claim SRU-142
# → SRU-142 est réservé par benjamin, mais son bail a expiré le 22/10 14:30.
# → Reprendre myproject/SRU-142 à benjamin ?  Oui / Non
```

### Consulter et enrichir les briefs

```bash
# Afficher le brief d'un ticket
oh takeover-brief show SRU-142

# Lister tous les briefs du projet
oh takeover-brief list

# Enrichir un brief avec une analyse IA du code source : touche `e` de la vue Reprise de la TUI,
# ou en ligne de commande (sans enregistrer le brief) :
oh run brief-enrich --headless -i ticket=SRU-142
```

L'enrichissement lance le workflow `brief-enrich` sans interface (`oh run brief-enrich --headless -i ticket=<id>` ; le workflow lit lui-même le brief dans l'espace d'équipe) ; la vue Reprise de la TUI enregistre ensuite le brief enrichi dans team-state (`oh takeover-brief enrich`, retirée en 5.1, le faisait en ligne de commande). L'agent `brief-enricher` sert à :
- Lire les fichiers mentionnés dans le brief
- Identifier les décisions architecturales
- Repérer les questions ouvertes (TODO, FIXME)
- Proposer les prochaines étapes

### Configuration du stale

Dans `config.toml` du repo team-state :

```toml
[takeover]
stale_days = 3   # Nombre de jours d'inactivité pour considérer un ticket stale
```

## 7. Bibliothèque de patterns

Les patterns sont des décompositions de tickets réutilisables. Ils accélèrent
le planning en offrant une base éprouvée pour des types de travaux récurrents
(CRUD, intégration API, migration DB, etc.).

### Gérer les patterns

```bash
# Lister les patterns disponibles
oh patterns list
oh patterns list --tags backend,api

# Voir le contenu d'un pattern
oh patterns show crud-api

# Ajouter un pattern manuellement
oh patterns add                 # interactif
oh patterns add mon-pattern.md  # depuis un fichier

# Valider un pattern proposé par un agent
oh patterns validate crud-api

# Supprimer un pattern
oh patterns remove crud-api
```

### Alimentation automatique

Le planner et le pathfinder peuvent proposer des patterns automatiquement :
- Après un planning réussi (tous tickets complétés), le planner propose la décomposition
- Les patterns proposés par les agents sont en `validated=false` jusqu'à validation humaine

### Structure dans team-state

```
team-state/
  patterns/
    index.toml          # Catalogue des patterns (métadonnées)
    crud-api.md         # Contenu du pattern
    migration-db.md
    ...
```

## 8. Sessions parallèles

`oh run ticket --tickets` lance **une session par ticket**, dans un seul serveur de groupe ; chaque session qui écrit travaille dans son propre worktree Git. L'ancien mode parallèle (`oh start --parallel`, moniteur plein écran, vue de fusion) a été retiré en v5 : voir [Mode parallèle — remplacé en v5](parallel-mode.fr.md).

### Lancement

```bash
# Lancer 3 tickets, une session par ticket
oh run ticket --tickets bd-42,bd-43,bd-44

# Tous les tickets dans une seule session
oh run ticket --tickets bd-42,bd-43,bd-44 --one-session
```

`oh start --parallel --tickets …` a été retiré en 5.1 : utilisez `oh run ticket --tickets …`. `--priority` et `--max-sessions` n'ont plus d'effet ; pour limiter les sessions qui travaillent en même temps, utilise la restriction `max_active_sessions` (`oh budget set`, voir [Sessions v5 › Restrictions](sessions-v5.fr.md#restrictions)).

### Suivi

Les sessions apparaissent dans la vue **Sessions** de la TUI (omnibar `sessions`) et dans `oh session list` : état, décisions en attente, flux en direct, résultats (`oh session results <id> --mr`). Voir [Sessions v5](sessions-v5.fr.md).

### Fusion

Il n'y a plus de fusion proposée par oh : chaque session laisse sa branche, prête pour une MR/PR.

### Configuration

Dans `config.toml` du repo team-state :

```toml
[claim]
done_retention_days = 7    # Jours avant que les claims done soient nettoyés

[tracker]
enabled = true
type = "gitlab"             # "gitlab" ou "jira"
auto_sync = true            # sync à l'ouverture des vues d'équipe
sync_interval_minutes = 5
auto_plan_assigned = true   # crée des claims planned pour les issues assignées
max_auto_plan_per_member = 5
push_labels = true          # repousse les labels hub vers le tracker (requiert write_enabled)
ticket_patterns = { "T-SRU" = "SRU-(\\d+)" }

[tracker.projects]
"T-SRU" = "42"              # ID projet hub → ID/path projet GitLab
```

> **Note :** Les credentials de connexion (token, URL) sont réutilisés depuis `[mcp.gitlab]` / `[mcp.jira]` dans `hub.toml` — pas de duplication. Le jeton GitLab est cherché dans un seul ordre pour toutes les commandes (`GITLAB_TOKEN`, `tracker_token_key`, projet, équipe, MCP, ancienne clé du tracker) : voir [`oh team sync-tracker`](../reference/cli-team.fr.md#oh-team-sync-tracker). `oh mr create` trouve aussi le jeton de l'équipe.
>
> **Deux GitLab (dépôt d'équipe et tracker).** Chaque jeton n'est envoyé qu'au serveur pour lequel il est déclaré : le jeton du dépôt d'équipe (`openhub.team.<équipe>.gitlab.token`) ne sert que pour le GitLab du dépôt (l'hôte du `state_repo`, ou le `gitlab_url` déclaré quand le dépôt est joint par un alias SSH). Pour le tracker, stockez son jeton (`oh secrets set <clé>`), puis réglez la clé dans `[tracker]` de la config d'équipe ou de `hub.toml` (`oh config set tracker.tracker_token_key <clé>`, l'assistant `oh team config` → Tracker sync, les assistants `oh init` et `oh team rejoin`, ou la vue `team-detail` de la TUI, dont le champ « Jeton du tracker » affiche la clé et le serveur réellement déclarés — « non configuré » sinon — et range le jeton sous cette clé) : elle est valable pour le tracker, sans autre déclaration ; `tracker_token_url` ne sert que si le jeton est celui d'un autre serveur. Un jeton MCP sans `mcp.gitlab.url` est valable pour `https://gitlab.com` seulement ; réglez `mcp.gitlab.url` pour un autre GitLab. `GITLAB_TOKEN` demande toujours `OH_GITLAB_URL` (ou `GITLAB_URL`). Si `state_repo` passe par un alias SSH (`git@mon-alias:…`), déclarez le GitLab réel du dépôt (`gitlab_url` de votre équipe dans `hub.toml`, demandé par `oh init` et `oh team rejoin`, ou `--gitlab-url`) : le jeton de l'équipe est alors valable pour ce GitLab et non pour l'alias. Voir [le serveur de chaque jeton GitLab](../reference/config.fr.md#le-serveur-de-chaque-jeton-gitlab).
>
> **`enabled = false` :** une section `[tracker]` du `hub.toml` local avec `enabled = false` coupe la synchronisation dans la commande, les écrans d'équipe (`s`) et la synchronisation automatique. Depuis la v5.0, les écrans la respectent aussi.

## 9. Gestion du cycle de vie des équipes

### Gestion multi-équipe

Lister toutes les équipes configurées :

```bash
oh teams list
```

Ajouter une nouvelle équipe :

```bash
oh teams add --repo git@gitlab.com:org/team-state.git --member-id alice
```

Supprimer une équipe (détache tous les projets associés) :

```bash
oh teams remove <team-id>
```

### Détacher un projet de son équipe

Retirer l'affiliation d'un projet sans supprimer l'équipe :

```bash
oh teams detach <project-name>
```

### Archiver / Restaurer une équipe

Désactiver temporairement une équipe sans la supprimer. Les équipes archivées sont ignorées par toutes les commandes et agents :

```bash
oh teams archive <team-id>
oh teams restore <team-id>
```

L'archivage met `enabled = false` dans hub.toml. La restauration réactive l'équipe.

## Dépannage

### "team-state repo not cloned"

Lance `oh team init` pour configurer les fonctions d'équipe.

### "sync conflict after retries"

Le repo team-state a des changements conflictuels. Résous manuellement :

```bash
cd ~/.oh/team-state
git pull --rebase
# Résoudre les conflits éventuels
git push
```

### Notifications ne fonctionnent pas

1. Vérifie que `config.toml` a `enabled = true`
2. Vérifie l'URL du webhook
3. Pour Mattermost : vérifie que le channel existe
4. Pour Slack/Discord/Teams : teste le webhook avec `curl -X POST -d '{"text":"test"}' <webhook_url>`
5. Vérifie que `oh team status` fonctionne (confirme l'accès au repo)

<a id="limites-connues"></a>

## Modèle de statut

Un ticket existe à deux endroits : dans **Beads** (ou le tracker), où `oh board` et le Board de la TUI le lisent, et dans une **réservation** (claim) du team-state, que lit le board d'équipe. Chacun a ses statuts ; ce tableau (colonnes par défaut) dit comment ils se correspondent.

| Colonne | Ticket Beads | Réservation (team-state) |
|---|---|---|
| TODO | `open` | `planned` |
| IN PROGRESS | `in_progress` | `in_progress` |
| **REVIEW** | **`in_progress` + label `ready-for-review`** | **`review`** |
| VALIDATION | statut personnalisé (`validation`, `testing`, `qa`, `pre-release`) | `validation` |
| BLOCKED | `blocked` | `blocked` |
| DONE | `closed` | `done` |

Le contrat :

- **Sous Beads, REVIEW est un label, pas un statut.** Beads 1.3 n'a pas de statut `review` (sauf si le projet en déclare un avec `bd config set status.custom`). Un agent qui a fini met le label (`bd update <ID> --add-label ready-for-review`) et laisse le statut à `in_progress` ; la review retire le label pour renvoyer le ticket en développement, ou clôt le ticket. `oh board` et le Board de la TUI classent en REVIEW un ticket `in_progress` qui porte ce label.
- **Côté réservation, REVIEW est le statut `review`.** Il est posé quand une revue est publiée (`oh mr create`), depuis le board d'équipe ou par le vote de review MCP (approuvé : `review` puis `done` ; rejeté : retour à `in_progress`). Le board d'équipe affiche ce statut.
- **Les deux ne se recopient pas.** Mettre le label dans Beads ne change pas le statut de la réservation, et passer la réservation en `review` ne met pas le label.
- **Une seule équivalence, à la fin d'une session** : pour `teamstate.DecideClaimEnd`, le label `ready-for-review` (ou `review`), sur le ticket ou sur la réservation, vaut le statut `review` de la réservation. Dans les deux cas la réservation survit à la session (voir ci-dessous).
- **Une session distante n'écrit pas un statut que le projet ne connaît pas.** Le faux `bd` du job refuse, tout de suite et avec le message d'erreur de `bd` (code 1, avant toute écriture), un `bd update` ou `bd create` dont `--status` n'est pas dans la configuration Beads du projet (les statuts sont dans l'instantané envoyé au job ; la casse doit être exacte). Un agent distant qui écrit `--status review` sans statut `review` configuré est donc arrêté, au lieu d'échouer au rejeu ; il doit utiliser le label.

## Bail et fin de session

Une réservation en cours de travail a un **bail** : une date limite (`lease_until` dans le fichier de la réservation) après laquelle elle est *expirée*. Seule une réservation avec un propriétaire et du travail commencé a un bail : une réservation du pool (sans propriétaire), `planned`, dans la colonne initiale ou terminée n'en a pas, et ni `oh doctor` ni la synchronisation ne la jugent jamais expirée. **Une réservation en revue n'a pas non plus de bail qui s'épuise** (voir *Réservation en revue* ci-dessous).

**Réglages** (section `[claim]` du `config.toml` du team-state, modifiables dans `team-detail`) :

| Clé | Défaut | Bornes | Rôle |
|---|---|---|---|
| `lease_hours` | 24 | 1 à 168 | durée du bail d'une réservation travaillée par une session |
| `heartbeat_minutes` | 60 | 0 (désactivé) ou 5 à 1440, au plus la moitié du bail | période du signal de vie du démon qui renouvelle les baux |
| `sleeping_renewal_days` | 3 | 1 à 90 | une session en veille depuis plus longtemps ne renouvelle plus |
| `remote_renewal_days` | 7 | 1 à 90 | une session distante lancée depuis plus longtemps ne renouvelle plus |
| `lease_hours_manual` | 72 | `lease_hours` à 720 | durée du bail d'une réservation prise à la main |
| `release_expired_on_sync` | `false` | | la synchronisation du tracker libère les réservations expirées |

**Bail de session.** Quand une session démarre sur un ticket, la réservation reçoit la session et un bail de `lease_hours` ; si vous déteniez déjà la réservation (prise à la main, ou gardée par une session précédente), la session la reprend et le bail repart de zéro. Le démon (`ohd`, ou le processus `oh` sous Windows) envoie un signal de vie toutes les `heartbeat_minutes` : il renouvelle le bail des réservations des sessions ouvertes (ni terminées, ni échouées, ni arrêtées ; une session en veille est ouverte) quand il reste moins de la moitié du bail, en un commit par dépôt d'équipe (aucun tant que les baux sont longs). `heartbeat_minutes = 0` supprime ce renouvellement : les baux ne sont alors plus prolongés que par une reprise de session ou par `oh team claim`.

**Plafonds du renouvellement.** Pour qu'une réservation ne survive pas indéfiniment à une session que personne ne reprend, le signal de vie s'arrête : pour une session en veille, `sleeping_renewal_days` jours après sa mise en veille ; pour une session distante, `remote_renewal_days` jours après son lancement. Le bail n'est plus prolongé, la réservation expire normalement et `oh doctor` la signale.

**Réservation prise à la main.** Une réservation sans session (`oh team claim`, le board d'équipe, le plan du tracker) a un bail de `lease_hours_manual` que rien ne renouvelle tout seul. `oh team claim <ticket>` sur sa propre réservation le renouvelle, expiré ou non, sans confirmation (`lease_hours_manual` si aucune session n'est attachée à la réservation, `lease_hours` sinon). Démarrer une session sur le ticket remplace ce bail par celui de la session.

**Expiration et reprise.** Une réservation est expirée quand son bail est dépassé ; une réservation qui devrait avoir un bail mais n'en a pas (écrite avant la v5.1) l'est quand elle est inactive depuis plus de `takeover.stale_days` jours (3 par défaut, section `[takeover]` du même fichier). Pour la reprise, `oh team claim` applique cette règle d'inactivité à toute réservation sans bail. Une réservation expirée n'est pas supprimée toute seule :

- `oh doctor` (`claims_expired`, avertissement qui ne change pas le code de sortie) liste vos réservations expirées et celles que la fin d'une session a marquées `session-ended`, avec la commande pour les garder (`oh team claim <ticket>`) ou les libérer (`oh team release <ticket> -p <projet>`) ;
- un autre membre peut la reprendre avec `oh team claim <ticket>` : après la confirmation commune (`--yes` sans terminal, sinon refus, code 2), un brief de reprise est généré et la réservation lui revient avec un nouveau bail ;
- si l'équipe active `release_expired_on_sync`, la synchronisation du tracker libère les réservations expirées (un commit `release: N expired claim(s)`). Une réservation en revue (statut `review` ou label `review` / `ready-for-review`) n'en fait jamais partie.

**Fin de session.** Quand une session se termine, `teamstate.DecideClaimEnd` décide, pour chaque réservation de ses tickets (celles d'un autre membre ou d'une autre session sont laissées telles quelles), avec la première règle qui s'applique :

| Cas | Décision |
|---|---|
| la réservation est déjà terminée | gardée (la rétention des réservations terminées la nettoie) |
| le ticket est clos | la réservation passe au statut terminal |
| le ticket ou la réservation est en revue (label `review` ou `ready-for-review`, ou statut `review`) | **gardée** |
| un checkpoint attend une décision | gardée, label `session-ended` |
| le checkpoint de commit (`cp-2`) a été reporté alors que la session a laissé du travail (ou qu'on ne sait pas le dire) | gardée, label `session-ended` |
| l'état des checkpoints est illisible | gardée, label `session-ended` |
| sinon | libérée |

Une réservation gardée n'est plus renouvelée par la session qui s'est terminée : elle expire à la fin de son bail si personne ne la reprend (`oh team claim <ticket>` la renouvelle ; une nouvelle session sur le ticket aussi, et retire le label `session-ended`). La réservation en revue fait exception : elle n'expire pas (paragraphe suivant).

**Réservation en revue.** Une réservation dont le statut est `review`, ou qui porte le label `review` ou `ready-for-review`, **n'expire pas**, quel que soit son bail : elle attend une personne (le relecteur, la fusion), pas une session. Elle reste réservée jusqu'à la fusion (la réservation est alors terminée) ou jusqu'à ce que son propriétaire la libère (`oh team release <ticket> -p <projet>`) ; ni `oh team claim` d'un autre membre ni `release_expired_on_sync` ne la prennent. Si la revue renvoie le travail (statut `in_progress`, label retiré) et que son bail est dépassé, elle en reçoit un nouveau à partir de ce moment. `oh doctor` (`claims_review`, avertissement) liste vos réservations en revue depuis plus de `takeover.stale_days` jours, sans les dire expirées, avec la commande pour les libérer si le travail est abandonné.

**File durable.** Ce que la fin d'une session décide (libérer, terminer, marquer `session-ended`) et les renouvellements de bail qui n'ont pas pu être poussés sont d'abord écrits dans une file locale, `.git/oh-claims-queue.json` du clone du team-state (jamais committée). Chaque opération porte ses conditions : la réservation doit encore appartenir au même membre et à la même session, sinon elle est abandonnée et journalisée. La file est appliquée en un commit (un pull, un push) dès que le dépôt d'équipe répond : à la fin de la session (attente bornée à 15 s dont 10 s pour le verrou du clone ; le démon patiente jusqu'à 2 min), à chaque battement du démon, par la tâche `claims-queue` du démon (au démarrage, puis toutes les 15 minutes). Hors ligne, ou si le dépôt reste occupé, rien n'est perdu : les opérations restent dans la file et oh le dit à la fin de la session. Une session qui démarre hors ligne sur un ticket que vous déteniez déjà reprend la réservation au retour du réseau ; ce que la session précédente avait mis en file sur cette réservation est annulé. `oh doctor` (`claims_queue`) avertit quand des opérations attendent depuis plus de 15 minutes, ou quand le fichier est illisible (il est alors mis de côté, pas supprimé) ; `oh doctor --fix` applique la file après confirmation.

**Une réservation libérée volontairement le reste.** La synchronisation du tracker ne prend plus une réservation libérée par son titulaire (`oh team release`, fin de session, board, rétention des réservations terminées) pour une réservation perdue par accident et ne la recrée plus. Une issue assignée à un membre dont la réservation a été libérée ne redevient une réservation `planned` que si elle est mise à jour sur le tracker ensuite.

**Issues laissées hors du board.** `max_auto_plan_per_member` (5 par défaut) borne les réservations qu'une synchro crée pour un membre, par run (ce n'est pas un plafond permanent : la synchro suivante repart de zéro, et les issues retenues entrent en premier). Les issues déjà en cours passent en premier et sont toujours acceptées, hors limite. Ce que la limite laisse dehors est compté : le résultat de la synchro et le board d'équipe disent `N issue(s) hors du board, limite atteinte`.

**Autres signalements.** `oh doctor` signale aussi les réservations orphelines (ticket clos, ou toutes les sessions qui l'ont traité terminées), avec la commande à lancer ; les réservations prises sous l'ancien ID d'un projet recréé sont rattachées au projet local par `oh doctor --fix` (voir [`oh doctor`](../reference/cli-infra.fr.md#oh-doctor)). Une réservation oubliée se libère à la main : `oh team release <ticket-id> -p <projet>`.
