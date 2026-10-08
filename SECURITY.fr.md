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

### Alternative

Envoyez un email à **security@example.com** avec les informations suivantes :

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

Décisions : [ADR-041](docs/architecture/adr/041-closed-world-isolation.fr.md) (monde fermé), [ADR-044](docs/architecture/adr/044-credential-proxy-session-limits.fr.md) (proxy d'identifiants, restrictions), [ADR-045](docs/architecture/adr/045-execution-environments.fr.md) (environnements d'exécution), [ADR-046](docs/architecture/adr/046-beads-gateways.fr.md) (passerelles).

### Stockage des identifiants

openhub stocke les jetons d'API et les secrets via deux mécanismes :

- **Trousseau du système** (`go-keyring`) — méthode préférée, utilise le stockage sécurisé natif (macOS Keychain, GNOME Keyring, Windows Credential Manager)
- **Fichier chiffré (repli)** — quand le trousseau du système est indisponible, les secrets sont chiffrés au repos en **AES-256-GCM** avec dérivation de clé **Argon2id** (paramètres minimum OWASP : t=3, memory=64 Mo, threads=4). La phrase de passe est lue depuis la variable d'environnement `OH_PASSPHRASE` ou demandée de manière interactive.

Les secrets ne sont jamais stockés en clair sur le disque. `~/.oh` est en 0700 ; `oh.db` et ses fichiers WAL en 0600.

### Proxy d'identifiants LLM

Le démon oh (`ohd`) fait tourner un proxy d'identifiants sur `127.0.0.1` (une écoute par machine ; sous Linux, une seconde écoute sur l'adresse vue des conteneurs).

- **Jetons de groupe.** Chaque groupe de serveur (version du paquet, projet, environnement d'exécution) reçoit un jeton aléatoire de 256 bits `ohs_…`, donné à opencode dans la variable du fournisseur. Il est révoqué à la veille, à l'arrêt ou à l'abandon du groupe ; sans jeton valide, le proxy répond 401.
- **La clé reste dans le trousseau.** Le proxy lit l'identifiant dans le trousseau (clé du projet, clé du fournisseur pour le projet, clé de l'équipe, clé du hub, puis un profil AWS pour Bedrock) et l'applique dans le transport, sur la requête finale : en-tête remplacé, ou signature AWS SigV4. Si l'identifiant ne peut pas être appliqué (par exemple des identifiants AWS expirés), la requête est refusée (502) et rien n'est envoyé au fournisseur.
- **Liste blanche des chemins.** Seuls les chemins d'inférence de chaque fournisseur sont relayés ; tout autre chemin reçoit 404. Chaque segment est décodé une seule fois ; segments vides, `.`, `..` et double échappement sont refusés.
- **Liste blanche des modèles.** Quand une liste de modèles est active (restrictions des sessions), la clé JSON `model` doit correspondre exactement ; un doublon ou une variante de casse est refusé (400).
- **Taille limitée.** Les corps de requête sont limités à 64 Mio (413 au-delà).
- **Pas de repli silencieux.** La configuration rendue interdit tout fournisseur autre que celui de la session : opencode ne peut plus basculer sur ses modèles hébergés quand un modèle est indisponible.
- **Empreintes seules.** La base ne contient que l'empreinte SHA-256 de chaque jeton et une référence à l'identifiant, jamais le jeton ni le secret. Les jetons gardés en clair par une version précédente sont remplacés par leur empreinte au démarrage du démon. Une empreinte lue dans `oh.db` n'est pas acceptée par le proxy.

### Le démon oh

