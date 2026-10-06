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
3. `opencode.default_provider` dans `~/.oh/hub.toml`
4. `"bedrock"` (valeur par defaut en dur)

## Configuration au niveau Hub

Definir le fournisseur par defaut pour tous les projets :

```bash
oh config set opencode.default_provider bedrock
```

Dans `~/.oh/hub.toml` :

```toml
[opencode]
default_provider = "bedrock"
```

## Configuration detaillee du provider (hub.toml)

```toml
[provider.bedrock]
aws_profile = "default"           # profil AWS a utiliser
aws_region = "eu-west-1"          # region Bedrock
auth_mode = "bearer"              # "bearer" | "profile"
```

Cette section est facultative — si absente, `oh` utilise les credentials de l'environnement.

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

## Changer de fournisseur

```bash
# Temporairement (une session)
oh run <workflow> --provider anthropic

# Definitivement (defaut hub)
oh config set opencode.default_provider anthropic

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
