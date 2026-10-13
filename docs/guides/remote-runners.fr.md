> [Read in English](remote-runners.en.md)

# Exécution distante : installer les runners GitLab CI

Une session oh peut tourner sur un **runner GitLab CI** au lieu de votre machine. Ce guide couvre la mise en place : le runner, le projet central `oh-runner` d'un groupe GitLab, et `oh remote setup`. L'exécution distante est disponible sur macOS et Linux ; sous Windows, oh reste en local.

> **Expérimental, désactivé par défaut (v5.0).** L'exécution distante n'a jamais été validée sur un vrai runner. Elle reste désactivée tant que vous ne l'activez pas explicitement sur votre machine :
>
> ```bash
> oh config set remote.experimental true
> ```
>
> Sans ce réglage, `oh remote setup`, `oh run --runtime remote`, l'option ☁ de la fiche de lancement, `oh session fetch` et `oh session resolve` refusent avec le message « mode distant expérimental, non validé : oh config set remote.experimental true ». Une fois activée, un avertissement reste affiché dans le récapitulatif du lancement, dans `oh remote status` et dans `oh doctor`. Réservez-la aux essais, avec des projets et des jetons de test : les protections du job (étapes git finales, construction de l'image, rejeu du journal Beads) sont en place et testées, mais le drapeau ne sera levé qu'après une recette sur un vrai runner.

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
- **L'image du job** est l'image de développement du projet (son Dockerfile de dev, ou une base Debian par défaut) plus une couche fine (oh, opencode, faux `bd`) ; elle doit contenir git 2.32 ou plus récent. Elle est construite sur le runner et gardée dans le registre de conteneurs d'`oh-runner`, avec un tag égal à son hash.

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

