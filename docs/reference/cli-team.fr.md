> [Read in English](cli-team.en.md)

# Reference CLI — Equipe

Les fonctions d'equipe reposent sur un depot git **team-state** partage (claims, evenements, wiki, policies, patterns, workflows d'equipe). Voir [Workflows d'equipe](../guides/team-workflows.fr.md) pour les workflows et l'[ADR-040](../architecture/adr/040-workflows-team-state-governance.fr.md).

## Collaboration (oh team)

### oh team init

Assistant interactif de configuration de l'equipe, en 4 etapes adaptatives : configuration globale (reprise, sessions), identite (inscription dans l'equipe), notifications Mattermost (optionnel), policies d'equipe (conventions automatisees). Le depot team-state doit exister au prealable sur GitLab ou GitHub ; s'il est deja configure, seules les etapes utiles sont proposees.

```
oh team init [--solo [--id <id>] [--name <nom>] [--member-id <id>] [--project <projet>]]
```

#### Espace solo (`--solo`)

Pour un projet sans équipe : crée un team-state local (dépôt git sans remote) dans `~/.oh/teams/<id>/`, qui accueille les workflows du projet. Le seul membre a le rôle `lead`. Les fonctions d'équipe (board, claims, notifications, serveur MCP `team`) restent désactivées ; l'équipe active n'est jamais un espace solo.

```bash
oh team init --solo --project web-app
```

