> [Read in English](remote-runners.en.md)

# Exécution distante : installer les runners GitLab CI

Une session oh peut tourner sur un **runner GitLab CI** au lieu de votre machine. Ce guide couvre la mise en place : le runner, le projet central `oh-runner` d'un groupe GitLab, et `oh remote setup`. L'exécution distante est disponible sur macOS et Linux ; sous Windows, oh reste en local.

> **v5.0 :** l'exécution distante est couverte par les tests (répondeur, contrat machine ↔ job, rejeu du journal, essai local complet de `oh runner run`), mais n'a pas encore été validée sur un vrai runner. Les serveurs MCP qui lisent un jeton dans le trousseau de la machine (gitlab, jira, figma…) ne sont pas disponibles dans le job (`workflow` l'est), et les build args de la config Exécution ne sont pas transmis à l'image distante.

## Principe

```
votre machine (oh)                          GitLab                                   runner (Docker)
──────────────────                          ──────                                   ───────────────
oh run … --runtime remote ──paquet──▶ registre de packages (oh-runner)
                          ──déclenche──▶ pipeline oh-runner ───────────────▶ job : image du projet
                                                                                oh runner : clone du projet,
                                                                                opencode + proxy LLM, branche + MR
oh session fetch ◀──artefacts── journal Beads, résumé, export de session ◀──
```

- **Un projet `oh-runner` par groupe GitLab.** Son `.gitlab-ci.yml` est **généré par oh** (`oh remote setup` le réécrit). La CI des projets n'est jamais modifiée.
- **Les secrets de votre machine ne partent pas.** Le job ne reçoit que des **variables CI masquées et protégées** du projet `oh-runner` : la clé LLM des jobs, un jeton d'accès par projet cible, un jeton d'écriture du team-state. Votre trousseau garde seulement le jeton API GitLab (le vôtre) et le jeton de déclenchement.
- **Beads reste sur votre machine.** Le job reçoit un instantané des tickets concernés et renvoie un journal de ses écritures, que vous rejouez à la récupération (avec résolution des conflits).
- **L'image du job** est l'image de développement du projet (son Dockerfile de dev, ou une base Debian par défaut) plus une couche fine (oh, opencode, faux `bd`). Elle est construite sur le runner et gardée dans le registre de conteneurs d'`oh-runner`, avec un tag égal à son hash.

## Ce qu'il faut

| Élément | Détail |
|---|---|
| Instance GitLab | gitlab.com ou auto-hébergée (16.0 ou plus récente). **Registre de conteneurs** et **registre de packages** activés (auto-hébergée : à activer par l'administrateur). |
| Groupe | Vous y êtes **Maintainer** (pour créer `oh-runner`, ses variables et son déclencheur). |
| Runner | Une machine Linux (ou un Mac avec Colima) avec Docker et `gitlab-runner`, exécuteur **Docker**, étiquette `oh`. 2 CPU et 4 Go de mémoire au minimum par job simultané. |
| Accès réseau du runner | GitLab et son registre ; `registry.npmjs.org` (binaire opencode Linux) ; `github.com` (binaire oh publié) ; le fournisseur LLM (ex. `bedrock-runtime.eu-west-1.amazonaws.com`) ; les registres des images de base (Docker Hub, `gcr.io` pour Kaniko). |
| Jeton personnel | Portée `api`, sur votre compte : utilisé par oh depuis votre machine (mise en place, envoi des paquets, suivi, artefacts). |
| Jeton de chaque projet cible | Jeton d'accès **de projet**, rôle Developer, portées `write_repository` et `read_api` : le job clone, pousse la branche et ouvre la MR (en brouillon, par les options de `git push`, sans portée `api`). |
| Jeton team-state | Jeton d'accès du dépôt team-state, portée `write_repository` : le job y publie l'avancement dans le claim du ticket. |
| Clé LLM des jobs | Une clé dédiée à la CI. Pour Bedrock : une **clé API Bedrock** longue durée (région `eu-west-1`) ; les profils AWS de votre machine ne sont pas utilisables dans un job. Anthropic et OpenRouter : une clé API. |

## 1. Installer le runner

### Créer le runner dans GitLab

Groupe › **Build › Runners › New group runner** (gitlab.com ou auto-hébergée) :

- **Tags** : `oh` ;
- décochez « Run untagged jobs » ;
- gardez le jeton `glrt-…` affiché.

Un runner d'instance convient aussi (auto-hébergée), du moment qu'il porte l'étiquette `oh` et qu'il est disponible pour le projet `oh-runner`.

### Installer `gitlab-runner` sur la machine

Linux (Debian/Ubuntu) :

```bash
curl -L "https://packages.gitlab.com/install/repositories/runner/gitlab-runner/script.deb.sh" | sudo bash
sudo apt-get install gitlab-runner
```

macOS avec Colima :

```bash
brew install gitlab-runner colima docker
colima start --cpu 4 --memory 8
brew services start gitlab-runner
```

### Enregistrer le runner

Choisissez la construction de l'image projet :

- **Kaniko** (par défaut, recommandé) : aucun mode privilégié.

  ```bash
  sudo gitlab-runner register --non-interactive \
    --url https://gitlab.example.com \
    --token glrt-XXXXXXXX \
    --executor docker \
    --docker-image alpine:3.20 \
    --description "oh runner"
  ```

- **Docker-in-Docker** : le runner doit être **privilégié**, ce qui donne au job le contrôle du démon Docker de la machine. À réserver à une machine dédiée.

  ```bash
  sudo gitlab-runner register --non-interactive \
    --url https://gitlab.example.com \
    --token glrt-XXXXXXXX \
    --executor docker \
    --docker-image docker:27 \
    --docker-privileged \
    --docker-volumes /certs/client \
    --description "oh runner (dind)"
  ```

  Puis `oh remote setup --builder dind`.

Dans `/etc/gitlab-runner/config.toml` (ou `~/.gitlab-runner/config.toml` sur macOS), réglez `concurrent` selon la mémoire disponible (un job ≈ 1 à 2 Go).

Vérifiez : `sudo gitlab-runner verify`, puis dans GitLab le runner apparaît « online ».

Architecture : un runner arm64 (Mac Apple Silicon, Graviton) fonctionne ; indiquez-le avec `oh remote setup --arch arm64`.

## 2. Créer les jetons

1. **Jeton personnel** (`api`) : Préférences › Jetons d'accès.
2. **Jeton de chaque projet cible** : projet › Paramètres › Jetons d'accès › rôle **Developer**, portées `write_repository` et `read_api`.
3. **Jeton team-state** : dépôt team-state › Paramètres › Jetons d'accès › rôle **Developer**, portée `write_repository`.
4. **Clé LLM des jobs** (ex. clé API Bedrock créée dans la console AWS, limitée à Bedrock).

Ne les collez jamais dans une commande : `oh remote setup` les demande en saisie masquée, ou les lit dans une variable d'environnement (`--token-env`, `--llm-key-env`, `--project-token-env`, `--teamstate-token-env`).

## 3. Configurer oh : `oh remote setup`

Depuis un projet du groupe (l'instance et le groupe sont proposés à partir de son remote `origin`) :

```bash
oh remote setup --llm --teamstate --project acme/dev/api
```

oh :

1. vérifie l'accès à l'API avec votre jeton (gardé dans le trousseau, clé `openhub.remote.<cible>.token`) ;
2. crée le projet `oh-runner` dans le groupe s'il n'existe pas (privé, registres activés), ou le valide ;
3. vérifie que sa branche par défaut est **protégée** (les variables protégées ne sont transmises qu'aux branches protégées) ;
4. écrit `.gitlab-ci.yml` (refuse de remplacer un fichier qui n'a pas été généré par oh, sauf `--force`) ;
5. crée le jeton de déclenchement et le garde dans le trousseau ;
6. pose les variables CI : `OH_LLM_PROVIDER`, `OH_LLM_REGION` (non secrètes), `OH_LLM_KEY`, `OH_PROJECT_TOKEN_<id du projet>`, `OH_TEAMSTATE_TOKEN` (masquées et protégées) ;
7. signale l'absence de runner en ligne avec l'étiquette ;
8. enregistre la cible dans `~/.oh/hub.toml` (`[[remote.targets]]`, sans aucun secret).

La commande est relançable : seul ce qui manque ou a changé est modifié. Ajoutez un projet cible plus tard avec `oh remote setup --project <chemin>`.

Plusieurs équipes, plusieurs instances : chaque groupe a sa cible (`--name`, `--url`, `--group`). Un projet est envoyé à la cible de la même instance dont le groupe le contient (le plus profond l'emporte) ; pour forcer, `[remote.projects]` dans `hub.toml` (`<id du projet oh> = "<cible>"`).

### Builds de développement

Une version publiée d'oh est téléchargée par le job depuis la release GitHub. Pour un build de développement :

```bash
make -C cli build-linux            # ARCH=arm64 pour un runner arm64
oh remote setup --oh-binary cli/bin/oh-linux-amd64
```

Le binaire est envoyé dans le registre de packages d'`oh-runner` (`oh-cli/<sha256>/oh-linux-<arch>`) ; à refaire à chaque nouveau build.

### Options

| Option | Effet |
|---|---|
| `--tag` | étiquette des runners (`oh`) |
| `--builder kaniko\|dind` | construction de l'image projet |
| `--arch amd64\|arm64` | architecture des runners |
| `--timeout 3h` | durée maximale d'un job |
| `--runner-project` | autre chemin que `<groupe>/oh-runner` |
| `--token-key` | réutiliser un jeton déjà dans le trousseau |
| `--no-create` | ne pas créer `oh-runner` |

Les mêmes réglages sont visibles dans la TUI : **Configuration › Distant** ; une modification est appliquée au pipeline par le prochain `oh remote setup`.

## 4. Vérifier : `oh remote status`

```bash
oh remote status
```

Affiche, par cible : accès API, projet, registres, branche protégée, pipeline à jour, jeton de déclenchement, variables manquantes, runners en ligne, binaire oh pour le job. `oh doctor` (et la vue Doctor de la TUI) reprend la même vérification.

## 5. Lancer une session distante

```bash
oh run ticket --tickets bd-42 --runtime remote
```

Avant l'envoi, oh vérifie que :

- le workflow autorise `remote` (`runtime.allowed`) et qu'aucun checkpoint qui attend une validation dans le mode choisi n'est `remote: forbid` ;
- le projet est sur une branche **poussée** : le job part de la branche distante (les modifications non commitées ne sont pas envoyées, un avertissement le rappelle) ;
- le Dockerfile de dev, s'il existe, est commité sur cette branche ;
- le pipeline d'`oh-runner` est celui de cette version d'oh.

Puis, pour chaque session :

1. **réservation** des tickets : `bd update <id> --claim` sur votre machine et claim dans le team-state (refus si un autre membre l'a déjà) ;
2. **instantané Beads** des tickets, de leurs dépendances et de leurs enfants (`beads-snapshot.json`), avec la révision de chacun pour détecter les conflits au retour ;
3. **envoi** dans le registre de packages d'`oh-runner` : le paquet de session par son hash (`oh-bundle/<hash>`, envoyé une seule fois) et l'enveloppe de la session (`oh-session/<hash>` : prompt, entrées, politique des checkpoints, instantané) ;
4. **déclenchement** du pipeline ; la session apparaît dans `oh session list` (☁) et l'avancement dans le claim du ticket.

Si le déclenchement échoue, les réservations faites par oh sont annulées. `--headless` ne s'applique pas au distant.

Politique des checkpoints dans le job (jamais d'approbation aveugle) : `remote: auto` est validé par le répondeur de politique d'oh ; `remote: defer` arrête proprement la session quand la MR est prête, la suite se fait en local après récupération.

## 6. Dans le job : `oh runner`

Le job `oh-run` exécute `oh runner run` dans l'image du projet :

1. **Récupération** du paquet et de l'enveloppe de session avec le jeton du job (`CI_JOB_TOKEN`), vérifiés par leur hash.
2. **Nettoyage** de l'environnement : toutes les variables secrètes (clé LLM, jetons, `CI_JOB_TOKEN`, mots de passe du registre) sont retirées du processus avant de lancer quoi que ce soit.
3. **Clone** du projet avec son jeton (`OH_PROJECT_TOKEN_<id>`, passé par l'environnement de git, jamais dans une URL ni un fichier) et création de la branche de la session.
4. **Session** : même code que sur votre machine — démon oh, proxy d'identifiants (seul détenteur de la clé LLM), serveur opencode et paquet en « monde fermé ». Le serveur opencode tourne sous le compte `oh` (créé dans l'image), qui ne peut lire ni les processus du job, ni la base oh, ni les secrets.
5. **Répondeur de politique** (jamais d'approbation aveugle) :

   | Demande | Réponse |
   |---|---|
   | checkpoint `remote: auto` | validé |
   | checkpoint `remote: defer` | la session s'arrête proprement : branche et MR poussées, suite en local |
   | autre permission (`ask`) | refusée, avec un message à l'agent |
   | question d'un agent | la session s'arrête (« question en attente »), à reprendre en local |
   | erreur, budget, coupe-circuit | la session s'arrête (échec) |

6. **Beads** : `bd` est le faux bd d'oh ; il passe par la passerelle Beads du démon du job, qui applique `beads.allow`. Les lectures viennent de l'instantané ; les écritures sont enregistrées dans `journal.jsonl` (et visibles par les lectures suivantes), puis rejouées sur votre machine à la récupération.
7. **Branche et MR** : ce qui n'est pas commité l'est, la branche est poussée et une **MR en brouillon** est ouverte par les options de `git push`. L'avancement est publié dans le claim du ticket (`OH_TEAMSTATE_TOKEN`).
8. **Artefacts** (`oh-out/`, 7 jours) : `journal.jsonl` (écritures Beads), `summary.json` (issue, coût, décisions, MR, sorties), `session.export` (transcription de la session et de ses sous-agents, pour la reprendre en local).

Limites : les serveurs MCP qui ont besoin d'un jeton de votre machine (gitlab, jira, figma…) ne sont pas disponibles dans le job ; le serveur `workflow` d'oh l'est. Si le job est annulé ou dépasse sa durée, la session est exportée et le travail poussé avant l'arrêt quand c'est possible.

## 7. Suivre, récupérer, rejouer

**Suivi.** `oh session list` et la vue Sessions de la TUI consultent l'état des pipelines (toutes les 30 s dans la TUI) : la session ☁ est « En cours » (pipeline en attente ou en cours), puis passe dans **« À récupérer »** quand le pipeline est terminé (ou en échec : ses artefacts restent récupérables). Une notification système le signale, même oh fermé : le démon oh continue de consulter les pipelines toutes les minutes (et entretient les baux Beads des tickets réservés, `bd heartbeat`) tant qu'une session distante tourne. L'avancement est aussi visible par l'équipe dans le claim du ticket.

**Récupération** — `oh session fetch <id>` ou `g` dans la vue Sessions :

1. téléchargement des artefacts du job `oh-run` (`journal.jsonl`, `summary.json`, `session.export`) dans `~/.oh/sessions/<id>/remote/` ;
2. résumé : issue (terminée, checkpoint différé, question, échec), coût, MR, demandes refusées par la politique ;
3. **import de la session** (et de ses sous-agents) dans un serveur local, dans un worktree de la branche poussée par le job : `oh session attach <id>` la reprend là où elle s'est arrêtée (checkpoint différé à valider, question à répondre, suite du travail).

`--no-import` ne télécharge que les artefacts.

**Rejeu du journal Beads** — `oh session resolve <id>` ou la fenêtre de la TUI :

- chaque écriture est **revérifiée** sur la machine (`beads.allow` du workflow, options refusées, fichiers du job, tickets de la session) ; les autres sont refusées ;
- un ticket **modifié sur la machine depuis l'envoi** (sa révision a changé) est un **conflit** : pour chacun, *Garder local* (rien n'est appliqué sur ce ticket), *Appliquer distant* (toutes ses écritures), *Fusionner les notes* (seulement les notes et commentaires) ;
- **rien n'est appliqué sans confirmation** ; les tickets créés dans le job reçoivent leur vrai identifiant et les écritures suivantes l'utilisent ;
- relançable : une écriture en échec est retentée, les autres ne sont pas rejouées.

```bash
oh session resolve <id> --dry-run                 # voir le rejeu
oh session resolve <id> --merge-notes bd-40 --yes # sans terminal
```

## Le pipeline généré

Trois jobs, déclenchés **uniquement** par oh (jamais à un push) :

| Job | Quand | Rôle |
|---|---|---|
| `oh-cli` | image absente du registre | récupère oh (release vérifiée par `checksums.txt`, ou binaire de développement vérifié par son SHA-256) et prépare la couche oh |
| `oh-image` | image absente du registre | construit l'image de base (Dockerfile de dev du projet, cloné avec le jeton du projet ; sinon Debian + git + ripgrep) puis la couche oh, et les pousse |
| `oh-run` | toujours | `oh runner run` dans l'image du projet ; artefacts dans `oh-out/` (7 jours) |

Les variables du déclenchement (identifiant de session, projet, branche, adresses du paquet et de l'image) ne sont pas secrètes : elles sont visibles dans la page du pipeline.

## Sécurité

- Les variables secrètes sont **masquées** (absentes des journaux) et **protégées** (branche par défaut d'`oh-runner` seulement). Restreignez les Maintainers d'`oh-runner` : ils peuvent lire ces variables.
- La clé LLM n'est lue que par le proxy d'identifiants d'oh dans le job ; opencode ne reçoit qu'un jeton de session.
- Les jetons de projet ont la portée minimale : dépôt (lecture/écriture) et lecture de l'API ; la MR est ouverte par les options de `git push`.
- Kaniko n'a pas besoin de mode privilégié ; Docker-in-Docker si, à réserver à une machine dédiée.

## Dépannage

| Symptôme | Cause probable |
|---|---|
| `Branche protégée — échec` | Protégez la branche par défaut d'`oh-runner` (Paramètres › Dépôt › Branches protégées). |
| `Runners en ligne — à vérifier` | Runner arrêté, sans l'étiquette `oh`, ou non disponible pour `oh-runner`. |
| `Registre de conteneurs et de packages — échec` | Registres désactivés sur le projet ou l'instance. |
| `Variable CI — échec — OH_LLM_KEY` | `oh remote setup --llm`. Une valeur de moins de 8 caractères ou sur plusieurs lignes ne peut pas être masquée par GitLab. |
| Job `oh-image` : `OH_PROJECT_TOKEN_<id> is missing` | `oh remote setup --project <chemin du projet>`. |
| Job `oh-cli` : échec du téléchargement | Le runner n'atteint pas `github.com` : envoyez le binaire avec `--oh-binary`. |
| `Pipeline généré — échec` (fichier non généré par oh) | `.gitlab-ci.yml` écrit à la main : `oh remote setup --force` le remplace. |
| Envoi refusé : schéma du pipeline différent | Le pipeline a été généré par une autre version d'oh (le schéma 2 passe la version de l'outil dans `OH_TOOL_VERSION`) : relancez `oh remote setup`. |
