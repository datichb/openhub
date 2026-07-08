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
├── config.toml           # Config notifications
├── projects/
│   └── T-SRU/
│       ├── claims/
│       │   └── SRU-142.toml
│       └── events/
│           └── 2026-07.jsonl
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

L'agent `documentarian` a en plus `team_wiki_write` pour proposer des entrées wiki (toujours en attente de validation humaine).

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
