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

La personne qui **crée** l'équipe exécute :

```bash
oh team init
```

Ou depuis le TUI : **Accueil › Équipes › ➕ Nouvelle équipe** (ou la touche `n` de la vue Équipes, ou la commande « Nouvelle équipe… » de la palette `Ctrl+P`) ouvre une fenêtre *Créer une équipe* / *Rejoindre une équipe* / *Espace solo*. « Créer une équipe » (`team init` dans la palette) Elle crée toujours une nouvelle équipe sur un nouveau dépôt, même si vous en avez déjà une. Les **autres membres** rejoignent avec `oh team rejoin --repo <url>` (ou « Rejoindre une équipe » dans la palette, ou `oh teams add`, qui en est l'alias). Pour **reconfigurer** une équipe qui existe : `oh team init --reconfigure` (option `--team <id>`), ou « Reconfigurer l'équipe » dans la palette.

### Mode CLI (terminal)

Le wizard s'exécute en plein écran avec 5 étapes adaptatives et un panneau d'info latéral.

### Mode TUI (wizard inline)

Le wizard tourne directement dans le shell TUI sans quitter l'interface. L'omnibar
et les toasts restent accessibles. Raccourcis : `Ctrl+S` valider, `Ctrl+B` (ou `Alt+←`, `PgUp`) retour,
`Esc` pour « Quitter l'assistant ? » (quitter, passer une étape facultative ou continuer).

### Phase 1 — Dépôt, accès et identité (séquentiel)

L'assistant suit le même ordre pour « Créer » comme pour « Rejoindre », dans `oh init` comme
dans la palette (`team init`, `team rejoin`) :

1. **Dépôt** : l'URL du repo team-state et, facultatif, le serveur GitLab quand l'URL est un alias SSH.
2. **Sonde d'accès** : `git ls-remote` en lecture seule, sans aucune invite, avec un indicateur
   d'attente. Pour « Créer », qui écrit en premier, c'est un `git push --dry-run` (rien n'est
   envoyé) : un token en lecture seule est refusé ici, pas au premier push. Elle utilise ce que git a déjà (agent SSH, assistant d'identifiants, `netrc`, dépôt
   public) et le token que `oh` garde pour ce dépôt. Rien n'est cloné ni écrit.
3. **Accès** : cette étape n'apparaît que si la sonde est refusée pour une raison d'accès.
   - **HTTPS** : l'assistant demande un token (nom d'utilisateur `oauth2` par défaut pour GitLab),
     le **vérifie** auprès du dépôt, puis le garde dans le trousseau de `oh`
     (`openhub.git.<hôte>/<dépôt>.token`, voir [config](../reference/config.fr.md#clés-connues)).
     `oh` le fournit à git par un `GIT_ASKPASS` à usage unique pour toutes les opérations du
     team-state (clone, pull, push, sonde, démon) ; il n'est jamais dans une URL, une ligne de
     commande ni le `credential.helper` de git. Un token refusé n'est pas gardé. C'est un token
     **git** (scopes `read_repository` et `write_repository`) : il n'est **jamais copié** vers le
     token de l'API GitLab. Ce dernier (vérification d'identité, tracker : scope `api`, ou
     `read_api` + `read_user`) a sa propre clé (`openhub.team.<équipe>.gitlab.token`) et sa propre
     étape « Token GitLab » dans « Rejoindre », demandée aussi pour un dépôt HTTPS ; elle peut être
     ignorée. Si GitLab refuse `GET /api/v4/user` avec le token (403 faute de scope, 401, GitLab
     injoignable), « Rejoindre » aboutit quand même, avec l'avertissement « Identité non
     vérifiée ».
   - **SSH** : le message donne la réponse de git et rappelle `ssh-add` ; « Réessayer » sonde de nouveau. Un serveur que la machine ne connaît pas encore est confirmé avant la sonde (voir [Premier contact avec un serveur SSH](#premier-contact-avec-un-serveur-ssh)).
4. **Identité** : « Créer » demande votre fiche (identifiant, nom affiché, handle GitLab facultatif,
   rôle). « Rejoindre » liste les membres (« Membres de l'équipe (N) ») : vous choisissez votre
   fiche, ou **« + Je ne suis pas dans la liste : créer ma fiche »**, qui ouvre le même formulaire.
   Voir [Votre fiche de membre](#votre-fiche-de-membre).
5. **Clone** dans `~/.oh/team-states/<hôte>/<chemin du dépôt>/` (ou pull si déjà cloné), création de la
   structure de répertoires (si premier membre) et détection des clones dupliqués.

Chaque écran d'un assistant porte son compteur : « Étape 2/5 — Identité ».

### Premier contact avec un serveur SSH

`oh` ne fait pas confiance à un serveur SSH qu'il n'a jamais vu sans que vous l'ayez confirmé. Quand l'URL du dépôt est en SSH (`git@hôte:org/repo.git`, `ssh://git@hôte:2222/org/repo.git`) et que l'hôte n'est pas dans votre `known_hosts` :

- **Dans un terminal** (assistants « Créer » / « Rejoindre » de `oh init` et de la palette, `oh team init`, `oh team rejoin`, `oh teams add`, `oh teams set <id> repo <url>`, `oh team promote --remote`), `oh` lit les clés du serveur (`ssh-keyscan`) et **affiche leurs empreintes** (type et `SHA256:…`, comme `ssh-keygen -lf`). Comparez-les à celles que publie l'administrateur du serveur ou votre forge (gitlab.com et github.com publient les leurs dans leur documentation). Dans la TUI, deux étapes « Serveur SSH » précèdent la sonde ; le premier bouton est **Refuser**, Échap ne peut pas passer l'étape. En ligne de commande, la question est `[y/N]` : Entrée seul refuse.
- **Confirmé**, les clés sont ajoutées au fichier que ssh écrit : le premier `UserKnownHostsFile` de votre `~/.ssh/config` pour cet hôte, sinon `~/.ssh/known_hosts` (créé en `0600`, dossier `~/.ssh` en `0700`). La question n'est posée qu'**une fois** par serveur. **Refusé**, rien n'est ajouté ni cloné, et le message donne la commande `ssh -T` pour vous connecter vous-même.
- **Sans terminal** (entrée non interactive : script, tube, CI), `oh` ne pose aucune question et **refuse** quand le serveur est inconnu, en donnant les deux commandes possibles, à lancer **après avoir vérifié l'empreinte** : `ssh -T git@<hôte>` (ssh affiche l'empreinte et demande ; ajoutez `-p <port>` si besoin) ou `ssh-keyscan <hôte> >> ~/.ssh/known_hosts`. Relancez ensuite la commande.
- **Un serveur déjà connu** ne déclenche rien. Les commandes courantes (pull, push, claims) ne changent pas.
- **Le démon ne se connecte pas à un serveur inconnu.** Une équipe ajoutée à la main dans `hub.toml`, ou dont le serveur a disparu de `known_hosts`, n'est plus acceptée en silence par le battement des claims, le vidage de la file ni la synchro automatique (`StrictHostKeyChecking=accept-new` aurait retenu le serveur à la première réponse) : le démon n'essaie pas, écrit un avertissement dans son journal et lève **une seule alerte « À traiter »** pour ce dépôt, sous la session la plus récente de l'équipe, avec la commande à lancer (`ssh -T git@<hôte>`, après avoir vérifié l'empreinte). Il reprend seul dès que le serveur est connu. Un serveur couvert par une ligne `@cert-authority` de `known_hosts` n'est pas signalé (ssh l'accepte sur certificat).
- **`oh doctor`** : le contrôle « Serveur SSH connu » liste, par équipe en SSH, les serveurs absents de `known_hosts` (avertissement avec la commande `ssh -T`). Il lit `known_hosts` et résout l'alias avec `ssh -G` : il ne contacte aucun serveur, et tourne aussi avec `--offline`. Il dit aussi ce qu'est devenue la commande SSH de git (voir plus bas) : les options que `oh` reprend, ou pourquoi il ne la suit pas.

Ce que `oh` suit de `~/.ssh/config` (via `ssh -G`) : `HostName` (un alias est résolu avant le contrôle), `Port`, `HostKeyAlias` (le serveur est enregistré sous cet alias, sans le port), `UserKnownHostsFile`, `HashKnownHosts`. Le port hors 22 est enregistré sous la forme `[hôte]:port`, comme ssh ; une adresse IPv6 s'écrit entre crochets dans une URL scp (`git@[2001:db8::1]:org/repo.git`).

**`HashKnownHosts yes`** (le réglage par défaut de Debian et d'Ubuntu) : `oh` écrit le serveur haché (`|1|<sel>|<empreinte>`, un sel propre à chaque clé), comme ssh, et non en clair. La recherche d'un serveur lit les deux formes (en clair, haché, `[hôte]:port`), et une clé que `known_hosts` contient déjà n'est pas ajoutée une seconde fois. La commande à lancer à la main porte alors `ssh-keyscan -H`.

**La commande SSH de git** (`GIT_SSH_COMMAND`, `core.sshCommand`, `GIT_SSH`). git lance le ssh que vous lui avez indiqué, dans cet ordre : `GIT_SSH_COMMAND`, puis `core.sshCommand`, puis `GIT_SSH`. `oh` suit ceux qui sont les vôtres : `GIT_SSH_COMMAND` de votre environnement et `core.sshCommand` de votre configuration git **globale** (`git config --global`, inclusions suivies ; pas `/etc/gitconfig`), puis `GIT_SSH`. Il reprend les **options** de cette commande pour `ssh -G` et pour le ssh de ses propres processus git (pull, push, clone, démon), afin que le `known_hosts` qu'il lit et écrit soit celui que la connexion vérifie : `-i`, `-F`, `-p`, `-l`, `-4`/`-6` et les autres indicateurs sans valeur (`-q`, `-v`…), et `-o` pour `UserKnownHostsFile`, `GlobalKnownHostsFile`, `HostKeyAlias`, `HashKnownHosts`, `StrictHostKeyChecking`, `CheckHostIP`, `UpdateHostKeys`, `IdentityFile`, `IdentitiesOnly`, `IdentityAgent`, `CertificateFile`, `PubkeyAuthentication`, `BatchMode`, `Port`, `User`, `ConnectTimeout`, `ServerAliveInterval` et `ServerAliveCountMax`. `oh` garde son `BatchMode=yes` en premier ; ce que vous avez choisi pour la vérification du serveur (`StrictHostKeyChecking`, `UserKnownHostsFile`) l'emporte sur son `accept-new`.

- **La commande n'est jamais exécutée.** Sa ligne est découpée comme le fait un shell (guillemets, `~`, `$HOME`) ; seules les options ci-dessus sont reconstruites et passées à `ssh`. Au moindre doute, `oh` **abandonne toute la commande, lance `ssh` tout simple et le dit** (un avertissement dans le journal, une fois, et `oh doctor`) : un autre programme que `ssh` (`plink`, un script), une syntaxe de shell (`;`, `|`, `` `…` ``, `$(…)`, une variable autre que `$HOME`, un joker), un guillemet non fermé, un fichier donné par un chemin relatif, une option qui n'est pas dans la liste ci-dessus (`ProxyCommand`, `ProxyJump`, `LocalCommand`, `Include`, `-L`…).
- **Le `core.sshCommand` d'un dépôt n'est pas suivi**, le `.git/config` du clone de l'équipe compris : un dépôt peut être écrit par autre chose que vous (un agent, un script), et les options d'une commande ssh décident quelles clés de serveur sont acceptées. Les processus git d'`oh` lancent **votre** commande ssh quoi que dise le clone, et le contrôle du serveur interroge `ssh -G` de même. Ce n'est pas une erreur : `oh doctor` le signale comme ignoré (« `core.sshCommand` est défini dans le dépôt … mettez-le dans la config globale de git ou dans `GIT_SSH_COMMAND` ») et le démon l'écrit une fois dans son journal (Info). Déplacez-le dans votre configuration globale (`git config --global core.sshCommand …`) ou dans `GIT_SSH_COMMAND` pour qu'il soit suivi. git lancé à la main dans le clone l'applique toujours.
- **`GIT_SSH`** désigne un programme, pas une ligne de commande : il n'a pas d'options à reprendre. `oh` fixe toujours `GIT_SSH_COMMAND` pour ses processus git, donc `GIT_SSH` n'est pas utilisé ; `oh doctor` le dit quand ce n'est pas `ssh`.
- `-F` désigne un fichier de configuration à vous (comme `~/.ssh/config`, il peut contenir des lignes `Match exec` ou `ProxyCommand`) : il est repris en chemin absolu, puisque votre environnement et votre configuration globale sont les seules sources.

Limites :

- Derrière un `ProxyJump` ou un `ProxyCommand`, `oh` ne peut pas lire les clés (`ssh-keyscan` n'y passe pas) : il refuse avec la commande `ssh -T` à lancer, qui suit le rebond.
- Avec `UserKnownHostsFile none` (ou `/dev/null`), vous avez choisi de ne pas mémoriser les serveurs : `oh` ne pose pas de question.
- `ssh-keyscan` et `ssh` doivent être installés (OpenSSH est livré avec macOS et Linux, et avec Git pour Windows) ; sinon `oh` le dit et donne les commandes à lancer à la main. Un chemin avec une espace dans `UserKnownHostsFile` n'est pas géré.
- `oh` garde `StrictHostKeyChecking=accept-new` pour git (pull, push, démon) plutôt que `yes` : sur un serveur déjà connu, `accept-new` refuse une clé qui a changé exactement comme `yes`, et `yes` casserait les configurations qui n'enregistrent pas les serveurs (`UserKnownHostsFile` vers un fichier absent ou en lecture seule, serveur reconnu par un autre nom). Le seul cas qui diffère — un serveur inconnu — est justement celui que les assistants confirment avant la première connexion.

#### Dépôt vide

Un dépôt distant sans aucun commit (un projet GitLab tout neuf) est un dépôt valide pour
**« Créer »** : pas de pull à faire (il n'y a rien à tirer), la structure puis la fiche sont les
deux premiers commits, poussés avec la branche (`push -u`). **« Rejoindre »** un dépôt vide dit
que l'équipe n'a pas encore été créée, et ce qu'il faut faire (la créer, ou la faire créer par
son responsable) ; un dépôt qui a des commits mais pas de `members.toml` reçoit un message
distinct (« ce dépôt ne contient pas d'équipe oh »).

Un dossier qui contient déjà le clone **d'un autre dépôt** n'est jamais réutilisé : l'assistant
l'indique (dossier, dépôt trouvé, dépôt demandé) et laisse le choix (supprimer le dossier, ou
saisir une autre URL).

Quand une étape échoue, l'écran d'erreur propose des boutons (Tab ou ←/→ pour choisir, `Entrée`
réessaie, `Ctrl+B` revient) :

| Bouton | Effet |
|--------|-------|
| **Configurer l'accès** | retourne à l'étape Accès (en premier quand git a refusé l'accès) |
| **Réessayer** | refait l'étape (focus par défaut) |
| **Modifier l'URL** | retourne à la saisie du dépôt |
| **Passer l'équipe** | abandonne la configuration d'équipe : toutes les étapes du groupe sont sautées et le récapitulatif indique « équipe non configurée » (dans la palette, le bouton quitte l'assistant) |

Dans `oh init`, passer une étape de l'équipe (ou « Passer l'équipe ») saute **tout** le groupe,
le reste de l'installation continue : `setup_done` est écrit, aucune équipe n'est ajoutée à
`hub.toml`. En type d'installation « équipe », où le fournisseur est censé venir de l'équipe,
l'assistant le redemande après le groupe, puis propose un espace solo pour le projet (non créé
sans votre accord).

### Phase 2 — Configuration (wizard interactif)

Le wizard détecte automatiquement l'état du repo et adapte le comportement :

| Étape | Si repo vide (1er membre) | Si repo déjà configuré |
|-------|---------------------------|------------------------|
| **Config globale** | Création `config.toml` (stale_days, sessions) | Mise à jour si modifié |
| **Identité** | Enregistrement de votre fiche + commit-and-push | Mise à jour de **votre** fiche (celle de `hub.toml`) + commit-and-push |
| **Notifications** | Webhook (Mattermost, Slack, Discord ou Teams, saisi masqué) : `enabled`, `type` et `webhook_url` | Mise à jour si modifié |
| **Policies** | Multi-select de policies recommandées | Merge avec existantes |

#### Champs du profil (étape Identité)

| Champ | Description | Exemple |
|-------|-------------|---------|
| Identifiant | Clé unique dans members.toml | `benjamin` |
| Nom d'affichage | Comment ton nom apparaît dans les notifications | `Benjamin` |
| Handle GitLab | Facultatif. Pour l'intégration GitLab et la vérification de votre token ; **unique dans l'équipe** | `bdatiche` |
| Rôle | Ton rôle dans l'équipe | `lead`, `dev`, ou `reviewer` |
| Username Mattermost | Pour les mentions dans les notifications du webhook (relecteur, auteur d'une revue, ticket transféré) ; modifiable ensuite avec `oh team profile --mattermost-username` et dans « Membres et ma fiche » | `benjamin.datiche` |
| Username tracker | Ton nom sur le tracker (`tracker_username`), mêmes écrans. **Indispensable quand le tracker est sur un autre serveur que le dépôt d'équipe** (voir ci-dessous) | `benjamin.datiche` |

#### Votre fiche de membre

Chacun crée **sa propre fiche** en rejoignant : aucun responsable ne la valide. Les cinq parcours
(« Créer » et « Rejoindre » de `oh init`, `oh team init`, « Créer une équipe » et
« Rejoindre une équipe » de la palette) posent le même formulaire et écrivent la même fiche. Les
valeurs par défaut sont alignées : le **créateur** de l'équipe est `lead` (comme le membre d'un
espace solo, il est le premier et règle l'équipe) ; qui **rejoint** est `dev` (le moins de
pouvoir, changeable d'un cran dans la liste) ; le mode par défaut de toute nouvelle fiche est
`semi-auto`.

La fiche d'un collègue n'est **jamais remplacée** :

- **Identifiant déjà pris** : refusé, avec le nom du collègue qui l'a. Dans « Créer », l'écran
  d'erreur propose **« C'est moi »** : la fiche existante est rattachée telle quelle à votre
  `hub.toml` ; ses champs vides (nom, handle, rôle) sont complétés, ceux qui sont remplis ne
  changent pas. « Modifier mon identité » revient au formulaire. Dans « Rejoindre », choisir sa
  fiche dans la liste est ce « C'est moi » ; le formulaire de nouvelle fiche refuse un identifiant
  pris et renvoie à la liste. Votre propre identifiant (celui que `hub.toml` a déjà pour cette
  équipe, à la réinstallation) reste accepté.
- **Handle GitLab déjà porté par un autre membre** : refusé (casse et `@` ignorés), quel que soit
  l'identifiant.
- Choisir dans « Rejoindre » une fiche **sans handle** (faite avant que le handle soit demandé)
  ajoute une étape facultative « Handle GitLab » qui la complète ; sans handle, la vérification
  d'identité du token est sautée.
- Le formulaire d'une nouvelle fiche repart de ce que vous êtes dans votre autre équipe
  (identifiant, nom, handle), s'il y en a une.
- Si la liste des membres vient d'un clone qui n'a pas pu être mis à jour (hors ligne), un
  avertissement la surmonte : « La liste peut ne pas être à jour : … ».

#### Ton nom sur le tracker (`tracker_username`)

La synchro du tracker attribue les issues aux membres **par nom d'utilisateur** : le `tracker_username` de la fiche du membre, à défaut son `gitlab_username`. Quand le tracker est sur un autre GitLab (ou Jira) que le dépôt d'équipe, ton nom n'y est pas forcément le même (`benjamin.datiche` d'un côté, `benjamin.datiche1` de l'autre) : sans `tracker_username`, aucune issue du tracker ne t'est attribuée. Il s'écrit dans ta fiche du dépôt d'équipe (`members.toml`, un commit) et se règle à ces endroits :

| Où | Comment |
|----|---------|
| `oh team init` → étape Identité | champ « Username tracker » (CLI et TUI) |
| `oh team rejoin` | option `--tracker-username <nom>`, ou la question posée quand le tracker de l'équipe est sur un autre serveur que le dépôt et que ta fiche n'a pas de nom |
| Assistant `oh init` (groupe équipe, « rejoindre ») et TUI `team rejoin` | étape « Nom sur le tracker », mêmes conditions |
| `oh team profile` | `oh team profile --tracker-username <nom>` ; sans option, affiche ta fiche ; une option donnée vide efface le champ |
| TUI, vue `team-detail` | section « Ma fiche de membre » : nom affiché, noms GitLab, Mattermost et tracker, enregistrés tout de suite (un commit), sans passer par `Ctrl+S` |

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

> **Plusieurs personnes modifient `config.toml`.** Le détail d'équipe, les services MCP et les modèles d'équipe de la TUI, l'éditeur de colonnes, la découverte du tracker et `oh team config` enregistrent **ce que vous avez changé** sur la dernière version du fichier (tirée juste avant le commit) : le réglage qu'un collègue a modifié ailleurs entre-temps est conservé. Si vous avez tous deux changé **le même** réglage, rien n'est écrit : l'écran le dit (« modifié par un collègue entre-temps : recharger ? », avec les clés) et `r` relit sa version. Un `config.toml` illisible (faute TOML) ou un clone absent n'est jamais montré comme une configuration vide : l'écran affiche « config.toml illisible : … — oh doctor », `Ctrl+S` est refusé, et `r` relit une fois le fichier corrigé.
>
> **Services MCP de l'équipe : trois états.** Pour chaque service (`gitlab`, `jira`, `figma`, `gslides`, `github`, `linear`), la vue « Services MCP » dit si l'équipe l'utilise (activé), ne l'utilise pas (désactivé) ou ne dit rien (hérité : chaque membre décide ; la clé est alors absente du fichier). `Espace` passe d'un état à l'autre, y compris pour revenir à « hérité ». Voir [les trois états](../reference/config.fr.md#ce-que-léquipe-dit-dun-service-mcp--trois-états).

### Mattermost

```toml
[notification]
enabled = true
type = "mattermost"
webhook_url = "https://mattermost.example.com/hooks/..."
channel = "#dev-ai"
bot_name = "OpenHub"
```

> **`enabled = true` est obligatoire** : sans lui, aucun message n'est envoyé, même avec un webhook. `type` peut manquer (il est déduit du serveur du webhook). Les anciennes équipes qui ont `mattermost_webhook` : `oh doctor --fix` le réécrit en `webhook_url` + `type`.
>
> **Le webhook est un secret écrit en clair dans le dépôt team-state.** `oh` le masque dans tous ses écrans et récapitulatifs, et `oh doctor` avertit si le dépôt est lisible par tout le monde : gardez ce dépôt privé.
>
> Sans éditer le fichier : `oh team config set notification.enabled true`, `oh team config set notification.webhook_url <url>`, ou le détail d'équipe de la TUI (section Notifications, `Ctrl+S` pour publier).

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
events = ["*"]                        # facultatif : par défaut la revue et le wiki
```

Chaque destination choisit les événements qu'elle reçoit (`events`, par défaut `review.*` et `wiki.*`) et si les personnes concernées sont mentionnées (`mention`, vrai par défaut : le relecteur demandé, l'auteur d'une revue, le membre qui reçoit un ticket, avec le `mattermost_username` de leur fiche). Voir le [guide des notifications](notifications.fr.md).

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

Hors session (pas de `OH_TEAM_ID`), le serveur MCP team retombe sur la team par défaut
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

**Depuis la TUI**, dans la vue Projet du projet :

- la ligne **👥 Équipe : Core › changer…** de l'accueil du projet (toujours présente, `Équipe : aucune` sans équipe) ;
- ou la commande **« Rattacher le projet à une équipe »** de la palette (`Ctrl+P`, `rattacher`) ;
- ou le champ **Équipe** de la configuration du projet (`project-config`), une liste.

Le choix propose toutes les équipes (l'équipe par défaut en premier), un espace solo, ou aucune équipe ; « Nouvelle équipe… » y mène à la fenêtre de création. Hors de la TUI : `oh project set-team <projet> <équipe>`. Le changement est pris en compte au prochain lancement de session (paquet reconstruit) — aucun redéploiement nécessaire.

### Team custom : résolution du state path

Pour le mode `custom`, le `StatePath` est déduit automatiquement de l'URL du remote
si non défini explicitement :

```
git@gitlab.com:acme/team-state.git  →  ~/.oh/team-states/gitlab.com/acme/team-state
https://gitlab.com/beta/team-state  →  ~/.oh/team-states/gitlab.com/beta/team-state
```

Chaque remote team-state distinct obtient son propre répertoire de clone local sous
`~/.oh/team-states/<hôte>/<chemin complet du dépôt>`, même quand deux dépôts portent le même nom
(`acme/team-state` et `beta/team-state`) : ils ont aussi deux identifiants d'équipe (`team-state`
et `beta-team-state`). « Rejoindre » une équipe déjà configurée réutilise son `state_path`.

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

**Le clone échoue avec "terminal prompts disabled" ou "Authentication failed" :**

Le remote requiert une authentification et git n'a pas de quoi répondre. `oh` positionne
`GIT_TERMINAL_PROMPT=0` pour ne pas bloquer sur une invite — le message est intentionnel. Dans
l'assistant, l'étape **Accès** (ou le bouton « Configurer l'accès » de l'écran d'erreur) demande
le token et le garde dans le trousseau de `oh`.

Solutions :
- **HTTPS :** donner le token à l'assistant (étape Accès), ou l'enregistrer à la main :
  `oh secrets set openhub.git.gitlab.com/org/team-state.token` (et `…/team-state.user` si le
  nom d'utilisateur n'est pas `oauth2`). Un `credential.helper` de git qui répond déjà reste
  utilisé quand `oh` n'a pas de token pour ce dépôt.
- **SSH plutôt que HTTPS :** utiliser `git@gitlab.com:org/team-state.git`
- **Clé SSH non chargée :** `ssh-add ~/.ssh/id_ed25519` ou `eval "$(ssh-agent)"`
- **« Le serveur SSH … n'est pas dans votre known_hosts » :** `oh` ne tourne pas dans un terminal, ou refuse de faire confiance sans votre accord. Vérifiez l'empreinte, puis `ssh -T git@<hôte>` ou `ssh-keyscan <hôte> >> ~/.ssh/known_hosts` (voir [Premier contact avec un serveur SSH](#premier-contact-avec-un-serveur-ssh)).

**Le dossier contient le clone d'un autre dépôt :**

Un clone existant dont l'origine n'est pas l'URL saisie n'est jamais réutilisé. Supprimez le
dossier (ou choisissez le bon dépôt). Les équipes configurées avant la 5.3.2 gardent leur
`state_path` (dont l'ancien `~/.oh/team-state/`) : rien n'est déplacé.

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

# Alice est partie : toutes ses réservations ouvertes passent à bob (ou `--release` les libère)
oh team claim transfer --from alice --to bob
```

Libérer ou transférer la réservation d'**un autre** membre demande une confirmation qui nomme son propriétaire (`--force` sans terminal). Chaque libération, transfert et prise laisse un événement (`claim.released`, `claim.transferred`, `claim.taken`) **signé du membre qui agit**, visible dans `oh team activity` (`--member` montre aussi ce qui est arrivé à un membre).

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

Par défaut, le board affiche 6 colonnes : **TODO** · **IN PROGRESS** · **REVIEW** · **VALIDATION** · **DONE** · **BLOCKED**. Les colonnes et les rattachements des labels du tracker se modifient dans l'éditeur de la TUI, depuis l'accueil de l'équipe (*Colonnes du board*) ou avec `C` sur le board d'équipe (`n` ajouter, `e` renommer, `d` supprimer après confirmation, `o` rôle, `K` / `J` ordre, `m` rattachements, où `v` déplace un label ou un statut vers une autre colonne, `Ctrl+S` enregistrer) : voir [l'éditeur de colonnes](../reference/tui.fr.md#éditeur-de-colonnes-du-board). Au retour de l'éditeur, le board relit ses tickets avec les nouvelles colonnes.

Le **rôle** d'une colonne décide de ce que `a` propose sur ses cartes : `initial` (cadrer, faire le ticket), la première colonne `active` (faire le ticket, consigne à la session en cours), `review` (review de la branche du claim, retours de review de sa MR), les autres (détail, statut, transfert). Le rôle `review` est nouveau : donnez-le à la colonne où le travail attend une review. Une équipe qui ne l'a pas posé garde le comportement attendu, car la colonne d'identifiant `review` (celle du board par défaut) en tient lieu ; une colonne `review` ne compte pas dans `max_in_progress_per_member`. Voir [`[board]`](../reference/config.fr.md#section-board).

Raccourcis clavier dans le board (et les autres vues d'équipe) :

| Touche | Action |
|--------|--------|
| `h` / `l`, `j` / `k` | Colonnes, cartes |
| `Entrée` | Détail du ticket |
| `a` | Actions de la colonne de la carte : cadrer, faire le ticket, review, consigne à la session… (voir [le menu `a`](../reference/tui.fr.md#le-menu-a-dune-carte)) |
| `[` / `]` | Passer d'un projet à l'autre |
| `/` | Filtrer : texte, **Mes tickets**, **Non assignés** (la réserve, à prendre avec `c`), assigné, label, effacer les filtres |
| `c` | Prendre le ticket (refusé à `max_in_progress_per_member`) |
| `L` | Libérer le claim sélectionné |
| `t` | Transférer le claim sélectionné |
| `s` | Changer le statut (refusé à la limite quand il met votre carte en cours) |
| `C` | Éditer les colonnes du board et les labels qui y sont rattachés |
| `r` | Synchroniser le tracker, tirer et rafraîchir |

> **Pull automatique :** toutes les vues d'équipe (board, status, activity, etc.) effectuent un `git pull` automatique à l'ouverture et sur la touche `r`. Un toast n'est affiché que si l'opération prend plus d'une seconde.

### Synchronisation avec le tracker externe

```bash
oh team sync-tracker   # sync les claims avec le tracker externe (GitLab/Jira)
```

La commande tire les états des issues depuis le tracker et met à jour les statuts des claims :
- Issue fermée → claim passé en `done`
- Issue réouverte → claim repassé en `in_progress`
- **Le tracker ne réécrit un statut que si l'issue a changé depuis le passage précédent** : son état (ouverte/fermée), son nom de statut (Jira) ou les labels rattachés à une colonne (`label_status_mapping`). Une issue qui reste ouverte sans label de statut ne ramène pas une carte que vous avez passée en review, en validation, en bloqué ou en fait sur le board (ni après `oh mr create`, un verdict ou la fin d'une session). Chaque claim garde l'état et les labels de l'issue vus au dernier passage (`tracker_state`, `tracker_labels`) ; un claim qu'aucune synchro de 5.3.3 n'a vu est seulement mémorisé au premier passage (rien n'est écrasé, sauf une issue fermée dont le claim n'est pas terminal)
- Avec `push_labels = true` (et `write_enabled`), le statut du hub est **repoussé** vers le tracker : le label rattaché à la colonne du claim est posé sur l'issue et les labels rattachés à d'autres colonnes en sont retirés. Sans `push_labels`, rien n'est écrit dans le tracker ; une colonne sans label rattaché n'est pas repoussée
- Les labels sont mirrorés : un label du tracker est ajouté au claim, et n'en est retiré que s'il était sur l'issue au passage précédent et n'y est plus. Les labels posés par le hub (`session-ended`, `review`, `ready-for-review`, `agent-reviewed`…) restent
- Si la poussée vers le team-state échoue, la commande échoue avec l'erreur et l'état local de synchro n'est pas enregistré
- Avec `auto_plan_assigned = true`, toute issue ouverte assignée à un membre arrive sur le board sous forme de claim, dans la colonne que son statut désigne, sans plafond par membre
- Avec `auto_plan_unassigned = true`, les issues qui portent **l'un quelconque** des `unassigned_labels` arrivent sur le board comme claims de réserve (à prendre avec `c`), ou comme claims du membre auquel elles sont assignées. La réserve est d'abord balayée en entier, en autant de passages qu'il faut, chacun créant au plus `max_unassigned_issues` claims (une réserve de 70 avec les 20 par défaut demande quatre passages) ; ensuite les passages ne regardent que ce qui a été mis à jour. Changer `unassigned_labels` la balaie de nouveau en entier. Un claim que vous avez libéré reste libéré pendant le balayage
- La colonne d'une issue vient de `label_status_mapping`, puis de `status_mapping`. Avec plusieurs labels rattachés, une colonne bloquée l'emporte toujours, sinon la colonne la plus avancée (la plus loin dans l'ordre du board), puis l'ordre alphabétique des labels : une issue « Bloqué » + « READY TO DEV » est bloquée, et celle qui garde un vieux « READY TO DEV » à côté de « DEV DOING » reste en cours. Une valeur qui n'est pas un statut du board (`todo` sur un board sans colonne todo) est lue comme le statut de la colonne initiale ; `oh doctor` la nomme et `--fix` la réécrit (voir [`[tracker]`](../reference/config.fr.md#section-tracker))
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
auto_sync = true            # synchro automatique : démon (un seul membre par intervalle) et ouverture du board
sync_interval_minutes = 5   # intervalle de la synchro automatique, et durée du bail
auto_plan_assigned = true   # toute issue assignée arrive sur le board
max_in_progress_per_member = 5   # tickets en cours par membre en même temps (0 = sans limite)
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

Ajouter une équipe (alias de `oh team rejoin` : l'accès au dépôt est vérifié et réglé, le team-state cloné, votre fiche choisie ou créée) :

```bash
oh teams add --repo git@gitlab.com:org/team-state.git --member-id alice
```

Le dépôt d'une équipe a déménagé ou été renommé : `oh teams set <id> repo <url>` vérifie l'accès et que c'est le même team-state, puis change l'URL (le clone, votre fiche et les clés restent).

### Quitter une équipe

```bash
oh teams leave <team-id>                  # votre fiche reste dans l'équipe (elle porte votre historique)
oh teams leave <team-id> --remove-member  # votre fiche part aussi
```

Dans cet ordre : vos réservations ouvertes sont libérées (une transaction), la file des opérations de réservation en attente est appliquée, le clone est supprimé, les clés du trousseau de l'équipe sont supprimées (token et utilisateur de l'accès git, `openhub.team.<id>.*`), puis l'équipe quitte `hub.toml` et ses projets sont détachés. La commande liste tout cela et demande d'abord. Sans accès au team-state rien n'est supprimé (`--force` continue quand même ; vos réservations s'éteignent alors avec leur bail). Un clone qui garde un travail jamais poussé n'est pas supprimé sans `--force`.

`oh teams remove <team-id>` retire seulement l'équipe de `hub.toml` (détache ses projets) : le clone, les réservations et les clés restent.

### Retirer un membre (lead)

```bash
oh team member remove bob --to alice      # ses réservations ouvertes passent à alice
oh team member remove bob --release       # ou elles sont libérées
```

Seul un membre de rôle `lead` retire la fiche d'**un autre** ; une équipe **sans lead** n'est pas verrouillée (comme pour les rôles et les réglages imposés). Les réservations, les briefs et la fiche sont un seul commit, avec un événement `member.removed`. `oh doctor` signale les réservations de membres qui n'ont plus de fiche, avec les commandes qui règlent le cas, et prévient un membre dont la propre fiche a été retirée.

**Dans la TUI**, la vue Équipes (`Ctrl+P` › `teams`, ou Accueil › Équipes) fait la même chose : `d` propose *Quitter l'équipe* (comme `oh teams leave`, la liste de ce qu'elle fait est dans la confirmation), *Quitter et retirer ma fiche*, ou *Retirer de hub.toml seulement* (la confirmation annonce les projets qu'elle détache, et la liste ce qui a été détaché), `x` l'archive, `R` la restaure, `e` la renomme, `*` en fait l'équipe par défaut (`★`, comme `oh teams default <id>`), `p` ouvre sa vue Équipe, `n` ouvre la fenêtre « Nouvelle équipe ». Si l'équipe archivée ou retirée était l'équipe par défaut, le message dit laquelle la remplace.

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

Crée l'équipe avec `oh team init`, ou rejoins-la avec `oh team rejoin --repo <url>`.

### Une alerte « Accès au team-state refusé » (token expiré)

Le token git du dépôt a expiré ou a été révoqué après que l'équipe a été rejointe. Le démon le dit **une fois** (alerte ✗ dans « À traiter », sous la session la plus récente de l'équipe) et cesse d'essayer ce dépôt tant que le token que `oh` garde pour lui ne change pas (une clé SSH est réessayée toutes les 30 minutes). Pendant ce temps, les claims de vos sessions ne sont plus renouvelés et les opérations en file attendent. `oh doctor` (contrôle « Accès au team-state ») le confirme. Renouvelez l'accès : `oh team rejoin --repo <url>` (étape « Accès »), ou « Rejoindre une équipe » dans la palette.

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

**Réservation en revue.** Une réservation dont le statut est `review` ou celui d'une colonne de rôle `review` (quel que soit son ID), ou qui porte le label `review` ou `ready-for-review`, **n'expire pas**, quel que soit son bail : elle attend une personne (le relecteur, la fusion), pas une session. Elle reste réservée jusqu'à la fusion (la réservation est alors terminée) ou jusqu'à ce que son propriétaire la libère (`oh team release <ticket> -p <projet>`) ; ni `oh team claim` d'un autre membre ni `release_expired_on_sync` ne la prennent. Si la revue renvoie le travail (statut `in_progress`, label retiré) et que son bail est dépassé, elle en reçoit un nouveau à partir de ce moment. `oh doctor` (`claims_review`, avertissement) liste vos réservations en revue depuis plus de `takeover.stale_days` jours, sans les dire expirées, avec la commande pour les libérer si le travail est abandonné.

**File durable.** Ce que la fin d'une session décide (libérer, terminer, marquer `session-ended`) et les renouvellements de bail qui n'ont pas pu être poussés sont d'abord écrits dans une file locale, `.git/oh-claims-queue.json` du clone du team-state (jamais committée). Chaque opération porte ses conditions : la réservation doit encore appartenir au même membre et à la même session, sinon elle est abandonnée et journalisée. La file est appliquée en un commit (un pull, un push) dès que le dépôt d'équipe répond : à la fin de la session (attente bornée à 15 s dont 10 s pour le verrou du clone ; le démon patiente jusqu'à 2 min), à chaque battement du démon, par la tâche `claims-queue` du démon (au démarrage, puis toutes les 15 minutes). Hors ligne, ou si le dépôt reste occupé, rien n'est perdu : les opérations restent dans la file et oh le dit à la fin de la session. Une session qui démarre hors ligne sur un ticket que vous déteniez déjà reprend la réservation au retour du réseau ; ce que la session précédente avait mis en file sur cette réservation est annulé. `oh doctor` (`claims_queue`) avertit quand des opérations attendent depuis plus de 15 minutes, ou quand le fichier est illisible (il est alors mis de côté, pas supprimé) ; `oh doctor --fix` applique la file après confirmation.

**Une réservation libérée volontairement le reste.** La synchronisation du tracker ne prend plus une réservation libérée par son titulaire (`oh team release`, fin de session, board, rétention des réservations terminées) pour une réservation perdue par accident et ne la recrée plus. Une issue assignée à un membre dont la réservation a été libérée ne redevient une réservation `planned` que si elle est mise à jour sur le tracker ensuite.

**Limite de travail en cours.** La synchro met **toutes** les issues ouvertes assignées à un membre sur le board, sans plafond par membre : le board reflète le tracker. Ce qui borne un membre, c'est le nombre de tickets sur lesquels il travaille en même temps, `max_in_progress_per_member` (5 par défaut ; `0` = sans limite ; `[tracker]` du `config.toml` de l'équipe, ou `[tracker]` du `hub.toml` du membre, qui l'emporte). Elle compte les réservations du membre en cours (les colonnes `active` du board, `in_progress` par défaut) ; une réservation en revue (statut `review` ou label `review` / `ready-for-review`) ou en validation, une réservation prévue, bloquée ou terminée, et un ticket de la réserve ne comptent pas. À la limite, oh **refuse** que le membre en prenne ou en démarre un de plus, en donnant le compte et la clé : `oh team claim` (aussi la reprise d'une réservation expirée et `oh team claim transfer` vers soi), la touche de prise (`c`) du board d'équipe, une carte passée dans une colonne active ou transférée à soi sur le board, et une session lancée sur des tickets (locale ou distante, avant que rien ne soit créé). Passer un claim à un autre membre et le verdict de revue de merge request (une revue rejetée renvoie le claim en `in_progress`) ne sont pas contrôlés. La synchro ne refuse jamais rien : un ticket déjà en cours sur le tracker arrive en cours, même au-delà de la limite, et compte dès lors. Cette limite remplace `max_auto_plan_per_member`, qui bornait ce qu'une synchro créait : elle est **dépréciée**, encore lue quand `max_in_progress_per_member` est absente (et `oh config set tracker.max_auto_plan_per_member` écrit la nouvelle clé), signalée par `oh doctor` avec la commande à lancer, et retirée par la prochaine écriture du `config.toml` ou du `hub.toml`. C'est un réglage distinct de la policy `max_ticket_wip` ([conventions d'équipe](team-conventions.fr.md)), qui ne contrôle que `oh team claim`.

**Autres signalements.** `oh doctor` signale aussi les réservations orphelines (ticket clos, ou toutes les sessions qui l'ont traité terminées), avec la commande à lancer ; les réservations prises sous l'ancien ID d'un projet recréé sont rattachées au projet local par `oh doctor --fix` (voir [`oh doctor`](../reference/cli-infra.fr.md#oh-doctor)). Une réservation oubliée se libère à la main : `oh team release <ticket-id> -p <projet>`.
