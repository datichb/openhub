# Migration `oc` -> `oh`

Guide de migration de la CLI bash (`oc`) vers la CLI Go (`oh`) v2.0.0.

---

## Pourquoi migrer

| | `oc` (bash) | `oh` (Go) |
|---|---|---|
| Runtime | bash + jq + node + bun + sqlite3 | Binaire Go unique (5.5 MB) |
| Dependencies | 6+ outils externes | Zero (git + opencode seuls requis) |
| Securite | Secrets en clair dans api-keys.local.md | Keychain OS + fallback AES-256-GCM |
| Deploy | Script sequentiel sans rollback | Transactionnel avec snapshot + rollback |
| MCP Servers | TypeScript (npx/bun) | Go natif (stdio, no build step) |
| TUI | Bash ANSI brut | BubbleTea (interactif, souris, scroll) |
| Tests | 0 | 213 tests, 24 packages |
| i18n | Partiel | 481 cles fr/en, parité verifiee |
| Shell completion | Aucune | Dynamique (bash, zsh, fish, powershell) |

---

## Installation de `oh`

```bash
brew install datichb/tap/openhub
```

Verifier :

```bash
oh version
```

---

## Migrer vos projets

### Premier setup

```bash
oh init
```

Le wizard interactif remplace `oc install` + `oc init`. Il :
1. Choisit la langue (fr/en)
2. Telecharge opencode si absent
3. Enregistre votre premier projet
4. Configure les services MCP
5. Initialise le tracker Beads (si disponible)
6. Deploie agents/skills/config dans le projet

### Projets supplementaires

```bash
oh project add --name mon-projet --path ~/workspace/mon-projet
oh deploy -p mon-projet
```

---

## Equivalence des commandes

| `oc` (bash) | `oh` (Go) | Notes |
|---|---|---|
| `oc install` | `oh init` | Wizard complet |
| `oc init PROJECT PATH` | `oh project add -n PROJECT -d PATH` | |
| `oc start PROJECT` | `oh start -p PROJECT` | Detection auto si dans le dossier |
| `oc start --dev` | `oh start --dev` | Picker interactif epics/tickets |
| `oc start --onboard` | `oh start --onboard` | Cree le wiki docs/wiki/ |
| `oc start --parallel` | `oh start --dev` (multi-tickets) | Orchestrator gere le parallele |
| `oc start --resume` | `oh start --resume ID` | |
| `oc start --worktree BRANCH` | `oh start --worktree BRANCH` | |
| `oc quick PROJECT "prompt"` | `oh quick` | Selection interactive |
| `oc deploy opencode PROJECT` | `oh deploy -p PROJECT` | |
| `oc deploy --check` | `oh deploy --check` | |
| `oc deploy --diff` | `oh deploy --diff` | |
| `oc sync` | `oh sync --all` | |
| `oc sync --dry-run` | `oh sync --dry-run` | |
| `oc status` | `oh status` | |
| `oc list` | `oh project list` | |
| `oc remove PROJECT` | `oh project remove PROJECT` | |
| `oc project rename` | `oh project rename` | |
| `oc project move` | `oh project move` | |
| `oc project configure` | `oh project configure` | |
| `oc config set` | `oh config set KEY VALUE` | |
| `oc config get` | `oh config get KEY` | |
| `oc config list` | `oh config list` | Support `--json` |
| `oc config unset` | `oh config unset KEY` | |
| `oc config language` | `oh config language [fr\|en]` | |
| `oc config websearch` | `oh config websearch [enable\|disable\|status]` | |
| `oc audit --type TYPE` | `oh audit --type TYPE` | |
| `oc review` | `oh review` | |
| `oc debug` | `oh debug` | Support `--issue` |
| `oc conventions` | *Supprimé* | Conventions gérées via skills |
| `oc doctor` | `oh doctor` | |
| `oc version` | `oh version` | |
| `oc metrics --period` | `oh metrics --period` | 7d/30d/all |
| `oc dashboard` | `oh dashboard` | TUI interactif |
| `oc optimize` | `oh optimize` | |
| `oc yield` | `oh yield` | |
| `oc worktree list` | `oh worktree list` | Support `--json` |
| `oc worktree create` | `oh worktree add` | |
| `oc worktree remove` | `oh worktree remove` | |
| `oc worktree cleanup` | `oh worktree cleanup` | |
| `oc plugin install rtk` | `oh plugin install rtk` | |
| `oc plugin remove rtk` | `oh plugin remove rtk` | |
| `oc plugin status` | `oh plugin status` | |
| `oc service setup` | `oh service setup` | Wizard interactif |
| `oc service status` | `oh service` | RunE par defaut |
| `oc service remove` | `oh service remove` | |
| `oc beads *` | `oh beads *` | Proxy transparent vers bd |
| `oc board` | `oh board` | TUI + `--watch` |
| `ocp` | `oh start --provider NAME` | Integre dans start |
| `oc upgrade opencode` | `oh upgrade opencode` | |
| `oc update` | `oh upgrade opencode` | bd se gere seul |

