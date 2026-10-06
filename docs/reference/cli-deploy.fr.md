> [Read in English](cli-deploy.en.md)

# Référence CLI — Déploiement (supprimé en v5)

`oh deploy` et `oh sync` copiaient agents, skills et configuration dans chaque projet (`.opencode/`, `opencode.json`). **oh v5 ne déploie plus rien dans les projets** : chaque session part d'un **paquet** construit au lancement, hors du projet (`~/.oh/bundles/<hash>/`), à partir de son workflow.

| Ancienne commande | En v5 |
|---|---|
| `oh deploy [-p <projet>]` | rien à faire : `oh run <workflow>` construit le paquet. Pour le voir : `oh bundle show <workflow> [-p <projet>] [--budget]` |
| `oh deploy --check`, `--diff` | `oh bundle show <workflow>` (agents, skills, permissions, budget) |
| `oh sync [--all]` | rien à faire : le paquet suit le hub et le workflow à chaque lancement |

Pendant v5.x, `oh deploy` et `oh sync` restent des alias qui affichent ce message de migration (code de sortie 0).

Les fichiers laissés par les anciens déploiements (`.opencode/agents`, `.opencode/skills`, `.opencode/.deploy-state`, `.opencode/context-manifest.json`, `.opencode/team.json`, clés d'oh dans `opencode.json`) se retirent avec `oh migrate deploy-cleanup`. Voir le [guide de migration v5](../guides/migration-v5.fr.md).
