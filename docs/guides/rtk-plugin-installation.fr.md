> [Read in English](rtk-plugin-installation.en.md)

# Plugin RTK — retiré en v5

`oh plugin install|remove|status` et la vue Plugins de la TUI installaient des plugins **globaux** opencode V1 (`~/.config/opencode/plugins/`). Ils ont été retirés avec opencode V1 (v5).

En v5, les plugins sont déclarés **par workflow** (`plugins:` du YAML, format V2 `{ id, setup }`) et livrés dans le paquet de session : voir [Workflows livrés › Plugins et code mode](../reference/workflows.fr.md#plugins-et-code-mode). Un `rtk.ts` laissé dans `~/.config/opencode/plugins/` peut être supprimé à la main ; l'outil `rtk` lui-même reste utilisable dans le shell des sessions.

Voir le [guide de migration v5](migration-v5.fr.md).
