> [Read in English](SECURITY.md)

# Politique de sécurité

## Versions supportées

| Version | Supportée |
|---------|-----------|
| 5.x     | Oui       |
| 4.x     | Non (opencode V1, fin de vie) |
| < 4.0   | Non       |

## Signaler une vulnérabilité

**Merci de ne pas signaler les vulnérabilités de sécurité via les issues publiques GitHub.**

### Méthode préférée

Utilisez les [GitHub Security Advisories](https://github.com/datichb/openhub/security/advisories/new) pour signaler une vulnérabilité de manière privée. Cela nous permet d'évaluer l'impact, de préparer un correctif et de coordonner la divulgation avant toute annonce publique.

### Si vous ne pouvez pas utiliser les GitHub Security Advisories

Ouvrez une issue publique qui demande seulement un contact privé (sans détail de la vulnérabilité). Un signalement contient :

- Description de la vulnérabilité
- Étapes de reproduction
- Version(s) affectée(s)
- Évaluation de l'impact potentiel
- Correctif suggéré (le cas échéant)

### À quoi s'attendre

- **Accusé de réception** sous 48 heures
- **Mise à jour du statut** sous 7 jours
- **Délai de correctif** selon la sévérité (critique : 72h, haute : 7 jours, moyenne : 30 jours)

Si la vulnérabilité est acceptée, nous allons :
1. Développer et tester un correctif
2. Publier un avis de sécurité
3. Publier une version corrigée
4. Créditer le rapporteur (sauf si l'anonymat est préféré)

Si elle est déclinée, nous fournirons une explication claire.

## Périmètre de sécurité

### Vue d'ensemble (oh v5)

- **Les secrets restent sur la machine.** opencode ne reçoit jamais de clé LLM ni de jeton d'intégration : il reçoit un jeton de groupe, échangé par le proxy d'identifiants du démon oh. Les conteneurs et les jobs distants ne reçoivent aucun secret de la machine.
- **Monde fermé.** Une session ne voit que les agents et les skills de son paquet de session, et c'est vérifié à chaque démarrage de serveur.
- **Beads reste sur la machine.** Hors machine, `bd` passe par une passerelle limitée par la liste blanche du workflow.
- **Le mode local n'est pas un bac à sable.** En local, le shell de l'agent tourne sous votre utilisateur (voir [Limites connues du mode local](#limites-connues-du-mode-local)). La vraie isolation vient des sessions en conteneur.
- **Garde-fous, pas barrières.** Les règles shell d'oh (motifs) arrêtent les erreurs d'un modèle et les contournements bon marché ; elles ne confinent pas un agent qui cherche à en sortir (voir [Ce que les règles shell ne protègent pas](#ce-que-les-règles-shell-ne-protègent-pas)). Les verrous qui ne dépendent pas des motifs sont : la validation des checkpoints vue par oh, les passerelles (Beads, MCP), le proxy d'identifiants et le conteneur.

Décisions : [ADR-041](docs/architecture/adr/041-closed-world-isolation.fr.md) (monde fermé), [ADR-044](docs/architecture/adr/044-credential-proxy-session-limits.fr.md) (proxy d'identifiants, restrictions), [ADR-045](docs/architecture/adr/045-execution-environments.fr.md) (environnements d'exécution), [ADR-046](docs/architecture/adr/046-beads-gateways.fr.md) (passerelles), [ADR-052](docs/architecture/adr/052-beads-identity-provenance.fr.md) (lecture seule Beads, auteur, provenance).

### Stockage des identifiants

openhub stocke les jetons d'API et les secrets via deux mécanismes :

- **Trousseau du système** (`go-keyring`) — méthode préférée, utilise le stockage sécurisé natif (macOS Keychain, GNOME Keyring, Windows Credential Manager)
- **Fichier chiffré (repli)** — quand le trousseau du système est indisponible, les secrets sont chiffrés au repos en **AES-256-GCM** avec dérivation de clé **Argon2id** (paramètres minimum OWASP : t=3, memory=64 Mo, threads=4). La phrase de passe est lue depuis la variable d'environnement `OH_PASSPHRASE` ou demandée de manière interactive.

Les secrets ne sont jamais stockés en clair sur le disque. `~/.oh` est en 0700 ; `oh.db` et ses fichiers WAL en 0600.

Les mots de passe des serveurs opencode (nécessaires pour attacher une session et pour que le démon pilote le serveur) ne sont pas dans `oh.db` : un fichier 0600 par groupe sous `~/.oh/run/server-passwords/` (dossier 0700), `oh.db` n'en garde que l'empreinte. Une base d'une version précédente est convertie au démarrage du démon (puis compactée) ; `oh doctor` signale un mot de passe resté en clair. Ces fichiers restent lisibles par votre utilisateur : le shell des agents n'y accède que par erreur (refus shell de `~/.oh/run`), pas par une barrière.

### Proxy d'identifiants LLM

Le démon oh (`ohd`) fait tourner un proxy d'identifiants sur `127.0.0.1` (une écoute par machine ; sous Linux, une seconde écoute sur l'adresse vue des conteneurs, acceptée seulement si c'est une adresse d'un pont du moteur de conteneurs de la machine — `docker0`, `br-…`, `podman…`, `cni…`, `virbr…` — et revérifiée au redémarrage du démon : jamais une adresse du réseau local ni d'un VPN, même si l'image du projet la donne).

- **Jetons de groupe.** Chaque groupe de serveur (version du paquet, projet, environnement d'exécution) reçoit un jeton aléatoire de 256 bits `ohs_…`, donné à opencode dans la variable du fournisseur. Il est révoqué à la veille, à l'arrêt ou à l'abandon du groupe ; sans jeton valide, le proxy répond 401.
- **La clé reste dans le trousseau.** Le proxy lit l'identifiant dans le trousseau (clé du projet, clé du fournisseur pour le projet, clé de l'équipe, clé du hub, puis un profil AWS pour Bedrock) et l'applique dans le transport, sur la requête finale : en-tête remplacé, ou signature AWS SigV4. Si l'identifiant ne peut pas être appliqué (par exemple des identifiants AWS expirés), la requête est refusée (502) et rien n'est envoyé au fournisseur.
- **Liste blanche des chemins.** Seuls les chemins d'inférence de chaque fournisseur sont relayés ; tout autre chemin reçoit 404. Chaque segment est décodé une seule fois ; segments vides, `.`, `..` et double échappement sont refusés.
- **Hôte du fournisseur vérifié.** La région Bedrock doit être un nom de région AWS (`eu-west-1`…) : vérifiée à la configuration (`oh config set provider.bedrock.aws_region`, `oh provider`), au lancement (y compris `AWS_REGION` et le profil), à l'émission du jeton et à chaque requête ; une valeur comme `x.evil.com#` est refusée avant tout envoi. Une région bien formée où Bedrock n'est pas connu donne un avertissement. Chaque requête ne part qu'en HTTPS vers l'hôte du fournisseur (`bedrock-runtime.<région>.amazonaws.com`, `api.anthropic.com`, `openrouter.ai`, `api.openai.com`).
- **Liste blanche des modèles.** Quand une liste de modèles est active (restrictions des sessions), la clé JSON `model` doit correspondre exactement ; un doublon, une variante de casse ou des données après l'objet JSON sont refusés (400). Chez OpenRouter, `route`, `preset` et un préréglage dans l'identifiant (`…@preset/x`) sont refusés, et chaque modèle de repli de `models` doit être dans la liste (403). Une liste de modèles illisible en base n'est jamais lue comme « tout modèle » : le jeton n'est pas restauré.
- **Taille limitée.** Les corps de requête sont limités à 64 Mio (413 au-delà).
- **Pas de repli silencieux.** La configuration rendue interdit tout fournisseur autre que celui de la session : opencode ne peut plus basculer sur ses modèles hébergés quand un modèle est indisponible.
- **Empreintes seules.** La base ne contient que l'empreinte SHA-256 de chaque jeton et une référence à l'identifiant, jamais le jeton ni le secret. Les jetons gardés en clair par une version précédente sont remplacés par leur empreinte au démarrage du démon. Une empreinte lue dans `oh.db` n'est pas acceptée par le proxy.

### Le démon oh

- **Socket.** `~/.oh/run/ohd.sock`, droits 0600. L'identité du pair est vérifiée par le noyau à chaque connexion (UID, `LOCAL_PEERCRED` sous macOS, `SO_PEERCRED` sous Linux) : seuls les processus de votre utilisateur sont acceptés. Sur les autres systèmes, seuls les droits 0600 s'appliquent.
- **Capacité d'émission.** Les routes qui donnent un accès (jetons du proxy et des passerelles, identifiants, écoutes supplémentaires du proxy) ou arrêtent le démon exigent en plus l'en-tête `X-Oh-Capability`. C'est un secret aléatoire gardé dans le trousseau du système (`openhub.daemon.capability`) ou, sans trousseau utilisable, dans `~/.oh/run/capability` (0600, signalé par `oh doctor`). Il n'est lu que par la CLI oh et le démon, comparé à temps constant, et jamais placé dans un environnement.
- **Crochets.** Les routes du plugin oh et des passerelles (`/oh/v1/hooks/*`) sont authentifiées par le jeton du groupe.
- **Routes du workflow.** `/v1/workflow/status`, `/checkpoint` et `/outputs` (dorsale du serveur MCP `workflow`) exigent le jeton du groupe de serveur de la session, que le serveur MCP lit dans l'environnement du serveur de l'outil (jamais dans le shell d'une session) ; la session doit appartenir à ce groupe (401/403 sinon). Les routes que seul oh appelle (`/v1/workflow/rules`, politiques de sortie `/groups/{g}/policy`, clients attachés `/v1/clients`) exigent la capacité. Un battement de cœur maintient un client 5 minutes au plus.
- **Checkpoints liés à la validation de l'utilisateur.** Un checkpoint en pause dans le mode de la session ne passe qu'avec une validation vue par oh : donnée dans oh (TUI, CLI), ou dans l'interface de l'outil (oh lit la réponse à la demande de permission dans le flux d'événements). Tout autre appel (socket, requête MCP forgée) est refusé et ne déverrouille rien ; l'agent reçoit la consigne d'attendre la validation. Une réponse « toujours » enregistrée par l'outil ne laisse pas passer les checkpoints suivants : oh redemande.
- **Contrôles.** `oh doctor` signale le stockage de la capacité, les droits et le propriétaire du socket, et les jetons restés en clair dans `oh.db`.
- **Windows.** Le démon tourne dans le processus oh (pas de démon en arrière-plan).

### Monde fermé

Une session ne voit que les agents et les skills de son paquet. C'est obligatoire et non réglable.

- Les agents natifs d'opencode sont désactivés ; les skills intégrées sont refusées ; pour chaque agent, la délégation n'est permise que vers les cibles du graphe du workflow.
- La configuration et les données opencode du projet sont exclues (`OPENCODE_DISABLE_PROJECT_CONFIG=1`, dossier de données propre au groupe).
- Le plugin oh retire tout agent ou skill hors paquet (deuxième barrière).
- **`Attest`** s'exécute après chaque démarrage de serveur et pour chaque nouveau dossier : il compare au paquet les agents, les skills visibles pour chaque agent (sur les règles effectives que renvoie l'outil) et les serveurs MCP. Tout élément inattendu fait échouer le lancement ; le serveur est arrêté et son jeton révoqué.
- `[execution] strict_isolation` masque en plus votre configuration opencode d'utilisateur en local. En conteneur et à distance, elle n'est jamais visible.

- **Intégrité du paquet de session** : un paquet est partagé par toutes les sessions de même empreinte. Ses fichiers sont en lecture seule une fois construit (0444, dossiers 0555) ; l'outil d'édition des agents le refuse ; son empreinte est recalculée quand il est réutilisé (un paquet modifié est reconstruit) et quand oh le charge (un paquet modifié n'est jamais utilisé). Un paquet téléchargé depuis un job distant est refusé s'il ne correspond pas à son empreinte.

Le monde fermé porte sur ce que le modèle voit, pas sur ce que le shell peut faire.

### Passerelles

**Passerelle Beads** (local, conteneur et job distant). Le faux `bd` envoie la commande au démon (`/oh-gateway/beads/v1/exec`), qui lance le vrai `bd` sur la machine. En local, le faux `bd` (`~/.oh/run/bin/bd`) passe en tête du `PATH` de la session et de ses sous-agents ; la règle shell de chaque agent refuse un `bd` appelé par un chemin (`/opt/homebrew/bin/bd …`, `./bd`), qui contournerait la passerelle :

- **Jetons `ohg_…`**, un par session et par sous-session, transmis par l'environnement de session. Le démon n'en garde que l'empreinte (`~/.oh/run/gateway.json`, 0600). Ils sont valides tant que la session est ouverte et éveillée, et révoqués avec le groupe.
- **Liste blanche** = `beads.allow` du workflow. Sans bloc `beads:`, lecture seule ; une liste vide refuse tout. Une liste qui ne nomme que des commandes de lecture (liste par défaut comprise) est en lecture seule : les formes d'écriture de ses commandes sont refusées aussi (`comments add`, `ready --claim`).
- **`--readonly` sur les lectures** : chaque commande de lecture (liste fermée d'oh, `show`, `list`, `query`, `comments`…) est lancée avec `--readonly`, pour que bd refuse lui-même une écriture derrière la liste blanche. Deuxième protection seulement : bd 1.3.1 laisse encore passer `config set`, `hooks install`, `backup` et `import` dans ce mode.
- **Auteur fixé par oh** : chaque écriture porte `--actor=<personne> via oh/<agent>` (`user.name` de git dans l'emplacement, fixé à l'émission du jeton ; agent d'entrée ou sous-agent), enregistré dans `bd history --events`. Une réservation garde la personne seule (bd fait de l'auteur l'assigné). Les agents ne peuvent pas choisir l'auteur (`--actor` refusé). oh relie les tickets réservés et fermés à la session et au commit (`bd provenance record --source=oh`) ; les agents ne peuvent pas enregistrer de provenance. Le démon renouvelle les baux (`bd heartbeat`) des tickets réservés par les sessions ouvertes. `find-duplicates --method ai`, qui envoie les tickets à un modèle extérieur, est refusé. Les mémoires du projet (`bd remember`) n'arrivent dans les prompts de session qu'entre balises de données.
- **Arguments lus comme `bd` les lit** : la passerelle analyse la commande avec la table des commandes et options de `bd` (générée depuis `bd` 1.3.1) : formes collées (`-sclosed`, `-s=closed`, `--status=closed`), options courtes combinées (`-qC/autre`), options avant ou après la sous-commande, alias (`done`). Une option ou une commande inconnue de la table est refusée (le vrai `bd` la refuserait aussi ; celle d'une version plus récente ne peut pas être vérifiée). Le vrai `bd` reçoit ensuite la forme canonique de ce qui a été vérifié (`--nom=valeur`, puis `--` et les arguments).
- **Options refusées** : les options qui changent de base ou de dossier (`--db`, `--database`, `-C`/`--directory`, `--global`, `--repo`…) ou l'auteur (`--actor`), sous toutes leurs formes.
- **Base fixée** : l'espace Beads de la session est déterminé par `bd where` quand le jeton est émis ; le vrai `bd` est ensuite toujours lancé avec `BEADS_DIR` sur cet espace (un `.beads` créé dans un sous-dossier est ignoré), et refusé si `redirect`, `metadata.json` ou `config.yaml` de cet espace ont changé depuis. Les agents ne peuvent pas écrire dans `.beads/` (outil d'édition, et règles shell sur `redirect`, `metadata.json`, `config.yaml` et les redirections vers `.beads/`).
- **Fermetures reconnues** : `close` (`done`), `update --status` vers `closed` ou un statut de catégorie `done` de la configuration du projet (demandé à `bd statuses`), `duplicate`, `supersede`, `import`, `batch`, `sql`, `todo done`, `gate resolve`, `mol squash|burn` passent par le verrou de fermeture du workflow.
- **Chemins** traduits vers la machine et limités aux emplacements de la session (liens résolus), y compris sous forme collée (`-o/etc/x`) ; le paquet et les données du groupe sont exclus.
- Le vrai `bd` est lancé sans shell, dans le dossier de la session (délai de 2 min, 8 Mio par flux).
- **Démarrage du shell** : le shell d'une session locale lit les fichiers de démarrage d'oh (`ZDOTDIR` pour zsh, `BASH_ENV` pour bash, `~/.oh/run/shell/`) : ils chargent les fichiers de l'utilisateur (`~/.zshenv`…), puis remettent le faux `bd` en tête, pour qu'un vrai `bd` installé par npm, volta ou dans `~/.local/bin` ne passe pas devant. Le démon le vérifie une fois par session (sinon alerte ✗ dans « À traiter »), `oh doctor` aussi.
- **Hooks git de Beads** (`bd hooks run <hook>`, lancés par git avec `BD_GIT_HOOK=1`) : ils passent par la passerelle quelle que soit la liste `beads.allow`, pour qu'un commit ne soit jamais bloqué par la liste. `BD_GIT_HOOK` seul n'ouvre rien : le faux `bd` d'une session locale demande au démon, sur son socket, un **jeton de hook à usage unique** (30 s, lié à la session et au hook) ; le démon ne le donne que si le processus appelant est lancé par git depuis le hook de l'emplacement de la session (arbre des processus lu sur la machine, macOS et Linux ; ailleurs, seule la forme de la commande est vérifiée). Seuls `hooks run <hook de bd>` avec les arguments que git passe à ce hook (nombre et forme) sont acceptés. Hors de la machine (conteneur, job distant), ils ne sont pas lancés sur la machine (les hooks que bd enchaîne s'y exécuteraient hors du conteneur).
- **Hooks git** (garde-fou) : sont refusés à tous les agents `--no-verify` et ses abréviations, `-n` seul ou combiné aux options courtes d'un commit (`-an`, `-qn`, `-aqn`…), `-c core.hooksPath=…` et toute option `git -c`, `git config core.hooksPath`, `GIT_CONFIG_*`, les inclusions de configuration, l'écriture dans `.git/`, `.beads/hooks/`, `.husky/`, `.githooks/` (shell), l'édition de `.git/`, du fichier `.git` d'un worktree et de `.beads/hooks/` ; l'édition des fichiers de crochets du projet (`.husky/*`, `.githooks/*`, `lefthook*.yml`, `.pre-commit-config.yaml`) demande confirmation.
- **Commit et fermeture verrouillés par un checkpoint** (`unlocks:`, ex. `cp-2` de `ticket`) : avant sa validation, `git commit` (et les commandes qui créent un commit : `git revert`, `pull`, `merge`, `cherry-pick`, `am`, `rebase`, `stash pop`), `git push` et la fermeture d'un ticket sont refusés à tous les agents ; ensuite la passerelle ne ferme un ticket que si le travail est commité : HEAD a avancé depuis le commit du checkpoint (sur ce commit), au moins un commit modifie des fichiers suivis hors de `.beads/` (un commit vide ne compte pas) et il ne reste rien à committer. Une erreur git ou du verrou refuse la fermeture.
- **Environnement de `bd`** : le vrai `bd` reçoit un environnement minimal (`PATH`, `HOME`, utilisateur, langue, dossier temporaire, dossiers XDG, `BEADS_*`), jamais les autres variables du démon. Le jeton d'un sous-agent est révoqué à la fin de son exécution et redonné s'il reprend. Les autorisations gardées dans `~/.oh/run/gateway.json` sont signées par le démon (clé dérivée de sa capacité) : un fichier modifié, mal formé ou lisible par d'autres est ignoré au redémarrage.
- **Garde-fou global** (dernier de chaque agent, quelle que soit sa base de permissions, aussi sans workflow à verrous ; appliqué par le plugin oh, rendu dans chaque agent sans lui) : commandes git qui jettent du travail n'importe où dans la ligne (après `env`, `X=y`, `sh -c`, `git -C`) ; options et variables git qui lancent un programme ou écrivent un fichier (`--output`, `--upload-pack`, `--receive-pack`, `--ext-diff`, `--textconv`, `fsmonitor`, `GIT_DIR`, `GIT_SSH*`, `GIT_EXTERNAL_DIFF`, `GIT_PAGER`…) ; le socket et les données d'oh (`ohd.sock`, `--unix-socket`, `~/.oh/run`, `~/.oh/bundles`, `oh.db`, `oh mcp serve`) ; les secrets et l'API du serveur opencode ; `find -exec|-delete|-ok…` ; les fichiers de démarrage du shell, `~/.ssh`, `~/.aws`, `~/.kube`, `~/.gitconfig`, `~/.netrc`, `~/.npmrc`, `crontab`, `LaunchAgents` ; les installations système (`npm -g` dans tous les ordres, `--location=global`, `yarn global`, `pnpm -g`, `uv --system`, `--break-system-packages`, `sudo`) ; les options docker/podman qui sortent du conteneur (`--privileged`, montages de la machine, `--network host`, socket docker…). Les outils de context-mode qui exécutent du code (`ctx_execute`…) n'ont jamais plus que le shell de l'agent.

**Passerelle MCP** (hors machine). Les serveurs MCP d'oh du paquet tournent sur la machine, lancés par le démon depuis le paquet immuable ; ils lisent eux-mêmes leurs jetons dans le trousseau. Le conteneur les joint en HTTP avec le jeton du groupe (`{env:…}`, jamais écrit dans la configuration). Un appel dont le `_meta` désigne une session d'un autre groupe est refusé (403).

### Gestion des jetons MCP

Les jetons des serveurs MCP (GitLab, Figma, Jira, Linear, GitHub, Google) sont :

- stockés dans le trousseau du système ou le fichier chiffré (jamais dans des fichiers de configuration en clair) ;
- lus par le processus `oh mcp serve` lui-même, sur la machine : le paquet ne contient que le nom de la clé du trousseau (`--token-key`) ;
- jamais écrits dans le paquet, la configuration opencode ou l'état de session, jamais journalisés, jamais transmis aux fournisseurs IA, jamais placés dans un conteneur.

### Sessions en conteneur

- **Aucun secret dans l'environnement du conteneur.** L'environnement est passé par `--env-file` (0600) et reste minimal, sans aucune variable de la machine. Les seuls identifiants que voit le shell sont le jeton `ohs_…` et un jeton `ohg_…` (avec `OH_GATEWAY_URL`).
- **Paquet en lecture seule** sur `/opt/oh/bundle`. `HOME` est un volume propre au groupe de serveur, vidé à chaque démarrage du conteneur : votre configuration n'est jamais visible et rien de ce qu'un agent y laisse (`.zshenv`…) ne passe à une autre session. `~/.oh` n'est pas monté : ni le socket du démon ni `oh.db` ne sont joignables.
- **Fichiers git qui lancent des programmes en lecture seule** : `.git/config`, `.git/hooks`, `info/attributes`, et pour un worktree son fichier `.git` et les liens `commondir`/`gitdir`, dans toutes les vues du dépôt dans le conteneur. Le git de la machine (oh, puis vous après la session) n'exécute donc rien qu'un agent y aurait déposé. Conséquence : `git push -u` et `git branch --set-upstream-to` (qui écrivent la configuration) échouent dans le conteneur ; les branches se créent sans configuration de suivi (`branch.autoSetupMerge=false`).
- **Conteneur durci** : `--cap-drop=ALL`, `--security-opt=no-new-privileges`, `--pids-limit=4096` ; les volumes de cache refusent les virgules, guillemets, `=`, `..` et les cibles sous `/opt/oh`. Le fichier d'environnement (jetons de la session) est supprimé peu après le démarrage du conteneur et à son arrêt.
- Le port d'opencode n'est publié que sur `127.0.0.1`. Un montage hors des dossiers partagés avec la VM est refusé.
- Le conteneur est supprimé à l'arrêt ou à la mise en veille de la session (`run --rm --init`).

### Sessions distantes (GitLab CI)

> **Expérimental, désactivé par défaut en 5.0.** Le mode distant n'a jamais été validé sur un vrai runner (ses protections, décrites ci-dessous, sont testées hors runner). oh le refuse tant que `[remote] experimental = true` n'est pas posé (`oh config set remote.experimental true`) ; ne l'activez que pour des essais, avec des projets et des jetons de test.

- **Variables CI masquées et protégées.** Le job ne reçoit que les variables CI du projet `oh-runner` : clé LLM des jobs, un jeton d'accès par projet cible, un jeton d'écriture du team-state. Elles sont masquées (absentes des journaux) et protégées (branche par défaut d'`oh-runner`, qui doit être protégée). `oh remote setup` lit les secrets en saisie masquée ou dans une variable d'environnement, jamais dans les arguments ; `[[remote.targets]]` dans `hub.toml` ne contient aucun secret. La machine ne garde que votre jeton API GitLab et le jeton de déclenchement, dans le trousseau.
- **Construction de l'image sans secret.** Les variables secrètes sont limitées à l'environnement du job `oh-run` (`oh-run/*`, et `oh-run/<id du projet>` pour le jeton d'un projet) : les jobs qui exécutent les `RUN` d'un Dockerfile de projet n'ont que leur `CI_JOB_TOKEN`, et lisent le contexte de build envoyé par la machine (jamais de jeton dans une URL). Ce jeton peut écrire dans le registre de conteneurs d'`oh-runner` : le job `oh-run` ne prend donc jamais son image par étiquette, seulement par empreinte (celle que le job de construction du même pipeline a poussée, ou celle que la machine a enregistrée à la première construction réussie et dont l'étiquette n'a pas changé). Risque résiduel : effacement ou remplacement d'étiquettes et de paquets (déni de service, pas d'exécution) ; ne regroupez dans une cible que des projets de même confiance.
- **Aucun secret dans les variables du pipeline ni dans les artefacts.** Les entrées et le prompt voyagent dans l'enveloppe de session (registre de packages), pas dans les variables du pipeline. Les artefacts (`journal.jsonl`, `summary.json`, `session.export`) et les messages d'erreur sont expurgés des valeurs secrètes.
- **Dans le job.** `ScrubEnv` retire toute variable dont le nom contient `TOKEN`, `PASSWORD`, `SECRET`, `_KEY`, `PASSPHRASE`, `CREDENTIAL`, `JWT` ou `AUTH` avant le démarrage du démon. La clé LLM n'est détenue que par le proxy du job. opencode tourne sous le compte `oh` (uid 10001), qui ne peut lire ni les processus du job, ni `oh.db`, ni les secrets. Le jeton du projet n'est remis à git que par un script `GIT_ASKPASS` éphémère (fichier 0600 du compte root, supprimé après l'appel), jamais en argument, dans une URL, une variable d'environnement ou la configuration git.
- **Étapes git finales sans rien de l'agent.** Avant le commit et le push de fin, oh tue tous les processus du compte `oh`. Il ne lance jamais git dans le dépôt de l'agent : il copie les objets, les références et `HEAD` dans un dépôt neuf qui lui appartient (ni configuration, ni hooks, ni index, ni `alternates` de l'agent), et lit l'arbre de travail à travers lui, avec `core.hooksPath=/dev/null`, `core.fsmonitor=false`, `GIT_CONFIG_GLOBAL=/dev/null` et `GIT_CONFIG_NOSYSTEM=1`. Les dépôts imbriqués et les fichiers qui n'appartiennent pas au compte `oh` (liens physiques vers des fichiers de root) ne sont pas commités. La branche est poussée vers l'URL du projet choisie par la machine, jamais vers un remote de l'agent.
- **Rejeu Beads authentifié et borné.** Le journal du job est scellé (HMAC) avec une clé propre à la session, gardée dans le trousseau de la machine et transmise au job **chiffrée** pour la clé publique d'`oh-runner` (X25519 + AES-256-GCM ; clé privée en variable CI masquée et protégée du seul job `oh-run`) : les membres qui lisent le registre de packages ne peuvent pas forger un journal ; avant tout rejeu, oh vérifie ce sceau et celui de sa copie locale de l'enveloppe (liste `beads.allow`, instantané). Chaque écriture est revérifiée, lue par l'analyseur d'arguments de `bd` de la passerelle Beads, puis exécutée sous sa forme canonique : courte liste d'options, tickets de la session seulement (y compris dans `--parent` et `--deps`), toute fermeture (statut `closed`/`done`, statut du projet de catégorie `done`, statut inconnu, alias) traitée comme `close`. Les caractères de contrôle du journal sont neutralisés à l'affichage.
- **Aucune approbation à l'aveugle.** Un répondeur de politique traite les décisions, jamais `--auto` : les checkpoints `auto` sont validés, toute autre permission est refusée, un checkpoint différé ou une question arrête le travail.

### Restrictions des sessions (I6)

Optionnelles et désactivées par défaut (`oh budget show|set|unset|raise`) : sessions actives max, budget par session et budget journalier en USD, plafond mémoire, liste des modèles autorisés (appliquée par le proxy sur le jeton du groupe). Cascade : workflow (`limits:`) > projet > hub > recommandé d'équipe ; l'imposé d'équipe est un plafond. Les budgets sont des plafonds souples : contrôlés en fin d'étape, ils lèvent alors une décision `$`.

### Limites connues du mode local

- Le shell de l'agent tourne sous votre utilisateur. Il peut toujours atteindre le socket du démon et `oh.db`, mais le socket ne fait plus passer un checkpoint (voir plus haut).
- **Environnement des autres processus** : un processus de votre utilisateur peut lire l'environnement d'un autre (`ps eww` sous macOS, `/proc/<pid>/environ` sous Linux), dont celui du serveur opencode (mot de passe, jetons) ; ces commandes sont refusées aux agents comme garde-fou, pas comme barrière.
- **git d'oh sur la machine** : oh lance git dans vos dépôts (état avant un checkpoint, worktrees, diff, nettoyage Beads) en neutralisant ce que la configuration du dépôt pourrait exécuter (`core.fsmonitor`, hooks, `diff.external`, textconv, `core.sshCommand`, `uploadpack`/`receivepack`, assistants d'identification, URL `ext::`), et refuse de passer un checkpoint qui déverrouille une opération quand la configuration ou les hooks git ont changé depuis le début de la session ; une alerte ✗ dans « À traiter » (`oh session inbox`, vue Sessions, notification) nomme le fichier modifié et dit quoi faire (vérifier, restaurer le fichier, ou relancer la session).
- Sans la capacité, il ne peut pas obtenir de nouveaux jetons, mais un processus de votre utilisateur peut lire l'entrée du trousseau ou le fichier de capacité, ou lancer `oh` lui-même. Ces protections compliquent l'abus ; elles n'isolent pas l'agent.
- La clé LLM n'est jamais dans l'environnement de l'agent ni dans la configuration d'opencode, mais votre trousseau est lisible par votre utilisateur.
- Le shell d'une session ne reçoit jamais le mot de passe du serveur opencode, sa configuration en ligne, le jeton du groupe ni celui du fournisseur : le plugin oh les retire de tout shell, y compris sans environnement de session (sous-agent pas encore adopté, session ouverte dans l'interface de l'outil). Sans plugin oh actif (mode de repli, signalé dans le journal), un shell sans environnement de session les hérite encore. Les commandes qui visent ces variables ou l'API du serveur (réponses aux permissions et aux questions, environnement de session) sont refusées à tous les agents, comme garde-fou.
- Sans isolation stricte, votre configuration opencode d'utilisateur est chargée ; `Attest` refuse ce qui ajoute des agents, des skills ou des serveurs MCP visibles, pas les autres réglages.
- Les agents de développement livrés (base `developer-rw`) n'installent des dépendances que dans le projet : `pip` seulement dans l'environnement virtuel du projet (`.venv/bin/…`, `uv`, `poetry`), pas de `npm -g`, `yarn global`, `pnpm add -g`, `gem install` ni `--break-system-packages`. Leur liste blanche est un garde-fou, pas un bac à sable : lanceurs de tests et outils de build exécutent le code du projet.

- **Infrastructure** : l'agent `infra` lit librement (`terraform plan`, `kubectl get|describe|diff`, `helm template`, `aws … describe-*|list-*`…), demande avant les lectures de secrets (`terraform output`, `kubectl get secret`, `vault kv get`, `aws … get-*`) et les tunnels ; toute modification (`terraform apply`, `kubectl apply|delete|exec…`, `helm install|upgrade`, `aws … create-*|delete-*|put-*…`, `pulumi up`, `vault write`…) est refusée tant qu'un checkpoint `unlocks: [apply]` n'est pas passé (toujours refusée dans un workflow qui n'en déclare pas), puis demandée une à une. L'extraction d'identifiants (`get-secret-value`, `--with-decryption`, `get-login-password`, `assume-role`, `print-access-token`, `--show-secrets`, `kubectl config view --raw`…) est refusée à tous les agents. Limite : `terraform plan` et `pulumi preview` exécutent du code du dépôt (fournisseurs, programmes) ; la vraie protection est un identifiant en lecture seule (rôle `ReadOnlyAccess`, kubeconfig `view`) ou une session en conteneur sans `~/.aws` ni `~/.kube`.
- **Reviewer** : shell en lecture (git en lecture, `bd show`) plus les commandes de build, de tests, de lint et de typecheck de la stack détectée (`go test`, `npm test`, `npm run build`, `pytest`, `cargo test`, `make test`… ; `npx --no`, `uv run --no-sync` : jamais d'installation), autorisées dans les workflows sur le code de l'équipe et demandées dans `review` (`risk: read` : lancer les tests d'une merge request d'un tiers exécute son code). Les options qui modifient le projet (`--fix`, `--write`, mise à jour de snapshots et de goldens) sont refusées. Comme tout lanceur de tests, ces commandes exécutent le code du projet.
- **Debugger et benchmarker** : le `debugger` n'a plus de shell libre (lecture, git en lecture, Beads, lanceurs de tests soumis à confirmation) ; le `benchmarker` ne lit plus l'environnement (`env`, `printenv`) et demande avant les outils qui lancent une commande arbitraire (`hyperfine`, `perf`, `wrk`, `py-spy`).

Considérez que l'agent agit avec les droits de votre utilisateur. La vraie isolation vient des sessions en conteneur.

### Ce que les règles shell ne protègent pas

Les règles shell sont comparées par opencode au texte brut de chaque commande de la ligne (vérifié sur opencode 2.0.20) :

- **Guillemets et espaces** ne sont pas normalisés : `git pu''sh` ou `git  push` (deux espaces) ne correspondent pas à `git push*`.
- **Déclarations** : `export X=…`, `declare`, `local`… ne sont pas vérifiées ; un motif sur un nom de variable (`*GIT_DIR*`) se contourne par `export GIT_DIR=… && git …`.
- **Interpréteurs et outils du projet** : tout agent qui peut éditer un fichier et lancer un outil de build, de test ou un interpréteur exécute ce qu'il veut (`npm run`, `make`, `pytest` avec un `conftest.py`, `.venv/bin/python`, `go run`…).
- **Hors du projet** : la permission « dossier extérieur » d'opencode ne porte que sur le dossier de travail et les `cd` ; une redirection (`> ~/fichier`) ou un argument écrit hors du projet sans demande. Seuls les motifs du garde-fou couvrent les fichiers sensibles.

Ce qui ne dépend pas des motifs : un checkpoint en pause ne passe qu'avec votre validation vue par oh ; Beads et les MCP d'oh passent par des passerelles qui appliquent la liste du workflow ; le proxy garde les clés ; le conteneur isole le système de fichiers. Pour un travail qui écrit, préférez le conteneur, et donnez aux outils d'infrastructure des identifiants en lecture seule.

### Limites connues du mode distant

- Les serveurs MCP qui lisent un jeton dans le trousseau de la machine (gitlab, jira, figma…) ne sont pas disponibles dans le job ; seul `workflow` fonctionne.
- Clé API seulement (pas de profil AWS), et pas de restrictions I6 dans le job.
- Le shell GitLab parent garde son propre environnement ; seul le compte `oh` (serveur, agents) en est protégé.
- Les Maintainers d'`oh-runner` peuvent lire ses variables CI (`OH_JOURNAL_KEY` compris) : restreignez-les.
- Le reste de l'enveloppe de session (prompt, entrées, instantané des tickets) est lisible des membres d'`oh-runner` et des jobs de construction d'image ; seule la clé du journal y est chiffrée.
- Un projet cible peut effacer ou remplacer les étiquettes et paquets d'`oh-runner` : les jobs le détectent (empreintes) et échouent ou reconstruisent (déni de service seulement). L'image par empreinte après une construction repose sur la lecture du dotenv par GitLab Runner dans `image:` (à valider à la recette).
- Le mode distant n'a pas encore été validé sur un vrai runner : il est **expérimental et désactivé par défaut** (`[remote] experimental = true` pour l'essayer).

### Mise à jour automatique

La commande `oh upgrade oh` télécharge les binaires exclusivement depuis GitHub Releases, avec les protections suivantes (opencode s'installe et se met à jour avec son propre outil) :

- **Liste blanche d'URL** — seuls `github.com` et `objects.githubusercontent.com` sont acceptés
- **HTTPS uniquement** — le HTTP en clair est rejeté
- **Sommes de contrôle SHA256** — le binaire téléchargé est vérifié contre le fichier de sommes publié ; l'installation est refusée si elles ne correspondent pas
- **Signature Cosign** — les sommes de contrôle de release sont signées avec [Sigstore](https://www.sigstore.dev/) (OIDC sans clé, identité : le workflow `release.yml` de ce dépôt sur un tag `v*`). Quand la CLI `cosign` est installée, `oh upgrade oh` et `install.sh` vérifient la signature et **refusent** une signature invalide. Sans `cosign`, ils ne vérifient que la somme de contrôle et affichent un avertissement ; `OH_REQUIRE_SIGNATURE=1` refuse toute mise à jour dont la signature ne peut pas être vérifiée
- **Remplacement atomique** — le binaire est remplacé de manière atomique, avec retour arrière automatique en cas d'échec

### Chaîne d'approvisionnement

- Les releases sont construites via [GoReleaser](https://goreleaser.com/) dans GitHub Actions avec `CGO_ENABLED=0` (binaires statiques, pas de dépendances C)
- Tous les artefacts de release sont signés avec [Cosign/Sigstore](https://www.sigstore.dev/) (sans clé, basé sur OIDC)
- Les mises à jour de dépendances sont gérées par Dependabot (cadence hebdomadaire)
- Les chemins critiques du code sont protégés par [CODEOWNERS](CODEOWNERS), qui exige une revue du mainteneur
- La couche opencode des images de conteneur est installée depuis le paquet npm vérifié par son intégrité sha512

## Bonnes pratiques pour les utilisateurs

1. **Utilisez le trousseau du système** — c'est le stockage le plus sûr. Évitez le repli fichier sauf si nécessaire.
2. **Définissez `OH_PASSPHRASE`** — si vous utilisez le repli fichier chiffré, définissez cette variable d'environnement de manière sûre (ex. : via un gestionnaire de secrets ou un profil shell, pas dans un fichier commité).
3. **Ne commitez pas `hub.toml` avec des jetons** — le fichier de configuration `hub.toml` ne doit jamais contenir de jetons en clair. Utilisez `oh init`, `oh provider setup`, `oh mcp setup` ou `oh secrets set` pour les stocker de manière sûre.
4. **Préférez le conteneur pour un travail non fiable** — en local, l'agent agit avec les droits de votre utilisateur.
5. **Gardez en lecture seule les workflows de lecture** — `risk: read` et une liste `beads.allow` étroite limitent ce qu'un agent peut changer ; les couches équipe et projet ne peuvent que durcir la sécurité.
6. **Protégez `oh-runner`** — protégez sa branche par défaut et restreignez ses Maintainers.
7. **Lancez `oh doctor`** — il vérifie la version d'opencode, la capacité du démon, le socket et les jetons restés en clair.
8. **Vérifiez les signatures des binaires** — après téléchargement, vérifiez la signature Cosign :
   ```bash
   cosign verify-blob --bundle checksums.txt.sigstore.json \
     --certificate-oidc-issuer https://token.actions.githubusercontent.com \
     --certificate-identity-regexp '^https://github\.com/datichb/openhub/\.github/workflows/release\.yml@refs/tags/v.*$' \
     checksums.txt
   ```
9. **Gardez openhub à jour** — lancez `oh upgrade oh` (ou `brew upgrade openhub`) régulièrement pour recevoir les correctifs de sécurité.

## Exigences de revue de code

Les chemins critiques pour la sécurité nécessitent une revue du mainteneur via CODEOWNERS :

| Chemin | Périmètre |
|--------|-----------|
| `.github/` | Workflows CI/CD et configuration de release |
| `cli/.goreleaser.yml` | Configuration de release et signature |
| `cli/internal/selfupdate/` | Logique de mise à jour automatique |
| `cli/internal/hubcontent/` | Intégrité du contenu embarqué |
| `cli/internal/storage/keychain/` | Opérations du trousseau du système |
| `cli/internal/storage/filecrypt/` | Chiffrement au repos |
| `permissions/` | Définitions des permissions des agents |
| `install.sh` | Script d'installation curl-pipe |
