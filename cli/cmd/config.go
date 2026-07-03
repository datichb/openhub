package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/tui/common"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Gestion de la configuration du hub",
}

func init() {
	rootCmd.AddCommand(configCmd)
	configCmd.AddCommand(configGetCmd())
	configCmd.AddCommand(configSetCmd())
	configCmd.AddCommand(configUnsetCmd())
	configCmd.AddCommand(configListCmd())
	configCmd.AddCommand(configPathCmd())
	configCmd.AddCommand(configLanguageCmd())
	configCmd.AddCommand(configWebsearchCmd())
}

func configGetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get <key>",
		Short: "Affiche la valeur d'une clé de configuration",
		Args:  cobra.ExactArgs(1),
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			if len(args) > 0 {
				return nil, cobra.ShellCompDirectiveNoFileComp
			}
			v := configViper()
			return v.AllKeys(), cobra.ShellCompDirectiveNoFileComp
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			key := args[0]

			v := configViper()
			if !v.IsSet(key) {
				return fmt.Errorf("%s", i18n.Tf("cmd.config.key_not_found", key))
			}

			fmt.Fprintln(os.Stdout, v.Get(key))
			return nil
		},
	}
	return cmd
}

func configSetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "set <key> <value>",
		Short: "Modifie une valeur de configuration",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			key, value := args[0], args[1]

			v := configViper()
			v.Set(key, value)

			// Ensure config directory exists
			cfgPath := config.ConfigPath()
			if err := os.MkdirAll(filepath.Dir(cfgPath), 0o755); err != nil {
				return fmt.Errorf("creating config directory: %w", err)
			}

			if err := v.WriteConfigAs(cfgPath); err != nil {
				return fmt.Errorf("writing config: %w", err)
			}

			fmt.Fprintf(os.Stdout, "%s %s = %s\n",
				common.SuccessStyle.Render(common.IconSuccess),
				common.Bold.Render(key), value)
			return nil
		},
	}
}

func configListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "Affiche toute la configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			v := configViper()
			keys := v.AllKeys()
			sort.Strings(keys)

			jsonOut, _ := cmd.Flags().GetBool("json")
			if jsonOut {
				m := make(map[string]interface{}, len(keys))
				for _, k := range keys {
					m[k] = v.Get(k)
				}
				return json.NewEncoder(os.Stdout).Encode(m)
			}

			if len(keys) == 0 {
				fmt.Fprintln(os.Stdout, common.Subtitle.Render(i18n.T("cmd.config.no_config")))
				return nil
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, i18n.T("cmd.config.list.header"))
			for _, k := range keys {
				fmt.Fprintf(w, "%s\t%v\n", k, v.Get(k))
			}
			w.Flush()
			return nil
		},
	}

	cmd.Flags().Bool("json", false, "Output in JSON format")
	return cmd
}

func configPathCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "path",
		Short: "Affiche le chemin du fichier de configuration",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Fprintln(os.Stdout, config.ConfigPath())
		},
	}
}

func configUnsetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "unset <key>",
		Short: "Supprime une clé de configuration",
		Long:  "Supprime une clé du fichier hub.toml. La valeur par défaut sera utilisée si elle existe.",
		Args:  cobra.ExactArgs(1),
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			if len(args) > 0 {
				return nil, cobra.ShellCompDirectiveNoFileComp
			}
			v := configViper()
			return v.AllKeys(), cobra.ShellCompDirectiveNoFileComp
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			key := args[0]

			v := configViper()
			if !v.IsSet(key) {
				return fmt.Errorf("%s", i18n.Tf("cmd.config.key_not_found", key))
			}

			// Viper doesn't have a native "unset" — we need to read the raw TOML,
			// remove the key, and rewrite. But for simplicity with Viper's API,
			// we set the value to its zero value based on type.
			// A cleaner approach: set to empty string and write.
			v.Set(key, nil)

			cfgPath := config.ConfigPath()
			if err := os.MkdirAll(filepath.Dir(cfgPath), 0o755); err != nil {
				return fmt.Errorf("creating config directory: %w", err)
			}

			if err := v.WriteConfigAs(cfgPath); err != nil {
				return fmt.Errorf("writing config: %w", err)
			}

			fmt.Fprintf(os.Stdout, "%s %s\n",
				common.SuccessStyle.Render(common.IconSuccess),
				i18n.Tf("cmd.config.key_deleted", common.Bold.Render(key)))
			return nil
		},
	}
	return cmd
}

func configLanguageCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "language [lang]",
		Short: "Affiche ou change la langue de l'interface",
		Long:  "Sans argument, affiche la langue actuelle. Avec argument (fr/en), change la langue.",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			v := configViper()

			if len(args) == 0 {
				lang := v.GetString("cli.language")
				fmt.Fprintf(os.Stdout, "%s\n", i18n.Tf("cmd.config.lang_current", common.Bold.Render(lang)))
				return nil
			}

			lang := args[0]
			switch lang {
			case "fr", "en":
				// valid
			default:
				return fmt.Errorf("%s", i18n.Tf("cmd.config.lang_invalid", lang))
			}

			v.Set("cli.language", lang)

			cfgPath := config.ConfigPath()
			if err := os.MkdirAll(filepath.Dir(cfgPath), 0o755); err != nil {
				return fmt.Errorf("creating config directory: %w", err)
			}

			if err := v.WriteConfigAs(cfgPath); err != nil {
				return fmt.Errorf("writing config: %w", err)
			}

			fmt.Fprintf(os.Stdout, "%s %s\n",
				common.SuccessStyle.Render(common.IconSuccess),
				i18n.Tf("cmd.config.lang_changed", common.Bold.Render(lang)))
			return nil
		},
	}
}

// configViper returns a pre-configured Viper instance for the hub config.
func configViper() *viper.Viper {
	v := viper.New()
	v.SetConfigName("hub")
	v.SetConfigType("toml")
	v.AddConfigPath(config.HubDir())
	v.AddConfigPath(".")

	// Defaults
	v.SetDefault("cli.language", "en")
	v.SetDefault("opencode.channel", "stable")
	v.SetDefault("opencode.auto_update", false)
	v.SetDefault("opencode.install_dir", filepath.Join(config.HubDir(), "bin"))
	v.SetDefault("websearch.enabled", false)

	_ = v.ReadInConfig() // OK if not found
	return v
}

func configWebsearchCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "websearch [enable|disable|status]",
		Short: "Gère les permissions WebSearch/WebFetch (Exa AI)",
		Long: `Active ou désactive les permissions websearch et webfetch pour les agents.
Lorsqu'activé, les agents peuvent effectuer des recherches web via Exa AI.
La permission est injectée globalement dans opencode.json au deploy.`,
		Args:  cobra.ExactArgs(1),
		ValidArgs: []string{"enable", "disable", "status"},
		RunE: func(cmd *cobra.Command, args []string) error {
			action := args[0]

			v := configViper()
			cfgPath := config.ConfigPath()

			switch action {
			case "enable":
				v.Set("websearch.enabled", true)
				if err := os.MkdirAll(filepath.Dir(cfgPath), 0o755); err != nil {
					return fmt.Errorf("creating config directory: %w", err)
				}
				if err := v.WriteConfigAs(cfgPath); err != nil {
					return fmt.Errorf("writing config: %w", err)
				}
				fmt.Fprintf(os.Stdout, "%s %s\n",
					common.SuccessStyle.Render(common.IconSuccess),
					i18n.T("cmd.config.websearch_enabled"))
				fmt.Fprintf(os.Stdout, "  %s\n", i18n.T("cmd.config.websearch_deploy_hint"))

			case "disable":
				v.Set("websearch.enabled", false)
				if err := os.MkdirAll(filepath.Dir(cfgPath), 0o755); err != nil {
					return fmt.Errorf("creating config directory: %w", err)
				}
				if err := v.WriteConfigAs(cfgPath); err != nil {
					return fmt.Errorf("writing config: %w", err)
				}
				fmt.Fprintf(os.Stdout, "%s %s\n",
					common.SuccessStyle.Render(common.IconSuccess),
					i18n.T("cmd.config.websearch_disabled"))

			case "status":
				enabled := v.GetBool("websearch.enabled")
				status := i18n.T("cmd.config.websearch_off")
				if enabled {
					status = i18n.T("cmd.config.websearch_on")
				}
				fmt.Fprintf(os.Stdout, "%s\n",
					i18n.Tf("cmd.config.websearch_status", status))

			default:
				return fmt.Errorf("action invalide %q : utiliser enable, disable ou status", action)
			}
			return nil
		},
	}
}
