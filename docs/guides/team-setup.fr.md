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

Le wizard s'exécute en deux phases :

### Phase 1 — Connexion au repo (séquentiel)

Un formulaire demande l'URL du repo team-state puis :
1. Clone le repo dans `~/.oh/team-state/` (ou pull si déjà cloné)
2. Crée la structure de répertoires (si premier membre)

### Phase 2 — Configuration (wizard interactif avec sidebar)

Un wizard BubbleTea côte-à-côte s'affiche avec 4 étapes adaptatives.
Le wizard détecte automatiquement l'état du repo et adapte le comportement :

| Étape | Si repo vide (1er membre) | Si repo déjà configuré |
|-------|---------------------------|------------------------|
| **Config globale** | Création `config.toml` (stale_days, sessions) | "✓ Modifier ?" ou skip |
| **Identité** | Enregistrement (ID, nom, rôle, usernames) | "✓ Mettre à jour ?" ou skip |
| **Notifications** | Configuration webhook Mattermost | "✓ Modifier ?" ou skip |
| **Policies** | Multi-select de policies recommandées | "✓ Modifier ?" ou skip |

#### Champs du profil (étape Identité)

| Champ | Description | Exemple |
|-------|-------------|---------|
| Identifiant | Clé unique dans members.toml | `benjamin` |
| Nom d'affichage | Comment ton nom apparaît dans les notifications | `Benjamin` |
| Username GitLab | Pour l'intégration GitLab | `bdatiche` |
| Username Mattermost | Pour les mentions dans les notifications | `benjamin.datiche` |
| Rôle | Ton rôle dans l'équipe | `lead`, `dev`, ou `reviewer` |

#### Policies recommandées (étape Policies)

Le wizard propose un set recommandé à multi-sélectionner :
- **Branch naming** — `feat/fix/chore/.../[a-z0-9-]+` (enforcement: refuse)
- **Commit format** — Conventional Commits (enforcement: warn)
- **Max WIP** — 2 tickets par membre (enforcement: warn)
- **Review required** — Review obligatoire avant merge (enforcement: refuse)

### Résultat

À la fin du wizard :
1. `config.toml` est créé/mis à jour dans le repo team-state
2. Le membre est enregistré dans `members.toml`
3. Les notifications sont configurées (optionnel)
4. Les policies sont créées dans `policies.toml` (optionnel)
5. `hub.toml` est mis à jour avec la configuration `[team]`
6. Tous les changements sont commités et pushés

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

## 4. Déployer vers les projets

Après le team init, redéploie vers tes projets pour injecter le serveur MCP `team` :

```bash
oh deploy        # un seul projet
oh sync --all    # tous les projets
```

Cela ajoute le serveur MCP `team` dans `opencode.json`, rendant les outils d'équipe accessibles aux agents IA.

## 4b. Configuration de la team par projet

Par défaut, chaque projet **hérite** de la configuration team du hub (le bloc `[team]` dans
`hub.toml`). Depuis `oh deploy` v3 (config team par projet), chaque projet peut
choisir son mode team indépendamment.

### Les trois modes

| Mode | Description |
|------|-------------|
| `inherit` | Utilise le team-state repo et le member ID du hub _(défaut)_ |
| `custom` | Utilise un repo team-state différent — member ID fall back sur le hub si non défini |
| `disabled` | Désactive explicitement la team — pas de `.opencode/team.json`, MCP team non injecté |

### Comment le mode est appliqué

Au moment du deploy, `oh deploy` résout la config team effective
(override projet → fallback hub) et écrit `.opencode/team.json` dans le projet :

```json
// .opencode/team.json — généré par oh deploy, ne pas éditer manuellement
{
  "enabled": true,
  "state_repo": "git@gitlab.com:acme/team-state.git",
  "state_path": "/Users/alice/.oh/team-states/team-state",
  "member_id": "alice"
}
```

Quand le mode est `disabled`, ce fichier est **supprimé** (ou jamais créé) et le serveur
MCP `team` n'est pas injecté dans `opencode.json`.

Le serveur MCP team lit `.opencode/team.json` en priorité ; si absent, il fall back sur
`hub.toml` pour la compatibilité ascendante avec les projets non encore redéployés.

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

