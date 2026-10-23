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
oh team rejoin [--repo <url>] [--member-id <id>] [--gitlab-url <url>] [--tracker-username <nom>] [--no-retro-tag]
```

| Flag | Type | Description |
|------|------|-------------|
| `--repo` | string | URL du dépôt git team-state |
| `--member-id` | string | Identifiant membre (interactif si omis) |
| `--gitlab-url` | string | GitLab réel du dépôt quand son adresse est un alias SSH (`git@mon-alias:…`) ; enregistré dans `hub.toml` (`gitlab_url`) et utilisé pour la vérification d'identité |
| `--tracker-username` | string | Votre nom d'utilisateur sur le tracker, écrit dans votre fiche de membre du dépôt d'équipe (vide = inchangé). Sans l'option, la commande le demande quand le tracker de l'équipe est sur un autre serveur que le dépôt d'équipe et que votre fiche n'a pas de nom (Entrée = laisser vide) |
| `--no-retro-tag` | bool | Ne pas rattacher les sessions existantes à votre identité |

Un nom écrit avec un `@` initial est enregistré sans (`@benjamin.datiche` → `benjamin.datiche`). Si l'écriture échoue (hors ligne), le rejoin réussit quand même et la commande à relancer est affichée : `oh team profile --tracker-username <nom>`. L'étape HTTPS des assistants (jeton du dépôt) est sautée quand le dépôt cloné répond à `git ls-remote` sans jeton.

Les assistants `oh init` (groupe équipe) et `oh team rejoin` (TUI) demandent le GitLab du dépôt (facultatif) et, si vous gardez le tracker de l'équipe sans qu'aucun jeton ne serve le serveur de ce tracker, où se trouve votre jeton : clé du trousseau, serveur et jeton. La clé et le serveur sont écrits dans `[tracker]` de `hub.toml` (`tracker_token_key`, `tracker_token_url`) quand la configuration en vigueur ne les donne pas déjà ; la clé par défaut de GitLab (`openhub.tracker.gitlab.token`) n'a pas besoin d'être déclarée, celle de Jira si.

### oh team profile

Affiche ou modifie **votre fiche de membre** dans le dépôt d'équipe (`members.toml`) : nom affiché, noms d'utilisateur GitLab, Mattermost et tracker. Sans option, affiche la fiche ; avec une option, la modifie en **un seul commit** (poussé) et ne touche pas aux autres champs.

```
oh team profile [--team <id>] [--display-name <nom>] [--gitlab-username <nom>] [--mattermost-username <nom>] [--tracker-username <nom>]
```

| Flag | Type | Description |
|------|------|-------------|
| `--team` | string | Équipe (identifiant) ; par défaut l'équipe du hub |
| `--display-name` | string | Nom affiché (ne peut pas être vide) |
| `--gitlab-username` | string | Nom d'utilisateur sur le GitLab du dépôt d'équipe (vérification d'identité au rejoin) |
| `--mattermost-username` | string | Nom d'utilisateur Mattermost (mentions des notifications) |
| `--tracker-username` | string | Nom d'utilisateur sur le tracker (GitLab ou Jira) |

Une option donnée **vide** efface le champ (`--tracker-username ""`) ; un `@` initial est retiré des noms GitLab et tracker. `tracker_username` est indispensable quand le tracker est sur un autre serveur que le dépôt d'équipe : sans lui, la synchro (`oh team sync-tracker`) n'attribue aucune issue du tracker au membre (vide, le nom GitLab est utilisé). `oh doctor` le contrôle (contrôles « Nom sur le tracker » et « Identité sur le tracker »). Le rôle et l'identifiant du membre ne se changent pas ici (`oh team rejoin`). Dans la TUI, la vue `team-detail` a la même fiche (section « Ma fiche de membre »).

```bash
oh team profile --tracker-username benjamin.datiche
```

### oh team config

Assistant de configuration des services MCP partagés (GitLab, Jira…) et de la synchronisation du tracker, au niveau équipe et/ou local. `oh team config status` affiche la configuration effective (résolution équipe + local).

Pour *Tracker sync*, l'assistant demande où se trouve le jeton du tracker : la clé du trousseau (`tracker_token_key`) et, pour GitLab, le serveur pour lequel elle est valable (`tracker_token_url`, vide = le serveur du tracker). **Il ne remplit plus la clé de lui-même** : une clé laissée vide reste vide, et l'assistant dit ce qui est lu alors (GitLab : `openhub.tracker.gitlab.token` pour le serveur du tracker ; Jira : rien). Au niveau local, la déclaration personnelle remplace celle de l'équipe (clé et serveur ensemble ; `-` pour effacer), et, si le dépôt d'équipe est joint par un alias SSH, le GitLab réel du dépôt (`gitlab_url`).

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

Réclame un ticket dans le team-state pour signaler à l'équipe que vous travaillez dessus. Si le ticket est déjà pris, un avertissement s'affiche (non bloquant). Si la réservation d'un autre membre a expiré (son bail est dépassé ; sans bail, inactive depuis `takeover.stale_days` jours), vous pouvez la reprendre : après confirmation, un brief de reprise est généré et la réservation vous revient avec un nouveau bail. Si l'expirée est la vôtre, son bail est renouvelé.

```
oh team claim <ticket-id> [options]
```

| Flag | Court | Type | Description |
|------|-------|------|-------------|
| `--planned` | | bool | Crée le claim en statut `planned` (colonne TODO) au lieu de `in_progress` |
| `--project` | `-p` | string | Projet (détecté depuis le dossier courant si omis) |
| `--worktree` | | string | Nom de la branche ou du worktree associé |
| `--yes` | `-y` | bool | Confirme la reprise d'une réservation expirée sans poser la question (obligatoire sans terminal : code de sortie 2 sinon) |

**Exemple :**

```bash
oh team claim TICKET-123
oh team claim TICKET-123 --yes   # reprendre une réservation expirée sans question
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

