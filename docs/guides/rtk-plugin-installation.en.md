> [Lire en français](rtk-plugin-installation.fr.md)

# RTK plugin — removed in v5

`oh plugin install|remove|status` and the TUI Plugins view installed **global** opencode V1 plugins (`~/.config/opencode/plugins/`). They were removed with opencode V1 (v5).

In v5, plugins are declared **per workflow** (`plugins:` in the YAML, V2 format `{ id, setup }`) and delivered in the session bundle: see [Shipped workflows › Plugins and code mode](../reference/workflows.en.md#plugins-and-code-mode). An `rtk.ts` left in `~/.config/opencode/plugins/` can be removed by hand; the `rtk` tool itself remains usable in the session shell.

See the [v5 migration guide](migration-v5.en.md).