- **Socket.** `~/.oh/run/ohd.sock`, droits 0600. L'identité du pair est vérifiée par le noyau à chaque connexion (UID, `LOCAL_PEERCRED` sous macOS, `SO_PEERCRED` sous Linux) : seuls les processus de votre utilisateur sont acceptés. Sur les autres systèmes, seuls les droits 0600 s'appliquent.
- **Capacité d'émission.** Les routes qui donnent un accès (jetons du proxy et des passerelles, identifiants, écoutes supplémentaires du proxy) ou arrêtent le démon exigent en plus l'en-tête `X-Oh-Capability`. C'est un secret aléatoire gardé dans le trousseau du système (`openhub.daemon.capability`) ou, sans trousseau utilisable, dans `~/.oh/run/capability` (0600, signalé par `oh doctor`). Il n'est lu que par la CLI oh et le démon, comparé à temps constant, et jamais placé dans un environnement.
- **Crochets.** Les routes du plugin oh et des passerelles (`/oh/v1/hooks/*`) sont authentifiées par le jeton du groupe.
- **Contrôles.** `oh doctor` signale le stockage de la capacité, les droits et le propriétaire du socket, et les jetons restés en clair dans `oh.db`.
- **Windows.** Le démon tourne dans le processus oh (pas de démon en arrière-plan).

### Monde fermé

Une session ne voit que les agents et les skills de son paquet. C'est obligatoire et non réglable.

- Les agents natifs d'opencode sont désactivés ; les skills intégrées sont refusées ; pour chaque agent, la délégation n'est permise que vers les cibles du graphe du workflow.
- La configuration et les données opencode du projet sont exclues (`OPENCODE_DISABLE_PROJECT_CONFIG=1`, dossier de données propre au groupe).
- Le plugin oh retire tout agent ou skill hors paquet (deuxième barrière).
- **`Attest`** s'exécute après chaque démarrage de serveur et pour chaque nouveau dossier : il compare au paquet les agents, les skills visibles pour chaque agent (sur les règles effectives que renvoie l'outil) et les serveurs MCP. Tout élément inattendu fait échouer le lancement ; le serveur est arrêté et son jeton révoqué.
- `[execution] strict_isolation` masque en plus votre configuration opencode d'utilisateur en local. En conteneur et à distance, elle n'est jamais visible.

Le monde fermé porte sur ce que le modèle voit, pas sur ce que le shell peut faire.

### Passerelles

**Passerelle Beads** (local, conteneur et job distant). Le faux `bd` envoie la commande au démon (`/oh-gateway/beads/v1/exec`), qui lance le vrai `bd` sur la machine. En local, le faux `bd` (`~/.oh/run/bin/bd`) passe en tête du `PATH` de la session et de ses sous-agents ; la règle shell de chaque agent refuse un `bd` appelé par un chemin (`/opt/homebrew/bin/bd …`, `./bd`), qui contournerait la passerelle :

- **Jetons `ohg_…`**, un par session et par sous-session, transmis par l'environnement de session. Le démon n'en garde que l'empreinte (`~/.oh/run/gateway.json`, 0600). Ils sont valides tant que la session est ouverte et éveillée, et révoqués avec le groupe.
- **Liste blanche** = `beads.allow` du workflow. Sans bloc `beads:`, lecture seule ; une liste vide refuse tout.
- **Options refusées** : les options globales qui changent de base ou de dossier (`--db`, `-C`, `--global`…), partout dans la commande.
- **Chemins** traduits vers la machine et limités aux emplacements de la session (liens résolus) ; le paquet et les données du groupe sont exclus.
- Le vrai `bd` est lancé sans shell, dans le dossier de la session (délai de 2 min, 8 Mio par flux).
- **Démarrage du shell** : le shell d'une session locale lit les fichiers de démarrage d'oh (`ZDOTDIR` pour zsh, `BASH_ENV` pour bash, `~/.oh/run/shell/`) : ils chargent les fichiers de l'utilisateur (`~/.zshenv`…), puis remettent le faux `bd` en tête, pour qu'un vrai `bd` installé par npm, volta ou dans `~/.local/bin` ne passe pas devant. Le démon le vérifie une fois par session (sinon alerte ✗ dans « À traiter »), `oh doctor` aussi.
- **Hooks git de Beads** (`bd hooks run <hook>`, lancés par git avec `BD_GIT_HOOK=1`) : ils passent par la passerelle quelle que soit la liste `beads.allow`, pour qu'un commit ne soit jamais bloqué par la liste ; seuls `hooks run` et les hooks de bd sont acceptés. Hors de la machine (conteneur), ils ne sont pas lancés sur la machine (les hooks que bd enchaîne s'y exécuteraient hors du conteneur).
- **Hooks git jamais contournés** : sont refusés à tous les agents `--no-verify` (`-n` pour un commit), `-c core.hooksPath=…`, `git config core.hooksPath`, `GIT_CONFIG_*`, les inclusions de configuration, et l'écriture dans `.git/` ou `.beads/hooks/` (shell et outil d'édition).
- **Commit et fermeture verrouillés par un checkpoint** (`unlocks:`, ex. `cp-2` de `ticket`) : avant sa validation, `git commit`, `git push` et la fermeture d'un ticket sont refusés à tous les agents ; ensuite la passerelle ne ferme un ticket que si le travail est commité.