| Flag | Type | Description |
|------|------|-------------|
| `--solo` | bool | Crée un espace solo au lieu de lancer l'assistant |
| `--id` | string | Identifiant de l'espace (défaut : `solo`) |
| `--name` | string | Nom affiché |
| `--member-id` | string | Identifiant du membre (défaut : celui d'une autre équipe, sinon l'utilisateur système) |
| `--project` | string | Projet à rattacher (id ou nom ; refusé s'il a déjà une équipe) |

Dans `hub.toml`, l'espace est ajouté aux équipes existantes avec `solo = true` (sans `state_repo`).

### oh team promote

Partage un espace solo : ajoute le remote et pousse tout l'historique. L'identifiant, le dossier, l'historique, les projets rattachés et les workflows publiés sont conservés.

```bash
oh team promote --remote git@gitlab.com:acme/team-state.git
```

| Flag | Type | Description |
|------|------|-------------|
| `--remote` | string | URL d'un dépôt git **vide** (obligatoire) |
| `--team` | string | Espace solo à partager, si vous en avez plusieurs |

En cas d'échec (dépôt absent, non vide, accès refusé), l'espace reste solo. Les autres membres rejoignent ensuite l'équipe avec `oh team init` en saisissant l'URL du dépôt.

### oh team rejoin

Rejoint une equipe existante apres une reinstallation du hub : clone le depot team-state, verifie l'identite via GitLab et restaure la configuration. Ne pas lancer pendant que la TUI effectue des operations d'equipe sur le meme clone (le verrou ne protege pas contre un autre processus).

```
oh team rejoin [--repo <url>] [--member-id <id>] [--no-retro-tag]
```

| Flag | Type | Description |
|------|------|-------------|
| `--repo` | string | URL du depot git team-state |
| `--member-id` | string | Identifiant membre (interactif si omis) |
| `--no-retro-tag` | bool | Ne pas rattacher les sessions existantes a votre identite |

### oh team config

Assistant de configuration des services MCP partages (GitLab, Jira…) et de la synchronisation du tracker, au niveau equipe et/ou local. `oh team config status` affiche la configuration effective (resolution equipe + local).

```bash
oh team config
oh team config status
```

---

### oh team status

Affiche qui travaille sur quoi : claims actifs, membres, tickets en cours.

```
oh team status [--detail]
```

| Flag | Type | Description |
|------|------|-------------|
| `--detail` | bool | Afficher les sous-tickets et l'avancement |

---

### oh team activity

Journal d'activite de l'equipe.

```
oh team activity [options]
```

| Flag | Type | Defaut | Description |
|------|------|--------|-------------|
| `--limit` | int | `20` | Nombre maximal d'evenements |
| `--member` | string | | Filtrer par membre |
| `--project` | string | | Filtrer par projet |
| `--today` | bool | `false` | Evenements du jour seulement |
| `--week` | bool | `false` | 7 derniers jours |

```bash
oh team activity --today
oh team activity --member alice --limit 50
```

---

### oh team board

Tableau kanban de l'equipe en plein ecran (tickets en cours, prevus, termines).

```
oh team board [--watch]
```

| Flag | Type | Description |
|------|------|-------------|
| `--watch` | bool | Rafraichissement automatique (touche `r` pour rafraichir a la main) |

---

### oh team claim

Reclame un ticket dans le team-state pour signaler a l'equipe que vous travaillez dessus. Si le ticket est deja pris, un avertissement s'affiche (non bloquant). Si le ticket est inactif depuis plusieurs jours, oh propose de generer un brief de reprise avant de le reclamer.

```
oh team claim <ticket-id> [options]
```

| Flag | Court | Type | Description |
|------|-------|------|-------------|
| `--planned` | | bool | Cree le claim en statut `planned` (colonne TODO) au lieu de `in_progress` |
| `--project` | `-p` | string | Projet (detecte depuis le dossier courant si omis) |
| `--worktree` | | string | Nom de la branche ou du worktree associe |

**Exemple :**

```bash
oh team claim TICKET-123
oh team claim TICKET-123 --planned
oh team claim TICKET-123 --worktree feat/ticket-123
```

### oh team claim transfer

Change le proprietaire d'un claim existant sans le liberer.

```
oh team claim transfer <ticket-id> --to <member-id> [-p <projet>]
```

| Flag | Court | Type | Description |
|------|-------|------|-------------|
| `--to` | | string | Membre destinataire (obligatoire) |
| `--project` | `-p` | string | Projet (detecte depuis le dossier courant si omis) |

```bash
oh team claim transfer TICKET-123 --to bob
```

### oh team release

Libere un ticket reclame (il redevient disponible pour les autres membres).

```
oh team release <ticket-id> [-p <projet>]
```

| Flag | Court | Type | Description |
|------|-------|------|-------------|
| `--project` | `-p` | string | Projet (detecte depuis le dossier courant si omis) |

```bash
oh team release TICKET-123
```

---

### oh team sync-tracker

Synchronise les claims avec le tracker externe (GitLab Issues, Jira, Linear) : tire l'etat des issues, met a jour le statut des claims, recopie les labels et cree des claims `planned` pour les issues assignees pas encore reclamees. Sans flag ; la configuration vient de `[tracker]` dans le `config.toml` du team-state, completee par `[tracker]` du `hub.toml` local.

```bash
oh team sync-tracker
```

---

### oh team notify test

Envoie un message de test au(x) webhook(s) configure(s) dans le team-state.

```
oh team notify test [-m <message>]
```

| Flag | Court | Type | Description |
|------|-------|------|-------------|
| `--message` | `-m` | string | Message personnalise (defaut : message de test standard) |

```bash
oh team notify test
oh team notify test --message "Deploiement en cours"
```

---

### oh team wiki

Wiki d'equipe : pages et propositions.

```bash
oh team wiki list                  # pages et propositions en attente
oh team wiki read <page>           # lire une page
oh team wiki review [id]           # reviser une proposition (interactif sans id)
```

---

## Gestion d'equipes (oh teams)

Un utilisateur peut appartenir a plusieurs equipes ; chaque projet est rattache a 0 ou 1 equipe.

### oh teams list

Lister les equipes configurees.

```bash
oh teams list
```

### oh teams add

Ajouter une equipe au hub.

```bash
oh teams add --repo git@github.com:org/team-state.git --member-id alice
```

| Flag | Type | Description |
|------|------|-------------|
| `--repo` | string | URL du depot git team-state (`git@…` ou `https://…`) |
| `--member-id` | string | Votre identifiant membre dans l'equipe |
| `--id` | string | Identifiant local de l'equipe (derive du depot si omis) |
| `--name` | string | Nom d'affichage (optionnel) |

### oh teams remove

Supprimer une equipe du hub ; les projets rattaches deviennent des projets sans equipe.

```
oh teams remove [team-id] [-f]
```

| Flag | Court | Type | Description |
|------|-------|------|-------------|
| `--force` | `-f` | bool | Supprimer sans confirmation |

### oh teams detach

Detacher un projet de son equipe sans supprimer l'equipe.

```bash
oh teams detach [project-name]
```

### oh teams archive / restore

Archiver une equipe (desactivee, configuration et team-state local conserves) ou la reactiver (et synchroniser son team-state) :

```bash
oh teams archive [team-id]
oh teams restore [team-id]
```

---

## Gouvernance

### oh conventions check

Verifie la branche courante et les derniers commits par rapport aux conventions du wiki projet (`docs/wiki/technical/conventions.md`) et du wiki d'equipe. Avertissements non bloquants.

```bash
oh conventions check
```

### oh patterns

Bibliotheque de patterns de decomposition, stockee dans le team-state.

```bash
oh patterns list [--all] [--tags a,b]   # patterns valides (--all : aussi les non valides)
oh patterns show <nom>                  # afficher un pattern
oh patterns add [fichier]               # ajouter (interactif ou depuis un fichier)
oh patterns validate <nom>              # valider un pattern propose par un agent
oh patterns remove <nom>                # supprimer un pattern
```

### oh policies

Regles d'equipe appliquees a tous les projets (`policies.toml` du team-state, surchargeables par projet dans `policies-override.toml`).

```bash
oh policies list [-p <projet>]                                       # policies actives (vue fusionnee avec -p)
oh policies check [-p <projet>] [--branch <nom>] [--commit <msg>]    # verifier l'etat courant
oh policies add                                                      # ajouter une policy (interactif)
```

---

## Takeover Briefs

### oh takeover-brief

Briefs de reprise generes lors des transferts de tickets. **Alias :** `oh tb`. Toutes les sous-commandes acceptent `-p, --project <projet>`.

```bash
oh takeover-brief show <ticket-id>    # voir le contexte de reprise
oh takeover-brief list                # lister les briefs
oh takeover-brief enrich <ticket-id>  # alias deprecie : lance `oh run brief-enrich --headless` avec le brief et enregistre le resultat
```