Cela ouvre un modal avec les trois choix. Après confirmation, redéploie le projet
pour appliquer le changement :

```bash
oh deploy          # un seul projet
oh sync --all      # tous les projets
```

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

Les claims suivent un cycle de vie en 5 statuts :

```
oh claim --planned → planned → (oh start --dev) → in_progress → review → done
                                                              ↘ blocked
```

| Statut | Colonne | Description |
|--------|---------|-------------|
| `planned` | TODO | Réservé mais pas encore commencé |
| `in_progress` | IN PROGRESS | En cours de traitement (défaut au claim) |
| `review` | REVIEW | En attente de review |
| `blocked` | BLOCKED | Bloqué |
| `done` | DONE | Terminé (conservé jusqu'à expiration de la rétention) |

```bash
# Réserver un ticket et démarrer immédiatement (statut : in_progress)
oh claim SRU-142

# Réserver pour plus tard sans démarrer (statut : planned, colonne TODO)
oh claim SRU-142 --planned

# Avec branche associée
oh claim SRU-142 --worktree feat/SRU-142-user-auth

# Libérer quand c'est terminé
oh release SRU-142

# Transférer à un autre membre
oh claim transfer SRU-142 --to alice
```

> **Note :** Démarrer une session sur un ticket déjà claimé en `planned` (`oh start --dev`) le fait passer automatiquement en `in_progress`.

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
├── config.toml           # Config notifications + takeover
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
│       └── policies-override.toml  # Overrides par projet (optionnel)
├── wiki/
│   ├── .pending/         # Propositions en attente
│   ├── decisions.md      # Décisions architecturales
│   └── patterns.md       # Patterns récurrents
└── reports/
```

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
oh claim transfer SRU-142 --to alice
# → Brief de reprise généré. oh takeover-brief show SRU-142
```

Si un ticket est inactif depuis plusieurs jours (configurable via `stale_days`
dans `config.toml`), le hub détecte le ticket comme "stale" et propose de
générer un brief lors du reclaim :

```bash
oh claim SRU-142
# → SRU-142 est assigné à benjamin depuis 5 jours sans activité.
# → Générer un brief de reprise et transférer ? [Y/n]
```

### Consulter et enrichir les briefs

```bash
# Afficher le brief d'un ticket
oh takeover-brief show SRU-142

# Lister tous les briefs du projet
oh takeover-brief list

# Enrichir un brief avec une analyse IA du code source
oh takeover-brief enrich SRU-142
```

L'enrichissement utilise un agent IA (`brief-enricher`) en mode headless pour :
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

Le mode parallèle permet de lancer plusieurs agents simultanément sur des
tickets différents. Chaque agent travaille dans un worktree Git isolé.

### Lancement

```bash
# Lancer 3 tickets en parallèle
oh start --parallel --tickets bd-42,bd-43,bd-44

# Avec un ticket prioritaire (merge en premier)
oh start --parallel --tickets bd-42,bd-43,bd-44 --priority bd-42

# Limiter le nombre de sessions
oh start --parallel --tickets bd-42,bd-43,bd-44 --max-sessions 2
```

### Interface de suivi

Un TUI plein écran affiche l'état de chaque session :
- Status en temps réel (pending / running / completed / failed)
- Fichiers modifiés par chaque session
- Conflits potentiels détectés

Navigation :
- `j/k` : naviguer entre les sessions
- `Enter` : s'attacher à une session (TUI opencode complet)
- `r` : rafraîchir
- `q` : quitter

### Merge

À la fin des sessions, le hub propose un merge séquentiel :
- **Tickets Beads** (locaux, préfixe `bd-`) : merge proposé avec validation humaine
- **Tickets externes** (GitLab/Jira) : pas de merge automatique, les branches restent prêtes pour MR/PR

### Configuration

Dans `config.toml` du repo team-state :

```toml
[parallel]
max_sessions = 3           # Max sessions simultanées
port_range_start = 4100    # Port de départ pour les serveurs opencode
auto_merge_beads = true    # Proposer le merge pour les tickets Beads

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

> **Note :** Les credentials de connexion (token, URL) sont réutilisés depuis `[mcp.gitlab]` / `[mcp.jira]` dans `hub.toml` — pas de duplication.

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
oh teams detach <team-id> --project <project-id>
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
