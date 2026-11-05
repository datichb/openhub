> [Read in English](cli-team.en.md)

# Référence CLI — Équipe

Les fonctions d'équipe reposent sur un dépôt git **team-state** partage (claims, événements, wiki, policies, patterns, workflows d'équipe). Voir [Workflows d'équipe](../guides/team-workflows.fr.md) pour les workflows et l'[ADR-040](../architecture/adr/040-workflows-team-state-governance.fr.md).

## Collaboration (oh team)

### Quelle équipe ? (`--team`)

Toutes les commandes `oh team …` (et `oh claim`, `oh release`, `oh policies`, `oh patterns`, `oh takeover-brief`…) agissent sur **une** équipe, choisie par la même règle, de la plus précise à la moins précise :

1. `--team <id>` (flag persistant de `oh team`) : l'équipe nommée ; un identifiant inconnu est une erreur, une équipe archivée est refusée avec le renvoi vers `oh teams restore` ;
2. l'équipe choisie dans la TUI (mode Équipe, ou équipe ouverte depuis la liste) ;
3. l'équipe du projet : `-p, --project <projet>` quand la commande l'accepte, sinon le projet du dossier courant ;
4. l'équipe par défaut (`default = true` dans `[[teams]]`, `oh teams default <id>`) ;
5. à défaut, la première équipe activée de `hub.toml`.

**Exception : `oh team init` et `oh team rejoin`.** `oh team init` crée toujours une nouvelle équipe sur un nouveau dépôt et `oh team rejoin` rejoint l'équipe du dépôt donné par `--repo` : `--team` y est **refusé** (au lieu d'être ignoré en silence), sauf avec `oh team init --reconfigure`, où il désigne l'équipe à reconfigurer.

Un espace solo n'est jamais choisi sans `--team`, ni une équipe archivée. Le dépôt team-state et **votre identité de membre** viennent toujours de la même équipe. `oh team claim -p api --team beta` est refusé si le projet `api` appartient à une autre équipe que `beta` : rien n'est écrit dans le team-state de la mauvaise équipe.

```bash
oh team status --team beta        # l'équipe beta, quel que soit le dossier
oh team claim -p api 42           # l'équipe du projet api
oh team status                    # l'équipe du projet du dossier courant, sinon l'équipe par défaut
```

### oh team init

