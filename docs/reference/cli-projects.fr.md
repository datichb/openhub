> [Read in English](cli-projects.en.md)

# Référence CLI — Projets

## Projets

`oh project` gere les projets enregistres dans le hub. **Alias :** `oh p`.

### oh project list

Liste les projets enregistres.

**Alias :** `oh project ls`

```
oh project list [options]
```

| Flag | Court | Type | Description |
|------|-------|------|-------------|
| `--status` | `-s` | string | Filtrer par statut (`active`, `archived`) |
| `--json` | | bool | Sortie au format JSON |

**Exemple :**

```bash
oh project list
oh project ls -s active
oh project list --json
```

---

### oh project add

Enregistre un projet dans le hub. Sans flag, lance un assistant interactif.

**Alias :** `oh project register`

```
oh project add [options]
```

| Flag | Court | Type | Description |
|------|-------|------|-------------|
| `--name` | `-n` | string | Nom du projet |
| `--path` | `-d` | string | Chemin du projet (défaut : répertoire courant) |
| `--language` | `-l` | string | Langage principal |
| `--yes` | `-y` | bool | Beads zéro impact : retirer ce que `bd init` a écrit sans confirmation et annuler son commit s'il n'est pas poussé (sinon : liste, annulation du commit comprise, puis confirmation ; sans terminal, rien n'est changé) |

**Exemple :**

```bash
oh project add -n api -d ./services/api -l go
oh project register -n frontend --language typescript
oh project add
```

---

### oh project remove

Supprime un projet du registre (les fichiers sur disque ne sont pas supprimes).

**Alias :** `oh project rm`

```
oh project remove [project-id] [options]
```

| Flag | Court | Type | Description |
|------|-------|------|-------------|
| `--yes` | `-y` | bool | Supprimer sans confirmation (obligatoire sans terminal). Anciens noms : `--force`, `-f` (dépréciés, même effet, avec un avertissement) |

**Exemple :**

```bash
oh project remove mon-projet
oh project rm -y legacy-app
```

---

### oh project rename

Change le nom d'affichage d'un projet. L'ID ne change pas. Interactif si les arguments sont omis.

```
oh project rename [project-id] [new-name]
```

**Exemple :**

```bash
oh project rename api api-v2
oh project rename
```

---

### oh project move

Met a jour le chemin enregistre d'un projet (après un deplacement sur le disque). Ne deplace pas le dossier. Interactif si les arguments sont omis.

```
oh project move [project-id] [new-path]
```

**Exemple :**

```bash
oh project move api ../new-location/api
oh project move
```

---

### oh project configure

Configure les paramètres d'un projet : provider LLM, modèle, langage. Ils sont utilises au lancement des sessions (`oh run`). Sans flag, lance un assistant interactif.

```
oh project configure [project-id] [options]
```

| Flag | Court | Type | Description |
|------|-------|------|-------------|
| `--provider` | `-P` | string | Provider LLM : `bedrock`, `anthropic`, `openrouter`, `github-copilot` (valeur non vérifiée) |
| `--model` | | string | Modèle LLM (l'ancienne forme courte `-m` reste acceptée, avec un avertissement) |
| `--language` | `-l` | string | Langage principal |

Le workflow et l'environnement d'exécution par défaut du projet se reglent dans la TUI (Config projet › Exécution), voir [`oh run`](cli-workflows.fr.md#oh-run).

**Exemple :**

```bash
oh project configure api -P anthropic -m claude-sonnet-4-20250514
oh project configure api -l go
oh project configure
```

---

## Merge requests

### oh mr create

Crée (ou retrouve) la merge request GitLab de la branche courante, vers la branche de base du projet, et assigne si besoin un relecteur de l'équipe. Remplace `oh review --publish`. La review elle-même est un workflow : [`oh run review`](cli-workflows.fr.md#oh-run). Demande l'accès en écriture à GitLab (`oh mcp setup gitlab`) ; la fusion reste une action manuelle.

```
oh mr create [-p <projet>] [--reviewer <membre>]
```

| Flag | Court | Type | Description |
|------|-------|------|-------------|
| `--project` | `-p` | string | Projet (détecté depuis le dossier courant) |
| `--reviewer` | | string | Identifiant du membre de l'équipe à assigner comme relecteur |

```bash
oh mr create
oh mr create --reviewer alice
```

---

## Wiki du projet

### oh wiki check

Vérifie le wiki du projet (`docs/wiki/` du plus proche dossier parent qui en a un) : frontmatter (`updated` en date AAAA-MM-JJ, `confidence` dans `confirmed | mixed | uncertain` et cohérent avec les tags de la page, `agents` ; aucune autre clé, `synced` mis à part dans `index.md` : clé inconnue `frontmatter_key`, en avertissement, en erreur dans une spec de `specs/` comme à `verify.spec`), liens relatifs et ancres, lignes de tag (`` — `TAG` · agent · AAAA-MM-JJ[ · source] ``, une source pour `CONFIRMÉ`), pages sans tag (sauf `specs/`, `decisions/`, `postmortems/`, `retros/`), arborescence (page hors de `index.md`, `glossary.md`, `technical/`, `business/`, `specs/`, `decisions/`, `postmortems/`, `retros/`), fraîcheur (page plus vieille que le dernier commit d'un fichier qu'elle cite ; fichier cité absent) et god nodes calculés par oh face à la table de `index.md`. Affiche aussi les god nodes calculés. Code de sortie non nul s'il y a des erreurs (utilisable en CI).

```
oh wiki check [dir] [--fix] [--json] [--path <fichier>]… [--base <ref> [--head <ref>]] [--since-sync]
```

| Flag | Type | Description |
|------|------|-------------|
| `--fix` | bool | Répare ce qu'oh sait calculer : `confidence` des pages, table des god nodes de `index.md` (et son `updated`) |
| `--json` | bool | Rapport en JSON (`pages`, `god_nodes`, `issues` avec `page`, `line`, `kind`, `severity`, `message`, `fixable` ; `scope` pour un changement) |
| `--path` | string, répétable | Restreint à un changement : fichiers modifiés (relatifs à la racine) |
| `--base`, `--head` | string | Restreint aux fichiers modifiés de la base commune de `--base` et `--head` jusqu'à `--head` (sans `--head` : l'arbre de travail, fichiers non commités compris) |
| `--since-sync` | bool | Restreint aux commits depuis la dernière synchronisation (`synced:` d'`index.md`, sinon le dernier commit de `docs/wiki/`) |

Restreinte à un changement, la vérification ne garde que les problèmes des pages du wiki qu'il modifie, liste les commits, les fichiers et les **pages concernées** (qui citent un fichier modifié ou portent un god node touché), et ajoute les dérives : `drift_confirmed` (page `CONFIRMÉ` qui cite un fichier modifié, non mise à jour) et `drift_god_node` (god node touché dont aucune page n'est mise à jour). C'est l'axe documentation du `reviewer` et le point de départ de [`doc-sync`](workflows.fr.md#doc-sync).

oh ne suit jamais un lien symbolique dans `docs/wiki/` ; une page de plus de 256 Ko, ou au-delà de 400 pages, n'est pas lue (avertissement).

### oh wiki pending

Liste les propositions des agents pour le wiki du projet ([`wiki_propose`](cli-mcp.fr.md#serveur-mcp-wiki)) encore à appliquer : acceptées à la carte du commit (oh les applique au passage du checkpoint) ou en attente (workflows sans commit, pour `doc-sync`).

```
oh wiki pending [dir] [--all] [--session <id>] [--json]
```

| Flag | Type | Description |
|------|------|-------------|
| `--all` | bool | Toutes les propositions, appliquées et refusées comprises |
| `--session` | string | Seulement celles d'une session |
| `--json` | bool | Sortie JSON |

Le projet est celui qu'oh connaît pour le dossier (ou un dossier parent). Un remplacement de ligne (`replaces`) s'affiche « ancienne ligne » → « nouvelle ligne » ; une proposition dont la ligne a disparu à l'application est `stale` (périmée, non appliquée, visible avec `--all`). Avec `--all`, une proposition refusée avec un motif ajoute une colonne `MOTIF DU REFUS` (le motif est aussi le champ `reason` de `--json`). Pour décider d'une proposition hors de la TUI : [`oh wiki decide`](#oh-wiki-decide).

### oh wiki decide

Enregistre la décision de l'utilisateur sur des propositions de la file, hors de la TUI : mêmes choix que sur la carte du checkpoint de commit, même service.

```
oh wiki decide <id>... (--accept | --edit | --content <texte> | --file <fichier|-> | --refuse [--reason <motif>] | --pending) [--dir <dossier>]
```

| Flag | Type | Description |
|------|------|-------------|
| `--accept` | bool | Accepter : oh l'applique au passage du checkpoint de commit de sa session, sinon `doc-sync` |
| `--edit` | bool | Modifier le contenu dans `$EDITOR` (plusieurs lignes), puis accepter ; un contenu vidé ou inchangé n'enregistre rien |
| `--content` | string | Modifier sans éditeur : le nouveau contenu |
| `--file` | string | Modifier sans éditeur : fichier du nouveau contenu (`-` : entrée standard) |
| `--refuse` | bool | Refuser (alias `--reject`) |
| `--reason` | string | Motif d'un refus (facultatif, avec `--refuse` seulement) |
| `--pending` | bool | Remettre en attente |
| `--dir` | string | Dossier du projet (défaut : le dossier courant) |

Une décision et une seule ; une modification porte sur une seule proposition. Le contenu modifié est nettoyé comme une proposition et l'original est conservé. Les identifiants (`wp-<n>`) sont ceux du projet, donnés par `oh wiki pending` ; une proposition déjà appliquée ou refusée n'est plus modifiable. Avec plusieurs identifiants, oh décide de chacun et signale ceux qui échouent (code de sortie non nul). La décision est signée `oh` (`decided_by`).

**Motif d'un refus.** `--reason` enregistre pourquoi la proposition est refusée, comme le motif facultatif demandé par `n` dans la TUI. Le motif est nettoyé (une seule ligne, sans caractère de contrôle) et borné à 500 caractères ; vide, il est simplement omis. Il est conservé avec la proposition (colonne `reason`, affiché par `oh wiki pending --all` et dans la TUI sur la proposition refusée). Une acceptation, une modification ou une remise en attente ne prend pas de motif : `--reason` sans `--refuse` est une erreur.

### oh wiki stats

Métriques du wiki, calculées par oh : pages (dont specs, décisions, postmortems), **couverture des domaines** (pages `business/<domaine>.md` et domaines de la carte de l'index ; couvert = la page existe et porte au moins un enrichissement tagué), **âge médian des pages** (jours depuis `updated`, et la plus ancienne), **part de `INCERTAIN`** parmi les enrichissements tagués, **propositions en attente** (en file et acceptées non appliquées) et dernière synchronisation (`doc-sync`). Mêmes chiffres dans la vue « Wiki » du mode projet de la TUI.

```
oh wiki stats [dir] [--json]
```
