> [Read in English](cli-mcp.en.md)

# Reference CLI — MCP & Plugins

## Gestion MCP

Serveurs MCP integres a oh : `figma`, `github`, `gitlab`, `gslides`, `jira`, `linear`, `team`, plus les serveurs personnalises declares dans `~/.oh/mcp/<nom>/manifest.json`. Une session recoit les serveurs MCP du projet (ou seulement ceux listes par le champ `mcp:` du workflow) dans son paquet de session. Pages par serveur : [Figma](mcp-figma.fr.md), [GitHub](mcp-github.fr.md), [GitLab](mcp-gitlab.fr.md), [Google Slides](mcp-gslides.fr.md), [Jira](mcp-jira.fr.md), [Linear](mcp-linear.fr.md), [Team](mcp-team.fr.md).

Toutes les commandes ci-dessous (sauf `list` et `serve`) acceptent `-p, --project <nom ou ID>`.

### oh mcp enable

Active un service MCP au niveau hub ou pour un projet. Avec `--project` et sans token, propose d'heriter le token du hub ou d'en configurer un nouveau.

```
oh mcp enable <service> [options]
```

| Flag | Court | Type | Description |
|------|-------|------|-------------|
| `--project` | `-p` | string | Nom ou ID du projet |

**Exemple :**

```bash
oh mcp enable figma
oh mcp enable gitlab --project mon-projet
```

---

### oh mcp disable

Desactive un service MCP au niveau hub ou pour un projet. Avec `--project`, le service est desactive pour ce projet quelle que soit la config du hub.

```
oh mcp disable <service> [options]
```

| Flag | Court | Type | Description |
|------|-------|------|-------------|
| `--project` | `-p` | string | Nom ou ID du projet |

**Exemple :**

```bash
oh mcp disable figma
oh mcp disable gitlab --project mon-projet
```

---

### oh mcp reset

Supprime l'override projet d'un service (retour a la config du hub). Exige `--project`.

```
oh mcp reset <service> --project <name>
```

| Flag | Court | Type | Description |
|------|-------|------|-------------|
| `--project` | `-p` | string | **(requis)** Nom ou ID du projet |

**Exemple :**

```bash
oh mcp reset figma --project mon-projet
```

---

### oh mcp setup

Configure un service MCP (assistant interactif : token, options). Services : `figma`, `gitlab`, `gslides`, `jira` ; sans argument, le service est choisi dans une liste. Le mode ecriture est propose pour les services qui ont des outils d'ecriture (`gitlab`, `jira`). Avec `--project`, le token est stocke dans le trousseau sous une cle propre au projet.

```
oh mcp setup [service] [options]
```

| Flag | Court | Type | Description |
|------|-------|------|-------------|
| `--project` | `-p` | string | Nom ou ID du projet |

**Exemple :**

```bash
oh mcp setup
oh mcp setup jira --project mon-projet
```

---

### oh mcp status

Affiche le statut de tous les services MCP. Avec `--project`, affiche la config effective (overrides du projet compris).

```
oh mcp status [options]
```

| Flag | Court | Type | Description |
|------|-------|------|-------------|
| `--project` | `-p` | string | Nom ou ID du projet |

**Exemple :**

```bash
oh mcp status
oh mcp status --project mon-projet
```

---

### oh mcp serve

Lance un serveur MCP integre sur stdin/stdout (JSON-RPC). Utilise par opencode, via la declaration du serveur dans le paquet de session ; rarement lance a la main.

```
oh mcp serve <name> [--token-key <cle>]
```

| Flag | Type | Description |
|------|------|-------------|
| `--token-key` | string | Cle du trousseau d'ou lire le token du service (pose `FIGMA_TOKEN`, `GITLAB_TOKEN` ou `GOOGLE_ACCESS_TOKEN` s'ils ne sont pas deja definis) |

Les outils sont exposes sans le nom du serveur (`get_project`) : l'outil (opencode) le prefixe, la session voit `gitlab_get_project` (nom cite par les agents, les skills et les regles de permission). Un appel sous l'ancien nom prefixe (`gitlab_get_project` cote serveur) reste accepte.

`oh mcp serve workflow` est interne : serveur MCP `workflow` (voir [Serveur MCP workflow](#serveur-mcp-workflow)) injecte dans chaque paquet de session, absent de `list`, `setup` et `enable`.

**Exemple :**

```bash
oh mcp serve gitlab
oh mcp serve figma --token-key openhub.mcp.figma.token
```

---

### Serveur MCP workflow

Injecté dans chaque session lancée depuis un workflow ; sans état, il transmet chaque appel au démon oh pour la session qui appelle (un sous-agent est rattaché à la session oh qui l'a délégué). Les agents voient ses outils sous le nom `workflow_<outil>`.

| Outil | Paramètres | Rôle |
|-------|------------|------|
| `workflow_status` | — | État du workflow : mode, checkpoints, prochain checkpoint, agents et opérations verrouillés, sorties |
| `workflow_checkpoint` | `id`, `summary`, `reports` (facultatif) | Passe un checkpoint (attend la validation quand il est en pause dans le mode) ; `reports` cite des rapports `rp-…` |
| `workflow_outputs` | `type`, `value`, `id` | Déclare une sortie typée |
| `workflow_report` | `title`, `body` | Dépose le rapport complet de l'agent ; oh répond par son identifiant `rp-<n>` |
| `workflow_report_get` | `id`, `section` (facultatif) | Lit un rapport, en entier ou une section (titre Markdown) |
| `workflow_reports` | — | Liste les rapports de la session (identifiant, auteur, titre, taille, heure, sections) |

**Rapports (`workflow_report`).** L'agent dépose son rapport complet ; oh le garde et les coordinateurs ne transmettent que son identifiant. L'auteur est l'agent qu'oh voit appeler l'outil (un paramètre `agent` serait ignoré). Les rapports déposés depuis le checkpoint précédent, et ceux que l'agent cite dans `reports`, sont joints à la carte du checkpoint ; chaque dépôt s'inscrit dans la frise (`▤ rp-2 (developer)`). Limites : 64 Ko par rapport, 100 rapports et 4 Mo par session ; au-delà, l'outil refuse et l'agent inclut le rapport dans son bloc. Sans démon (ou quand il ne garde pas les rapports : `OH_WORKFLOW_REPORTS=off`), l'outil répond « non enregistré » sans erreur et l'agent revient au rapport complet dans son bloc. Les agents ne lisent ni n'écrivent les rapports autrement que par ces outils (refus de `~/.oh/sessions` par les gardes d'oh, avant les exceptions des brouillons de session). Lecture côté utilisateur : [`oh session report`](cli-sessions.fr.md#oh-session-report).

---

### oh mcp list

Liste tous les serveurs MCP d'oh (`figma`, `github`, `gitlab`, `gslides`, `jira`, `linear`, `team`, puis les serveurs personnalises de `~/.oh/mcp/`) et leur commande.

**Alias :** `oh mcp ls`

```
oh mcp list [options]
```

| Flag | Type | Description |
|------|------|-------------|
| `--json` | bool | Sortie au format JSON |

**Exemple :**

```bash
oh mcp list
oh mcp ls --json
```

---

### oh service (deprecie)

`oh service`, `oh service setup` et `oh service remove` existent encore mais sont deprecies et masques de l'aide : utilisez `oh mcp status`, `oh mcp setup` et `oh mcp disable`.

---

## Gestion des plugins

`oh plugin` (plugins globaux opencode V1, dont RTK) a été supprimé en v5 : les plugins se déclarent par workflow (`plugins:`), voir [Workflows livrés](workflows.fr.md#plugins-et-code-mode).
