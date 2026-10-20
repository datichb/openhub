> [Read in English](providers.en.md)

# Configuration des fournisseurs

Ce guide couvre la résolution des fournisseurs LLM par OpenCode Hub, la gestion des tokens API et la transmission des paramètres de fournisseur aux sessions.

## Fournisseurs supportés

| Fournisseur | Backend | Méthode d'authentification |
|-------------|---------|----------------------------|
| **bedrock** (par défaut) | AWS Bedrock | Bearer Token AWS (via keychain) |
| **anthropic** | API Anthropic | Clé API (via env ou keychain) |
| **github-copilot** | GitHub Copilot | OAuth |
| **openrouter** | OpenRouter | Clé API |

## Ordre de résolution du fournisseur

Quand `oh run` lance une session, le fournisseur est résolu dans cet ordre :

1. Flag `--provider` / `-P` sur `oh run` (priorité la plus haute)
2. Override au niveau projet (`project.Provider` dans la base de données)
3. `llm.default_provider` dans `~/.oh/hub.toml`
4. `"bedrock"` (valeur par défaut en dur)

## Configuration au niveau Hub

Définir le fournisseur par défaut pour tous les projets :

```bash
oh config set llm.default_provider bedrock
```

Dans `~/.oh/hub.toml` :

```toml
[llm]
default_provider = "bedrock"
```

## Configuration détaillée du provider (hub.toml)

```toml
[provider.bedrock]
aws_profile = "default"           # profil AWS a utiliser
aws_region = "eu-west-1"          # region Bedrock
auth_mode = "bearer"              # "bearer" | "profile" | "env"
```

Cette section est facultative — si absente, `oh` utilise les credentials de l'environnement.

`auth_mode` décide de l'identifiant Bedrock (⚠ v5.0 : il est désormais respecté strictement) :

| `auth_mode` | Identifiant utilisé |
|-------------|---------------------|
| `bearer` | uniquement la clé API Bedrock enregistrée (projet, équipe, hub) ; sans clé, le lancement est refusé (`auth_mode bearer`) et `oh doctor` l'explique : « auth_mode vaut bearer mais aucune clé API Bedrock n'est enregistrée… : lancez oh provider setup bedrock » |
| `profile` | signature SigV4 avec le profil AWS (`aws_profile`) ; une clé enregistrée est ignorée |
| `env` | signature SigV4 avec la chaîne AWS par défaut (variables `AWS_ACCESS_KEY_ID`…, SSO…) ; une clé enregistrée est ignorée |
| (absent) | clés enregistrées, puis profil AWS |

La région doit être un nom de région AWS (`eu-west-1`) : elle désigne l'hôte qui reçoit l'identifiant. Elle est vérifiée dès la saisie — vue Fournisseur de la TUI, assistants `oh init` et `oh provider setup` (« Personnalisé... ») : le message s'affiche sous le champ, une valeur invalide est refusée, une région où Bedrock n'est pas connu donne un simple avertissement — puis par `oh config set provider.bedrock.aws_region`, au lancement et par le proxy d'identifiants.

## Override au niveau projet

Surcharger le fournisseur pour un projet spécifique :

```bash
oh project configure my-project --provider anthropic --model claude-sonnet-4-5
```

## Gestion des tokens

### AWS Bedrock (Bearer Token)

Les tokens sont stockés dans le keychain du système. Configuration via :

```bash
oh provider setup bedrock
# Selectionner le mode bearer -> entrer votre bearer token
# Stocke sous la cle : bedrock-token-default (ou bedrock-token-<project-id>)
```

Le token ne quitte jamais la machine et n'est jamais transmis à opencode : le proxy d'identifiants du démon oh le détient et signe les appels ; la session ne reçoit qu'un jeton `ohs_…` propre à son groupe (voir [Sessions v5 › Clés LLM](sessions-v5.fr.md#clés-llm)).

