# Politique de securite

## Versions supportees

| Version | Supportee |
|---------|-----------|
| 4.x     | Oui       |
| 3.x     | Non (fin de vie) |
| < 3.0   | Non       |

## Signaler une vulnerabilite

**Merci de ne pas signaler les vulnerabilites de securite via les issues publiques GitHub.**

### Methode preferee

Utilisez les [GitHub Security Advisories](https://github.com/datichb/openhub/security/advisories/new) pour signaler une vulnerabilite de maniere privee. Cela nous permet d'evaluer l'impact, de preparer un correctif et de coordonner la divulgation avant toute annonce publique.

### Alternative

Envoyez un email a **security@example.com** avec les informations suivantes :

- Description de la vulnerabilite
- Etapes de reproduction
- Version(s) affectee(s)
- Evaluation de l'impact potentiel
- Correctif suggere (le cas echeant)

### A quoi s'attendre

- **Accuse de reception** sous 48 heures
- **Mise a jour du statut** sous 7 jours
- **Delai de correctif** selon la severite (critique : 72h, haute : 7 jours, moyenne : 30 jours)

Si la vulnerabilite est acceptee, nous allons :
1. Developper et tester un correctif
2. Publier un avis de securite
3. Publier une version corrigee
4. Crediter le rapporteur (sauf si l'anonymat est prefere)

Si elle est declinee, nous fournirons une explication claire.

## Perimetre de securite

### Stockage des identifiants

openhub stocke les tokens API et secrets via deux mecanismes :

- **Trousseau systeme** (`go-keyring`) — methode preferee, utilise le stockage securise natif (macOS Keychain, GNOME Keyring, Windows Credential Manager)
- **Fichier chiffre (fallback)** — quand le trousseau systeme est indisponible, les secrets sont chiffres au repos en **AES-256-GCM** avec derivation de cle **Argon2id** (parametres minimum OWASP : t=3, memory=64 Mo, threads=4). La passphrase est lue depuis la variable d'environnement `OH_PASSPHRASE` ou demandee interactivement.

Les secrets ne sont jamais stockes en clair sur le disque.

### Mise a jour automatique

La commande `oh upgrade` telecharge les binaires exclusivement depuis GitHub Releases avec les protections suivantes :

- **Liste blanche d'URLs** — seuls `github.com` et `objects.githubusercontent.com` sont acceptes
- **HTTPS uniquement** — le HTTP en clair est rejete
- **Checksums SHA256** — le binaire telecharge est verifie contre le fichier de checksums publie ; l'installation est refusee si les checksums ne correspondent pas
- **Signature Cosign** — les checksums de release sont signes avec [Sigstore](https://www.sigstore.dev/) (OIDC sans cle). La verification cote client est actuellement indicative (warning dans les logs) ; l'application stricte est prevue pour une version future
- **Remplacement atomique** — le binaire est remplace de maniere atomique avec rollback automatique en cas d'echec

### Chaine d'approvisionnement

- Les releases sont construites via [GoReleaser](https://goreleaser.com/) dans GitHub Actions avec `CGO_ENABLED=0` (binaires statiques, pas de dependances C)
- Tous les artefacts de release sont signes avec [Cosign/Sigstore](https://www.sigstore.dev/) (sans cle, base OIDC)
- Les mises a jour de dependances sont gerees par Dependabot (cadence hebdomadaire)
- Les chemins critiques du code sont proteges par [CODEOWNERS](CODEOWNERS) exigeant une review du mainteneur

### Gestion des tokens MCP

Les tokens des serveurs MCP (GitLab, Figma, Jira, Linear, Google) sont :
- Stockes dans le trousseau systeme ou le fichier chiffre (jamais dans des fichiers de config en clair)
- Transmis aux sous-processus MCP via des variables d'environnement
- Jamais logues, jamais ecrits dans l'etat de session, jamais transmis aux fournisseurs IA

### Identifiants LLM et démon oh (sessions opencode V2)

Avec opencode V2, le serveur de l'outil ne reçoit jamais la clé LLM. Un démon par utilisateur (`ohd`, socket Unix `~/.oh/run/ohd.sock`, droits 0600) fait tourner un proxy d'identifiants sur `127.0.0.1`. Chaque groupe de serveurs reçoit un jeton de session aléatoire de 256 bits. Le proxy remplace ce jeton par la vraie clé (ou signe avec AWS SigV4) et ne relaie que vers l'adresse du fournisseur, sur une liste blanche de chemins d'inférence par fournisseur (autres chemins, segments `.`/`..` et double échappement refusés). Une requête dont les identifiants ne peuvent pas être appliqués (par exemple des identifiants AWS expirés) est refusée, et les corps sont limités à 64 Mio. La base ne contient que l'empreinte SHA-256 de chaque jeton et une référence à l'identifiant, jamais le jeton ni le secret (les jetons gardés en clair par une version précédente sont remplacés par leur empreinte au démarrage du démon) ; une empreinte lue dans `oh.db` n'est pas acceptée par le proxy. `~/.oh` est en 0700, `oh.db` et ses fichiers WAL en 0600.

Le socket du démon n'accepte que les connexions des processus de votre utilisateur (identité du pair vérifiée par le noyau à chaque connexion, en plus des droits 0600). Les routes qui donnent un accès (jetons du proxy et des passerelles, identifiants, écoutes supplémentaires du proxy) ou arrêtent des sessions exigent en plus une capacité d'émission : un secret aléatoire gardé dans le trousseau du système (ou, sans trousseau utilisable, dans `~/.oh/run/capability`, en 0600, signalé par `oh doctor`), lu par la CLI oh et le démon, et jamais placé dans l'environnement des serveurs de l'outil. La vérification d'isolation de chaque serveur s'appuie aussi sur les règles de permission que l'outil indique pour chaque agent, et pas seulement sur la configuration rendue par oh.

**Limite connue du mode local :** le shell de l'agent tourne sous votre utilisateur. Il peut voir le jeton du proxy de son serveur, et il peut toujours atteindre le socket du démon et `oh.db`. Sans la capacité, il ne peut pas obtenir de nouveaux jetons, mais un processus de votre utilisateur peut lire l'entrée du trousseau ou le fichier de capacité, ou lancer `oh` lui-même : ces protections compliquent l'abus, elles n'isolent pas l'agent. Il ne voit jamais la clé LLM elle-même. La vraie isolation vient des sessions en conteneur (ni le socket ni `oh.db` n'y sont montés). Considérez que l'agent agit avec les droits de votre utilisateur, comme avec opencode V1.

## Bonnes pratiques pour les utilisateurs

1. **Utilisez le trousseau systeme** — c'est le stockage le plus securise. Evitez le fallback fichier sauf si necessaire.
2. **Definissez `OH_PASSPHRASE`** — si vous utilisez le fallback fichier chiffre, definissez cette variable d'environnement de maniere securisee (ex : via un gestionnaire de secrets ou un profil shell, pas dans un fichier commite).
3. **Ne commitez pas `hub.toml` avec des tokens** — le fichier de configuration `hub.toml` ne doit jamais contenir de tokens en clair. Utilisez `oh init` pour les stocker de maniere securisee.
4. **Verifiez les signatures des binaires** — apres telechargement, verifiez la signature Cosign :
   ```bash
   cosign verify-blob --bundle checksums.txt.sigstore.json checksums.txt
   ```
5. **Gardez openhub a jour** — executez `oh upgrade` regulierement pour recevoir les correctifs de securite.

## Exigences de revue de code

Les chemins critiques pour la securite necessitent une revue du mainteneur via CODEOWNERS :

| Chemin | Perimetre |
|--------|-----------|
| `.github/` | Workflows CI/CD et configuration de release |
| `cli/.goreleaser.yml` | Configuration de release et signature |
| `cli/internal/selfupdate/` | Logique de mise a jour automatique |
| `cli/internal/hubcontent/` | Integrite du contenu embarque |
| `cli/internal/storage/keychain/` | Operations trousseau systeme |
| `cli/internal/storage/filecrypt/` | Chiffrement au repos |
| `permissions/` | Definitions des permissions agents |
| `install.sh` | Script d'installation curl-pipe |