Depuis un projet du groupe (l'instance et le groupe sont proposés à partir de son remote `origin`), après avoir activé le mode expérimental :

```bash
oh config set remote.experimental true
oh remote setup --llm --teamstate --project acme/dev/api
```

oh :

1. vérifie l'accès à l'API avec votre jeton (gardé dans le trousseau, clé `openhub.remote.<cible>.token`) ;
2. crée le projet `oh-runner` dans le groupe s'il n'existe pas (privé, registres activés), ou le valide ;
3. vérifie que sa branche par défaut est **protégée** (les variables protégées ne sont transmises qu'aux branches protégées) ;
4. écrit `.gitlab-ci.yml` (refuse de remplacer un fichier qui n'a pas été généré par oh, sauf `--force`) ;
5. crée le jeton de déclenchement et le garde dans le trousseau ;
6. pose les variables CI : `OH_LLM_PROVIDER`, `OH_LLM_REGION` (non secrètes), `OH_LLM_KEY`, `OH_PROJECT_TOKEN_<id du projet>`, `OH_TEAMSTATE_TOKEN` (masquées et protégées) ;
7. crée, s'il manque, la **paire de clés du journal** d'`oh-runner` : clé privée X25519 en variable `OH_JOURNAL_KEY` (masquée, protégée, limitée au job `oh-run`), clé publique dans le dépôt (`oh-journal-key.pub`, lue par la machine de chaque membre à l'envoi). Une paire complète est gardée : relancer `oh remote setup` depuis une autre machine ne casse pas les sessions déjà envoyées ;
8. signale l'absence de runner en ligne avec l'étiquette ;
9. enregistre la cible dans `~/.oh/hub.toml` (`[[remote.targets]]`, sans aucun secret).

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
3. **envoi** dans le registre de packages d'`oh-runner` : le paquet de session par son hash (`oh-bundle/<hash>`, envoyé une seule fois) et l'enveloppe de la session (`oh-session/<hash>` : prompt, entrées, politique des checkpoints, instantané, et la clé du journal de la session **chiffrée** pour le job avec la clé publique d'`oh-runner`) ;
4. **déclenchement** du pipeline ; la session apparaît dans `oh session list` (☁) et l'avancement dans le claim du ticket.

Si le déclenchement échoue, les réservations faites par oh sont annulées. `--headless` ne s'applique pas au distant.

Politique des checkpoints dans le job (jamais d'approbation aveugle) : `remote: auto` est validé par le répondeur de politique d'oh ; `remote: defer` arrête proprement la session quand la MR est prête, la suite se fait en local après récupération. Un checkpoint vérifié (`verify:`) n'est jamais passé sans sa vérification : elle tourne dans le job (section 6, « Vérification des checkpoints »).

## 6. Dans le job : `oh runner`

Le job `oh-run` exécute `oh runner run` dans l'image du projet :

1. **Récupération** du paquet et de l'enveloppe de session avec le jeton du job (`CI_JOB_TOKEN`), vérifiés par leur hash ; la clé du journal est déchiffrée en mémoire avec `OH_JOURNAL_KEY` (jamais écrite, jamais visible du compte de l'agent ni des artefacts).
2. **Nettoyage** de l'environnement : toutes les variables secrètes (clé LLM, jetons, `CI_JOB_TOKEN`, mots de passe du registre) sont retirées du processus avant de lancer quoi que ce soit.
3. **Clone** du projet avec son jeton (`OH_PROJECT_TOKEN_<id>`, remis à git par un script `GIT_ASKPASS` éphémère, jamais dans une URL, une variable ni la configuration git) et création de la branche de la session.
4. **Session** : même code que sur votre machine — démon oh, proxy d'identifiants (seul détenteur de la clé LLM), serveur opencode et paquet en « monde fermé ». Le serveur opencode tourne sous le compte `oh` (créé dans l'image), qui ne peut lire ni les processus du job, ni la base oh, ni les secrets.
5. **Répondeur de politique** (jamais d'approbation aveugle) :

   | Demande | Réponse |
   |---|---|
   | checkpoint `remote: auto` | validé ; s'il est vérifié (`verify:`), il ne passe qu'une fois la vérification réussie |
   | checkpoint `remote: auto` dont la vérification a échoué | refusé (« corriger d'abord »), jamais passé outre : l'agent corrige et redemande |
   | checkpoint dont la vérification ne peut pas tourner | différé : la session s'arrête proprement, vous décidez en local |
   | checkpoint `remote: defer` | la session s'arrête proprement : branche et MR poussées, suite en local |
   | autre permission (`ask`) | refusée, avec un message à l'agent |
   | question d'un agent | la session s'arrête (« question en attente »), à reprendre en local |
   | erreur, budget, coupe-circuit | la session s'arrête (échec) |

6. **Vérification des checkpoints** (`verify:`) : lancée par le démon du job dans le clone, avec les mêmes commandes et les mêmes garanties que sur votre machine (environnement filtré, délai, sortie bornée, groupe de processus), **sous le compte `oh`** des agents, jamais sous root ; ses lectures git (état du travail, chemins modifiés) aussi. Le job attend la fin des vérifications en cours avant l'étape 8.

   **Lectures git d'oh dans le clone.** Le démon tourne sous root et le clone appartient au compte `oh` : git refuse alors de lire le dépôt (« dubious ownership »), et il a raison, car la configuration du dépôt est écrite par l'agent (un filtre, par exemple, qu'aucune option de ligne de commande ne neutralise, tournerait sous root). oh ne lève jamais ce refus (pas de `safe.directory`) : **toutes** ses lectures git dans le clone — fenêtre `commit`, garde de fermeture des tickets, contrôle de la configuration git du dépôt, référence de départ de la session, provenance des tickets fermés, nom de l'auteur Beads, jeton des hooks — tournent sous le compte `oh`, avec ses dossiers et sans rien de la configuration de root. Si une lecture échoue quand même, la protection reste fermée : la fenêtre `commit` se referme, `bd close` est refusé, la vérification est « impossible » (checkpoint différé), une condition `paths` est tenue pour vraie. Voir [Checkpoints › Sessions distantes](../reference/checkpoints.fr.md#sessions-distantes).
7. **Beads** : `bd` est le faux bd d'oh ; il passe par la passerelle Beads du démon du job, qui applique `beads.allow`. Les lectures viennent de l'instantané ; les écritures sont enregistrées dans `journal.jsonl` (et visibles par les lectures suivantes), puis rejouées sur votre machine à la récupération.
8. **Branche et MR** : tous les processus du compte `oh` sont d'abord tués ; oh ne lance jamais git dans le dépôt de l'agent (ses hooks, son `core.fsmonitor`, ses filtres et ses remotes ne servent pas) : il copie les objets et les références dans un dépôt neuf à lui. Ce qui n'est pas commité l'est (sans les dépôts imbriqués ni les fichiers qui n'appartiennent pas au compte `oh`), la branche est poussée et une **MR en brouillon** est ouverte par les options de `git push`. L'avancement est publié dans le claim du ticket (`OH_TEAMSTATE_TOKEN`).
9. **Artefacts** (`oh-out/`, 7 jours) : `journal.jsonl` (écritures Beads), `summary.json` (issue, coût, décisions, MR, sorties, résultat des vérifications de checkpoint), `session.export` (transcription de la session et de ses sous-agents, pour la reprendre en local), `reports.json` (rapports déposés par les agents avec `workflow_report`, schéma de pipeline 5).

Limites : les serveurs MCP qui ont besoin d'un jeton de votre machine (gitlab, jira, figma…) ne sont pas disponibles dans le job ; le serveur `workflow` d'oh l'est. Si le job est annulé ou dépasse sa durée, la session est exportée et le travail poussé avant l'arrêt quand c'est possible.

## 7. Suivre, récupérer, rejouer

**Suivi.** `oh session list` et la vue Sessions de la TUI consultent l'état des pipelines (toutes les 30 s dans la TUI) : la session ☁ est « En cours » (pipeline en attente ou en cours), puis passe dans **« À récupérer »** quand le pipeline est terminé (ou en échec : ses artefacts restent récupérables). Une notification système le signale, même oh fermé : le démon oh continue de consulter les pipelines toutes les minutes (et entretient les baux Beads des tickets réservés, `bd heartbeat`) tant qu'une session distante tourne. L'avancement est aussi visible par l'équipe dans le claim du ticket.

**Récupération** — `oh session fetch <id>` ou `g` dans la vue Sessions :

1. téléchargement des artefacts du job `oh-run` (`journal.jsonl`, `summary.json`, `session.export`, `reports.json`) dans `~/.oh/sessions/<id>/remote/` ;
2. résumé : issue (terminée, checkpoint différé, question, échec), coût, MR, demandes refusées par la politique, vérifications faites dans le job (importées pour la carte du checkpoint et la frise ; elles ne remplacent pas la vérification locale, relancée quand la session reprend) ;
3. **import de la session** (et de ses sous-agents) dans un serveur local, dans un worktree de la branche poussée par le job : `oh session attach <id>` la reprend là où elle s'est arrêtée (checkpoint différé à valider, question à répondre, suite du travail) ; les rapports du job sont importés dans la session (nettoyés et bornés de nouveau) : `oh session report <id>` et la carte du checkpoint les montrent.

`--no-import` ne télécharge que les artefacts.

**Rejeu du journal Beads** — `oh session resolve <id>` ou la fenêtre de la TUI :

- le journal est **authentifié** : le job le scelle (HMAC) avec une clé propre à la session, que votre machine garde dans son trousseau et qui ne figure ni dans les artefacts ni dans la copie locale de l'enveloppe, et qui ne voyage que chiffrée pour le job (les membres qui lisent le registre de packages d'`oh-runner` ne peuvent pas forger un journal) ; un journal, un instantané ou une liste `beads.allow` modifiés, ou une clé absente, et rien n'est rejoué ;
- chaque écriture est **revérifiée** sur la machine, lue comme `bd` la lit (même analyseur que la passerelle Beads : `-sclosed`, `-s=closed`, alias `done`…) : `beads.allow` du workflow (toute fermeture — `close`, `update --status closed|done`, statut du projet de catégorie `done` d'après `bd statuses`, statut inconnu, `duplicate`… — demande `close`), options d'une courte liste seulement (`--id`, `--db`, `-C`, `--repo`, fichiers du job refusés), et **chaque ticket nommé** — en argument comme dans `--parent` ou `--deps` — doit être un ticket de la session (ou créé par elle) ; les autres écritures sont refusées ;
- l'affichage neutralise les caractères de contrôle venus du job (séquences ANSI/OSC, retours chariot) : ils sont montrés, jamais interprétés ;
- un ticket **modifié sur la machine depuis l'envoi** (sa révision a changé) est un **conflit** : pour chacun, *Garder local* (rien n'est appliqué sur ce ticket), *Appliquer distant* (toutes ses écritures), *Fusionner les notes* (seulement les notes et commentaires) ;
- **rien n'est appliqué sans confirmation** ; `bd` exécute la forme canonique de ce qui a été vérifié ; les tickets créés dans le job reçoivent leur vrai identifiant et les écritures suivantes l'utilisent ;
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
| `oh-image` | image absente du registre | construit l'image de base (Dockerfile de dev du projet, avec le contexte de build envoyé par votre machine dans le registre de packages ; sinon Debian + git + ripgrep) puis la couche oh **à partir de la base désignée par son empreinte**, et les pousse ; publie l'image par son empreinte (`oh-image.env` : `OH_RUN_IMAGE=<dépôt>@sha256:…`, Kaniko `--digest-file`, Docker `docker push`). Ce job exécute les `RUN` du Dockerfile : il n'a que le jeton du job (`CI_JOB_TOKEN`), aucun secret d'oh ni jeton de projet |
| `oh-run` | toujours | `oh runner run` dans l'image du projet **désignée par son empreinte** (`OH_RUN_IMAGE`, jamais par son étiquette), dans l'environnement `oh-run/<id du projet>` ; artefacts dans `oh-out/` (7 jours) |

Les variables du déclenchement (identifiant de session, projet, branche, adresses du paquet et de l'image) ne sont pas secrètes : elles sont visibles dans la page du pipeline.

## Sécurité

- Les variables secrètes sont **masquées** (absentes des journaux), **protégées** (branche par défaut d'`oh-runner` seulement) et **limitées à l'environnement du job `oh-run`** : la clé LLM et le jeton team-state à `oh-run/*`, le jeton d'un projet à `oh-run/<id du projet>`. Les jobs de construction d'image ne les reçoivent pas, et le job d'un projet ne reçoit pas le jeton des autres. `oh remote setup` remplace les variables visibles de tous les jobs laissées par une version précédente ; `oh remote status` les signale.
- L'étiquette de l'image de base est l'empreinte du Dockerfile **et** des fichiers qu'il lit (sources des `COPY`/`ADD`, montages `RUN --mount=type=bind` ; tout l'arbre si elles ne peuvent pas être isolées). Seuls ces fichiers, au commit de départ, sont envoyés comme contexte de build.
- **Image du job par empreinte.** Le jeton du job de construction (`CI_JOB_TOKEN`) peut écrire dans le registre de conteneurs d'`oh-runner` : un Dockerfile malveillant d'un projet cible peut y remplacer l'étiquette de l'image d'un autre projet de la même cible. Le job `oh-run` ne prend donc jamais une image par son étiquette : après une construction, par l'empreinte que le job `oh-image` du même pipeline a poussée ; sinon par l'empreinte que **votre machine** a enregistrée lors de la première construction réussie (`oh.db`), et seulement si l'étiquette désigne toujours cette empreinte (sinon l'image est reconstruite, avec un avertissement). Une étiquette présente dans le registre mais jamais enregistrée par votre machine (construite par un autre membre, ou posée par un autre job) n'est pas utilisée : l'image est reconstruite une fois par machine.
- **Clé du journal chiffrée.** L'enveloppe de session (registre de packages) est lisible par les membres d'`oh-runner` et par les jobs de construction ; la clé du journal n'y figure que chiffrée pour le job (X25519, HKDF-SHA256, AES-256-GCM, liée à la session). Seul le job `oh-run`, qui reçoit `OH_JOURNAL_KEY`, l'ouvre.
- Risques résiduels : un projet cible peut encore effacer ou remplacer les étiquettes et les paquets génériques d'`oh-runner` (le pipeline suivant échoue sur l'empreinte ou reconstruit l'image : déni de service, pas d'exécution) ; le reste de l'enveloppe (prompt, entrées, instantané des tickets) reste lisible des membres d'`oh-runner` et des jobs de construction ; la prise en compte du dotenv dans `image:` relève de GitLab Runner (à valider sur votre instance : sans elle, le job `oh-run` échoue, il ne prend jamais l'étiquette). Ne regroupez dans une cible que des projets de même confiance. Restreignez les Maintainers d'`oh-runner` : ils peuvent lire les variables secrètes, `OH_JOURNAL_KEY` compris.
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
| Envoi refusé : « oh-runner n'a pas de paire de clés du journal » | `oh-runner` configuré par une version précédente d'oh : relancez `oh remote setup`. |
| Job `oh-run` : `the journal key of the session cannot be opened` | La paire de clés a été remplacée après l'envoi (variable `OH_JOURNAL_KEY` supprimée ou modifiée) : relancez `oh remote setup`, puis renvoyez la session. |
| Job `oh-run` : image vide ou invalide (`OH_RUN_IMAGE`) | Le job `oh-image` n'a pas publié l'empreinte, ou le runner ne lit pas le dotenv dans `image:` : voir le journal du job `oh-image`. |
| Envoi refusé : schéma du pipeline différent | Le pipeline a été généré par une autre version d'oh (le schéma 2 passe la version de l'outil dans `OH_TOOL_VERSION`) : relancez `oh remote setup`. |
