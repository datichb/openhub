# Guide de test exhaustif — Feature Team (`oh`)

> Guide complet pour tester de fond en comble la feature Team d'OpenHub.
> Comparaison CLI / TUI pour chaque action, schémas d'architecture, machines à états, et scénarios end-to-end.

---

## Table des matières

- [Architecture globale](#architecture-globale)
- [Pré-requis](#pré-requis)
- [1. Initialisation de l'équipe](#1-initialisation-de-léquipe)
- [2. Résolution de configuration](#2-résolution-de-configuration)
- [3. Gestion multi-équipes](#3-gestion-multi-équipes)
- [4. Configuration par projet](#4-configuration-par-projet)
- [5. Cycle de vie des Claims](#5-cycle-de-vie-des-claims)
- [6. Board Kanban](#6-board-kanban)
- [7. Statut d'équipe](#7-statut-déquipe)
- [8. Activité](#8-activité)
- [9. Policies](#9-policies--vérification-et-enforcement)
- [10. Configuration détaillée (Team Detail)](#10-configuration-détaillée-team-detail)
- [11. Synchronisation Tracker](#11-synchronisation-tracker-gitlab--jira)
- [12. Takeover Briefs](#12-takeover-briefs)
- [13. Wiki d'équipe](#13-wiki-déquipe)
- [14. Pattern Library](#14-pattern-library)
- [15. Sessions parallèles](#15-sessions-parallèles)
- [16. Notifications](#16-notifications)
- [17. Intégration MCP (Agents IA)](#17-intégration-mcp-agents-ia)
- [18. Credentials et sécurité](#18-credentials--sécurité)
- [19. Concurrence Git](#19-concurrence-git-withwritelock)
- [20. Scénarios end-to-end](#20-scénarios-end-to-end)

---

## Architecture globale

```
┌─────────────────────────────────────────────────────────────────────┐
│                           UTILISATEUR                                │
├──────────────────┬─────────────────────┬────────────────────────────┤
│   CLI (oh ...)   │    TUI (oh --tui)   │    Agents IA (MCP)         │
├──────────────────┴─────────────────────┴────────────────────────────┤
│                         CLI Layer (cobra)                            │
│  team.go │ teams.go │ team_board.go │ team_config.go │ team_sync_*  │
├─────────────────────────────────────────────────────────────────────┤
│                    TUI Layer (tview/tcell/huh)                       │
│  teams_view │ teamstatus_view │ teamboard_view │ team_detail_view   │
├─────────────────────────────────────────────────────────────────────┤
│                      Internal Packages                               │
│  teamstate/  │  config/  │  notify/  │  tracker/  │  mcp/team/      │
├─────────────────────────────────────────────────────────────────────┤
│                      Team-State Git Repo                             │
│  members.toml │ config.toml │ policies.toml │ projects/ │ wiki/     │
├─────────────────────────────────────────────────────────────────────┤
│              Services externes                                       │
│  GitLab/Jira │ Mattermost/Slack/Discord/Teams │ OS Keychain         │
└─────────────────────────────────────────────────────────────────────┘
```

### Structure du repo team-state

```
team-state/
├── config.toml                          # Configuration partagée
├── members.toml                         # Registre des membres
├── policies.toml                        # Règles d'équipe
├── projects/
│   └── <project>/
│       ├── claims/
│       │   └── <ticketID>.toml          # Réservation de ticket
│       ├── events/
│       │   └── YYYY-MM.jsonl            # Journal d'événements
│       ├── policies-override.toml       # Surcharges projet
│       └── takeover-briefs/
│           ├── <ticket>_<date>.toml     # Brief structuré
│           ├── <ticket>_<date>.md       # Brief lisible
│           └── <ticket>_<date>.enriched.md  # Brief enrichi IA
├── wiki/
│   ├── .pending/                        # Propositions en attente
│   ├── decisions.md
│   └── patterns.md
├── patterns/
│   ├── index.toml                       # Catalogue
│   └── <pattern-name>.md               # Contenu pattern
└── reports/
```

---

## Pré-requis

```
┌──────────────────────────────────────────────────────────┐
│  PRÉ-REQUIS                                              │
├──────────────────────────────────────────────────────────┤
│  1. oh installé          →  oh version                   │
│  2. oh init effectué     →  hub.toml existe              │
│  3. Repo Git team-state  →  vide ou avec README          │
│     (GitLab/GitHub)         Visibilité: Internal/Private │
│  4. Push access           →  SSH key ou token            │
│  5. ≥ 1 projet configuré →  oh project list              │
│  6. (Optionnel) Tracker   →  GitLab/Jira + token API    │
│  7. (Optionnel) Webhook   →  URL Mattermost/Slack/etc.  │
└──────────────────────────────────────────────────────────┘
```

| # | Élément | Commande de vérification | Résultat attendu |
|---|---------|--------------------------|------------------|
| 1 | CLI `oh` installée | `oh version` | Version affichée |
| 2 | Hub configuré | `ls ~/.oh/hub.toml` | Fichier existe |
| 3 | Repo Git créé | Vérifier sur GitLab/GitHub | Repo visible |
| 4 | Accès push | `git ls-remote <url>` | Pas d'erreur auth |
| 5 | Projet existant | `oh project list` | ≥ 1 projet listé |
| 6 | Token tracker | `echo $GITLAB_TOKEN` | Non vide (optionnel) |
| 7 | Webhook | curl test vers l'URL | HTTP 200 (optionnel) |

---

## 1. Initialisation de l'équipe

### Flux d'initialisation

```
                          oh team init
                               │
                    ┌──────────┴───────────┐
                    │  CLI (tview wizard)   │
                    │  5 étapes sidebar     │
                    └──────────┬───────────┘
                               │
                    ┌──────────┴───────────┐
                    │  teamInitCore()       │
                    └──────────┬───────────┘
                               │
              ┌────────────────┼────────────────┐
              ▼                ▼                 ▼
    repo.EnsureReady()   repo.InitStructure()  AddMember()
    (clone ou pull)      (dirs + .gitkeep)     (members.toml)
              │                │                 │
              └────────────────┼────────────────┘
                               ▼
                    config.Save() → hub.toml
                    [[teams]] mis à jour
```

### Comparaison CLI / TUI

| Aspect | CLI (`oh team init`) | TUI (vue Teams → `a` / omnibar `team init`) |
|--------|---------------------|----------------------------------------------|
| **Format** | Wizard tview plein écran, sidebar 5 étapes | Modal pas-à-pas, 3-4 étapes |
| **Étape 1** | URL repo team-state | URL repo team-state |
| **Étape 2** | Config globale (stale_days) | Auth HTTPS (si nécessaire) |
| **Étape 3** | Identité complète (5 champs) | Identité (member_id, display_name, role) |
| **Étape 4** | Notifications (webhook, channel, bot) | — (configurable après via detail) |
| **Étape 5** | Policies (checkboxes) | — (configurable après) |
| **Adaptatif** | Si repo non vide → propose "Modifier ?" | Toujours le même flow |
| **Résultat** | Commit + push + hub.toml | Idem via `teamInitCore()` |

### Champs d'identité

| Champ | Obligatoire | Description |
|-------|-------------|-------------|
| `member_id` | Oui | Clé unique dans members.toml |
| `display_name` | Non | Nom affiché dans les notifications |
| `gitlab_username` | Non | Pour le mapping tracker |
| `mattermost_username` | Non | Pour les mentions |
| `role` | Oui | `lead` / `dev` / `reviewer` |
| `default_mode` | Non | `manual` / `semi-auto` / `auto` (défaut: semi-auto) |

### Points de test

| # | Scénario | Input | Résultat attendu |
|---|----------|-------|------------------|
| 1 | Premier membre (repo vide) | URL repo vide + identité | `config.toml`, `members.toml`, structure créés, commit poussé |
| 2 | Membre suivant (repo existant) | Même URL + nouvel ID | `members.toml` enrichi, config non écrasée |
| 3 | HTTPS sans token | URL https:// | TUI: étape auth apparaît / CLI: timeout 30s, PullWarning |
| 4 | SSH key valide | URL git@... | Clone direct sans étape auth |
| 5 | Repo inaccessible | URL invalide | Erreur explicite, pas de crash |
| 6 | member_id déjà existant | ID déjà dans members.toml | ErrMemberExists, propose UpdateMember |
| 7 | hub.toml déjà team | Relancer init | Détecte config existante, propose modification |

---

## 2. Résolution de configuration

### Algorithme de résolution

```
                    ┌─────────────────┐
                    │  Projet demande │
                    │  team config    │
                    └────────┬────────┘
                             │
                   ┌─────────┴──────────┐
                   │  project.TeamID    │
                   │  défini ?          │
                   └─────────┬──────────┘
                     oui │         │ non
                         ▼         ▼
              ┌──────────────┐  ┌────────────────────┐
              │ Lookup dans  │  │ project.TeamConfig  │
              │ hub.Teams[]  │  │ (legacy mode)       │
              │ par ID       │  └─────────┬──────────┘
              └──────┬───────┘            │
                     │           ┌────────┼─────────┐
                     │           │        │         │
                     │      inherit   custom    disabled
                     │           │        │         │
                     ▼           ▼        ▼         ▼
              ┌──────────┐  hub team   custom    Enabled:
              │ ID trouvé │  config     repo     false
              │ + enabled │  direct    + merge
              └──────────┘
```

### Règles de fallback

```
┌────────────────────────────────────────────────────────────┐
│  Fallback MemberID:                                        │
│  project.MemberID || hub.Teams[x].MemberID                 │
│                                                            │
│  Fallback StatePath (auto-dérivé):                         │
│  ~/.oh/team-states/<host>/<repo-name>                      │
│                                                            │
│  Backward compat:                                          │
│  Si ~/.oh/team-states/<repo-name> existe (legacy)          │
│  ET ~/.oh/team-states/<host>/<repo-name> n'existe pas      │
│  → utilise le chemin legacy                                │
└────────────────────────────────────────────────────────────┘
```

### Points de test

| # | Scénario | Résultat attendu |
|---|----------|------------------|
| 1 | Mode inherit (défaut) | `project.TeamConfig = nil` → utilise hub `ActiveTeam()` |
| 2 | Mode custom, MemberID vide | Fallback sur `hub.Teams[0].MemberID` |
| 3 | Mode disabled | `ResolvedTeamConfig.Enabled = false` |
| 4 | TeamID vers team archivée | `Enabled: false` (graceful) |
| 5 | TeamID inconnu | `Enabled: false` (pas de panic) |
| 6 | StatePath auto-dérivé | URL `git@gitlab.com:acme/ts.git` → `~/.oh/team-states/gitlab.com/ts` |
| 7 | Legacy path existant | Utilise l'ancien chemin si le nouveau n'existe pas |

---

## 3. Gestion multi-équipes

### Schéma d'état

```
                    hub.toml
                    ┌────────────────────────────┐
                    │  [[teams]]                 │
                    │  ├── id = "acme"           │
                    │  │   enabled = true        │──► Projet A (TeamID="acme")
                    │  │   state_repo = ...      │──► Projet B (TeamID="acme")
                    │  │   member_id = "bd"      │
                    │  │                         │
                    │  ├── id = "beta"           │
                    │  │   enabled = false       │──► (archivée, ignorée)
                    │  │   state_repo = ...      │
                    │  │                         │
                    │  └── id = "gamma"          │
                    │      enabled = true        │──► Projet C (TeamID="gamma")
                    │      state_repo = ...      │
                    └────────────────────────────┘
```

### Comparaison CLI / TUI

| Action | CLI | TUI |
|--------|-----|-----|
| **Lister** | `oh teams list` → tableau (ID, nom, member, repo tronqué 40c, statut) | Vue `teams` : liste interactive, ✓ vert = actif |
| **Ajouter** | `oh teams add --repo <url> --member-id <id> [--id x] [--name y]` | Touche `a` → wizard 4 modals (URL, member-id, ID court, nom) |
| **Supprimer** | `oh teams remove <id> [-f]` (confirmation sauf `-f`) | Touche `d` → modal confirmation |
| **Détacher projet** | `oh teams detach <project-name>` | — |
| **Archiver** | `oh teams archive <id>` (met `enabled = false`) | — |
| **Restaurer** | `oh teams restore <id>` (remet `enabled = true`) | — |
| **Sync** | — | Touche `s` → git pull async |
| **Undo** | — | Touche `u` (pile 10 niveaux) |
| **Navigation** | — | `:` omnibar → `teams.add`, `teams.refresh`, `teams.board`, `teams.status`, `teams.sync.<id>` |

### Points de test

| # | Scénario | Résultat attendu |
|---|----------|------------------|
| 1 | `oh teams list` | Tableau avec: ID, nom, member_id, repo (tronqué), statut |
| 2 | `oh teams add` sans `--repo` | Erreur: flag required |
| 3 | `oh teams add` sans `--id` | ID dérivé automatiquement du nom du repo |
| 4 | `oh teams remove` sans `-f` | Demande confirmation interactive |
| 5 | `oh teams remove` avec projets liés | Tous les projets détachés |
| 6 | `oh teams archive` | `enabled = false` dans hub.toml |
| 7 | `oh teams restore` | `enabled = true` dans hub.toml |
| 8 | `oh teams detach` | Projet repasse en mode solo |
| 9 | TUI: add → delete → `u` | Undo restaure l'entrée |

---

## 4. Configuration par projet

### Modes disponibles

| Mode | Comportement | `.opencode/team.json` | MCP team |
|------|-------------|----------------------|----------|
| `inherit` | Utilise la team du hub | Créé avec config hub | Injecté |
| `custom` | Repo team-state dédié | Créé avec config custom | Injecté |
| `disabled` | Pas de team | Fichier supprimé | Non injecté |

### Comparaison CLI / TUI

| | CLI | TUI |
|---|-----|-----|
| **Configurer** | Proposé dans `oh project add` / `oh init` | Omnibar `team configure` → modal 3 choix |
| **Déployer** | `oh deploy` / `oh sync --all` | Automatique au deploy |
| **Vérifier** | Inspecter `.opencode/team.json` | Vue `team.detail` (touche `Enter`) |

### Points de test

| # | Scénario | Résultat attendu |
|---|----------|------------------|
| 1 | `oh deploy` avec team enabled | `opencode.json` contient serveur MCP `team` |
| 2 | `oh deploy` avec team disabled | Pas de serveur MCP `team` |
| 3 | Mode custom | Clone un repo différent du hub |
| 4 | Mode inherit, member_id absent | Provient du hub |
| 5 | Changement inherit → disabled + redeploy | MCP team retiré |

---

## 5. Cycle de vie des Claims

### Machine à états

```
                 oh claim --planned
                        │
                        ▼
              ┌─────────────────┐
              │     PLANNED     │ ◄─── tracker auto_plan
              │     (TODO)      │
              └────────┬────────┘
                       │ oh claim / board 'c'
                       ▼
              ┌─────────────────┐
         ┌───►│  IN_PROGRESS    │◄──────────────────┐
         │    │  (IN PROGRESS)  │                   │
         │    └───┬─────────┬───┘                   │
         │        │         │                       │
         │  board 's'  board 's'                    │
         │        │         │                       │
         │        ▼         ▼                       │
         │  ┌──────────┐  ┌──────────┐             │
         │  │  REVIEW  │  │ BLOCKED  │             │
         │  └────┬─────┘  └────┬─────┘             │
         │       │              │                   │
         │       │ board 's'    │ board 's'         │
         │       ▼              └───────────────────┘
         │  ┌──────────┐
         │  │   DONE   │
         │  └────┬─────┘
         │       │ reopen (tracker)
         └───────┘
```

### Transitions valides

| De | Vers | Déclencheur |
|----|------|-------------|
| `planned` | `in_progress` | `oh claim` / board `c` / board `s` |
| `in_progress` | `review` | board `s` / agent review.ready |
| `in_progress` | `blocked` | board `s` |
| `review` | `done` | board `s` / tracker issue fermée |
| `review` | `in_progress` | board `s` (retour) |
| `blocked` | `in_progress` | board `s` |
| `done` | `in_progress` | Tracker issue rouverte |

### Stockage d'un claim

```
team-state/projects/T-SRU/claims/SRU-142.toml
┌─────────────────────────────────────────────┐
│ claimed_by = "benjamin"                     │
│ claimed_at = 2026-07-15T09:30:00Z           │
│ worktree = "feat/SRU-142-user-auth"         │
│ status = "in_progress"                      │
│ last_activity = 2026-07-16T14:22:00Z        │
│ labels = ["agent-reviewed"]                 │
│ external_iid = 142                          │
└─────────────────────────────────────────────┘
```

### Labels spéciaux

| Label | Signification | Affichage board |
|-------|---------------|-----------------|
| `agent-reviewed` | Revu par un agent IA | `[AI]` |
| `needs-human-review` | Nécessite review humaine | `[!]` |
| `hub:done` | Marqué terminé par le hub | — (poussé vers tracker) |

### Comparaison CLI / Board

| Action | CLI | Board (CLI `oh team board` / TUI `team.board`) |
|--------|-----|------------------------------------------------|
| Réclamer | `oh claim SRU-142` | Touche `c` sur le ticket |
| Planifier | `oh claim SRU-142 --planned` | — |
| Avec branche | `oh claim SRU-142 --worktree feat/...` | — |
| Libérer | `oh release SRU-142` | Touche `x` |
| Transférer | `oh claim transfer SRU-142 --to alice` | Touche `t` → modal membre |
| Changer statut | — | Touche `s` → modal 5 choix |

### Événements générés

| Action | Événement | Données |
|--------|-----------|---------|
| Claim | `claim.taken` | `{actor, ticket, project}` |
| Double-claim | `claim.conflict` | `{actor, ticket, owner}` |
| Transfer | `claim.transferred` | `{actor, ticket, data:{to}}` |
| Release | `claim.released` | `{actor, ticket}` |

### Points de test

| # | Scénario | Résultat attendu |
|---|----------|------------------|
| 1 | Claim standard | Fichier TOML créé, status=in_progress, event loggé |
| 2 | Claim --planned | status=planned |
| 3 | Double-claim | ErrClaimExists + event claim.conflict |
| 4 | Transfer | ClaimedBy changé + brief auto-généré |
| 5 | Release | Fichier claim supprimé |
| 6 | Max WIP policy | Claim refusé si limite atteinte |
| 7 | Transition invalide (planned→done) | ErrInvalidTransition |
| 8 | Claim avec worktree | Champ worktree renseigné |

---

## 6. Board Kanban

### Layout

```
┌──────────────────────────────────────────────────────────────────────────┐
│  oh team board                                            [FILTRE] r:5s  │
├──────────────┬──────────────┬──────────────┬──────────────┬─────────────┤
│    TODO      │ IN PROGRESS  │   REVIEW     │   BLOCKED    │    DONE     │
├──────────────┼──────────────┼──────────────┼──────────────┼─────────────┤
│              │              │              │              │             │
│ SRU-145      │▶SRU-142     │ SRU-139      │              │ SRU-138     │
│ @alice       │ @benjamin   │ @charlie [AI]│              │ @alice      │
│              │ [!]         │              │              │             │
│ SRU-146      │ SRU-143     │              │              │ SRU-137     │
│ @bob         │ @alice      │              │              │ @bob        │
│              │              │              │              │             │
├──────────────┴──────────────┴──────────────┴──────────────┴─────────────┤
│  h/l:colonnes  j/k:items  c:claim  x:release  t:transfer  s:status     │
│  /:search  f:filter  r:refresh  q:quit                                  │
└──────────────────────────────────────────────────────────────────────────┘

Légende:
  [AI] = agent-reviewed       ▶ = sélection courante
  [!]  = needs-human-review   @nom = assigné
```

### Raccourcis clavier

| Touche | Action | Contexte |
|--------|--------|----------|
| `h` / `←` | Colonne précédente | Navigation |
| `l` / `→` | Colonne suivante | Navigation |
| `j` / `↓` | Item suivant dans la colonne | Navigation |
| `k` / `↑` | Item précédent dans la colonne | Navigation |
| `c` | Réclamer (claim) le ticket sélectionné | Action |
| `x` | Libérer (release) le ticket | Action |
| `t` | Transférer → modal de sélection du membre | Action |
| `s` | Changer le statut → modal 5 choix | Action |
| `r` | Rafraîchir (git pull + reload) | Maintenance |
| `/` | Recherche texte (filtre par ID, titre, assigné) | Filtrage |
| `f` | Menu filtres (mes tickets, par assigné, par label, effacer) | Filtrage |
| `Esc` | Effacer les filtres actifs | Filtrage |
| `q` | Quitter (CLI board uniquement) | Navigation |

### Système de filtrage

```
                 ┌─────────────┐
                 │  Tous les   │
                 │  tickets    │
                 └──────┬──────┘
                        │
           ┌────────────┼────────────┐
           │            │            │
     ┌─────┴─────┐ ┌───┴───┐ ┌─────┴─────┐
     │ Texte (/) │ │ Assi- │ │ Label (f) │
     │ ID/titre/ │ │ gné   │ │ exact     │
     │ assigné   │ │ (f)   │ │ match     │
     └─────┬─────┘ └───┬───┘ └─────┬─────┘
           │            │            │
           └────────────┼────────────┘
                        │ AND (combinés)
                        ▼
                 ┌─────────────┐
                 │  Tickets    │
                 │  affichés   │
                 └─────────────┘

   [Esc] efface tous les filtres actifs
   Indicateur [FILTRE] jaune dans la barre hints
```

### Auto-refresh et concurrence

```
   ┌──────────────────────────────────────────────┐
   │  Ticker (5s par défaut, configurable)        │
   │                                              │
   │  tick → actionInProgress ?                   │
   │          ├── oui → skip (pas de double exec) │
   │          └── non → async {                   │
   │                      SyncFunc() (git pull)   │
   │                      LoadClaims()            │
   │                      RenderBoard()           │
   │                    }                         │
   │                                              │
   │  Action utilisateur (c/x/t/s) :              │
   │    1. actionInProgress = true                │
   │    2. Exécution (commitAndPush)              │
   │    3. Reload board                           │
   │    4. actionInProgress = false               │
   └──────────────────────────────────────────────┘
```

### Points de test

| # | Scénario | Résultat attendu |
|---|----------|------------------|
| 1 | Board vide | Message approprié, pas de crash |
| 2 | Claim (`c`) | Ticket passe en IN PROGRESS, assigné courant |
| 3 | Release (`x`) | Ticket disparaît ou repasse en planned |
| 4 | Transfer (`t`) | Modal listant les autres membres, confirmation |
| 5 | Change status (`s`) | Modal 5 choix, ticket déplacé de colonne |
| 6 | Filtre `/` | Seuls les tickets matchant visibles, indicateur `[FILTRE]` |
| 7 | Filtre `f` > "Mes tickets" | Seuls mes tickets |
| 8 | `--watch` (CLI) | Board se met à jour automatiquement (5s) |
| 9 | Labels affichés | `[AI]` et `[!]` correctement rendus |
| 10 | Double-press action | Debouncing: pas d'exécution simultanée |
| 11 | Filtre combiné (`/` + `f`) | Intersection (AND) des deux filtres |

---

## 7. Statut d'équipe

### Comparaison CLI / TUI

| | CLI | TUI |
|---|-----|-----|
| **Commande** | `oh team status [--detail]` | Vue `team.status` (omnibar `teams.status`) |
| **Rafraîchir** | Relancer la commande | Touche `r` (async pull + re-render) |
| **Retour** | — | `Esc` |

### Informations affichées

```
┌──────────────────────────────────────────────────────┐
│  Statut de l'équipe                                  │
│  Repo: git@gitlab.com:acme/team-state.git            │
│  Membre: benjamin                                    │
├──────────────────────────────────────────────────────┤
│  --- Membres (3) ---                                 │
│  @benjamin [2 tickets] SRU-142 (in_progress),        │
│                         SRU-145 (review)     ← accent│
│  @alice    [1 ticket]  SRU-143 (in_progress)         │
│  @charlie  [0 tickets]                               │
├──────────────────────────────────────────────────────┤
│  --- Résumé ---                                      │
│  planned: 3 | in_progress: 2 | review: 1            │
│  blocked: 0 | done: 5                               │
├──────────────────────────────────────────────────────┤
│  --- Activité récente ---                            │
│  il y a 5m   benjamin  a terminé     SRU-138        │
│  il y a 1h   alice     a pris        SRU-143        │
│  hier        charlie   review prête  SRU-139        │
│  il y a 2j   benjamin  a transféré   SRU-140        │
│  il y a 3j   alice     a libéré      SRU-137        │
└──────────────────────────────────────────────────────┘
```

### Points de test

| # | Scénario | Résultat attendu |
|---|----------|------------------|
| 1 | Sans team configurée | Message "Équipe non configurée pour ce projet" |
| 2 | Repo non cloné | Message "Repo non cloné. Exécutez 'oh team init'" |
| 3 | `--detail` | Montre les sous-beads avec progression |
| 4 | Timestamps relatifs | "il y a 5m", "hier", etc. correctement calculés |
| 5 | Git pull auto | Données fraîches à l'ouverture (TUI) |
| 6 | Membre courant | Highlight avec couleur accent |

---

## 8. Activité

### Comparaison CLI / TUI

| | CLI | TUI |
|---|-----|-----|
| **Commande** | `oh team activity [flags]` | Intégré dans `team.status` (5 derniers) |
| **Navigation** | — | Omnibar `teams.activity` |

### Flags CLI

| Flag | Description | Exemple |
|------|-------------|---------|
| `--today` | Événements du jour uniquement | `oh team activity --today` |
| `--week` | 7 derniers jours | `oh team activity --week` |
| `--member <id>` | Filtrer par membre | `oh team activity --member alice` |
| `--project <name>` | Filtrer par projet | `oh team activity --project T-SRU` |
| `--limit <n>` | Maximum d'événements (défaut: 20) | `oh team activity --limit 5` |

### Types d'événements

| Type | Description | Format notification |
|------|-------------|---------------------|
| `session.complete` | Session terminée | `[project] actor a terminé ticket (duration)` |
| `review.ready` | Prêt pour review | `[project] Review prête pour ticket — mr_url` |
| `audit.finding` | Résultat d'audit | `[project] Audit domain: N finding(s)` |
| `claim.taken` | Ticket réclamé | `[project] actor a pris ticket` |
| `claim.conflict` | Conflit de claim | `[project] ⚠ Conflit sur ticket (déjà pris par owner)` |
| `claim.transferred` | Ticket transféré | `[project] ticket transféré de actor à target` |
| `claim.released` | Ticket libéré | `[project] actor a libéré ticket` |
| `wiki.proposal` | Proposition wiki | `[Équipe] Proposition wiki (page) par actor` |
| `wiki.accepted` | Wiki acceptée | `[Équipe] Wiki page mis à jour par actor` |
| `wiki.rejected` | Wiki rejetée | — |

### Points de test

| # | Scénario | Résultat attendu |
|---|----------|------------------|
| 1 | Sans flag | 20 derniers événements |
| 2 | `--today` | Filtre sur la date du jour |
| 3 | `--week` | Filtre sur 7 jours |
| 4 | `--member alice` | Seuls les événements d'alice |
| 5 | `--project T-SRU` | Seuls les événements du projet |
| 6 | `--limit 5` | Exactement 5 résultats max |
| 7 | Combinaison | `--today --member alice` fonctionne |

---

## 9. Policies — Vérification et enforcement

### Schéma d'évaluation

```
              ┌──────────────────┐
              │  PolicyContext   │
              │  ├── BranchName  │
              │  ├── CommitMsg   │
              │  ├── DiffLines   │
              │  ├── ActiveClaims│
              │  ├── HasReview   │
              │  ├── HasTests    │
              │  └── HasCoverage │
              └────────┬─────────┘
                       │
                       ▼
   ┌─────────────────────────────────────┐
   │  LoadPolicies(project)              │
   │  = policies.toml (global)           │
   │  + projects/<p>/policies-override   │
   │                                     │
   │  RÈGLE: override ne peut que        │
   │  rendre plus strict                 │
   │  (warn→refuse OK,                   │
   │   refuse→warn IMPOSSIBLE)           │
   └────────────────┬────────────────────┘
                    │
        ┌───────────┼───────────┬───────────────┐
        ▼           ▼           ▼               ▼
   ┌─────────┐ ┌─────────┐ ┌──────────┐ ┌────────────────┐
   │  regex  │ │ boolean │ │  limit   │ │ forbidden_pat  │
   │         │ │         │ │          │ │                │
   │ branch? │ │ review? │ │ max_wip? │ │ diff_only?     │
   │ commit? │ │ tests?  │ │          │ │ modified_files?│
   │         │ │ cover?  │ │          │ │ all_files?     │
   └────┬────┘ └────┬────┘ └────┬─────┘ └───────┬────────┘
        │           │            │               │
        └───────────┴────────────┴───────────────┘
                              │
                              ▼
                    ┌──────────────────┐
                    │  []PolicyResult  │
                    │  ├── Passed bool │
                    │  ├── Enforcement │
                    │  └── Message     │
                    └────────┬─────────┘
                             │
                   ┌─────────┴─────────┐
                   ▼                   ▼
             ┌──────────┐       ┌──────────┐
             │  REFUSE  │       │   WARN   │
             │  → block │       │  → log   │
             │  action  │       │  continu │
             └──────────┘       └──────────┘
```

### Types de policies

| Type | Paramètres | Exemple | Évaluation |
|------|-----------|---------|------------|
| `regex` | `rule` (pattern) | `^(feat\|fix\|chore)/[A-Z]+-\d+` | Match branch/commit/files |
| `boolean` | `enabled` | Review required | Vérifie HasReview/HasTests/HasCoverage |
| `limit` | `max`, `unit` | Max WIP = 2 | Compare ActiveClaims >= Max |
| `forbidden_pattern` | `patterns[]`, `scope` | Pas de `console.log` | String-contains sur DiffLines |

### Scopes pour `forbidden_pattern`

| Scope | Cible |
|-------|-------|
| `diff_only` | Lignes ajoutées dans le diff uniquement |
| `modified_files` | Contenu complet des fichiers modifiés |
| `all_files` | Tous les fichiers du projet |
| `per_feature_branch` | Commits de la feature branch |

### Double enforcement (CLI + Agents)

```
   ┌──────────────────────────────────────────────────────────────┐
   │                                                              │
   │  CLI (hard enforcement)              Agent (soft)            │
   │  ─────────────────────               ─────────────           │
   │  oh claim → max_wip                  Avant branch:           │
   │  oh start → branch_naming              branch_naming         │
   │  oh release → review_required        Avant commit:           │
   │             → tests_required           commit_format         │
   │                                                              │
   │  Action: BLOQUE si refuse            Action: INFORME         │
   │          WARN si warn                 (ne bloque pas)        │
   │                                                              │
   └──────────────────────────────────────────────────────────────┘
```

### Comparaison CLI / TUI

| Action | CLI | TUI |
|--------|-----|-----|
| Lister policies | `oh policies list [--project P]` | — |
| Vérifier | `oh policies check --branch <n> --commit "<m>"` | Automatique (agents) |
| Ajouter | `oh policies add` (interactif) | — |
| Voir statut config | `oh team config status` | Vue `team.detail` |

### Points de test

| # | Scénario | Résultat attendu |
|---|----------|------------------|
| 1 | `oh policies list` | Toutes les policies avec type et enforcement |
| 2 | Branch invalide + policy refuse | Action bloquée |
| 3 | Branch invalide + policy warn | Warning affiché, action continue |
| 4 | Commit invalide | Rejet ou warning selon enforcement |
| 5 | Max WIP atteint | Claim refusé |
| 6 | Override projet (warn→refuse) | Accepté, policy plus stricte |
| 7 | Override projet (refuse→warn) | Rejeté, impossible d'assouplir |
| 8 | `forbidden_pattern` dans diff | Détecte le pattern, signale |
| 9 | Policy disabled | Toujours Passed=true |

---

## 10. Configuration détaillée (Team Detail)

### Layout TUI

```
┌──────────────────────────────────────────────────────────────────┐
│  Configuration équipe: acme                          [r]efresh   │
├──────────────────────────────────────────────────────────────────┤
│                                                                  │
│  ═══ MCP Services ═══                                            │
│                                                                  │
│  --- gitlab ---                                                  │
│  [✓] Enabled                              (enforced par l'equipe)│
│  URL: https://gitlab.company.com          (enforced par l'equipe)│
│  Token: ●●●●●●●● ✓                       [local]                │
│  [✓] Write enabled                       [local]                │
│                                                                  │
│  --- jira ---                                                    │
│  [ ] Enabled                                                     │
│  URL:                                                            │
│                                                                  │
│  ═══ Tracker ═══                                                 │
│                                                                  │
│  Type: [GitLab ▾]                                                │
│  [✓] Enabled                                                     │
│  [✓] Auto-sync (interval: 5 min)                                │
│  [✓] Push labels                                                 │
│  [✓] Auto-plan assigned (max: 5/membre)                         │
│                                                                  │
│  ═══ Mappings projets ═══                                        │
│  T-SRU → 42                                   [a]jouter [d]sup   │
│  T-BILLING → 87                                                  │
│                                                                  │
│  ═══ Notifications ═══                                           │
│  Type: [Mattermost ▾]                                            │
│  Webhook: https://mm.company.com/hooks/...                       │
│  Channel: #dev-ai                                                │
│  Bot: OpenHub                                                    │
│                                                                  │
│  ═══ Collaboration ═══                                           │
│  Max sessions: 3                                                 │
│  Stale days: 3                                                   │
│  Done retention: 7 jours                                         │
│                                                                  │
│  ═══ Modèles (recommandations) ═══                               │
│  Default: claude-sonnet-4-20250514                               │
│  --- families ---                                                │
│  anthropic → claude-sonnet-4-20250514                            │
│  --- agents ---                                                  │
│  orchestrator-dev → claude-sonnet-4-20250514                     │
│                                                                  │
│  ═══ Overrides locaux ═══                                        │
│  Tracker enabled: [hériter]  (cycle: hériter/oui/non)           │
│  Auto-sync: [hériter]                                            │
│  Push labels: [hériter]                                          │
│                                                                  │
├──────────────────────────────────────────────────────────────────┤
│  [w]save  [s]sync-tracker  [t]test  [a]add  [d]del  [u]reload   │
└──────────────────────────────────────────────────────────────────┘
```

### Raccourcis TUI detail

| Touche | Action |
|--------|--------|
| `j`/`k` | Naviguer entre champs (skip headers/spacers) |
| `Space` | Toggle bool / cycle tri-state (hériter→oui→non→hériter) |
| `Enter` | Éditer champ (modal input/select/password) |
| `w` | Sauvegarder (2 passes: team commit+push + local hub.toml) |
| `s` | Sync tracker (async pull depuis tracker externe) |
| `t` | Tester connexion tracker (affiche username si OK) |
| `a` | Ajouter entrée dynamique (mappings, families, agents) |
| `d` | Supprimer entrée dynamique |
| `u` | Recharger (undo modifications non sauvegardées) |
| `r` | Refresh (git pull + reload données) |

### Types de champs

| Kind | Comportement |
|------|-------------|
| `bool` | Space toggle true/false, checkmark vert / croix rouge |
| `tri-state` | Cycle: hériter → oui → non → hériter |
| `string` | Enter ouvre modal input |
| `select` | Enter ouvre modal avec options prédéfinies |
| `password` | Enter ouvre modal password, affichage masqué + indicateur |

### Comparaison CLI / TUI

| | CLI | TUI |
|---|-----|-----|
| **Accéder** | `oh team config` (wizard interactif) | `Enter` sur une team → vue `team.detail` |
| **Voir statut** | `oh team config status` | Intégré dans la vue |
| **Sauvegarder** | Automatique en fin de wizard | Touche `w` (explicite) |
| **Granularité** | Wizard complet | Champ par champ |

### Points de test

| # | Scénario | Résultat attendu |
|---|----------|------------------|
| 1 | Champs enforced | Non éditables, grisés avec "(enforced)" |
| 2 | Token masqué | Affiche `●●●●` + indicateur ✓/✗ |
| 3 | Tri-state cycle | hériter → oui → non → hériter correctement |
| 4 | `w` save | 2 passes: team (commit+push) + local (hub.toml) |
| 5 | Quitter sans sauver | Warning toast affiché |
| 6 | `s` sync tracker | Modal résumé (created/updated/pushed) |
| 7 | `t` test connexion | Username affiché si succès, erreur sinon |
| 8 | `a`/`d` dynamique | Ajout/suppression de mappings fonctionnel |
| 9 | `oh team config status` | Affiche config effective (merge team + local) |

---

## 11. Synchronisation Tracker (GitLab / Jira)

### Algorithme détaillé

```
   oh team sync-tracker
          │
          ▼
   ┌────────────────────────────────────────────────┐
   │  1. Resolve team config (hub + project)        │
   │  2. Load config.toml [tracker]                 │
   │  3. Merge: team config + local overrides       │
   │  4. Resolve credentials (env var / keychain)   │
   │  5. Create tracker client (GitLab / Jira)      │
   └───────────────────────┬────────────────────────┘
                           │
                           ▼
   ┌────────────────────────────────────────────────┐
   │  Pour chaque projet dans [tracker.projects]:   │
   │                                                │
   │  ┌──────────────────────────────────────────┐  │
   │  │  PULL (tracker → claims):                │  │
   │  │  • Fetch issue par external_iid          │  │
   │  │  • Issue fermée → claim = "done"         │  │
   │  │  • Issue rouverte → claim = "in_progress"│  │
   │  │  • Labels mirrorés (sauf hub:*)          │  │
   │  │  • Warning si assignee ≠ claim owner     │  │
   │  └──────────────────────────────────────────┘  │
   │                                                │
   │  ┌──────────────────────────────────────────┐  │
   │  │  PUSH (claims → tracker):                │  │
   │  │  Si push_labels enabled:                 │  │
   │  │  • Push "agent-reviewed"                 │  │
   │  │  • Push "hub:done"                       │  │
   │  └──────────────────────────────────────────┘  │
   │                                                │
   │  ┌──────────────────────────────────────────┐  │
   │  │  AUTO-PLAN:                              │  │
   │  │  Si auto_plan_assigned enabled:          │  │
   │  │  • Fetch issues assignées par membre     │  │
   │  │  • Créer claim "planned" si absent       │  │
   │  │  • Respecter max_auto_plan_per_member    │  │
   │  └──────────────────────────────────────────┘  │
   └───────────────────────┬────────────────────────┘
                           │
                           ▼
   ┌────────────────────────────────────────────────┐
   │  Résultat:                                     │
   │  • Claims created: N                           │
   │  • Claims updated: N                           │
   │  • Labels pushed: N                            │
   │  • Warnings: [...]                             │
   │  • Errors: [...]                               │
   └────────────────────────────────────────────────┘
```

### Gestion d'erreurs

| Erreur | Comportement |
|--------|-------------|
| Token invalide | 3 échecs consécutifs → `ShouldAutoSync() = false` |
| Rate limited | Stop tous les projets immédiatement |
| Issue not found | Ignore (supprimée sur tracker) |
| Erreur sur un projet | Log + continue avec le projet suivant |

### Comparaison CLI / TUI

| | CLI | TUI |
|---|-----|-----|
| **Lancer** | `oh team sync-tracker` | Touche `s` dans `team.detail` ou omnibar |
| **Résultat** | Sortie texte dans terminal | Modal résumé |
| **Token manquant** | Erreur + message explicite | Wizard propose configuration token |
| **Test connexion** | — | Touche `t` (affiche username si OK) |

### Points de test

| # | Scénario | Résultat attendu |
|---|----------|------------------|
| 1 | Sans config tracker | Erreur explicite |
| 2 | Sans token | Wizard propose configuration / erreur CLI |
| 3 | Sync réussie | Résumé avec compteurs |
| 4 | Issue fermée sur tracker | Claim passe à `done` |
| 5 | Issue rouverte | Claim passe à `in_progress` |
| 6 | `auto_plan_assigned` | Nouvelles claims `planned` créées |
| 7 | `push_labels` | Labels visibles sur GitLab/Jira |
| 8 | `max_auto_plan_per_member` | Pas plus de N claims auto-créées |
| 9 | Mapping projets | Seuls les projets configurés synchronisés |

---

## 12. Takeover Briefs

### Flux de génération

```
   oh claim transfer SRU-142 --to alice
          │
          ├── 1. TransferClaim() → claim.ClaimedBy = "alice"
          ├── 2. AppendEvent(claim.transferred)
          └── 3. GenerateRawBrief()
                    │
                    ├── Collecte events ticket (claim.*, session.*)
                    ├── Calcule SessionsCount, First/Last session
                    ├── Extracte Git info (branch, commits, files)
                    └── SaveBrief()
                          │
                          ├── SRU-142_2026-07-16.toml (structuré)
                          └── SRU-142_2026-07-16.md   (lisible)

   oh takeover-brief enrich SRU-142
          │
          └── Agent brief-enricher (headless)
                    │
                    ├── Analyse fichiers modifiés
                    ├── Examine décisions architecturales
                    ├── Identifie questions ouvertes
                    └── Écrit SRU-142_2026-07-16.enriched.md

   Priorité de lecture: .enriched.md > .md > .toml
```

### Structure du brief

```
team-state/projects/T-SRU/takeover-briefs/SRU-142_2026-07-16.toml
┌──────────────────────────────────────────────┐
│ [meta]                                       │
│ ticket_id = "SRU-142"                        │
│ project = "T-SRU"                            │
│ transferred_from = "benjamin"                │
│ transferred_to = "alice"                     │
│ transfer_date = 2026-07-16T10:00:00Z         │
│ reason = "transfer"  # ou "stale"            │
│                                              │
│ [activity]                                   │
│ sessions_count = 4                           │
│ first_session = 2026-07-12T09:00:00Z         │
│ last_session = 2026-07-15T16:30:00Z          │
│ total_duration_minutes = 240                 │
│                                              │
│ [git]                                        │
│ branch = "feat/SRU-142-user-auth"            │
│ commits_count = 12                           │
│ last_commit_message = "feat: add JWT valid"  │
│ last_commit_date = 2026-07-15T16:25:00Z      │
│                                              │
│ [[git.files_modified]]                       │
│ path = "internal/auth/jwt.go"                │
│ additions = 85                               │
│ deletions = 12                               │
│                                              │
│ [[git.files_created]]                        │
│ path = "internal/auth/jwt_test.go"           │
│ additions = 120                              │
│ deletions = 0                                │
│                                              │
│ [[events]]                                   │
│ timestamp = 2026-07-15T16:30:00Z             │
│ type = "session.complete"                    │
│ summary = "Implémenté validation JWT"        │
│                                              │
│ [[events]]                                   │
│ timestamp = 2026-07-14T11:00:00Z             │
│ type = "session.complete"                    │
│ summary = "Ajouté middleware auth"           │
└──────────────────────────────────────────────┘
```

### Détection stale

```
   IsStale(claim, staleDays) :
     lastActivity = claim.LastActivity || claim.ClaimedAt
     return time.Since(lastActivity) > staleDays * 24h

   Défaut: staleDays = 3 (configurable dans config.toml [takeover])
```

### Comparaison CLI / TUI

| Action | CLI | TUI |
|--------|-----|-----|
| Lister | `oh takeover-brief list` | — |
| Consulter | `oh takeover-brief show <ticket>` | — |
| Enrichir (IA) | `oh takeover-brief enrich <ticket>` | Action `runTakeoverEnrich()` |

### Points de test

| # | Scénario | Résultat attendu |
|---|----------|------------------|
| 1 | Transfer claim | Brief auto-généré (.toml + .md) |
| 2 | Brief contient contexte | Sessions, commits, fichiers, events |
| 3 | `oh takeover-brief enrich` | `.enriched.md` créé par agent |
| 4 | Lecture via MCP | `team_takeover_brief` retourne le brief |
| 5 | Ticket stale (> stale_days) | Proposition de générer un brief |
| 6 | Priorité lecture | enriched > md > toml |
| 7 | Brief pour ticket inexistant | ErrBriefNotFound |

---

## 13. Wiki d'équipe

### Flux de contribution

```
   Agent documentarian
          │
          ▼
   team_wiki_write(page, content, confidence, project)
          │
          ├── Validation (taille, format)
          ├── WikiCreateProposal() → wiki/.pending/<page>.md
          ├── AppendEvent(wiki.proposal)
          └── Notification envoyée
                    │
                    ▼
   oh team wiki review
          │
          ├── Lister .pending/
          ├── Humain accepte/rejette
          │     ├── Accept → WikiAcceptProposal() → wiki/<page>.md
          │     │            AppendEvent(wiki.accepted)
          │     └── Reject → WikiRejectProposal() → supprimé
          │                  AppendEvent(wiki.rejected)
          └── Commit + push
```

### Comparaison CLI / TUI

| Action | CLI | TUI |
|--------|-----|-----|
| Lister pages | `oh team wiki list` | — |
| Lire | `oh team wiki read <page>` | — |
| Review | `oh team wiki review` | — |
| Écriture | Via agent `documentarian` uniquement | — |

### Points de test

| # | Scénario | Résultat attendu |
|---|----------|------------------|
| 1 | `wiki list` | Pages existantes listées |
| 2 | `wiki read decisions` | Contenu markdown affiché |
| 3 | Agent propose page | Apparaît dans `.pending/` |
| 4 | `wiki review` + accept | Page déplacée vers `wiki/` |
| 5 | `wiki review` + reject | Page supprimée |
| 6 | Taille max dépassée | ErrProposalTooLarge |
| 7 | Agents lisent wiki | MCP `team_wiki_read` fonctionne |

---

## 14. Pattern Library

### Flux

```
   Agent (planner/pathfinder) → planning réussi
          │
          ▼
   team_patterns_propose(name, tags, complexity, content)
          │
          ├── CreatePattern() → patterns/<name>.md + index.toml (validated=false)
          └── Commit + push

   oh patterns validate <name>
          │
          └── ValidatePattern() → index.toml: validated=true
```

### Commandes CLI

| Commande | Description |
|----------|-------------|
| `oh patterns list` | Lister tous les patterns (filtrable par tags) |
| `oh patterns show <name>` | Afficher le contenu d'un pattern |
| `oh patterns add` | Ajouter manuellement un pattern |
| `oh patterns validate <name>` | Valider un pattern proposé par un agent |
| `oh patterns remove <name>` | Supprimer un pattern |

### Points de test

| # | Scénario | Résultat attendu |
|---|----------|------------------|
| 1 | Pattern proposé par agent | `validated = false` dans index |
| 2 | `oh patterns validate` | Passe à `validated = true` |
| 3 | Stockage | `patterns/<name>.md` + entrée dans `index.toml` |
| 4 | Filtrage par tags | `team_patterns_list(tags: ["api"])` retourne les matchs |
| 5 | `oh patterns remove` | Fichier et entrée supprimés |

---

## 15. Sessions parallèles

### Commande

```bash
oh start --parallel --tickets bd-42,bd-43,bd-44 [--priority] [--max-sessions]
```

### Architecture

```
   oh start --parallel --tickets bd-42,bd-43,bd-44
          │
          ├── Claim bd-42 (worktree: feat/bd-42)
          ├── Claim bd-43 (worktree: feat/bd-43)
          └── Claim bd-44 (worktree: feat/bd-44)
                    │
          ┌────────┼────────┐
          ▼        ▼        ▼
     ┌────────┐┌────────┐┌────────┐
     │Session ││Session ││Session │
     │  bd-42 ││  bd-43 ││  bd-44 │
     │worktree││worktree││worktree│
     │ isolé  ││ isolé  ││ isolé  │
     └────┬───┘└────┬───┘└────┬───┘
          │         │         │
          ▼         ▼         ▼
     TUI plein écran: statut temps réel
     ├── Fichiers modifiés par session
     ├── Conflits potentiels détectés
     └── Navigation: j/k, Enter, r, q
```

### Configuration (config.toml)

```toml
[parallel]
max_sessions = 3
port_range_start = 4100
auto_merge_beads = true
```

### Points de test

| # | Scénario | Résultat attendu |
|---|----------|------------------|
| 1 | Lancement parallèle | Chaque ticket dans un worktree isolé |
| 2 | TUI temps réel | Statut, fichiers modifiés visibles |
| 3 | `max_sessions` respecté | Pas plus de N sessions simultanées |
| 4 | Conflits détectés | Avertissement si mêmes fichiers touchés |
| 5 | Navigation | `j/k` (sessions), `Enter` (attacher), `q` (quitter) |

---

## 16. Notifications

### Flux de dispatch

```
   ┌──────────────┐     ┌──────────────┐     ┌──────────────────┐
   │  Événement   │     │  AppendEvent │     │  notify.Dispatch │
   │  se produit  │────►│  (JSONL log) │     │  (HTTP POST)     │
   └──────────────┘     └──────────────┘     └────────┬─────────┘
                                                      │
                              ┌────────────────────────┘
                              │ (seulement pour certains events)
                              ▼
   ┌─────────────────────────────────────────────────────────────┐
   │  Events qui DÉCLENCHENT une notification:                   │
   │  • wiki.proposal   (MCP team_wiki_write)                    │
   │  • review.ready    (audit/review terminé)                   │
   │  • custom          (MCP team_notify)                        │
   │                                                             │
   │  Events LOGGÉS mais PAS notifiés automatiquement:           │
   │  • claim.taken                                              │
   │  • claim.released                                           │
   │  • claim.transferred                                        │
   │  • claim.conflict                                           │
   │  • session.complete                                         │
   └─────────────────────────────────────────────────────────────┘
```

### Plateformes supportées

| Plateforme | Payload | Champs config |
|------------|---------|---------------|
| **Mattermost** | `{"channel", "username", "text"}` | `webhook_url`, `channel`, `bot_name` |
| **Slack** | `{"text", "username"}` | `webhook_url`, `bot_name` |
| **Discord** | `{"content", "username"}` | `webhook_url`, `bot_name` |
| **Microsoft Teams** | MessageCard `{"@type", "text"}` | `webhook_url` |

Timeout HTTP: 10 secondes pour tous les clients.

### Architecture multi-destination

```
   config.toml [notification]
   ┌────────────────────────────────┐
   │  enabled = true                │
   │  bot_name = "OpenHub"          │
   │                                │
   │  [[notification.destinations]] │
   │  type = "mattermost"          │──► POST webhook
   │  webhook_url = "https://..."   │
   │  channel = "#dev-ai"          │
   │                                │
   │  [[notification.destinations]] │
   │  type = "slack"               │──► POST webhook
   │  webhook_url = "https://..."   │
   │                                │
   │  [[notification.destinations]] │
   │  type = "discord"             │──► POST webhook
   │  webhook_url = "https://..."   │
   └────────────────────────────────┘
```

### Points de test

| # | Scénario | Résultat attendu |
|---|----------|------------------|
| 1 | wiki.proposal | Notification envoyée sur toutes les destinations |
| 2 | review.ready | Notification formatée avec lien MR |
| 3 | team_notify (custom) | Message brut envoyé tel quel |
| 4 | `enabled = false` | Aucun HTTP POST |
| 5 | Webhook invalide (404/500) | Erreur loggée, pas de crash |
| 6 | Multi-destination | Message reçu sur chaque plateforme |
| 7 | Bot name correct | Nom affiché = config bot_name |
| 8 | Channel correct (Mattermost) | Message dans le bon canal |

---

## 17. Intégration MCP (Agents IA)

### Schéma d'accès

```
   ┌───────────────────────────────────────────────────────────────┐
   │  opencode.json (généré par oh deploy)                         │
   │  ┌─────────────────────────────────────────────────────────┐  │
   │  │  "mcpServers": {                                        │  │
   │  │    "team": {                                            │  │
   │  │      "command": "oh",                                   │  │
   │  │      "args": ["mcp", "team", "--project", "T-SRU"]     │  │
   │  │    }                                                    │  │
   │  │  }                                                      │  │
   │  └─────────────────────────────────────────────────────────┘  │
   └──────────────────────────────┬────────────────────────────────┘
                                  │
                                  ▼
   ┌───────────────────────────────────────────────────────────────┐
   │  MCP Team Server (cli/internal/mcp/team/server.go)            │
   │  Cache TTL: 30s (évite git pull répétitifs)                   │
   └───────────────────────────────────────────────────────────────┘
```

### Tools MCP disponibles

| Tool | Params | Description | Accès |
|------|--------|-------------|-------|
| `team_members` | — | Liste membres + rôles | Tous agents |
| `team_claims` | `project?` | Claims actifs | Tous agents |
| `team_wiki_list` | — | Pages wiki | Tous agents |
| `team_wiki_read` | `page` (requis) | Lire une page | Tous agents |
| `team_wiki_write` | `page`, `content`, `confidence`, `project` | Proposer page | documentarian |
| `team_events` | `project?`, `limit?` (max 200) | Événements récents | Tous agents |
| `team_notify` | `message` (requis) | Envoyer notification | Tous agents |
| `team_policies` | `project?` | Policies mergées | Tous agents |
| `team_takeover_brief` | `project`, `ticket_id` (requis) | Lire brief | Tous agents |
| `team_patterns_list` | `tags?` | Lister patterns | Tous agents |
| `team_patterns_read` | `name` (requis) | Lire pattern | Tous agents |
| `team_patterns_propose` | `name`, `tags`, `complexity`, `content` | Proposer pattern | orchestrator + planner |

### Skills injectés automatiquement

```
   ┌─────────────────────────────────────────────────────────────┐
   │  Bucket A (TOUS les agents quand team_enabled):             │
   │  ├── team-awareness          (règles collaboration)         │
   │  ├── team-policies-enforcement (vérif avant action)         │
   │  └── team-wiki-protocol      (contribution wiki)            │
   │                                                             │
   │  Bucket B (orchestrator-dev seulement):                     │
   │  └── team-coordination       (sélection tickets, claims)    │
   └─────────────────────────────────────────────────────────────┘
```

### Points de test

| # | Scénario | Résultat attendu |
|---|----------|------------------|
| 1 | `oh deploy` avec team | MCP `team` dans `opencode.json` |
| 2 | Agent appelle `team_members` | Liste des membres retournée |
| 3 | Agent appelle `team_claims` | Claims actifs retournés |
| 4 | `team_wiki_write` par non-documentarian | Refusé par permissions |
| 5 | `team_patterns_propose` par agent | Pattern créé `validated=false` |
| 6 | Skills Bucket A | Injectés dans tous les agents |
| 7 | Skill Bucket B | Injecté uniquement dans orchestrator-dev |
| 8 | Cache 30s | Pas de git pull à chaque appel tool |

---

## 18. Credentials — Sécurité

### Principe fondamental

> **Jamais de secret dans le repo team-state partagé.**

### Architecture de séparation

```
   ┌─────────────────┐     ┌─────────────────┐     ┌─────────────────┐
   │  Team-State     │     │  hub.toml       │     │  OS Keychain    │
   │  (Git partagé)  │     │  (local)        │     │  (local)        │
   ├─────────────────┤     ├─────────────────┤     ├─────────────────┤
   │ Recommandations │     │ token_key       │     │ Secrets réels   │
   │ • enabled       │     │ (nom de clé)    │────►│ • gitlab token  │
   │ • url           │     │ • url override  │     │ • jira token    │
   │ • write_recomm  │     │ • write_enabled │     │ • git creds     │
   │ • *_enforced    │     │                 │     │                 │
   │                 │     │ Pattern clé:    │     │ Pattern clé:    │
   │ PAS DE TOKEN    │     │ openhub.mcp.    │     │ openhub.mcp.    │
   │                 │     │ <service>.token │     │ <service>.token │
   └─────────────────┘     └─────────────────┘     └─────────────────┘
```

### Résolution des tokens

```
   1. Variable d'environnement (GITLAB_TOKEN / JIRA_TOKEN)
          │
          ▼ (si absent)
   2. OS Keychain via token_key (hub.toml ou project config)
          │
          ▼ (si absent)
   3. Erreur + wizard propose configuration
```

### Cascade de résolution MCP

| Paramètre | Priorité (haute → basse) |
|-----------|--------------------------|
| `Enabled` | team enforced > project override > hub config > team recommendation |
| `URL` | team enforced > project override > hub config > team recommendation > default |
| `WriteEnabled` | hub `write_enabled` (gate locale) ; team `write_recommended` (advisory) |
| `Token` | Toujours local (hub ou project keychain key) |

### Points de test

| # | Scénario | Résultat attendu |
|---|----------|------------------|
| 1 | Token dans env var | Utilisé en priorité |
| 2 | Token dans keychain | Utilisé si env var absent |
| 3 | Token absent partout | Erreur + wizard (TUI) ou message (CLI) |
| 4 | Enforced enabled | Pas de possibilité de désactiver localement |
| 5 | Enforced URL | Override local ignoré |
| 6 | Aucun secret dans team-state | Vérifier `git log` du repo partagé |

---

## 19. Concurrence Git (withWriteLock)

### Pattern critique

```
   ┌──────────────────────────────────────────────────┐
   │  Toute mutation (CreateClaim, Transfer, etc.)    │
   │                                                  │
   │  1. mu.Lock()           ← sérialise les écrit.  │
   │  2. git pull --rebase   ← best-effort (fresh)   │
   │  3. fn(ctx)             ← mutation fichier TOML  │
   │  4. git add + commit                            │
   │  5. git push            ← retry loop:           │
   │     │                                           │
   │     ├── Succès → pull (pick up changes)         │
   │     ├── Auth error → erreur permanente          │
   │     └── Autre erreur:                           │
   │         ├── pull --rebase                       │
   │         ├── backoff: 500ms × 2^attempt          │
   │         └── retry (max 3 tentatives)            │
   │                                                  │
   │  6. mu.Unlock()                                  │
   └──────────────────────────────────────────────────┘

   Environnement Git:
   • GIT_TERMINAL_PROMPT=0 (jamais de prompt interactif)
   • GIT_SSH_COMMAND="ssh -o BatchMode=yes
                          -o StrictHostKeyChecking=accept-new"
```

### Erreurs sentinelles

| Erreur | Signification |
|--------|---------------|
| `ErrNotCloned` | Repo pas encore cloné |
| `ErrSyncConflict` | Push échoué après 3 retries |
| `ErrMemberExists` | member_id déjà dans members.toml |
| `ErrMemberNotFound` | Membre introuvable |
| `ErrClaimExists` | Ticket déjà réclamé |
| `ErrClaimNotFound` | Claim inexistant |
| `ErrInvalidStatus` | Statut de claim inconnu |
| `ErrInvalidTransition` | Transition de statut interdite |
| `ErrPolicyViolation` | Policy refuse (enforcement=refuse) |
| `ErrBriefNotFound` | Brief de takeover introuvable |
| `ErrUnsafeName` | Path traversal / caractères invalides |
| `ErrProposalTooLarge` | Proposition wiki trop grande |

### Points de test

| # | Scénario | Résultat attendu |
|---|----------|------------------|
| 1 | Push concurrent | Retry avec rebase, max 3 tentatives |
| 2 | Auth error | Erreur permanente immédiate (pas de retry) |
| 3 | Conflit irrésoluble | ErrSyncConflict après 3 retries |
| 4 | Path traversal (`../`) | ErrUnsafeName rejeté |
| 5 | Caractères spéciaux dans ticketID | `/` remplacé par `_` |
| 6 | Mutations simultanées (même process) | Sérialisées par mutex |
| 7 | Pull warning (réseau) | PullWarning retourné, pas d'erreur fatale |

---

## 20. Scénarios end-to-end

### Scénario A : Parcours complet d'un nouveau membre

```
 1. [  ] oh team init → wizard 5 étapes
 2. [  ] Vérifier hub.toml ([[teams]] ajouté)
 3. [  ] Vérifier remote: members.toml contient le nouveau membre
 4. [  ] oh teams list → montre la team active
 5. [  ] oh deploy → opencode.json contient MCP team
 6. [  ] oh team status → montre les membres
 7. [  ] oh claim SRU-142 → statut in_progress
 8. [  ] oh team board → ticket visible en IN PROGRESS
 9. [  ] Board: touche 's' → passer en review
10. [  ] oh team activity --today → event claim.taken visible
11. [  ] oh team sync-tracker → claims synchronisés
12. [  ] oh claim transfer SRU-142 --to alice → brief généré
13. [  ] oh takeover-brief show SRU-142 → brief lisible
14. [  ] oh policies check --branch "feat/SRU-142-auth"
15. [  ] TUI: teams view → touche 'a' (ajouter 2e team)
16. [  ] oh teams archive <team2> → enabled=false
17. [  ] oh teams restore <team2> → enabled=true
18. [  ] TUI: team.detail → modifier config + touche 'w'
19. [  ] Vérifier notification sur webhook configuré
20. [  ] oh teams detach <project> → projet solo
```

### Scénario B : Conflit et résolution

```
 1. [  ] Membre A: oh claim TICKET-1
 2. [  ] Membre B: oh claim TICKET-1 → ErrClaimExists
 3. [  ] Vérifier event claim.conflict dans JSONL
 4. [  ] Board: ticket montre assigné = Membre A
 5. [  ] Membre A: oh claim transfer TICKET-1 --to B
 6. [  ] Takeover brief auto-généré
 7. [  ] Membre B: MCP team_takeover_brief → brief accessible
```

### Scénario C : Policy enforcement

```
 1. [  ] Configurer policy branch_naming (enforce=refuse)
 2. [  ] oh start avec branche invalide → REFUSÉ
 3. [  ] oh start avec branche valide → OK
 4. [  ] Configurer policy max_wip=1
 5. [  ] oh claim avec 1 ticket déjà actif → REFUSÉ
 6. [  ] oh release → claim libéré
 7. [  ] oh claim → OK (sous la limite)
```

### Scénario D : Workflow tracker complet

```
 1. [  ] Configurer [tracker] dans config.toml (type=gitlab, enabled=true)
 2. [  ] Configurer [tracker.projects] "T-SRU" = "42"
 3. [  ] oh team sync-tracker → sync initiale
 4. [  ] Fermer une issue sur GitLab
 5. [  ] oh team sync-tracker → claim passe à "done"
 6. [  ] Rouvrir l'issue sur GitLab
 7. [  ] oh team sync-tracker → claim passe à "in_progress"
 8. [  ] Vérifier labels poussés sur GitLab (push_labels=true)
 9. [  ] Vérifier auto-plan (issues assignées → claims planned)
```

### Scénario E : Wiki et patterns

```
 1. [  ] Agent documentarian appelle team_wiki_write
 2. [  ] Vérifier wiki/.pending/<page>.md créé
 3. [  ] oh team wiki review → accepter
 4. [  ] Vérifier wiki/<page>.md déplacé
 5. [  ] oh team wiki read <page> → contenu affiché
 6. [  ] Agent planner propose un pattern
 7. [  ] oh patterns list → pattern visible (validated=false)
 8. [  ] oh patterns validate <name>
 9. [  ] oh patterns list → validated=true
```

### Scénario F : Sessions parallèles

```
 1. [  ] oh start --parallel --tickets bd-42,bd-43
 2. [  ] Vérifier 2 worktrees Git isolés créés
 3. [  ] Vérifier claims créés pour les 2 tickets
 4. [  ] TUI: voir statut des 2 sessions en temps réel
 5. [  ] Modifier un même fichier dans les 2 sessions
 6. [  ] Vérifier détection de conflit potentiel
 7. [  ] Terminer les sessions
 8. [  ] Vérifier merge proposé (Beads) ou branches (externe)
```

---

## Résumé des raccourcis TUI par vue

### Vue Teams (`teams`)

| Touche | Action |
|--------|--------|
| `Enter` | Ouvrir détail team |
| `a` | Ajouter team |
| `d` | Supprimer team |
| `s` | Sync (git pull) |
| `r` | Rafraîchir liste |
| `u` | Undo (pile 10) |
| `?` | Aide |
| `:` | Omnibar |

### Vue Status (`team.status`)

| Touche | Action |
|--------|--------|
| `r` | Rafraîchir |
| `Esc` | Retour |

### Vue Board (`team.board`)

| Touche | Action |
|--------|--------|
| `h`/`l` | Colonnes |
| `j`/`k` | Items |
| `c` | Claim |
| `x` | Release |
| `t` | Transfer |
| `s` | Status |
| `r` | Refresh |
| `/` | Search |
| `f` | Filter |
| `Esc` | Clear filter |
| `q` | Quit (CLI) |

### Vue Detail (`team.detail`)

| Touche | Action |
|--------|--------|
| `j`/`k` | Naviguer |
| `Space` | Toggle |
| `Enter` | Éditer |
| `w` | Sauvegarder |
| `s` | Sync tracker |
| `t` | Test connexion |
| `a` | Ajouter dynamique |
| `d` | Supprimer dynamique |
| `u` | Recharger |
| `r` | Refresh |
