> [Lire en francais](cli-config.fr.md)

# CLI Reference — Configuration

## Configuration

### oh config list

List all configuration values. Aliases: `ls`

| Flag | Type | Description |
|------|------|-------------|
| `--json` | bool | Output in JSON format |

```bash
oh config list
oh config ls --json
```

---

### oh config get

Get a configuration value.

```bash
oh config get default_provider
oh config get language
```

---

### oh config set

Set a configuration value.

```bash
oh config set default_provider anthropic
oh config set language en
```

---

### oh config unset

Remove a configuration value.

```bash
oh config unset default_provider
```

---

### oh config path

Print the configuration file path.

```bash
oh config path
```

---

### oh config language

Set or display the interface language.

```bash
oh config language fr
oh config language en
oh config language
```

---

### oh config websearch

Enable, disable, or check web search status.

```bash
oh config websearch enable
oh config websearch disable
oh config websearch status
```

---

---

## Provider & Model Configuration

### oh provider setup

Interactive setup for LLM provider credentials.

```bash
oh provider setup
oh provider setup anthropic      # setup specific provider
```

### oh config model

Manage model assignments at various levels.

```bash
oh config model default <model>           # set global default
oh config model family <family> <model>   # set for agent family
oh config model agent <agent> <model>     # set for specific agent
oh config model show                      # display current config
oh config model unset <level> [name]      # remove an override
```

---
