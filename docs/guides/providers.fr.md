> [Read in English](providers.en.md)

# Configuration des fournisseurs

Ce guide couvre la resolution des fournisseurs LLM par OpenCode Hub, la gestion des tokens API et le deploiement des parametres de fournisseur dans les projets.

## Fournisseurs supportes

| Fournisseur | Backend | Methode d'authentification |
|-------------|---------|----------------------------|
| **bedrock** (par defaut) | AWS Bedrock | Bearer Token AWS (via keychain) |
| **anthropic** | API Anthropic | Cle API (via env ou keychain) |
| **openai** | OpenAI / OpenRouter | Cle API |
| **openrouter** | OpenRouter | Cle API |

## Ordre de resolution du fournisseur

Quand `oh start` lance opencode, le fournisseur est resolu dans cet ordre :

1. Flag `--provider` / `-P` sur `oh start` (priorite la plus haute)
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
oh service setup
# Selectionner le fournisseur -> entrer votre bearer token
# Stocke sous la cle : bedrock-token-default (ou bedrock-token-<project-id>)
```

Au lancement, `oh start` recupere le token depuis le keychain et le passe comme variable d'environnement `AWS_BEARER_TOKEN_BEDROCK` a opencode.

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

Definir votre cle API dans le bloc provider du `opencode.json` du projet (injecte par `oh deploy`) :

```bash
oh deploy -j my-project --provider anthropic
```

Ou configurer via des variables d'environnement lues directement par opencode.

## Tokens des services MCP

Les serveurs MCP (Figma, GitLab, Google Slides) necessitent leurs propres tokens :

```bash
oh service setup
```

Configuration par projet (surcharge le hub) :

```bash
oh service setup --project my-project
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

## Flux de deploiement

Quand vous lancez `oh deploy`, la configuration du fournisseur est ecrite dans le `opencode.json` du projet :

```json
{
  "model": "claude-sonnet-4-5",
  "provider": {
    "anthropic": {
      "options": { "apiKey": "..." }
    }
  }
}
```

Le moteur de deploiement lit votre fournisseur et modele configures, puis genere le bloc provider correspondant.

## Changer de fournisseur

```bash
# Temporairement (une session)
oh start --provider anthropic

# Definitivement (defaut hub)
oh config set opencode.default_provider anthropic

# Definitivement (un projet)
oh project configure my-project --provider openai
oh deploy -j my-project   # re-deployer pour appliquer
```

## Verifier la configuration

```bash
oh status                  # affiche le fournisseur/modele du projet courant
oh doctor                  # valide que les cles API sont configurees
oh config list             # affiche toute la config hub dont le fournisseur
```

## Bonnes pratiques de securite

- Ne jamais stocker les cles API en clair dans des fichiers
- Utiliser `oh service setup` qui stocke dans le keychain du systeme
- Pour la CI/headless : utiliser la variable d'env `OH_PASSPHRASE` pour le fallback chiffre
- Les tokens Bedrock sont injectes par session, jamais ecrits sur disque
- `opencode.json` peut contenir des options provider mais doit etre gitignore

## Depannage

| Probleme | Solution |
|----------|----------|
| "Token not configured" | Lancer `oh service setup` |
| Fournisseur non reconnu | Verifier l'orthographe : bedrock, anthropic, openai, openrouter |
| Mauvais modele | Utiliser `oh project configure --model <nom>` puis `oh deploy` |
| Acces keychain refuse | Autoriser le terminal dans Preferences Systeme > Confidentialite |
| Erreurs du store fallback | Verifier `OH_PASSPHRASE` ou ressaisir quand demande |