- **Attribution par nom.** Les issues sont attribuées aux membres par `tracker_username`, à défaut `gitlab_username`. Quand des issues vues par la synchro (issues de vos réservations, réserve `auto_plan_unassigned`) sont assignées à des noms qui ne sont ceux d'aucun membre, et que votre fiche n'a pas de `tracker_username` (ou aucun nom, sur un tracker du même serveur que le dépôt), le résumé ajoute un avertissement qui liste ces noms (cinq au plus) et donne la commande : `oh team profile --team <équipe> --tracker-username <nom>`. Avec `auto_plan_assigned` seul, la synchro n'interroge que les noms connus et ne voit pas les autres : `oh doctor` (contrôle « Identité sur le tracker ») couvre ce cas.

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

  **Chaque source est rattachée à un serveur et ne répond que pour lui** (v5.1) : le jeton du dépôt d'équipe n'est jamais envoyé à un tracker qui est sur un autre GitLab. Le serveur de la variable d'environnement est `OH_GITLAB_URL` (sinon `GITLAB_URL`) : sans l'une des deux, `GITLAB_TOKEN` n'est utilisé nulle part. Celui de `tracker_token_key` et de l'ancienne clé du tracker est `tracker_token_url`, sinon (implicite) le tracker ; celui des jetons MCP est `[mcp.gitlab] url`, sinon (implicite) `https://gitlab.com` ; celui du jeton d'équipe est le `gitlab_url` de l'équipe s'il est déclaré (dépôt joint par un alias SSH), sinon l'hôte du `state_repo`. Sans jeton pour le serveur appelé, la commande s'arrête en nommant le serveur, les sources lues et la commande qui déclare la clé ; voir [le serveur de chaque jeton GitLab](config.fr.md#le-serveur-de-chaque-jeton-gitlab). Un refus du serveur (HTTP 401 / 403) nomme la source, la clé et le serveur. `oh team config status` lit le même ordre (clé et serveur de `hub.toml` compris) et affiche `token_key` et `token_url`.

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
| `--gitlab-url` | string | GitLab réel du dépôt quand son adresse est un alias SSH (optionnel) |

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
- **Bail.** Une réservation en cours de travail a un bail (`lease_hours`, 24 h par défaut), que le démon renouvelle (`heartbeat_minutes`) tant qu'une session ouverte travaille sur le ticket. Quand le bail est dépassé, un autre membre peut la reprendre avec `oh team claim <ticket>` (confirmation commune, `--yes` sans terminal) : un brief de reprise est généré et la réservation lui revient avec un nouveau bail. Une réservation sans bail (créée avant la v5.1) est jugée sur `takeover.stale_days` jours d'inactivité (3 par défaut, section `[takeover]` du `config.toml` du team-state). Une réservation prise à la main, sans session, n'est pas renouvelée par le démon : `oh team claim <ticket>` renouvelle son bail.
- **Hors ligne, la fin d'une session attend le réseau.** Ce que la fin d'une session décide (libérer, terminer, marquer `session-ended`) est écrit tout de suite dans une file locale (`.git/oh-claims-queue.json` du clone du team-state), puis appliqué en un commit dès que le dépôt d'équipe répond : à chaque battement du démon, par sa tâche `claims-queue` (aussi au démarrage), ou à la fin d'une session. Une session qui démarre hors ligne sur un ticket que vous aviez déjà réservé reprend la réservation au retour du réseau, avant ce que la session précédente avait mis en file (qui est annulé).
- **Réservation oubliée :** `oh team release <ticket-id> -p <projet>` la libère à la main.
- **`oh doctor` signale les réservations orphelines** (ticket clos, ou toutes les sessions qui l'ont traité terminées), avec la commande à lancer. Les réservations prises sous l'ancien ID d'un projet recréé sont signalées aussi, et `oh doctor --fix` les rattache au projet local (voir [`oh doctor`](cli-infra.fr.md#oh-doctor)).
- **La colonne REVIEW suit le label `ready-for-review`.** Dans `oh board` et dans le Board TUI, un ticket Beads `in_progress` qui porte ce label est en REVIEW (Beads n'a pas de statut `review`). Le board d'équipe affiche le statut `review` des réservations.

oh avertit une fois par processus sur stderr (commandes d'équipe, première réservation prise par un lancement), et `oh doctor` liste le mode bêta comme un avertissement qui ne change pas son code de sortie.
