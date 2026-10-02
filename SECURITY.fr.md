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