---

## Configuration

### hub.json -> hub.toml

L'ancien `config/hub.json` est remplace par `~/.oh/hub.toml` :

```toml
# ~/.oh/hub.toml
[cli]
language = "fr"

[opencode]
version = ""
default_provider = "bedrock"

[websearch]
enabled = true

[mcp.figma]
enabled = true

[mcp.gitlab]
enabled = true

[worktree]
auto_cleanup = true
base_branch = "main"
```

Gerer via `oh config set/get/list/unset`.

### api-keys.local.md -> Keychain

Les tokens ne sont plus stockes en fichier texte. Ils sont dans le keychain OS :

```bash
oh service setup    # wizard interactif, stocke dans le keychain
```

Fallback : si le keychain OS n'est pas disponible, `oh` utilise un fichier chiffre
(`~/.oh/secrets.enc`, AES-256-GCM + Argon2id).

### providers/*.json -> hub.toml

Les templates de providers sont remplaces par une simple cle :

```bash
oh config set opencode.default_provider bedrock
```

Le provider peut aussi etre specifie par commande :

```bash
oh start --provider anthropic
```

---

## Variables d'environnement

| Ancien (`oc`) | Nouveau (`oh`) | Usage |
|---|---|---|
| `OC_NON_INTERACTIVE` | `OH_NON_INTERACTIVE` | Desactiver les prompts interactifs |
| `OC_LOCK_PROJECTS` | `OH_LOCK_PROJECTS` | Verrouiller la liste des projets |
| `OH_PASSPHRASE` | `OH_PASSPHRASE` | Passphrase pour le fallback chiffre |

---

## Breaking changes

### Commandes supprimees

| Commande | Raison |
|----------|--------|
| `oc uninstall` | `brew uninstall openhub` |
| `oc upgrade` (hub via git pull) | `brew upgrade openhub` |
| `oc update` (adapter + bd + skills) | `oh upgrade opencode` pour opencode ; bd se gere seul |
| `oc agent create/edit` | Gestion manuelle des fichiers .md |
| `oc agent select/mode/validate/discover` | Report post-release |
| `oc skills install/remove/update/search/info/add/sync` | Gestion manuelle des fichiers skill |
| `oc config init-providers` | Remplace par `oh config set opencode.default_provider` |
| `ocp` (binary separee) | Integre dans `oh start --provider` |

### Flags renommes

| Ancien | Nouveau | Commande |
|--------|---------|----------|
| `--project/-p PROJECT` | `--project/-p PROJECT` | `oh start`, `oh deploy`, etc. |
| `--dev --label` | `--dev --label` (meme) | `oh start` |
| N/A (nouveau) | `--onboard` | `oh start` |
| N/A (nouveau) | `--refresh` | `oh start` |
| N/A (nouveau) | `--json` | 7 commandes list |

> Note : `-p` est uniformément réservé à `--project` sur toutes les commandes.
> `--prompt` utilise `-m`, `--path` utilise `-d`.

---

## Desinstallation de `oc`

```bash
# Supprimer le hub bash
rm -rf ~/.openhub

# Retirer l'alias du rc file
# Editez ~/.zshrc ou ~/.bashrc et supprimez la ligne:
# alias oc="~/.openhub/oc.sh"

# Recharger le shell
source ~/.zshrc
```

---

## FAQ

### Mes projets existants sont-ils perdus ?

Non. `oh init` vous propose d'enregistrer vos projets. Les agents deployes dans
vos projets (`.opencode/agents/`) restent en place et sont compatibles.

### Puis-je utiliser `oc` et `oh` en parallele ?

Oui, temporairement. Les deux CLI lisent les memes fichiers agents/skills dans
les projets. Cependant la configuration (hub.json vs hub.toml) et les secrets
(api-keys vs keychain) sont separes.

### Comment migrer mes secrets ?

```bash
oh service setup    # re-configurez chaque service interactivement
```

Les tokens seront stockes dans le keychain OS de maniere securisee.

### Le binaire `oh` est-il compatible avec toutes les versions d'opencode ?

`oh doctor` verifie la compatibilite. En cas d'incompatibilite, un warning est
affiche au `oh start` avec la commande de mise a jour.
