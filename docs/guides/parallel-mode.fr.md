> [Read in English](parallel-mode.en.md)

# Mode parallèle — remplacé en v5

L'ancien mode parallèle (`oh start --parallel`, un serveur opencode par ticket, moniteur et vue de fusion) a été retiré avec opencode V1 (v5).

Équivalent v5 : **une session par ticket, dans un seul serveur de groupe**, chacune dans son worktree quand elle écrit :

```bash
oh run ticket --tickets BD-42,BD-43,BD-44
```

- `oh start --parallel --tickets a,b` reste un alias déprécié de cette commande ; sans `--tickets`, il est refusé.
- Le suivi se fait dans la vue **Sessions** de la TUI ou par `oh session list` ; la fusion des branches passe par vos merge requests.
- `--max-sessions` et `--priority` n'ont plus d'effet.

Voir [Sessions v5](sessions-v5.fr.md) et le [guide de migration v5](migration-v5.fr.md).