**Passerelle MCP** (hors machine). Les serveurs MCP d'oh du paquet tournent sur la machine, lancés par le démon depuis le paquet immuable ; ils lisent eux-mêmes leurs jetons dans le trousseau. Le conteneur les joint en HTTP avec le jeton du groupe (`{env:…}`, jamais écrit dans la configuration). Un appel dont le `_meta` désigne une session d'un autre groupe est refusé (403).

### Gestion des jetons MCP

Les jetons des serveurs MCP (GitLab, Figma, Jira, Linear, GitHub, Google) sont :

- stockés dans le trousseau du système ou le fichier chiffré (jamais dans des fichiers de configuration en clair) ;
- lus par le processus `oh mcp serve` lui-même, sur la machine : le paquet ne contient que le nom de la clé du trousseau (`--token-key`) ;
- jamais écrits dans le paquet, la configuration opencode ou l'état de session, jamais journalisés, jamais transmis aux fournisseurs IA, jamais placés dans un conteneur.

### Sessions en conteneur

- **Aucun secret dans l'environnement du conteneur.** L'environnement est passé par `--env-file` (0600) et reste minimal, sans aucune variable de la machine. Les seuls identifiants que voit le shell sont le jeton `ohs_…` et un jeton `ohg_…` (avec `OH_GATEWAY_URL`).
- **Paquet en lecture seule** sur `/opt/oh/bundle`. `HOME` est un volume propre au projet : votre configuration n'est jamais visible. `~/.oh` n'est pas monté : ni le socket du démon ni `oh.db` ne sont joignables.
- Le port d'opencode n'est publié que sur `127.0.0.1`. Un montage hors des dossiers partagés avec la VM est refusé.
- Le conteneur est supprimé à l'arrêt ou à la mise en veille de la session (`run --rm --init`).

### Sessions distantes (GitLab CI)

