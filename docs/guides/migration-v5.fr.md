> [Read in English](migration-v5.en.md)

# Migrer vers oh v5

oh v5 lance chaque session à partir d'un **workflow** (`oh run <workflow>`), dans un **paquet de session** construit hors du projet, sur un serveur **opencode V2**. Ce guide liste ce qui change pour un utilisateur de la v4.

## 1. opencode V2 obligatoire

opencode V1 n'est plus pris en charge. Sans opencode V2 (version minimale : 2.0.0), oh refuse de lancer une session et l'explique :

```text
opencode 1.18.29 n'est plus pris en charge : oh v5 demande opencode V2 (2.0.0 ou plus récent).
```

1. Installez opencode V2 avec son propre outil (`brew install anomalyco/tap/opencode`, ou https://opencode.ai).
2. Vérifiez avec `oh doctor` : la ligne **opencode V2** doit être verte.

`oh upgrade opencode`, l'installation gérée dans `~/.oh/bin` et les clés `[opencode] version`, `channel`, `auto_update`, `install_dir` de `hub.toml` sont supprimées (ignorées si elles restent). `[opencode] default_provider` reste.

## 2. Anciennes commandes : alias de `oh run`

Elles marchent encore pendant v5.x, avec un avertissement qui donne la commande à utiliser :

| Ancienne commande | Équivalent v5 |
|---|---|
| `oh start` | `oh run feature` |
| `oh start --agent <id>` | `oh run libre --agent <id>` |
| `oh start --dev [-t <id>]` | `oh run ticket --tickets <id>` |
| `oh start --onboard [--refresh]` | `oh run onboarding [-i refresh=true]` |
| `oh start --parallel --tickets a,b` | `oh run ticket --tickets a,b` |
| `oh start --sweep <objectif>` | `oh run sweep -i goal=<objectif>` |
| `oh start --resume <id>` | `oh session attach <id> --how here` |
| `oh audit`, `oh review`, `oh debug` | `oh run audit`, `oh run review`, `oh run debug` |
| `oh review feedback <mr>` | `oh run review-feedback` (les discussions sont lues par oh) |
| `oh takeover-brief enrich` | `oh run brief-enrich --headless` |

Ne fonctionnent plus :

- `oh start --parallel` **sans** `--tickets` (refusé) ; `--max-sessions`, `--priority`, `--sweep-branch-prefix` sont sans effet.
- `oh start --agent` combiné à `--dev`, `--onboard`, `--parallel` ou `--sweep` (refusé).
- Le moniteur et la vue de fusion du mode parallèle : suivez les sessions dans la vue **Sessions** ou `oh session list`.

## 3. Session libre : workflow `libre`

L'ancien lancement « par agent » devient le workflow livré **`libre`** : l'agent de ton choix (par défaut `orchestrator`) et les agents qu'il peut appeler, sans checkpoint, dans un monde fermé.

```bash
oh run libre --agent debugger -i request="le test X échoue depuis hier"
```

Dans la TUI : commande `coder`, ou ouverture d'une session depuis la vue Worktrees.

## 4. Plugins

`oh plugin` et la vue Plugins (plugins globaux opencode V1, dont RTK) sont supprimés. Les plugins se déclarent par workflow (`plugins:`) ; voir [Workflows livrés](../reference/workflows.fr.md#plugins-et-code-mode).

## 5. Métriques

`oh metrics`, le tableau de bord et la vue Métriques lisent désormais le registre des sessions d'oh (`~/.oh/oh.db`), alimenté par le démon pour chaque session v5. Les sessions lancées hors d'oh, ou avant v5, ne sont plus comptées.
