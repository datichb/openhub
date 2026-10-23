> [Read in English](cli-team.en.md)

# Référence CLI — Équipe

> **Bêta (v5.0).** Le mode équipe est publié en bêta : il fonctionne, mais quelques limites sont connues (réservation libérée à l'arrêt de la session, pas d'expiration, libération qui peut se perdre). Lisez les [limites connues](../guides/team-setup.fr.md#limites-connues) avant de vous y fier.

Les fonctions d'équipe reposent sur un dépôt git **team-state** partage (claims, événements, wiki, policies, patterns, workflows d'équipe). Voir [Workflows d'équipe](../guides/team-workflows.fr.md) pour les workflows et l'[ADR-040](../architecture/adr/040-workflows-team-state-governance.fr.md).

## Collaboration (oh team)

### oh team init

Assistant interactif de configuration de l'équipe, en 4 étapes adaptatives : configuration globale (reprise, sessions), identité (inscription dans l'équipe), notifications Mattermost (optionnel), policies d'équipe (conventions automatisées). Le dépôt team-state doit exister au préalable sur GitLab ou GitHub ; s'il est déjà configuré, seules les étapes utiles sont proposées.

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

Rejoint une équipe existante après une réinstallation du hub : clone le dépôt team-state, vérifie l'identité via GitLab et restaure la configuration. Ne pas lancer pendant que la TUI effectue des opérations d'équipe sur le même clone (le verrou ne protège pas contre un autre processus).

```
oh team rejoin [--repo <url>] [--member-id <id>] [--no-retro-tag]
```

| Flag | Type | Description |
|------|------|-------------|
| `--repo` | string | URL du dépôt git team-state |
| `--member-id` | string | Identifiant membre (interactif si omis) |
| `--no-retro-tag` | bool | Ne pas rattacher les sessions existantes à votre identité |

### oh team config

Assistant de configuration des services MCP partagés (GitLab, Jira…) et de la synchronisation du tracker, au niveau équipe et/ou local. `oh team config status` affiche la configuration effective (résolution équipe + local).

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

Journal d'activité de l'équipe.

```
oh team activity [options]
```

| Flag | Type | Défaut | Description |
|------|------|--------|-------------|
| `--limit` | int | `20` | Nombre maximal d'événements |
| `--member` | string | | Filtrer par membre |
| `--project` | string | | Filtrer par projet |
| `--today` | bool | `false` | Événements du jour seulement |
| `--week` | bool | `false` | 7 derniers jours |

```bash
oh team activity --today
oh team activity --member alice --limit 50
```

---

### oh team board

Tableau kanban de l'équipe en plein écran (tickets en cours, prévus, terminés).

```
oh team board [--watch]
```

| Flag | Type | Description |
|------|------|-------------|
| `--watch` | bool | Rafraîchissement automatique (touche `r` pour rafraîchir à la main) |

---

### oh team claim

Réclame un ticket dans le team-state pour signaler à l'équipe que vous travaillez dessus. Si le ticket est déjà pris, un avertissement s'affiche (non bloquant). Si le ticket est inactif depuis plusieurs jours, oh propose de générer un brief de reprise avant de le réclamer.

```
oh team claim <ticket-id> [options]
```

| Flag | Court | Type | Description |
|------|-------|------|-------------|
| `--planned` | | bool | Crée le claim en statut `planned` (colonne TODO) au lieu de `in_progress` |
| `--project` | `-p` | string | Projet (détecté depuis le dossier courant si omis) |
| `--worktree` | | string | Nom de la branche ou du worktree associé |

**Exemple :**

```bash
oh team claim TICKET-123
oh team claim TICKET-123 --planned
oh team claim TICKET-123 --worktree feat/ticket-123
```

### oh team claim transfer

Change le propriétaire d'un claim existant sans le libérer.

```
oh team claim transfer <ticket-id> --to <member-id> [-p <projet>]
```

| Flag | Court | Type | Description |
|------|-------|------|-------------|
| `--to` | | string | Membre destinataire (obligatoire) |
| `--project` | `-p` | string | Projet (détecté depuis le dossier courant si omis) |

```bash
oh team claim transfer TICKET-123 --to bob
```

### oh team release

Libère un ticket réclamé (il redevient disponible pour les autres membres).

```
oh team release <ticket-id> [-p <projet>]
```

| Flag | Court | Type | Description |
|------|-------|------|-------------|
| `--project` | `-p` | string | Projet (détecté depuis le dossier courant si omis) |

```bash
oh team release TICKET-123
```

---

### oh team sync-tracker

Synchronise les claims avec le tracker externe (GitLab Issues, Jira, Linear) : tire l'état des issues, met à jour le statut des claims, recopie les labels et crée des claims `planned` pour les issues assignées pas encore réclamées. Sans flag ; la configuration vient de `[tracker]` dans le `config.toml` du team-state, complétée par `[tracker]` du `hub.toml` local.

```bash
oh team sync-tracker
```

La commande, la touche de synchronisation (`s`) des écrans d'équipe et la synchronisation automatique suivent les mêmes règles (service équipe) :