- **Variables CI masquées et protégées.** Le job ne reçoit que les variables CI du projet `oh-runner` : clé LLM des jobs, un jeton d'accès par projet cible, un jeton d'écriture du team-state. Elles sont masquées (absentes des journaux) et protégées (branche par défaut d'`oh-runner`, qui doit être protégée). `oh remote setup` lit les secrets en saisie masquée ou dans une variable d'environnement, jamais dans les arguments ; `[[remote.targets]]` dans `hub.toml` ne contient aucun secret. La machine ne garde que votre jeton API GitLab et le jeton de déclenchement, dans le trousseau.
- **Aucun secret dans les variables du pipeline ni dans les artefacts.** Les entrées et le prompt voyagent dans l'enveloppe de session (registre de packages), pas dans les variables du pipeline. Les artefacts (`journal.jsonl`, `summary.json`, `session.export`) et les messages d'erreur sont expurgés des valeurs secrètes.
- **Dans le job.** `ScrubEnv` retire toute variable dont le nom contient `TOKEN`, `PASSWORD`, `SECRET`, `_KEY`, `PASSPHRASE`, `CREDENTIAL`, `JWT` ou `AUTH` avant le démarrage du démon. La clé LLM n'est détenue que par le proxy du job. opencode tourne sous le compte `oh` (uid 10001), qui ne peut lire ni les processus du job, ni `oh.db`, ni les secrets. Le jeton du projet est donné à git par `GIT_CONFIG_*` dans l'environnement du processus git, jamais en argument, dans une URL ou dans un fichier.
- **Aucune approbation à l'aveugle.** Un répondeur de politique traite les décisions, jamais `--auto` : les checkpoints `auto` sont validés, toute autre permission est refusée, un checkpoint différé ou une question arrête le travail.

### Restrictions des sessions (I6)

Optionnelles et désactivées par défaut (`oh budget show|set|unset|raise`) : sessions actives max, budget par session et budget journalier en USD, plafond mémoire, liste des modèles autorisés (appliquée par le proxy sur le jeton du groupe). Cascade : workflow (`limits:`) > projet > hub > recommandé d'équipe ; l'imposé d'équipe est un plafond. Les budgets sont des plafonds souples : contrôlés en fin d'étape, ils lèvent alors une décision `$`.

### Limites connues du mode local

- Le shell de l'agent tourne sous votre utilisateur. Il peut voir le jeton du proxy de son serveur, et il peut toujours atteindre le socket du démon et `oh.db`.
- Sans la capacité, il ne peut pas obtenir de nouveaux jetons, mais un processus de votre utilisateur peut lire l'entrée du trousseau ou le fichier de capacité, ou lancer `oh` lui-même. Ces protections compliquent l'abus ; elles n'isolent pas l'agent.
- La clé LLM n'est jamais dans l'environnement de l'agent ni dans la configuration d'opencode, mais votre trousseau est lisible par votre utilisateur.
- Le mot de passe des serveurs opencode reste en clair dans `oh.db` (nécessaire pour attacher une session).
- Sans isolation stricte, votre configuration opencode d'utilisateur est chargée ; `Attest` refuse ce qui ajoute des agents, des skills ou des serveurs MCP visibles, pas les autres réglages.

Considérez que l'agent agit avec les droits de votre utilisateur. La vraie isolation vient des sessions en conteneur.

### Limites connues du mode distant

- Les serveurs MCP qui lisent un jeton dans le trousseau de la machine (gitlab, jira, figma…) ne sont pas disponibles dans le job ; seul `workflow` fonctionne.
- Clé API seulement (pas de profil AWS), et pas de restrictions I6 dans le job.
- Le shell GitLab parent garde son propre environnement ; seul le compte `oh` (serveur, agents) en est protégé.
- Les Maintainers d'`oh-runner` peuvent lire ses variables CI : restreignez-les.
- Le mode distant n'a pas encore été validé sur un vrai runner.

### Mise à jour automatique

La commande `oh upgrade oh` télécharge les binaires exclusivement depuis GitHub Releases, avec les protections suivantes (opencode s'installe et se met à jour avec son propre outil) :

- **Liste blanche d'URL** — seuls `github.com` et `objects.githubusercontent.com` sont acceptés
- **HTTPS uniquement** — le HTTP en clair est rejeté
- **Sommes de contrôle SHA256** — le binaire téléchargé est vérifié contre le fichier de sommes publié ; l'installation est refusée si elles ne correspondent pas
- **Signature Cosign** — les sommes de contrôle de release sont signées avec [Sigstore](https://www.sigstore.dev/) (OIDC sans clé). La vérification côté client est actuellement indicative (avertissement dans les journaux) ; l'application stricte est prévue pour une version future
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
   cosign verify-blob --bundle checksums.txt.sigstore.json checksums.txt
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
