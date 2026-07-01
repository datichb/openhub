package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/datichb/openhub/cli/internal/config"
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
	configCmd.AddCommand(configListCmd())
	configCmd.AddCommand(configPathCmd())
}

func configGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <key>",
		Short: "Affiche la valeur d'une clé de configuration",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			key := args[0]

			v := configViper()
			if !v.IsSet(key) {
				return fmt.Errorf("clé introuvable: %s", key)
			}

			fmt.Fprintln(os.Stdout, v.Get(key))
			return nil
		},
	}
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
	return &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "Affiche toute la configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			v := configViper()
			keys := v.AllKeys()
			sort.Strings(keys)

			if len(keys) == 0 {
				fmt.Fprintln(os.Stdout, common.Subtitle.Render("Aucune configuration. Lancez `oh init` pour commencer."))
				return nil
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "CLÉ\tVALEUR")
			for _, k := range keys {
				fmt.Fprintf(w, "%s\t%v\n", k, v.Get(k))
			}
			w.Flush()
			return nil
		},
	}
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

	_ = v.ReadInConfig() // OK if not found
	return v
}
