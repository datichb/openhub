> [Read in English](cli-deploy.en.md)

# Référence CLI — Déploiement (supprimé en v5)

`oh deploy` et `oh sync` copiaient agents, skills et configuration dans chaque projet (`.opencode/`, `opencode.json`). **oh v5 ne déploie plus rien dans les projets** : chaque session part d'un **paquet de session** construit au lancement, hors du projet (`~/.oh/bundles/<hash>/`, immuable), à partir de son workflow. Voir l'[ADR-043](../architecture/adr/043-session-bundle-deploy-removal.fr.md).

## Alias de migration

```
oh deploy [anciens drapeaux…]
oh sync [anciens drapeaux…]
```

Pendant v5.x, les deux commandes restent des **alias de migration** : elles acceptent tous les anciens drapeaux et arguments (`-p`, `--check`, `--diff`, `--all`…, ignorés, y compris `--help`), ne modifient rien, affichent ce message sur la sortie d'erreur et sortent avec le **code 0** (les scripts existants ne cassent pas) :

```
! oh deploy n'existe plus : oh ne déploie plus rien dans les projets, chaque session part d'un paquet construit au lancement, hors du projet.
  Voir le paquet d'un workflow : oh bundle show <workflow> (oh bundle build pour le construire).
  Retirer les anciens fichiers déployés (.opencode/agents, skills…) : oh migrate deploy-cleanup.
```

## Équivalents v5

| Ancienne commande | En v5 |
|---|---|
| `oh deploy [-p <projet>]` | Rien à faire : [`oh run <workflow>`](cli-workflows.fr.md#oh-run) construit le paquet à chaque lancement. Pour le construire à l'avance : [`oh bundle build <workflow> [-p <projet>]`](cli-workflows.fr.md#oh-bundle-build--show) |
| `oh deploy --check`, `--diff` | [`oh bundle show <workflow> [-p <projet>] [--budget] [--json]`](cli-workflows.fr.md#oh-bundle-build--show) (agents, skills, permissions, budget) |
| `oh sync [--all]` | Rien à faire : le paquet suit le hub, l'équipe, le projet et le workflow à chaque lancement |
| Restes des anciens déploiements | [`oh migrate deploy-cleanup [--dry-run] [--diff] [--yes] [--json] [-p <projet>]`](cli-infra.fr.md#oh-migrate-deploy-cleanup) |

Les fichiers laissés par les anciens déploiements (`.opencode/agents`, `.opencode/skills`, `.opencode/.deploy-state`, `.opencode/context-manifest.json`, `.opencode/team.json`, clés d'oh inchangées dans `opencode.json`) se retirent avec `oh migrate deploy-cleanup` ; oh le propose une fois au démarrage (la TUI affiche son écran de nettoyage, omnibar `cleanup`). Voir le [guide de migration v5](../guides/migration-v5.fr.md).
