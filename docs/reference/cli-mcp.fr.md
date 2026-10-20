> [Read in English](cli-mcp.en.md)

# Référence CLI — MCP & Plugins

## Gestion MCP

Serveurs MCP integres a oh : `figma`, `github`, `gitlab`, `gslides`, `jira`, `linear`, `team`, plus les serveurs personnalises déclarés dans `~/.oh/mcp/<nom>/manifest.json`. Une session reçoit les serveurs MCP du projet (ou seulement ceux listes par le champ `mcp:` du workflow) dans son paquet de session. Pages par serveur : [Figma](mcp-figma.fr.md), [GitHub](mcp-github.fr.md), [GitLab](mcp-gitlab.fr.md), [Google Slides](mcp-gslides.fr.md), [Jira](mcp-jira.fr.md), [Linear](mcp-linear.fr.md), [Team](mcp-team.fr.md).

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

Configure un service MCP (assistant interactif : token, options). Services : `figma`, `gitlab`, `gslides`, `jira` ; sans argument, le service est choisi dans une liste. Le mode écriture est propose pour les services qui ont des outils d'écriture (`gitlab`, `jira`). Avec `--project`, le token est stocke dans le trousseau sous une clé propre au projet.

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
| `--token-key` | string | Clé du trousseau d'ou lire le token du service (pose `FIGMA_TOKEN`, `GITLAB_TOKEN` ou `GOOGLE_ACCESS_TOKEN` s'ils ne sont pas déjà définis) |

Les outils sont exposes sans le nom du serveur (`get_project`) : l'outil (opencode) le prefixe, la session voit `gitlab_get_project` (nom cite par les agents, les skills et les règles de permission). Un appel sous l'ancien nom prefixe (`gitlab_get_project` côté serveur) reste accepte.

