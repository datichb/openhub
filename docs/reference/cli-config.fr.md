> [Read in English](cli-config.en.md)

# Reference CLI — Configuration

## Configuration

### oh config list

Affiche la configuration.

**Alias :** `oh config ls`

```
oh config list [options]
```

| Flag | Description |
|------|-------------|
| `--json` | Sortie au format JSON |

**Exemple :**

```bash
oh config list
oh config ls --json
```

---

### oh config get

Lire une valeur de configuration.

```
oh config get <key>
```

**Exemple :**

```bash
oh config get default_provider
oh config get language
```

---

### oh config set

Definir une valeur de configuration.

```
oh config set <key> <value>
```

**Exemple :**

```bash
oh config set default_provider anthropic
oh config set language fr
```

---

### oh config unset

Supprimer une cle de configuration.

```
oh config unset <key>
```

**Exemple :**

```bash
oh config unset custom_model
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
# /home/user/.config/opencode-hub/config.yaml
```

---

### oh config language

Changer la langue de l'interface.

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

Gerer les permissions de recherche web (WebSearch).

```
oh config websearch [enable|disable|status]
```

**Exemple :**

```bash
oh config websearch status
oh config websearch enable
oh config websearch disable
```

---

---

## Configuration provider et modeles

### oh provider setup

Configuration interactive des credentials du provider LLM.

```bash
oh provider setup
oh provider setup anthropic      # configurer un provider specifique
```

### oh config model

Gerer les assignations de modeles a differents niveaux.

```bash
oh config model default <modele>            # definir le defaut global
oh config model family <famille> <modele>   # definir pour une famille d'agents
oh config model agent <agent> <modele>      # definir pour un agent specifique
oh config model show                        # afficher la config actuelle
oh config model unset <niveau> [nom]        # supprimer un override
```

---