- **`[tracker] enabled = false`** (dans le `hub.toml` local ou dans la config d'équipe) coupe la synchronisation partout. La commande affiche un avis et sort avec le code 0 ; les écrans refusent avec un message ; la synchronisation automatique ne fait rien. Avant la v5.0, les écrans synchronisaient quand même.
- Un projet sans équipe utilise l'équipe du hub (équipe par défaut de `hub.toml`) pour son tracker.
- **Ordre de recherche du jeton GitLab** (le premier trouvé l'emporte ; `GITLAB_TOKEN` toujours en premier) :

  | # | Source | Clé du trousseau |
  |---|--------|------------------|
  | 1 | Variable d'environnement | `GITLAB_TOKEN` |
  | 2 | `tracker_token_key` de la config du tracker | la clé que vous avez réglée |
  | 3 | Jeton du projet | `openhub.mcp.gitlab.token.<projet>` |
  | 4 | Jeton de l'équipe | `openhub.team.<équipe>.gitlab.token` |
  | 5 | Jeton MCP du hub | clé de `[mcp.gitlab] token`, puis `openhub.mcp.gitlab.token` (et son ancien nom `gitlab-token`) |
  | 6 | Ancienne clé du tracker | `openhub.tracker.gitlab.token` |

  **Chaque source est rattachée à un serveur et ne répond que pour lui** (v5.1) : le jeton du dépôt d'équipe n'est jamais envoyé à un tracker qui est sur un autre GitLab. Le serveur de la variable d'environnement est `OH_GITLAB_URL` (sinon `GITLAB_URL`), celui de `tracker_token_key` et de l'ancienne clé du tracker est `tracker_token_url`, celui des jetons MCP est `[mcp.gitlab] url`, celui du jeton d'équipe est l'hôte du `state_repo`. Une source dont le serveur n'est pas déclaré n'est pas utilisée. Sans jeton pour le serveur appelé, la commande s'arrête en nommant le serveur, les sources lues et la commande qui déclare la clé ; voir [le serveur de chaque jeton GitLab](config.fr.md#le-serveur-de-chaque-jeton-gitlab). Un refus du serveur (HTTP 401 / 403) nomme la source, la clé et le serveur. `oh team config status` lit le même ordre (clé et serveur de `hub.toml` compris) et affiche `token_key` et `token_url`.

  Jira garde son propre ordre (`JIRA_TOKEN`, puis `tracker_token_key`).

  **Ce qui change en v5.0** (rupture : le même ordre s'applique désormais à toutes les commandes) :

  | Commande | Avant | Maintenant |
  |----------|-------|------------|
  | `oh team sync-tracker`, écrans d'équipe, synchronisation automatique | `GITLAB_TOKEN`, puis la seule `tracker_token_key` | tout l'ordre ci-dessus : un jeton d'équipe ou MCP est trouvé sans `tracker_token_key` |
  | `oh mr create` (`oh review --publish`), `oh review feedback` et les entrées du workflow `review-feedback` | `GITLAB_TOKEN`, puis la clé MCP de `hub.toml`, puis `openhub.mcp.gitlab.token` | tout l'ordre ci-dessus : **le jeton de l'équipe (et celui du projet) passe avant le jeton MCP du hub**, et l'équipe du projet est trouvée ; l'ancienne clé du tracker est le dernier recours |
  | Vérification d'identité de `oh team init` / `rejoin` | `GITLAB_TOKEN`, équipe, `openhub.mcp.gitlab.token`, ancienne clé du tracker | même ordre ; la clé de `[mcp.gitlab] token` et le nom `gitlab-token` sont désormais lus aussi |
  | `oh mcp enable --project` (jeton présent ?) | `GITLAB_TOKEN`, jeton du projet, jeton du hub | même ordre ; la clé de `[mcp.gitlab] token` est désormais lue. Les jetons d'équipe et de l'ancien tracker ne comptent pas : le serveur MCP ne les reçoit pas |

  Si vous gardez un jeton d'équipe différent de celui du serveur MCP, vérifiez lequel l'emporte pour les commandes ci-dessus.

---

### oh team notify test

Envoie un message de test au(x) webhook(s) configuré(s) dans le team-state.

```
oh team notify test [-m <message>]
```

| Flag | Court | Type | Description |
|------|-------|------|-------------|
| `--message` | `-m` | string | Message personnalisé (défaut : message de test standard) |

```bash
oh team notify test
oh team notify test --message "Deploiement en cours"
```

---

### oh team wiki

Wiki d'équipe : pages et propositions.

```bash
oh team wiki list                  # pages et propositions en attente
oh team wiki read <page>           # lire une page
oh team wiki review [id]           # reviser une proposition (interactif sans id)
```

---

## Gestion d'équipes (oh teams)

Un utilisateur peut appartenir à plusieurs équipes ; chaque projet est rattaché à 0 ou 1 équipe.

`oh teams` gère les équipes de cette machine (lister, ajouter, retirer, détacher un projet, archiver, restaurer) ; `oh team` (au singulier) agit sur l'équipe active (statut, board, activité, config, synchronisation du tracker). Les deux commandes n'ont aucune sous-commande en commun : `oh teams` n'est donc pas un alias de `oh team`.

### oh teams list

Lister les équipes configurées.

```bash
oh teams list
```

### oh teams add

Ajouter une équipe au hub.

```bash
oh teams add --repo git@github.com:org/team-state.git --member-id alice
```

| Flag | Type | Description |
|------|------|-------------|
| `--repo` | string | URL du dépôt git team-state (`git@…` ou `https://…`) |
| `--member-id` | string | Votre identifiant membre dans l'équipe |
| `--id` | string | Identifiant local de l'équipe (dérive du dépôt si omis) |
| `--name` | string | Nom d'affichage (optionnel) |

### oh teams remove

Supprimer une équipe du hub ; les projets rattachés deviennent des projets sans équipe.

```
oh teams remove [team-id] [--yes]
```

| Flag | Court | Type | Description |
|------|-------|------|-------------|
| `--yes` | `-y` | bool | Supprimer sans confirmation (obligatoire sans terminal). Anciens noms : `--force`, `-f` (dépréciés, même effet, avec un avertissement) |

### oh teams detach

Détacher un projet de son équipe sans supprimer l'équipe.

```bash
oh teams detach [project-name]
```

### oh teams archive / restore

Archiver une équipe (désactivée, configuration et team-state local conservés) ou la réactiver (et synchroniser son team-state) :

```bash
oh teams archive [team-id]
oh teams restore [team-id]
```

---

## Gouvernance

### oh conventions check

Vérifie la branche courante et les derniers commits par rapport aux conventions du wiki projet (`docs/wiki/technical/conventions.md`) et du wiki d'équipe. Avertissements non bloquants.

```bash
oh conventions check
```

### oh patterns

Bibliothèque de patterns de décomposition, stockée dans le team-state.

```bash
oh patterns list [--all] [--tags a,b]   # patterns valides (--all : aussi les non valides)
oh patterns show <nom>                  # afficher un pattern
oh patterns add [fichier]               # ajouter (interactif ou depuis un fichier)
oh patterns validate <nom>              # valider un pattern propose par un agent
oh patterns remove <nom>                # supprimer un pattern
```

### oh policies

Règles d'équipe appliquées à tous les projets (`policies.toml` du team-state, surchargeables par projet dans `policies-override.toml`).

```bash
oh policies list [-p <projet>]                                       # policies actives (vue fusionnee avec -p)
oh policies check [-p <projet>] [--branch <nom>] [--commit <msg>]    # verifier l'etat courant
oh policies add                                                      # ajouter une policy (interactif)
```

---

## Takeover Briefs

### oh takeover-brief

Briefs de reprise générés lors des transferts de tickets. **Alias :** `oh tb`. Toutes les sous-commandes acceptent `-p, --project <projet>`.

```bash
oh takeover-brief show <ticket-id>    # voir le contexte de reprise
oh takeover-brief list                # lister les briefs
oh takeover-brief enrich <ticket-id>  # alias deprecie : lance `oh run brief-enrich --headless` avec le brief et enregistre le resultat
```

## Limites connues

Le mode équipe est en bêta en v5.0. Ces limites sont connues ; la correction complète est prévue pour la v5.1.

- **Une réservation est libérée à l'arrêt de sa session**, même si une revue (checkpoint `cp-2`) est encore en attente : le ticket redevient disponible pour les autres membres avant la fin de la revue. Si le ticket est clos dans Beads, la réservation est terminée au lieu d'être libérée.
- **Pas d'expiration.** Une réservation reste jusqu'à ce que quelqu'un la libère. Après `takeover.stale_days` jours d'inactivité (3 par défaut, section `[takeover]` du `config.toml` du team-state), oh propose seulement un brief de reprise : reprendre le ticket reste manuel (`oh team claim transfer`).
- **Une libération peut se perdre.** Elle s'exécute en arrière-plan à la fin de la session : si la machine est hors ligne, ou si le démon s'arrête avant le push, la réservation reste en place.
- **Réservation oubliée :** `oh team release <ticket-id> -p <projet>` la libère à la main.
- **`oh doctor` signale les réservations orphelines** (ticket clos, ou toutes les sessions qui l'ont traité terminées), avec la commande à lancer. Les réservations prises sous l'ancien ID d'un projet recréé sont signalées aussi, et `oh doctor --fix` les rattache au projet local (voir [`oh doctor`](cli-infra.fr.md#oh-doctor)).
- **La colonne REVIEW suit le label `ready-for-review`.** Dans `oh board` et dans le Board TUI, un ticket Beads `in_progress` qui porte ce label est en REVIEW (Beads n'a pas de statut `review`). Le board d'équipe affiche le statut `review` des réservations.

oh avertit une fois par processus sur stderr (commandes d'équipe, première réservation prise par un lancement), et `oh doctor` liste le mode bêta comme un avertissement qui ne change pas son code de sortie.
