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

Le wizard demande :

| Champ | Description | Exemple |
|-------|-------------|---------|
| URL du repo | URL Git SSH/HTTPS du repo team-state | `git@gitlab.company.com:team/team-state.git` |
| Identifiant | Clé unique dans members.toml | `benjamin` |
| Nom d'affichage | Comment ton nom apparaît dans les notifications | `Benjamin` |
| Username GitLab | Pour l'intégration GitLab future | `bdatiche` |
| Username Mattermost | Pour les mentions dans les notifications | `benjamin.datiche` |
| Rôle | Ton rôle dans l'équipe | `lead`, `dev`, ou `reviewer` |

Cette commande :
1. Clone le repo dans `~/.oh/team-state/`
2. Crée la structure de répertoires (si premier membre)
3. T'ajoute dans `members.toml`
4. Met à jour `hub.toml` avec la configuration `[team]`
5. Push les changements

## 3. Configurer les notifications (optionnel)

Édite `config.toml` dans le repo team-state :

```toml
[notification]
mattermost_webhook = "https://mattermost.company.com/hooks/votre-webhook-id"
channel = "dev-ai-sessions"
enabled = true
bot_name = "OpenHub"
```

Pour obtenir l'URL du webhook : Mattermost > Intégrations > Webhooks entrants > Ajouter.

Commite et push :

```bash
cd ~/.oh/team-state
git add config.toml
git commit -m "config: activation notifications Mattermost"
git push
```

## 4. Déployer vers les projets

Après le team init, redéploie vers tes projets pour injecter le serveur MCP `team` :

```bash
oh deploy        # un seul projet
oh sync --all    # tous les projets
```

Cela ajoute le serveur MCP `team` dans `opencode.json`, rendant les outils d'équipe accessibles aux agents IA.

## Usage Quotidien

### Claims — Réservation de tickets

```bash
# Réserver un ticket avant de commencer
oh claim SRU-142

# Avec branche associée
oh claim SRU-142 --worktree feat/SRU-142-user-auth

# Libérer quand c'est terminé
oh release SRU-142

# Transférer à un autre membre
oh claim transfer SRU-142 --to alice
```

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
```

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
3. Vérifie que le channel Mattermost existe
4. Vérifie que `oh team status` fonctionne (confirme l'accès au repo)