Le proxy ne relaie au fournisseur que les en-têtes et les paramètres de query des vrais clients (la liste est dans [SECURITY › Proxy d'identifiants LLM](../../SECURITY.fr.md)). Un en-tête personnalisé ajouté aux options du fournisseur d'opencode (autre organisation, option de passerelle…) est retiré ; le proxy en journalise le nom au niveau debug dans `~/.oh/run/ohd.log`.

Configuration via la commande dédiée :

```bash
oh provider setup              # wizard interactif (hub-level)
oh provider setup --project X  # override par projet
oh provider setup bedrock      # configurer un provider specifique
```

Ordre de résolution du bearer token :

1. `bedrock-token-<project-id>` (par projet)
2. `bedrock-token-default` (global)

**Clé du shell (`AWS_BEARER_TOKEN_BEDROCK`)** : oh ne transmet jamais cette variable à l'outil, par conception (seules les clés du trousseau sont utilisées). Pour utiliser la clé de votre shell, enregistrez-la :

```bash
oh provider setup bedrock --from-env           # cle utilisee par un lancement d'ici (sinon cle du hub)
oh provider setup bedrock --from-env -p my-app # cle du projet
```

L'assistant fait de même avec « Utiliser l'existant » quand la variable est définie, et `oh run` le propose quand aucune clé n'est enregistrée. `auth_mode` passe à `bearer`.

**Clé refusée** : si AWS refuse la clé (les clés API Bedrock à court terme expirent au bout de 12 h au plus), le démon le journalise et lève une alerte ✗ dans « À traiter » avec le nom de la clé, sa portée et sa date d'enregistrement. `oh doctor` valide la clé auprès d'AWS (`--offline` ou `OH_DOCTOR_OFFLINE=1` pour s'en passer) et signale une `AWS_BEARER_TOKEN_BEDROCK` du shell inutilisée ou différente de la clé enregistrée. Voir [Dépannage](troubleshooting.fr.md#fournisseur-et-authentification).

### Anthropic / OpenAI / OpenRouter

Le bloc provider est généré dans le paquet de session au lancement (`oh deploy` supprimé en v5) ; la clé reste dans le keychain, côté proxy du démon oh :

```bash
oh run <workflow> -p my-project --provider anthropic
```

## Tokens des services MCP

Les serveurs MCP (Figma, GitLab, Google Slides) nécessitent leurs propres tokens :

```bash
oh mcp setup
```

Configuration par projet (surcharge le hub) :

```bash
oh mcp setup --project my-project
```
Le token est stocké dans le keychain avec une clé spécifique au projet.

Assistant interactif qui :

1. Demande quel service configurer (Figma, GitLab, Google Slides)
2. Demande le token API (saisie masquée)
3. Stocké dans le keychain du système
4. Active le service dans `hub.toml`

Les tokens sont lus par les serveurs MCP au runtime via des variables d'environnement :

- `FIGMA_TOKEN`
- `GITLAB_TOKEN` (+ `GITLAB_URL` pour les instances self-hosted)
- `GOOGLE_ACCESS_TOKEN`

## Stockage des secrets

**Principal :** Keychain du système (macOS Keychain, Linux secret-service, Windows Credential Manager)

**Clés de secrets connues :**

| Clé | Usage |
|-----|-------|
| `bedrock-token-default` | Token Bearer AWS pour Bedrock |
| `bedrock-token-<project-id>` | Token Bedrock par projet |
| `anthropic-api-key-default` | Clé API Anthropic |
| `anthropic-api-key-<project-id>` | Clé API Anthropic par projet |
| `openrouter-api-key-default` | Clé API OpenRouter |
| `openrouter-api-key-<project-id>` | Clé API OpenRouter par projet |
| `openhub.mcp.figma.token` | Token API Figma |
| `openhub.mcp.gitlab.token` | Token API GitLab |
| `openhub.mcp.gslides.token` | Token OAuth Google Slides |

**Fallback :** Fichier chiffré à `~/.oh/secrets.enc`

- Chiffrement : AES-256-GCM
- Dérivation de clé : Argon2id (t=3, memory=64Mo, threads=4)
- Passphrase : variable d'env `OH_PASSPHRASE` ou saisie interactive (min 8 caractères)

## Flux de lancement

À chaque lancement (`oh run <workflow>`), la configuration du fournisseur est écrite dans le paquet de session (`~/.oh/bundles/<hash>/`), pas dans le projet :

La construction du paquet lit votre fournisseur et modèle configurés, puis génère le bloc provider correspondant. Ce bloc pointe vers le proxy d'identifiants du démon oh : il ne contient jamais la vraie clé (la session s'authentifie auprès du proxy avec un jeton `ohs_…`). Pour voir le paquet : `oh bundle show <workflow>`.

Avant de construire quoi que ce soit, oh vérifie que le fournisseur a des identifiants (projet, équipe, hub ; profil AWS pour Bedrock) ; sinon le lancement est refusé avec « oh provider setup ». Dans la TUI, le récap de la fiche de lancement affiche ce message et un bouton **Configurer le fournisseur** : il ouvre la vue Fournisseur ; au retour (`Esc`), le récap vérifie à nouveau les identifiants.

## Changer de fournisseur

```bash
# Temporairement (une session)
oh run <workflow> --provider anthropic

# Definitivement (defaut hub)
oh config set llm.default_provider anthropic

# Definitivement (un projet)
oh project configure my-project --provider github-copilot   # pris en compte au prochain lancement
```

## Vérifier la configuration

```bash
oh status                  # affiche le fournisseur/modele du projet courant
oh doctor                  # valide que les cles API sont configurees
oh config list             # affiche toute la config hub dont le fournisseur
```

## Bonnes pratiques de sécurité

- Ne jamais stocker les clés API en clair dans des fichiers
- Utiliser `oh provider setup` et `oh mcp setup`, qui stockent dans le keychain du système
- Pour la CI/headless : utiliser la variable d'env `OH_PASSPHRASE` pour le fallback chiffré
- Les clés du fournisseur restent dans le keychain : opencode ne reçoit qu'un jeton de proxy `ohs_…` par groupe de sessions
- oh n'écrit plus de `opencode.json` dans le projet ; si vous en gardez un avec des options provider, il doit être gitignore

## Dépannage

| Problème | Solution |
|----------|----------|
| "Token not configured" | Lancer `oh provider setup` (fournisseur) ou `oh mcp setup` (service MCP) |
| Fournisseur non reconnu | Vérifier l'orthographe : bedrock, anthropic, openrouter, github-copilot |
| Mauvais modèle | Utiliser `oh project configure --model <nom>` ; pris en compte au prochain lancement (paquet reconstruit) |
| Accès keychain refusé | Autoriser le terminal dans Préférences Système > Confidentialité |
| Erreurs du store fallback | Vérifier `OH_PASSPHRASE` ou ressaisir quand demandé |
