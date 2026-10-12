> [Read in English](providers.en.md)

# Configuration des fournisseurs

Ce guide couvre la resolution des fournisseurs LLM par OpenCode Hub, la gestion des tokens API et la transmission des parametres de fournisseur aux sessions.

## Fournisseurs supportes

| Fournisseur | Backend | Methode d'authentification |
|-------------|---------|----------------------------|
| **bedrock** (par defaut) | AWS Bedrock | Bearer Token AWS (via keychain) |
| **anthropic** | API Anthropic | Cle API (via env ou keychain) |
| **github-copilot** | GitHub Copilot | OAuth |
| **openrouter** | OpenRouter | Cle API |

## Ordre de resolution du fournisseur

Quand `oh run` lance une session, le fournisseur est resolu dans cet ordre :

1. Flag `--provider` / `-P` sur `oh run` (priorite la plus haute)
2. Override au niveau projet (`project.Provider` dans la base de donnees)
3. `llm.default_provider` dans `~/.oh/hub.toml`
4. `"bedrock"` (valeur par defaut en dur)

## Configuration au niveau Hub

Definir le fournisseur par defaut pour tous les projets :

```bash
oh config set llm.default_provider bedrock
```

Dans `~/.oh/hub.toml` :

```toml
[llm]
default_provider = "bedrock"
```

## Configuration detaillee du provider (hub.toml)

```toml
[provider.bedrock]
aws_profile = "default"           # profil AWS a utiliser
aws_region = "eu-west-1"          # region Bedrock
auth_mode = "bearer"              # "bearer" | "profile" | "env"
```

Cette section est facultative — si absente, `oh` utilise les credentials de l'environnement.

`auth_mode` decide de l'identifiant Bedrock (⚠ v5.0 : il est desormais respecte strictement) :

| `auth_mode` | Identifiant utilise |
|-------------|---------------------|
| `bearer` | uniquement la cle API Bedrock enregistree (projet, equipe, hub) ; sans cle, le lancement est refuse (`auth_mode bearer`) et `oh doctor` l'explique : « auth_mode vaut bearer mais aucune clé API Bedrock n'est enregistrée… : lancez oh provider setup bedrock » |
| `profile` | signature SigV4 avec le profil AWS (`aws_profile`) ; une cle enregistree est ignoree |
| `env` | signature SigV4 avec la chaine AWS par defaut (variables `AWS_ACCESS_KEY_ID`…, SSO…) ; une cle enregistree est ignoree |
| (absent) | cles enregistrees, puis profil AWS |

La region doit etre un nom de region AWS (`eu-west-1`) : elle designe l'hote qui recoit l'identifiant. Elle est verifiee des la saisie — vue Fournisseur de la TUI, assistants `oh init` et `oh provider setup` (« Personnalisé... ») : le message s'affiche sous le champ, une valeur invalide est refusee, une region ou Bedrock n'est pas connu donne un simple avertissement — puis par `oh config set provider.bedrock.aws_region`, au lancement et par le proxy d'identifiants.

## Override au niveau projet

Surcharger le fournisseur pour un projet specifique :

```bash
oh project configure my-project --provider anthropic --model claude-sonnet-4-5
```

## Gestion des tokens

### AWS Bedrock (Bearer Token)

Les tokens sont stockes dans le keychain du systeme. Configuration via :

```bash
oh provider setup bedrock
# Selectionner le mode bearer -> entrer votre bearer token
# Stocke sous la cle : bedrock-token-default (ou bedrock-token-<project-id>)
```

Le token ne quitte jamais la machine et n'est jamais transmis a opencode : le proxy d'identifiants du demon oh le detient et signe les appels ; la session ne recoit qu'un jeton `ohs_…` propre a son groupe (voir [Sessions v5 › Cles LLM](sessions-v5.fr.md#clés-llm)).

Configuration via la commande dediee :

```bash
oh provider setup              # wizard interactif (hub-level)
oh provider setup --project X  # override par projet
oh provider setup bedrock      # configurer un provider specifique
```

Ordre de resolution du bearer token :

1. `bedrock-token-<project-id>` (par projet)
2. `bedrock-token-default` (global)

**Cle du shell (`AWS_BEARER_TOKEN_BEDROCK`)** : oh ne transmet jamais cette variable a l'outil, par conception (seules les cles du trousseau sont utilisees). Pour utiliser la cle de votre shell, enregistrez-la :

```bash
oh provider setup bedrock --from-env           # cle utilisee par un lancement d'ici (sinon cle du hub)
oh provider setup bedrock --from-env -p my-app # cle du projet
```

L'assistant fait de meme avec « Utiliser l'existant » quand la variable est definie, et `oh run` le propose quand aucune cle n'est enregistree. `auth_mode` passe a `bearer`.

