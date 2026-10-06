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
| `oh review feedback <mr>` | toujours disponible : lit les discussions GitLab puis lance `oh run review-feedback` (lancé directement, ce workflow attend l'entrée `feedback`) |
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

## 6. Plus de déploiement dans les projets

`oh deploy` et `oh sync` sont supprimés : chaque session part d'un paquet construit au lancement, hors du projet (`oh bundle show <workflow>` pour le voir). Pendant v5.x, les deux commandes affichent un message de migration. Sont aussi retirés :

- les étapes **Agents** et **Déploiement** de l'assistant d'initialisation, de `oh project add` et de `oh project configure` (le paquet contient les agents du workflow ; la sélection d'agents par projet, colonne `projects.agents`, est supprimée par les migrations v36 et v37) ;
- la section Déploiement du mode projet, les commandes `deploy`/`sync` de l'omnibar et le toast d'écart ;
- `[deploy] disable_native_agents` (le monde fermé du paquet masque toujours les agents natifs) ; `[deploy] instruction_files` reste ;
- les liens `.opencode` des worktrees (et la resynchronisation `s` de la vue Worktrees) ;
- l'ancienne vue Workflow (remplacée par le catalogue et l'éditeur de workflows).

Le serveur MCP `team` lit l'équipe de la session dans son environnement (`OH_TEAM_ID`) au lieu de `.opencode/team.json`.

## 7. Catalogue des briques

L'ancienne vue « Agents » du projet (sélection des agents déployés) devient le **Catalogue des briques**, en lecture seule : agents et skills, origine (hub ou catalogue d'équipe), coût estimé, skills chargées, dépendances (`requires`) et workflows qui les livrent. Accès : Config projet › « Briques », ou la commande `bricks` de l'omnibar. Pour changer les agents d'une session, on modifie (ou on étend) son workflow.

## 8. Nettoyer les anciens déploiements : `oh migrate deploy-cleanup`

Les fichiers écrits par `oh deploy` restent dans les projets tant qu'on ne les retire pas. Après la mise à jour, oh le propose **une fois** : message au démarrage d'une commande (`oh: anciens déploiements trouvés dans N projet(s)…`) ou écran « Nettoyage des anciens déploiements » dans la TUI (Nettoyer / Voir le diff / Plus tard ; commande `cleanup` de l'omnibar pour le rouvrir). `oh doctor` les signale tant qu'il en reste.

```bash
oh migrate deploy-cleanup --dry-run   # ce qui serait retiré, avec le diff d'opencode.json
oh migrate deploy-cleanup             # récapitulatif puis confirmation
oh migrate deploy-cleanup -p web --yes
```

Pour chaque projet enregistré, sont retirés : `.opencode/agents`, `.opencode/skills`, `.opencode/.deploy-state`, `.opencode/context-manifest.json`, `.opencode/team.json` (le dossier `.opencode` aussi s'il est vide ; les autres fichiers, comme `package.json`, restent).

Dans `opencode.json`, seules les clés **écrites par oh et inchangées depuis le dernier déploiement** sont retirées : blocs `agent.<id>` des agents du hub et des agents natifs désactivés, serveurs `mcp.<nom>` lancés par `oh mcp serve`, `context-mode` dans `plugin`, fichiers d'instructions ajoutés par oh, `compaction`, `subagent_depth`, `enabled_providers`, `model`, le bloc du fournisseur configuré, `permission.websearch`/`webfetch`. La référence est la copie d'`opencode.json` gardée dans `.deploy-state` (`config_snapshot`) : une clé que vous avez modifiée depuis est **conservée** (et signalée), vos propres clés ne sont jamais touchées, l'ordre du fichier est gardé. Sans `.deploy-state`, `opencode.json` n'est pas modifié (à vérifier à la main). Un `opencode.json` qui ne contenait que des clés d'oh est supprimé.

## 9. Anciennes surcharges du workflow (migration v38)

Les surcharges de l'ancien workflow unique (`hub.toml [workflow]`, `[workflow]` du `config.toml` d'équipe, configuration de workflow des projets) sont migrées **automatiquement** au démarrage vers des workflows `feature` du team-state (équipe, projet ; un espace solo est créé pour un projet sans équipe). Rien n'est perdu : la configuration d'origine est archivée à côté (`workflows/migrated/…`, `~/.oh/migrated/` pour `hub.toml`). Détails : [Workflows d'équipe › Migration des anciennes surcharges](team-workflows.fr.md).