**Crée une équipe** : assistant interactif sur un **nouveau** dépôt team-state, toujours (même si vous avez déjà une ou plusieurs équipes ; chacune a son dépôt). Le dépôt doit exister au préalable sur GitLab ou GitHub (vide ou non). Rien n'est cloné, tiré ni poussé avant le premier écran : le dépôt, la **sonde d'accès**, l'accès et le clone sont des étapes de l'assistant (avec un indicateur d'attente), puis la configuration globale (reprise, sessions), votre identité (votre fiche de membre), les notifications Mattermost (optionnel) et les policies d'équipe (conventions automatisées).

- **Sonde d'accès** : pour « Créer », c'est un `git push --dry-run` (rien n'est envoyé ni créé) : un token qui peut lire sans pouvoir écrire (GitLab : `read_repository` seul) est refusé ici, avant toute question. Un token ou une clé refusés ouvrent l'étape « Accès » (token HTTPS vérifié puis gardé dans le trousseau de oh ; clé SSH à charger dans l'agent). Sur un GitLab, un « dépôt introuvable » est traité comme un accès probablement manquant (GitLab répond 404 à un token qui ne voit pas le projet).
- **Identité** : si l'identifiant est déjà pris, l'écran d'erreur propose « Modifier mon identité » et **« C'est moi »** (la fiche qui existe est rattachée telle quelle, rien n'est écrasé). L'identifiant d'une fiche d'un collègue n'est jamais pris pour le vôtre parce que vous l'avez dans une autre équipe.
- `hub.toml` est écrit **juste après** que votre fiche est dans le team-state : un assistant interrompu ensuite laisse une équipe configurée, pas une fiche dans une équipe que le hub ne connaît pas.
- L'assistant crée les dossiers du dépôt (`projects/`, `wiki/`, `catalog/`…) et les commite (« team: init structure »). Si ce commit échoue (hors ligne, refus du dépôt distant), l'inscription continue et l'assistant le dit : les dossiers existent en local et sont commités à la prochaine exécution.
- Le dépôt d'une équipe que le hub a déjà est refusé : c'est « Reconfigurer » (ci-dessous), ou rejoindre.

#### Reconfigurer (`--reconfigure`)

Reprend l'assistant sur l'équipe déjà configurée : sonde et accès, synchronisation du clone (cloné s'il ne l'a jamais été, par exemple après `oh teams add` d'une ancienne version), puis les mêmes étapes de configuration, de fiche, de notifications et de policies, préremplies. `--team <id>` (option persistante de `oh team`) désigne l'équipe ; sans elle, c'est l'équipe par défaut. Dans la TUI : commande « Reconfigurer l'équipe » de la palette, en mode équipe. Une équipe archivée ou solo n'est pas reconfigurable.

```
oh team init [--reconfigure [--team <id>]] [--solo [--id <id>] [--name <nom>] [--member-id <id>] [--project <projet>]]
```

#### Espace solo (`--solo`)

Pour un projet sans équipe : crée un team-state local (dépôt git sans remote) dans `~/.oh/teams/<id>/`, qui accueille les workflows du projet. Le seul membre a le rôle `lead`. Les fonctions d'équipe (board, claims, notifications, serveur MCP `team`) restent désactivées ; l'équipe par défaut n'est jamais un espace solo.

```bash
oh team init --solo --project web-app
```

| Flag | Type | Description |
|------|------|-------------|
| `--reconfigure` | bool | Reconfigure l'équipe déjà configurée au lieu d'en créer une nouvelle |
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
| `--team` | string | Espace solo à partager, si vous en avez plusieurs (le flag `--team` de `oh team`) |

Le droit d'écrire est vérifié avant tout push (`git push --dry-run`). En cas d'échec (dépôt absent, non vide, accès refusé ou en lecture seule), l'espace reste solo. Les autres membres rejoignent ensuite l'équipe avec `oh team rejoin --repo <url>`.

### oh team rejoin

Rejoint une équipe existante : nouveau membre, ou réinstallation du hub. Dans l'ordre : l'**accès** au dépôt (sonde ; pour un dépôt HTTPS la commande demande le nom d'utilisateur et le token, les vérifie, puis les garde dans le trousseau de oh ; pour SSH elle dit quoi vérifier), le **clone** du team-state, **qui vous êtes** (une fiche de la liste, ou la création de la vôtre), puis le nom sur le tracker, le token de l'API GitLab, l'écriture de `hub.toml` et la vérification d'identité via GitLab. Rien n'est cloné avant que l'accès réponde. Ne pas lancer pendant que la TUI effectue des opérations d'équipe sur le même clone (le verrou ne protège pas contre un autre processus).

```
oh team rejoin [--repo <url>] [--member-id <id>] [--gitlab-url <url>] [--tracker-username <nom>] [--no-retro-tag]
               [--new-member [--display-name <nom>] [--gitlab-username <handle>] [--role <rôle>]]
```

| Flag | Type | Description |
|------|------|-------------|
| `--repo` | string | URL du dépôt git team-state |
| `--member-id` | string | Identifiant membre (interactif si omis) |
| `--gitlab-url` | string | GitLab réel du dépôt quand son adresse est un alias SSH (`git@mon-alias:…`) ; enregistré dans `hub.toml` (`gitlab_url`) et utilisé pour la vérification d'identité |
| `--tracker-username` | string | Votre nom d'utilisateur sur le tracker, écrit dans votre fiche de membre du dépôt d'équipe (vide = inchangé). Sans l'option, la commande le demande quand le tracker de l'équipe est sur un autre serveur que le dépôt d'équipe et que votre fiche n'a pas de nom (Entrée = laisser vide) |
| `--no-retro-tag` | bool | Ne pas rattacher les sessions existantes à votre identité |
| `--new-member` | bool | Crée votre propre fiche (`--member-id` en est l'identifiant ; sans l'option, le premier choix de la liste « Je ne suis pas dans la liste » fait de même) |
| `--display-name`, `--gitlab-username`, `--role` | string | Nom affiché, handle GitLab (facultatif, unique dans l'équipe) et rôle (`lead`, `dev`, `reviewer` ; `dev` par défaut) de la fiche créée |

**Votre fiche.** Un identifiant déjà pris est refusé, avec « C'est ma fiche ? » : la fiche qui existe est alors rattachée telle quelle. Avant de **publier** une fiche neuve avec un handle GitLab, la commande vérifie (sans bloquer) que le token GitLab dont elle dispose est bien celui de ce handle ; s'il appartient à quelqu'un d'autre, elle le dit et demande s'il faut publier quand même. Les réponses sont lues sur l'entrée du terminal (le token n'est pas affiché).

Un nom écrit avec un `@` initial est enregistré sans (`@benjamin.datiche` → `benjamin.datiche`). Si l'écriture échoue (hors ligne), le rejoin réussit quand même et la commande à relancer est affichée : `oh team profile --tracker-username <nom>`. L'étape HTTPS des assistants (jeton du dépôt) est sautée quand le dépôt cloné répond à `git ls-remote` sans jeton.

Les assistants `oh init` (groupe équipe) et `oh team rejoin` (TUI) demandent le GitLab du dépôt (facultatif) et, si vous gardez le tracker de l'équipe sans qu'aucun jeton ne serve le serveur de ce tracker, où se trouve votre jeton : clé du trousseau, serveur et jeton. La clé et le serveur sont écrits dans `[tracker]` de `hub.toml` (`tracker_token_key`, `tracker_token_url`) quand la configuration en vigueur ne les donne pas déjà ; la clé par défaut de GitLab (`openhub.tracker.gitlab.token`) n'a pas besoin d'être déclarée, celle de Jira si.

### oh team profile

Affiche ou modifie **votre fiche de membre** dans le dépôt d'équipe (`members.toml`) : nom affiché, noms d'utilisateur GitLab, Mattermost et tracker. Sans option, affiche la fiche ; avec une option, la modifie en **un seul commit** (poussé) et ne touche pas aux autres champs.

```
oh team profile [--team <id>] [--display-name <nom>] [--gitlab-username <nom>] [--mattermost-username <nom>] [--tracker-username <nom>]
```

| Flag | Type | Description |
|------|------|-------------|
| `--team` | string | Équipe (identifiant) ; par défaut l'équipe choisie [comme ci-dessus](#quelle-équipe----team) |
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

**Les questions se répondent dans un terminal.** `Entrée` garde la valeur proposée entre crochets, c'est-à-dire la valeur actuelle (une équipe sur Jira reste sur Jira) ; `-` efface une valeur texte ; une réponse incomprise est dite et la question reposée. Pour les réglages de ce poste (`tracker.enabled`, `auto_sync`, `push_labels`), le choix **« hériter — équipe : … »** retire la clé de `hub.toml` ; tant qu'on ne le choisit pas, rien n'y est écrit. Pour les recommandations MCP de l'équipe, « aucune recommandation » laisse `enabled` absent. Une limite qui n'est pas un entier ≥ 0 est signalée et la limite reste. Sans terminal (entrée standard redirigée) la commande refuse de poser les questions, et change un réglage seul avec `oh config set`. Si l'entrée se ferme (Ctrl+D) avant la fin, **rien n'est écrit**. Les réponses sont lues sur l'entrée de l'application, jamais devinées.

**Les écritures sont sûres.** Les réglages de `hub.toml` passent par la même voie que `oh config set` (verrou, sauvegarde, valeurs typées : une clé du trousseau faite de chiffres reste du texte). La configuration d'équipe est écrite **sur la dernière version** de `config.toml` : ce qu'un collègue a changé entre-temps ailleurs dans le fichier est conservé ; si le même réglage a été changé des deux côtés, rien n'est écrasé, la commande le dit et il suffit de la relancer. Un `config.toml` illisible arrête l'assistant avec la cause (`oh doctor`) au lieu de repartir d'une configuration vide.

**`oh team config status` ne fait que lire** : il n'écrit ni le team-state ni `hub.toml`. Il ne propose plus de supprimer ou de fusionner des « membres orphelins » (les collègues n'étant pas dans votre configuration locale, ils étaient tous pris pour tels).

```bash
oh team config
oh team config status
```

### oh team config get / set / unset / show

Lisent et changent les réglages de `config.toml` du team-state **un par un**, sans terminal, par leur clé pointée.

```bash
oh team config get claim.lease_hours
oh team config set notification.enabled true
oh team config set tracker.unassigned_labels "ready, bug"
oh team config set notification.destinations.0.type slack
oh team config unset notification.webhook_url
oh team config show --origin
```

La valeur est lue avec le type de la clé (booléen, entier, nombre, liste séparée par des virgules, texte) et vérifiée (bornes, valeurs possibles) : une valeur refusée est dite avec sa raison et rien n'est écrit. Une entrée de liste se désigne par son rang (`notification.destinations.0.type`), une entrée de table par son nom (`mcp.gitlab.url`, `models.families.opus`). `unset` remet la valeur par défaut (la clé n'est plus écrite) ou retire l'entrée. `show` liste les réglages avec leur valeur ; `--origin` dit d'où elle vient : `défaut`, `équipe` (écrit dans `config.toml`), `moi, pour cette équipe` (`[teams.tracker]` de `hub.toml`) ou `moi, hub.toml` (`[tracker]`). Les webhooks sont affichés masqués.

Changer un réglage est une **publication** : un commit, et un événement `config_changed` (les clés et l'auteur) dans `oh team activity`. Les réglages `*_enforced`, `[limits.enforced]` et `[governance]` sont réservés au rôle `lead` (voir [`[governance]`](config.fr.md#section-governance)). `--team <id>` vise une autre équipe.

---

### oh team status

Affiche qui travaille sur quoi : claims actifs, membres, tickets en cours. Le résumé (actifs, en revue, bloqués) compte par **rôle des colonnes du board** de l'équipe (`active`, `review`, `blocked`), pas par le nom de statuts fixes. Les tickets de la réserve (sans propriétaire) et ceux d'un membre qui n'est plus dans l'équipe sont listés après les membres.

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

Ouvre la TUI sur le **board d'équipe** (la vue `team.board` : tickets en cours, prévus, terminés, projets sur `[` / `]`, filtre sur `/`, réservation, libération, transfert). `Esc` revient au mode équipe, puis au hub. L'équipe est celle du hub (`oh team init` d'abord : le team-state doit être cloné, comme avant). Le board se rafraîchit toutes les 5 s et sur `r`.

```
oh team board
```

| Flag | Type | Description |
|------|------|-------------|
| `--watch` | bool | Accepté et sans effet (masqué) : le board se rafraîchit toujours seul |

Sans terminal, la commande sort avec le code **2** et nomme `oh team status`. Différences avec l'ancien board plein écran : c'est le board de la TUI, avec ses colonnes personnalisées (`[board]`), ses filtres et ses actions.

---

### oh team claim

Réclame un ticket dans le team-state pour signaler à l'équipe que vous travaillez dessus. Si le ticket est déjà pris, un avertissement s'affiche (non bloquant). Si la réservation d'un autre membre a expiré (son bail est dépassé ; sans bail, inactive depuis `takeover.stale_days` jours), vous pouvez la reprendre : après confirmation, un brief de reprise est généré et la réservation vous revient avec un nouveau bail. Si la réservation est la vôtre, son bail est renouvelé (expirée ou non, sans confirmation).

**Limite de travail en cours.** Un membre a au plus `max_in_progress_per_member` tickets en cours (5 par défaut, `0` = sans limite ; voir [`[tracker]`](config.fr.md#section-tracker)) ; les réservations en revue ou en validation ne comptent pas. Prendre un ticket de plus en cours est **refusé** (code de sortie 1) avec le compte et la clé : `Limite de travail en cours atteinte : alice a déjà 5 ticket(s) en cours (max_in_progress_per_member = 5)…`. La même limite s'applique à la reprise d'une réservation expirée, à `oh team claim transfer` vers soi, à la touche de prise et au changement de statut du board d'équipe, et à une session lancée sur des tickets. `--planned` n'y est pas soumis. Pour la relever : `oh config set tracker.max_in_progress_per_member <n>` (pour vous) ou `oh team config` (pour l'équipe). `max_ticket_wip` des policies d'équipe, qui contrôle aussi cette commande, est un réglage distinct.

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

Change le propriétaire d'un claim existant sans le libérer. Se passer un claim en cours à soi-même est un ticket de plus en cours : refusé à la limite (voir [`oh team claim`](#oh-team-claim)) ; le passer à un autre membre ne l'est pas.

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
- Seuls les projets rattachés à l'équipe (`team_id`) sont liés à son tracker ; un projet sans équipe ou d'une autre équipe n'est pas synchronisé avec elle. Sans projet de l'équipe, la commande le dit (« aucun projet du hub n'est lié au tracker »).
- **Synchronisation automatique** (`auto_sync`) : le démon et l'ouverture du board synchronisent à l'intervalle `sync_interval_minutes`, un seul membre par intervalle (bail `sync.lock`) ; la commande, elle, n'attend pas le bail. Voir [`config.fr.md`](config.fr.md#section-tracker).
- **Ordre de recherche du jeton GitLab** (le premier trouvé l'emporte ; `GITLAB_TOKEN` toujours en premier) :

  | # | Source | Clé du trousseau |
  |---|--------|------------------|
  | 1 | Variable d'environnement | `GITLAB_TOKEN` |
  | 2 | `tracker_token_key` de la config du tracker | la clé que vous avez réglée |
  | 3 | Jeton du projet | `openhub.mcp.gitlab.token.<projet>` |
  | 4 | Jeton de l'équipe | `openhub.team.<équipe>.gitlab.token` |
  | 5 | Jeton MCP du hub | clé de `[mcp.gitlab] token`, puis `openhub.mcp.gitlab.token` (et son ancien nom `gitlab-token`) |
  | 6 | Ancienne clé du tracker | `openhub.tracker.gitlab.token` |

  **Chaque source est rattachée à un serveur et ne répond que pour lui** (v5.1) : le jeton du dépôt d'équipe n'est jamais envoyé à un tracker qui est sur un autre GitLab. Le serveur de la variable d'environnement est `OH_GITLAB_URL` (sinon `GITLAB_URL`) : sans l'une des deux, `GITLAB_TOKEN` n'est utilisé nulle part. Celui de `tracker_token_key` et de l'ancienne clé du tracker est `tracker_token_url`, sinon (implicite) le tracker ; celui des jetons MCP est `[mcp.gitlab] url`, sinon (implicite) `https://gitlab.com` ; celui du jeton d'équipe est le `gitlab_url` de l'équipe s'il est déclaré (dépôt joint par un alias SSH), sinon l'hôte du `state_repo`. Sans jeton pour le serveur appelé, la commande s'arrête en nommant le serveur, les sources lues et la commande qui déclare la clé ; voir [le serveur de chaque jeton GitLab](config.fr.md#le-serveur-de-chaque-jeton-gitlab). Un refus du serveur nomme la source, la clé et le serveur : un 401 est un jeton refusé (à remplacer, il arrête la synchro), un 403 est un accès refusé à une ressource (portée du jeton, rôle dans le projet : il ne concerne que ce projet du tracker). `oh team config status` lit le même ordre (clé et serveur de `hub.toml` compris) et affiche `token_key` et `token_url`.

  Jira garde son propre ordre (`JIRA_TOKEN`, puis `tracker_token_key`).

  **Ce qui change en v5.0** (rupture : le même ordre s'applique désormais à toutes les commandes) :

  | Commande | Avant | Maintenant |
  |----------|-------|------------|
  | `oh team sync-tracker`, écrans d'équipe, synchronisation automatique | `GITLAB_TOKEN`, puis la seule `tracker_token_key` | tout l'ordre ci-dessus : un jeton d'équipe ou MCP est trouvé sans `tracker_token_key` |
  | `oh mr create` et les entrées du workflow `review-feedback` | `GITLAB_TOKEN`, puis la clé MCP de `hub.toml`, puis `openhub.mcp.gitlab.token` | tout l'ordre ci-dessus : **le jeton de l'équipe (et celui du projet) passe avant le jeton MCP du hub**, et l'équipe du projet est trouvée ; l'ancienne clé du tracker est le dernier recours |
  | Vérification d'identité de `oh team init` / `rejoin` | `GITLAB_TOKEN`, équipe, `openhub.mcp.gitlab.token`, ancienne clé du tracker | même ordre ; la clé de `[mcp.gitlab] token` et le nom `gitlab-token` sont désormais lus aussi |
  | `oh mcp enable --project` (jeton présent ?) | `GITLAB_TOKEN`, jeton du projet, jeton du hub | même ordre ; la clé de `[mcp.gitlab] token` est désormais lue. Les jetons d'équipe et de l'ancien tracker ne comptent pas : le serveur MCP ne les reçoit pas |

  Si vous gardez un jeton d'équipe différent de celui du serveur MCP, vérifiez lequel l'emporte pour les commandes ci-dessus.

---

### oh team notify test

Envoie un message de test au(x) webhook(s) configuré(s) dans le team-state (sans file d'attente : une erreur est dite tout de suite, sans l'adresse du webhook). Les événements que reçoit chaque destination se règlent avec `oh team config set notification.events …` ; voir le [guide des notifications](../guides/notifications.fr.md).

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

Refuser une proposition (`r`) demande un **motif** (obligatoire : un motif vide annule le refus). La proposition est retirée du team-state ; le motif est gardé dans le sujet du commit (`wiki: rejected proposal <id>: <motif>`) et dans les données de l'événement `wiki.rejected`, que `oh team activity`, la vue Activité et la notification affichent.

---

## Gestion d'équipes (oh teams)

Un utilisateur peut appartenir à plusieurs équipes ; chaque projet est rattaché à 0 ou 1 équipe.

`oh teams` gère les équipes de cette machine (lister, ajouter, retirer, détacher un projet, archiver, restaurer, choisir l'équipe par défaut) ; `oh team` (au singulier) agit sur une équipe, choisie comme [ci-dessus](#quelle-équipe----team) (statut, board, activité, config, synchronisation du tracker). Les deux commandes n'ont aucune sous-commande en commun : `oh teams` n'est donc pas un alias de `oh team`.

### oh teams list

Lister les équipes configurées ; l'équipe par défaut porte une étoile (`★`).

```bash
oh teams list
```

### oh teams default

Affiche l'équipe par défaut, ou en choisit une : l'équipe qui s'applique quand ni `--team`, ni l'équipe choisie dans la TUI, ni l'équipe du projet n'en désignent une.

```bash
oh teams default           # affiche l'équipe par défaut
oh teams default beta      # beta devient l'équipe par défaut
```

L'équipe est marquée `default = true` dans `hub.toml` (une seule l'a). Sans marque, la première équipe activée est utilisée. Une équipe archivée ou un espace solo ne peut pas être l'équipe par défaut ; archiver l'équipe par défaut retire la marque et le dit.

### oh teams add

Ajouter une équipe au hub : **alias de `oh team rejoin`** (sonde et réglage de l'accès, clone, choix ou création de votre fiche, vérification de l'identité), avec un identifiant et un nom d'équipe au choix. Il n'écrit plus `hub.toml` seul avec un `member_id` non vérifié (l'identifiant d'un collègue devenait « votre » fiche). Les sessions de cette machine ne sont pas rattachées à la nouvelle équipe.

```bash
oh teams add --repo git@github.com:org/team-state.git --member-id alice
```

| Flag | Type | Description |
|------|------|-------------|
| `--repo` | string | URL du dépôt git team-state (`git@…` ou `https://…`) |
| `--member-id` | string | Votre identifiant membre dans l'équipe (la liste est proposée si omis) |
| `--id` | string | Identifiant local de l'équipe (dérive du dépôt si omis) |
| `--name` | string | Nom d'affichage (optionnel) |
| `--gitlab-url` | string | GitLab réel du dépôt quand son adresse est un alias SSH (optionnel) |

Les autres options de `oh team rejoin` (`--new-member`, `--display-name`, `--gitlab-username`, `--role`, `--tracker-username`) sont acceptées.

### oh teams set

Change un réglage d'une équipe de `hub.toml` : `repo`, `name`, `member-id`, `enabled`, `default`.

```bash
oh teams set <id> repo <url> [--gitlab-url <url>]
oh teams set <id> name "Équipe API"       # nom affiché (vide : l'identifiant)
oh teams set <id> member-id ben           # votre fiche dans le team-state
oh teams set <id> enabled false           # archive (true : restaure)
oh teams set <id> default                 # équipe par défaut (default false : retire la marque)
```

`member-id` doit être une fiche du team-state quand le clone est là (l'identifiant d'un collègue ferait de sa fiche la vôtre). `repo` :
L'URL est vérifiée avant que `hub.toml` change : l'accès (sonde, token demandé et gardé comme pour un rejoin) puis le **même team-state** (le dépôt doit partager l'historique de votre clone, sinon c'est une autre équipe ; un dépôt vide est refusé). Le clone, son dossier, votre fiche, les claims et les clés de l'équipe restent tels quels ; seul l'`origin` du clone et `state_repo` changent.

### oh teams remove

Supprimer une équipe du hub ; les projets rattachés deviennent des projets sans équipe.

```
oh teams remove [team-id] [--yes]
```

| Flag | Court | Type | Description |
|------|-------|------|-------------|
| `--yes` | `-y` | bool | Supprimer sans confirmation (obligatoire sans terminal). Anciens noms `--force` et `-f` : retirés en 5.1 |

### oh teams detach

Détacher un projet de son équipe sans supprimer l'équipe.

```bash
oh teams detach [project-name]
```

### oh teams archive / restore

Archiver une équipe (désactivée, configuration et team-state local conservés ; elle cesse d'être l'équipe par défaut) ou la réactiver (et synchroniser son team-state) :

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
```

## Bail et fin de session

Résumé de référence ; le contrat complet (statuts, bail, fin de session, file) est dans le guide [Setup équipe](../guides/team-setup.fr.md#bail-et-fin-de-session).

**Statuts.** La colonne REVIEW est, côté Beads (`oh board`, Board TUI), un ticket `in_progress` qui porte le label `ready-for-review` (Beads 1.3 n'a pas de statut `review`) ; côté réservation (board d'équipe), le statut `review`. Les deux ne se recopient pas ; à la fin d'une session, l'un ou l'autre garde la réservation. Détail : [Modèle de statut](../guides/team-setup.fr.md#modèle-de-statut).

**Bail.** Une réservation en cours de travail (propriétaire et travail commencé ; pas `planned`, pas le pool, pas terminée) a un bail. Réglages de la section `[claim]` du `config.toml` du team-state (et de `team-detail`) :

| Clé | Défaut | Rôle |
|---|---|---|
| `lease_hours` | 24 (1 à 168) | bail d'une réservation travaillée par une session |
| `heartbeat_minutes` | 60 (0 = désactivé, 5 à 1440, au plus la moitié du bail) | signal de vie du démon : renouvelle le bail des sessions ouvertes quand il en reste moins de la moitié |
| `sleeping_renewal_days` | 3 (1 à 90) | plafond : une session en veille depuis plus longtemps ne renouvelle plus |
| `remote_renewal_days` | 7 (1 à 90) | plafond : une session distante lancée depuis plus longtemps ne renouvelle plus |
| `lease_hours_manual` | 72 (`lease_hours` à 720) | bail d'une réservation prise à la main (sans session), que rien ne renouvelle tout seul |
| `release_expired_on_sync` | `false` | la synchronisation du tracker libère les réservations expirées (une réservation en revue n'est jamais libérée) |

**Expiration et reprise.** Une réservation en revue (statut `review` ou label `review` / `ready-for-review`) **n'expire jamais** : elle reste réservée jusqu'à la fusion ou sa libération, et `oh doctor` (`claims_review`, avertissement) ne la signale que si elle y est depuis plus de `takeover.stale_days` jours. Toute autre réservation expirée (bail dépassé ; sans bail, inactive plus de `takeover.stale_days` jours) est signalée par `oh doctor` (`claims_expired`, avertissement). `oh team claim <ticket>` renouvelle votre propre réservation (expirée ou non, sans confirmation) et reprend celle d'un autre membre si elle a expiré (confirmation commune ; `--yes` sans terminal, sinon code 2).

**Fin de session.** Pour chaque réservation de la session : terminée → gardée ; ticket clos → statut terminal ; en revue (label `review` / `ready-for-review` ou statut `review`) → **gardée** ; checkpoint en attente, `cp-2` reporté avec du travail laissé, état des checkpoints illisible → gardée avec le label `session-ended` ; sinon libérée. Une réservation gardée n'est plus renouvelée et expire à la fin de son bail, sauf la réservation en revue, qui n'expire pas.

**File durable.** Les décisions de fin de session et les renouvellements non poussés sont écrits dans `.git/oh-claims-queue.json` du clone du team-state, puis appliqués en un commit dès que le dépôt répond (fin de session, battement du démon, tâche `claims-queue` toutes les 15 minutes). `oh doctor` (`claims_queue`) signale ce qui attend depuis plus de 15 minutes ; `oh doctor --fix` l'applique après confirmation.

**Réservation oubliée :** `oh team release <ticket-id> -p <projet>` la libère à la main. `oh doctor` signale aussi les réservations orphelines (ticket clos, ou toutes les sessions qui l'ont traité terminées) ; celles prises sous l'ancien ID d'un projet recréé sont rattachées au projet local par `oh doctor --fix` (voir [`oh doctor`](cli-infra.fr.md#oh-doctor)).