**Cle refusee** : si AWS refuse la cle (les cles API Bedrock a court terme expirent au bout de 12 h au plus), le demon le journalise et leve une alerte ✗ dans « À traiter » avec le nom de la cle, sa portee et sa date d'enregistrement. `oh doctor` valide la cle aupres d'AWS (`--offline` ou `OH_DOCTOR_OFFLINE=1` pour s'en passer) et signale une `AWS_BEARER_TOKEN_BEDROCK` du shell inutilisee ou differente de la cle enregistree. Voir [Depannage](troubleshooting.fr.md#fournisseur-et-authentification).

### Anthropic / OpenAI / OpenRouter

Le bloc provider est genere dans le paquet de session au lancement (`oh deploy` supprime en v5) ; la cle reste dans le keychain, cote proxy du demon oh :

```bash
oh run <workflow> -p my-project --provider anthropic
```

## Tokens des services MCP

Les serveurs MCP (Figma, GitLab, Google Slides) necessitent leurs propres tokens :

```bash
oh mcp setup
```

Configuration par projet (surcharge le hub) :

```bash
oh mcp setup --project my-project
```
Le token est stocke dans le keychain avec une cle specifique au projet.

Assistant interactif qui :

1. Demande quel service configurer (Figma, GitLab, Google Slides)
2. Demande le token API (saisie masquee)
3. Stocke dans le keychain du systeme
4. Active le service dans `hub.toml`

Les tokens sont lus par les serveurs MCP au runtime via des variables d'environnement :

- `FIGMA_TOKEN`
- `GITLAB_TOKEN` (+ `GITLAB_URL` pour les instances self-hosted)
- `GOOGLE_ACCESS_TOKEN`

## Stockage des secrets

**Principal :** Keychain du systeme (macOS Keychain, Linux secret-service, Windows Credential Manager)

**Cles de secrets connues :**

| Cle | Usage |
|-----|-------|
| `bedrock-token-default` | Token Bearer AWS pour Bedrock |
| `bedrock-token-<project-id>` | Token Bedrock par projet |
| `anthropic-api-key-default` | Cle API Anthropic |
| `anthropic-api-key-<project-id>` | Cle API Anthropic par projet |
| `openrouter-api-key-default` | Cle API OpenRouter |
| `openrouter-api-key-<project-id>` | Cle API OpenRouter par projet |
| `figma-token` | Token API Figma |
| `gitlab-token` | Token API GitLab |
| `gslides-token` | Token OAuth Google Slides |

**Fallback :** Fichier chiffre a `~/.oh/secrets.enc`

- Chiffrement : AES-256-GCM
- Derivation de cle : Argon2id (t=3, memory=64Mo, threads=4)
- Passphrase : variable d'env `OH_PASSPHRASE` ou saisie interactive (min 8 caracteres)

## Flux de lancement

A chaque lancement (`oh run <workflow>`), la configuration du fournisseur est ecrite dans le paquet de session (`~/.oh/bundles/<hash>/`), pas dans le projet :

La construction du paquet lit votre fournisseur et modele configures, puis genere le bloc provider correspondant. Ce bloc pointe vers le proxy d'identifiants du demon oh : il ne contient jamais la vraie cle (la session s'authentifie aupres du proxy avec un jeton `ohs_…`). Pour voir le paquet : `oh bundle show <workflow>`.

Avant de construire quoi que ce soit, oh verifie que le fournisseur a des identifiants (projet, equipe, hub ; profil AWS pour Bedrock) ; sinon le lancement est refuse avec « oh provider setup ». Dans la TUI, le recap de la fiche de lancement affiche ce message et un bouton **Configurer le fournisseur** : il ouvre la vue Fournisseur ; au retour (`Esc`), le recap verifie a nouveau les identifiants.

## Changer de fournisseur

```bash
# Temporairement (une session)
oh run <workflow> --provider anthropic

# Definitivement (defaut hub)
oh config set llm.default_provider anthropic

# Definitivement (un projet)
oh project configure my-project --provider github-copilot   # pris en compte au prochain lancement
```

## Verifier la configuration

```bash
oh status                  # affiche le fournisseur/modele du projet courant
oh doctor                  # valide que les cles API sont configurees
oh config list             # affiche toute la config hub dont le fournisseur
```

## Bonnes pratiques de securite

- Ne jamais stocker les cles API en clair dans des fichiers
- Utiliser `oh provider setup` et `oh mcp setup`, qui stockent dans le keychain du systeme
- Pour la CI/headless : utiliser la variable d'env `OH_PASSPHRASE` pour le fallback chiffre
- Les cles du fournisseur restent dans le keychain : opencode ne recoit qu'un jeton de proxy `ohs_…` par groupe de sessions
- oh n'ecrit plus de `opencode.json` dans le projet ; si vous en gardez un avec des options provider, il doit etre gitignore

## Depannage

| Probleme | Solution |
|----------|----------|
| "Token not configured" | Lancer `oh provider setup` (fournisseur) ou `oh mcp setup` (service MCP) |
| Fournisseur non reconnu | Verifier l'orthographe : bedrock, anthropic, openrouter, github-copilot |
| Mauvais modele | Utiliser `oh project configure --model <nom>` ; pris en compte au prochain lancement (paquet reconstruit) |
| Acces keychain refuse | Autoriser le terminal dans Preferences Systeme > Confidentialite |
| Erreurs du store fallback | Verifier `OH_PASSPHRASE` ou ressaisir quand demande |
