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
| `--keys` | bool | Lister les clés modifiables par `oh config set`/`unset` (au lieu des valeurs) |

**Exemple :**

```bash
oh config list
oh config ls --json
oh config list --keys
```

---

### oh config get

Affiche la valeur d'une cle (toute cle listee par `oh config list`).

```
oh config get <key>
```

**Exemple :**

```bash
oh config get llm.default_provider
oh config get cli.language
```

---

### oh config set

Modifie une valeur. Seules les cles connues sont acceptees (sinon : erreur « clé de configuration inconnue », `oh config list --keys` les liste) ; chaque valeur est verifiee comme dans les Reglages de la TUI (valeur refusee : message avec les valeurs attendues). Les booleens acceptent `true`/`false`, `1`/`0`, `yes`/`no`.

```
oh config set <key> <value>
```

| Groupe | Cles modifiables |
|--------|------------------|
| CLI | `cli.language`, `cli.setup_done` |
| Provider | `llm.default_provider`, `provider.bedrock.aws_profile`, `provider.bedrock.aws_region`, `provider.bedrock.auth_mode`, `provider.anthropic.auth_mode`, `provider.openrouter.auth_mode` |
| MCP | `mcp.figma.enabled`, `mcp.figma.token_key`, `mcp.gitlab.enabled`, `mcp.gitlab.token_key`, `mcp.gitlab.write_enabled`, `mcp.gitlab.url`, `mcp.jira.enabled`, `mcp.jira.token_key`, `mcp.jira.url`, `mcp.gslides.enabled`, `mcp.gslides.token_key` |
| Worktrees | `worktree.auto_cleanup`, `worktree.base_branch`, `worktree.branch_pattern` |
| Modeles | `models.default` |
| Recherche web | `websearch.enabled` |
| Tracker | `tracker.enabled`, `tracker.auto_sync`, `tracker.push_labels`, `tracker.auto_plan_assigned`, `tracker.max_auto_plan_per_member`, `tracker.tracker_url`, `tracker.tracker_token_key`, `tracker.write_enabled` |
| Sessions | `session.attach` (`auto`, `iterm`, `terminal`, `tmux`, `browser`, `suspend`), `session.iterm_style` (`tab`, `split`, `window`), `session.idle_sleep_minutes` (1 à 1440), `session.notify` (`on`, `off`) |
| Execution | `execution.runtime` (`local`, `container`), `execution.engine` (`auto`, `colima`, `podman`, `docker`), `execution.keep_images` (1 à 20), `execution.tool_version` (ex. `2.0.20`), `execution.strict_isolation` |
| Restrictions | `limits.max_active_sessions`, `limits.session_budget_usd`, `limits.daily_budget_usd`, `limits.memory_mb`, `limits.models` (motifs separes par des virgules) ; `0` ou `off` retire la restriction (comme `oh budget set`) |
| Distant | `remote.projects.<id-projet>` (nom d'une cible existante), `remote.targets.<cible>.tag`, `.builder` (`kaniko`, `dind`), `.arch` (`amd64`, `arm64`), `.timeout` (duree GitLab : `3h`, `1h 30m`) ; les cibles se creent avec `oh remote setup` |

Les valeurs par defaut (`auto`, `on`) sont enregistrees vides. `[[teams]]` se regle avec `oh team …`.

**Exemple :**

```bash
oh config set llm.default_provider anthropic
oh config set mcp.gitlab.url https://gitlab.example.com
oh config set worktree.auto_cleanup true
oh config set session.idle_sleep_minutes 10
oh config set execution.engine podman
oh config set limits.daily_budget_usd 20
oh config set remote.projects.t-sru-b267fbf1 acme
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
oh config model default <model> [-p <projet>]
oh config model family <famille> <model> [-p <projet>]
oh config model agent <agent-id> <model> [-p <projet>]
oh config model show [-p <projet>] [-w <workflow>] [--json]
oh config model unset default|family <famille>|agent <agent-id> [-p <projet>]
```

| Flag | Court | Type | Description |
|------|-------|------|-------------|
| `--project` | `-p` | string | Projet (sans : niveau hub ; `show` : projet du dossier courant par defaut) |
| `--workflow` | `-w` | string | `show` seulement : ajoute le niveau d'un workflow (`ticket`, `project:feature`…) |
| `--json` | | bool | `show` seulement : sortie JSON (`workflow`, `project`, `hub`, `team`) |

`show` affiche toute la cascade, dans l'ordre de priorite : workflow (avec `-w`), projet, hub, equipe du projet (recommandations), puis le frontmatter des agents.

Familles : `planning`, `developer`, `quality`, `auditor`, `design`, `documentation`.

Ordre de resolution (priorite decroissante) : workflow·agent > workflow (`models` du workflow) > projet·agent > projet·famille > projet > hub·agent > hub·famille > hub > recommandations d'equipe (agent, famille, global, `config.toml` du team-state) > `model:` du frontmatter de l'agent. Les deux niveaux workflow se reglent dans le workflow, pas avec cette commande. Voir [Resolution des modeles](model-resolution.fr.md).

```bash
oh config model default anthropic/claude-sonnet-4-5
oh config model family quality anthropic/claude-haiku-4-5
oh config model agent reviewer anthropic/claude-opus-4-1 -p mon-app
oh config model show -p mon-app --json
oh config model unset family quality
```