`oh mcp serve workflow` est interne : serveur MCP `workflow` (voir [Serveur MCP workflow](#serveur-mcp-workflow)) injecte dans chaque paquet de session, absent de `list`, `setup` et `enable`. De même, `oh mcp serve wiki` sert le [Serveur MCP wiki](#serveur-mcp-wiki).

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

Au passage d'un checkpoint qui déverrouille le commit, la réponse de `workflow_checkpoint` peut porter une ligne « Wiki : … » : les propositions du wiki acceptées à la carte, appliquées par oh, et les fichiers à committer avec le code (voir ci-dessous).

---

### Serveur MCP wiki

Wiki du projet (`docs/wiki/`), modernisation LD-J. Injecté dans les sessions lancées depuis un workflow quand le projet a un `docs/wiki/` ; sans état, comme `workflow`, il transmet chaque appel au démon oh (jeton du groupe de serveurs) pour la session qui appelle. Le wiki lu est celui du dossier de la session (projet ou worktree) ; la file de propositions est celle du projet. Les agents voient ses outils sous le nom `wiki_<outil>`.

| Outil | Paramètres | Rôle |
|-------|------------|------|
| `wiki_index` | — | Titre et sections de l'index, pages (titre, `updated`, `confidence`), god nodes **calculés par oh**, propositions en attente (projet, session) |
| `wiki_read` | `page`, `section` (facultatif) | Lit une page (chemin relatif à `docs/wiki/`), ou une section |
| `wiki_propose` | `page`, `content`, `tag`, `source` (obligatoire pour `CONFIRMÉ`), `section`, `replaces` (facultatifs) | Met une proposition en file (un ajout, ou le remplacement d'une ligne avec `replaces`) ; oh répond par son identifiant `wp-<n>` |
| `wiki_pending` | `session_only` (facultatif) | Propositions en attente (acceptées ou non), du projet ou de la session |
| `wiki_apply` | `ids` | Applique des propositions : **documentarian seulement** |
| `wiki_check` | `paths`, `base`, `head`, `since_sync` (facultatifs) | Vérifie le wiki (frontmatter, liens, tags, fraîcheur, arborescence, god nodes), comme [`oh wiki check`](cli-projects.fr.md#oh-wiki-check) ; restreint à un changement : commits, fichiers, pages concernées et dérives (`drift_confirmed`, `drift_god_node`) |

**Propositions (`wiki_propose`).** L'auteur est l'agent qu'oh voit appeler l'outil (un paramètre `agent` serait ignoré) ; la ligne de tag est écrite par oh (tag, auteur, date, source) et celles que le contenu porterait sont retirées. Le contenu est nettoyé (caractères de contrôle et séquences de terminal supprimés) et neutralisé : titres, marqueurs de frontmatter, HTML et commentaires (dont les marqueurs `oh:when`) échappés, bloc de code non fermé refermé. Toute référence aux données de session d'oh (`~/.oh/sessions`, dont les brouillons `scratch/`, ou le dossier des sessions réellement utilisé) est refusée. Limites : 4 Ko et 60 lignes par proposition, 50 propositions ouvertes par session, 500 par projet. Une proposition identique et ouverte de la même session n'est pas dupliquée. Sans démon, l'outil répond « non enregistrée » sans erreur et l'agent garde la proposition dans son rapport.

**Remplacer une ligne (`replaces`, p5-fixes).** Pour corriger une entrée au lieu d'en ajouter une corrigée : `replaces` est la ligne de la page recopiée exactement (une seule ligne, espaces autour mis à part), `content` la nouvelle ligne (une ligne, nettoyée et neutralisée comme un ajout). La ligne doit être dans la page une fois (dans la section `section` si elle est donnée), hors du frontmatter et des blocs de code ; un titre, un séparateur, une barrière de code ou une ligne de tag ne se remplacent pas. oh vérifie la ligne à la proposition (absente ou en double : refus) et de nouveau à l'application : si elle a disparu, la proposition passe en `stale` (périmée) et n'est pas appliquée. Appliquée, la nouvelle ligne prend la place de l'ancienne (même indentation, même puce), suivie de sa ligne de tag écrite par oh ; la ligne de tag de l'ancienne entrée, juste après, est retirée. La carte du checkpoint de commit (TUI), `oh wiki pending` et `wiki_pending` montrent l'ancienne ligne puis la nouvelle ; une modification à la carte change la nouvelle ligne.

**Validation et application.** Les propositions ouvertes de la session s'affichent sur la carte du checkpoint de commit (TUI) : accepter, modifier (sur une ligne dans le formulaire, ou sur plusieurs dans `$EDITOR` ; contenu nettoyé à nouveau ; l'original est conservé) ou refuser (avec un motif facultatif, conservé avec la proposition), une par une ou toutes à la fois. Hors de la TUI : [`oh wiki decide`](cli-projects.fr.md#oh-wiki-decide). Quand le checkpoint passe, oh applique les propositions acceptées avant de rendre la main à l'agent (entrée `✎ wp-…` dans la frise, statut `applied`, `applied_by: oh`). Les autres restent en file ([`oh wiki pending`](cli-projects.fr.md#oh-wiki-pending)), comme toutes celles des workflows sans commit.

**`wiki_apply`.** Refusé par une règle de permission à tous les agents sauf le `documentarian`, pour qui il est soumis à l'utilisateur (`ask`) ; le démon le refuse aussi (403) à tout autre agent qu'il voit appeler, quelles que soient les règles de la session. Il applique les propositions en attente ou acceptées du projet.

**Session distante.** Mêmes outils, servis par le démon du job sur le wiki de son clone ; `wiki_apply` y est refusé à tous les agents (« les propositions remontent sur la machine ») et le checkpoint de commit n'applique rien. Les propositions reviennent avec l'artefact `wiki.json` et entrent, en attente, dans la file du projet au `oh session fetch` (schéma 2 : une proposition qui remplace une ligne voyage à part, qu'une version d'oh au schéma 1 laisse de côté au lieu de l'ajouter ; un `wiki.json` d'un schéma plus récent fait échouer l'import : mettez oh à jour, puis relancez `oh session fetch`) ; les identifiants `wp-…` de la transcription distante ne sont pas renumérotés (limite connue) ; voir [Wiki documentaire vivant › Sessions distantes](../architecture/living-wiki.fr.md#sessions-distantes).

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

### oh service (déprécié)

`oh service`, `oh service setup` et `oh service remove` existent encore mais sont deprecies et masques de l'aide : utilisez `oh mcp status`, `oh mcp setup` et `oh mcp disable`.

---

## Gestion des plugins

`oh plugin` (plugins globaux opencode V1, dont RTK) a été supprimé en v5 : les plugins se déclarent par workflow (`plugins:`), voir [Workflows livrés](workflows.fr.md#plugins-et-code-mode).
