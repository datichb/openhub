> [Read in English](cli-config.en.md)

# Reference CLI — Configuration

## Configuration

`oh config` lit et modifie la configuration du hub (`hub.toml`, dans `~/.oh/` ou `$OH_HOME`). Les cles sont ecrites en notation pointee (`section.cle`). Voir la [reference de configuration](config.fr.md).

### oh config list

Affiche toute la configuration (cle, valeur).

**Alias :** `oh config ls`

```
oh config list [options]
```

| Flag | Type | Description |
|------|------|-------------|
| `--json` | bool | Sortie au format JSON |

**Exemple :**

```bash
oh config list
oh config ls --json
```

---

### oh config get

Affiche la valeur d'une cle (toute cle listee par `oh config list`).

```
oh config get <key>
```

**Exemple :**

```bash
oh config get opencode.default_provider
oh config get cli.language
```

---

### oh config set

Modifie une valeur. Seules les cles connues sont acceptees (sinon : erreur « unknown config key ») ; les booleens acceptent `true`/`false`, `1`/`0`, `yes`/`no`.

```
oh config set <key> <value>
```

| Groupe | Cles modifiables |
|--------|------------------|
| CLI | `cli.language`, `cli.setup_done` |
| Provider | `opencode.default_provider`, `provider.bedrock.aws_profile`, `provider.bedrock.aws_region`, `provider.bedrock.auth_mode`, `provider.anthropic.auth_mode`, `provider.openrouter.auth_mode` |
| MCP | `mcp.figma.enabled`, `mcp.figma.token_key`, `mcp.gitlab.enabled`, `mcp.gitlab.token_key`, `mcp.gitlab.write_enabled`, `mcp.gitlab.url`, `mcp.jira.enabled`, `mcp.jira.token_key`, `mcp.jira.url`, `mcp.gslides.enabled`, `mcp.gslides.token_key` |
| Worktrees | `worktree.auto_cleanup`, `worktree.base_branch`, `worktree.branch_pattern` |
| Modeles | `models.default` |
| Recherche web | `websearch.enabled` |
| Tracker | `tracker.enabled`, `tracker.auto_sync`, `tracker.push_labels`, `tracker.auto_plan_assigned`, `tracker.max_auto_plan_per_member`, `tracker.tracker_url`, `tracker.tracker_token_key`, `tracker.write_enabled` |

Les autres sections (`[execution]`, `[limits]`, `[[teams]]`…) se reglent dans la TUI (Reglages) ou avec leur commande (`oh budget set`, `oh team …`).

**Exemple :**

```bash
oh config set opencode.default_provider anthropic
oh config set mcp.gitlab.url https://gitlab.example.com
oh config set worktree.auto_cleanup true
```

---

### oh config unset

Supprime une cle connue de `hub.toml` ; la valeur par defaut s'applique ensuite, s'il y en a une.

```
oh config unset <key>
```

**Exemple :**

```bash
oh config unset provider.bedrock.aws_profile
```

---

### oh config path

Affiche le chemin du fichier de configuration.

```
oh config path
```

**Exemple :**

```bash
oh config path
# /Users/alice/.oh/hub.toml
```

---

### oh config language

Sans argument, affiche la langue de l'interface ; avec `fr` ou `en`, la change.

```
oh config language [fr|en]
```

**Exemple :**

```bash
oh config language        # Affiche la langue courante
oh config language fr     # Passe en francais
oh config language en     # Passe en anglais
```

---

### oh config websearch

Active ou desactive les permissions `websearch` et `webfetch` des agents (recherche web via Exa AI). Pris en compte au prochain lancement d'une session (le paquet de session est reconstruit). Un argument est obligatoire.

```
oh config websearch enable|disable|status
```

**Exemple :**

```bash
oh config websearch status
oh config websearch enable
oh config websearch disable
```

---

## Configuration provider et modeles

### oh provider setup

Assistant de configuration des identifiants du provider LLM (cle API, profil AWS, bearer token), au niveau hub ou projet. Sans argument, propose un selecteur.

```
oh provider setup [provider-name] [options]
```

| Flag | Court | Type | Description |
|------|-------|------|-------------|
| `--project` | `-p` | string | Configurer le provider d'un projet |

Providers : `bedrock`, `anthropic`, `openrouter`, `github-copilot`.

```bash
oh provider setup
oh provider setup anthropic
oh provider setup bedrock -p mon-app
```

---

### oh config model

Modeles par agent, par famille ou globaux, au niveau hub (`hub.toml [models]`) ou projet (base de donnees du hub). Le modele resolu est normalise vers le provider du projet a la construction du paquet de session.

```
oh config model default <model> [-j <projet>]
oh config model family <famille> <model> [-j <projet>]
oh config model agent <agent-id> <model> [-j <projet>]
oh config model show [-j <projet>] [--json]
oh config model unset default|family <famille>|agent <agent-id> [-j <projet>]
```

| Flag | Court | Type | Description |
|------|-------|------|-------------|
| `--project` | `-j` | string | Projet (sans : niveau hub). Attention : la forme courte est `-j`, pas `-p` |
| `--json` | | bool | `show` seulement : sortie JSON |

Familles : `planning`, `developer`, `quality`, `auditor`, `design`, `documentation`.

Ordre de resolution (priorite decroissante) : workflow·agent > workflow (`models` du workflow) > projet·agent > projet·famille > projet > hub·agent > hub·famille > hub > recommandations d'equipe (agent, famille, global, `config.toml` du team-state) > `model:` du frontmatter de l'agent. Les deux niveaux workflow se reglent dans le workflow, pas avec cette commande. Voir [Resolution des modeles](model-resolution.fr.md).

```bash
oh config model default anthropic/claude-sonnet-4-5
oh config model family quality anthropic/claude-haiku-4-5
oh config model agent reviewer anthropic/claude-opus-4-1 -j mon-app
oh config model show -j mon-app --json
oh config model unset family quality
```
