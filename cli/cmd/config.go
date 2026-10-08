package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/tui/theme"
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

// ─── configFieldMap: typed setter/unsetter for known config keys ─────────────

// configField defines a setter and an unsetter for a known hub.toml key.
type configField struct {
	Set   func(c *config.Config, value string) error
	Unset func(c *config.Config)
}

// configKey returns the current name of a configuration key (a former key
// is still accepted).
func configKey(key string) string {
	if k, ok := config.LegacyKeyAliases[key]; ok {
		return k
	}
	return key
}

// parseBool accepts "true"/"false"/"1"/"0".
func parseBoolValue(s string) (bool, error) {
	switch strings.ToLower(s) {
	case "true", "1", "yes":
		return true, nil
	case "false", "0", "no":
		return false, nil
	default:
		return false, errors.New(i18n.Tf("cmd.config.invalid_bool", s))
	}
}

// configFieldMap maps dotted TOML keys to typed setters on the Config struct.
// This is the canonical list of settable keys for "oh config set/unset".
var configFieldMap = map[string]configField{
	// CLI
	"cli.language": {
		Set:   func(c *config.Config, v string) error { c.CLI.Language = v; return nil },
		Unset: func(c *config.Config) { c.CLI.Language = "" },
	},
	"cli.setup_done": {
		Set: func(c *config.Config, v string) error {
			b, err := parseBoolValue(v)
			if err != nil {
				return err
			}
			c.CLI.SetupDone = b
			return nil
		},
		Unset: func(c *config.Config) { c.CLI.SetupDone = false },
	},
	// LLM (former key: config.LegacyKeyAliases)
	"llm.default_provider": {
		Set:   func(c *config.Config, v string) error { c.LLM.DefaultProvider = v; return nil },
		Unset: func(c *config.Config) { c.LLM.DefaultProvider = "" },
	},
	// Provider — Bedrock
	"provider.bedrock.aws_profile": {
		Set:   func(c *config.Config, v string) error { c.Provider.Bedrock.AWSProfile = v; return nil },
		Unset: func(c *config.Config) { c.Provider.Bedrock.AWSProfile = "" },
	},
	"provider.bedrock.aws_region": {
		Set:   func(c *config.Config, v string) error { c.Provider.Bedrock.AWSRegion = v; return nil },
		Unset: func(c *config.Config) { c.Provider.Bedrock.AWSRegion = "" },
	},
	"provider.bedrock.auth_mode": {
		Set:   func(c *config.Config, v string) error { c.Provider.Bedrock.AuthMode = v; return nil },
		Unset: func(c *config.Config) { c.Provider.Bedrock.AuthMode = "" },
	},
	// Provider — Anthropic
	"provider.anthropic.auth_mode": {
		Set:   func(c *config.Config, v string) error { c.Provider.Anthropic.AuthMode = v; return nil },
		Unset: func(c *config.Config) { c.Provider.Anthropic.AuthMode = "" },
	},
	// Provider — OpenRouter
	"provider.openrouter.auth_mode": {
		Set:   func(c *config.Config, v string) error { c.Provider.OpenRouter.AuthMode = v; return nil },
		Unset: func(c *config.Config) { c.Provider.OpenRouter.AuthMode = "" },
	},
	// MCP — Figma
	"mcp.figma.enabled": {
		Set: func(c *config.Config, v string) error {
			b, err := parseBoolValue(v)
			if err != nil {
				return err
			}
			c.MCP.Figma.Enabled = b
			return nil
		},
		Unset: func(c *config.Config) { c.MCP.Figma.Enabled = false },
	},
	"mcp.figma.token_key": {
		Set:   func(c *config.Config, v string) error { c.MCP.Figma.Token = v; return nil },
		Unset: func(c *config.Config) { c.MCP.Figma.Token = "" },
	},
	// MCP — GitLab
	"mcp.gitlab.enabled": {
		Set: func(c *config.Config, v string) error {
			b, err := parseBoolValue(v)
			if err != nil {
				return err
			}
			c.MCP.Gitlab.Enabled = b
			return nil
		},
		Unset: func(c *config.Config) { c.MCP.Gitlab.Enabled = false },
	},
	"mcp.gitlab.token_key": {
		Set:   func(c *config.Config, v string) error { c.MCP.Gitlab.Token = v; return nil },
		Unset: func(c *config.Config) { c.MCP.Gitlab.Token = "" },
	},
	"mcp.gitlab.write_enabled": {
		Set: func(c *config.Config, v string) error {
			b, err := parseBoolValue(v)
			if err != nil {
				return err
			}
			c.MCP.Gitlab.WriteEnabled = b
			return nil
		},
		Unset: func(c *config.Config) { c.MCP.Gitlab.WriteEnabled = false },
	},
	"mcp.gitlab.url": {
		Set:   func(c *config.Config, v string) error { c.MCP.Gitlab.URL = v; return nil },
		Unset: func(c *config.Config) { c.MCP.Gitlab.URL = "" },
	},
	// MCP — Jira
	"mcp.jira.enabled": {
		Set: func(c *config.Config, v string) error {
			b, err := parseBoolValue(v)
			if err != nil {
				return err
			}
			c.MCP.Jira.Enabled = b
			return nil
		},
		Unset: func(c *config.Config) { c.MCP.Jira.Enabled = false },
	},
	"mcp.jira.token_key": {
		Set:   func(c *config.Config, v string) error { c.MCP.Jira.Token = v; return nil },
		Unset: func(c *config.Config) { c.MCP.Jira.Token = "" },
	},
	"mcp.jira.url": {
		Set:   func(c *config.Config, v string) error { c.MCP.Jira.URL = v; return nil },
		Unset: func(c *config.Config) { c.MCP.Jira.URL = "" },
	},
	// MCP — Google Slides
	"mcp.gslides.enabled": {
		Set: func(c *config.Config, v string) error {
			b, err := parseBoolValue(v)
			if err != nil {
				return err
			}
			c.MCP.Gslides.Enabled = b
			return nil
		},
		Unset: func(c *config.Config) { c.MCP.Gslides.Enabled = false },
	},
	"mcp.gslides.token_key": {
		Set:   func(c *config.Config, v string) error { c.MCP.Gslides.Token = v; return nil },
		Unset: func(c *config.Config) { c.MCP.Gslides.Token = "" },
	},
	// Worktree
	"worktree.auto_cleanup": {
		Set: func(c *config.Config, v string) error {
			b, err := parseBoolValue(v)
			if err != nil {
				return err
			}
			c.Worktree.AutoCleanup = b
			return nil
		},
		Unset: func(c *config.Config) { c.Worktree.AutoCleanup = false },
	},
	"worktree.base_branch": {
		Set:   func(c *config.Config, v string) error { c.Worktree.BaseBranch = v; return nil },
		Unset: func(c *config.Config) { c.Worktree.BaseBranch = "" },
	},
	"worktree.branch_pattern": {
		Set:   func(c *config.Config, v string) error { c.Worktree.BranchPattern = v; return nil },
		Unset: func(c *config.Config) { c.Worktree.BranchPattern = "" },
	},
	// Models
	"models.default": {
		Set:   func(c *config.Config, v string) error { c.Models.Default = v; return nil },
		Unset: func(c *config.Config) { c.Models.Default = "" },
	},
	// Websearch
	"websearch.enabled": {
		Set: func(c *config.Config, v string) error {
			b, err := parseBoolValue(v)
			if err != nil {
				return err
			}
			c.Websearch.Enabled = b
			return nil
		},
		Unset: func(c *config.Config) { c.Websearch.Enabled = false },
	},
	// Tracker
	"tracker.enabled": {
		Set: func(c *config.Config, v string) error {
			b, err := parseBoolValue(v)
			if err != nil {
				return err
			}
			c.Tracker.Enabled = &b
			return nil
		},
		Unset: func(c *config.Config) { c.Tracker.Enabled = nil },
	},
	"tracker.auto_sync": {
		Set: func(c *config.Config, v string) error {
			b, err := parseBoolValue(v)
			if err != nil {
				return err
			}
			c.Tracker.AutoSync = &b
			return nil
		},
		Unset: func(c *config.Config) { c.Tracker.AutoSync = nil },
	},
	"tracker.push_labels": {
		Set: func(c *config.Config, v string) error {
			b, err := parseBoolValue(v)
			if err != nil {
				return err
			}
			c.Tracker.PushLabels = &b
			return nil
		},
		Unset: func(c *config.Config) { c.Tracker.PushLabels = nil },
	},
	"tracker.auto_plan_assigned": {
		Set: func(c *config.Config, v string) error {
			b, err := parseBoolValue(v)
			if err != nil {
				return err
			}
			c.Tracker.AutoPlanAssigned = &b
			return nil
		},
		Unset: func(c *config.Config) { c.Tracker.AutoPlanAssigned = nil },
	},
	"tracker.max_auto_plan_per_member": {
		Set: func(c *config.Config, v string) error {
			n, err := strconv.Atoi(v)
			if err != nil {
				return errors.New(i18n.Tf("cmd.config.invalid_int", v))
			}
			c.Tracker.MaxAutoPlanPerMember = &n
			return nil
		},
		Unset: func(c *config.Config) { c.Tracker.MaxAutoPlanPerMember = nil },
	},
	// Tracker — connection (independent of MCP)
	"tracker.tracker_url": {
		Set:   func(c *config.Config, v string) error { c.Tracker.TrackerURL = v; return nil },
		Unset: func(c *config.Config) { c.Tracker.TrackerURL = "" },
	},
	"tracker.tracker_token_key": {
		Set:   func(c *config.Config, v string) error { c.Tracker.TrackerTokenKey = v; return nil },
		Unset: func(c *config.Config) { c.Tracker.TrackerTokenKey = "" },
	},
	"tracker.write_enabled": {
		Set: func(c *config.Config, v string) error {
			b, err := parseBoolValue(v)
			if err != nil {
				return err
			}
			c.Tracker.WriteEnabled = &b
			return nil
		},
		Unset: func(c *config.Config) { c.Tracker.WriteEnabled = nil },
	},
}

// configFieldKeys returns sorted keys from configFieldMap for completions.
func configFieldKeys() []string {
	keys := make([]string, 0, len(configFieldMap))
	for k := range configFieldMap {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// ─── Commands ────────────────────────────────────────────────────────────────

func configGetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get <key>",
		Short: "Affiche la valeur d'une clé de configuration",
		Args:  cobra.ExactArgs(1),
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			if len(args) > 0 {
				return nil, cobra.ShellCompDirectiveNoFileComp
			}
			cfg, err := config.Load()
			if err != nil {
				return nil, cobra.ShellCompDirectiveNoFileComp
			}
			m := cfg.ToMap()
			keys := make([]string, 0, len(m))
			for k := range m {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			return keys, cobra.ShellCompDirectiveNoFileComp
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			key := configKey(args[0])

			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("%s: %w", i18n.T("cmd.config.load_failed"), err)
			}
			m := cfg.ToMap()
			val, ok := m[key]
			if !ok {
				return fmt.Errorf("%s", i18n.Tf("cmd.config.key_not_found", key))
			}

			fmt.Fprintln(os.Stdout, val)
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
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			if len(args) == 0 {
				return configFieldKeys(), cobra.ShellCompDirectiveNoFileComp
			}
			return nil, cobra.ShellCompDirectiveNoFileComp
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			key, value := configKey(args[0]), args[1]

			field, ok := lookupConfigField(key)
			if !ok {
				return errors.New(i18n.Tf("cmd.config.unknown_key", key))
			}

			var setErr error
			if err := config.Update(func(c *config.Config) error {
				setErr = field.Set(c, value)
				return setErr
			}); err != nil {
				if setErr != nil {
					return setErr
				}
				return fmt.Errorf("%s: %w", i18n.T("cmd.config.save_failed"), err)
			}

			fmt.Fprintf(os.Stdout, "%s %s = %s\n",
				theme.SuccessStyle.Render(theme.IconSuccess),
				theme.Bold.Render(key), value)
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
			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("%s: %w", i18n.T("cmd.config.load_failed"), err)
			}
			jsonOut, _ := cmd.Flags().GetBool("json")
			if onlyKeys, _ := cmd.Flags().GetBool("keys"); onlyKeys {
				keys := append(configFieldKeys(), configKeyPatterns...)
				if jsonOut {
					return json.NewEncoder(os.Stdout).Encode(keys)
				}
				for _, k := range keys {
					fmt.Fprintln(os.Stdout, k)
				}
				return nil
			}
			m := cfg.ToMap()
			keys := make([]string, 0, len(m))
			for k := range m {
				keys = append(keys, k)
			}
			sort.Strings(keys)

			if jsonOut {
				return json.NewEncoder(os.Stdout).Encode(m)
			}

			if len(keys) == 0 {
				fmt.Fprintln(os.Stdout, theme.Subtitle.Render(i18n.T("cmd.config.no_config")))
				return nil
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, i18n.T("cmd.config.list.header"))
			for _, k := range keys {
				fmt.Fprintf(w, "%s\t%v\n", k, m[k])
			}
			w.Flush()
			return nil
		},
	}

	cmd.Flags().Bool("json", false, "Output in JSON format")
	cmd.Flags().Bool("keys", false, "Liste les clés modifiables par oh config set|unset")
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
			return configFieldKeys(), cobra.ShellCompDirectiveNoFileComp
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			key := configKey(args[0])

			field, ok := lookupConfigField(key)
			if !ok {
				return errors.New(i18n.Tf("cmd.config.unknown_key", key))
			}

			if err := config.Update(func(c *config.Config) error {
				field.Unset(c)
				return nil
			}); err != nil {
				return fmt.Errorf("%s: %w", i18n.T("cmd.config.save_failed"), err)
			}

			fmt.Fprintf(os.Stdout, "%s %s\n",
				theme.SuccessStyle.Render(theme.IconSuccess),
				i18n.Tf("cmd.config.key_deleted", theme.Bold.Render(key)))
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
			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("%s: %w", i18n.T("cmd.config.load_failed"), err)
			}

			if len(args) == 0 {
				fmt.Fprintf(os.Stdout, "%s\n", i18n.Tf("cmd.config.lang_current", theme.Bold.Render(cfg.CLI.Language)))
				return nil
			}

			lang := args[0]
			switch lang {
			case "fr", "en":
				// valid
			default:
				return fmt.Errorf("%s", i18n.Tf("cmd.config.lang_invalid", lang))
			}

			if err := config.Update(func(c *config.Config) error {
				c.CLI.Language = lang
				return nil
			}); err != nil {
				return fmt.Errorf("%s: %w", i18n.T("cmd.config.save_failed"), err)
			}

			fmt.Fprintf(os.Stdout, "%s %s\n",
				theme.SuccessStyle.Render(theme.IconSuccess),
				i18n.Tf("cmd.config.lang_changed", theme.Bold.Render(lang)))
			return nil
		},
	}
}

func configWebsearchCmd() *cobra.Command {
	return &cobra.Command{
		Use:       "websearch [enable|disable|status]",
		Short:     "Gère les permissions WebSearch/WebFetch (Exa AI)",
		Long:      i18n.T("cmd.config.websearch.long"),
		Args:      cobra.ExactArgs(1),
		ValidArgs: []string{"enable", "disable", "status"},
		RunE: func(cmd *cobra.Command, args []string) error {
			action := args[0]

			switch action {
			case "enable":
				if err := config.Update(func(c *config.Config) error {
					c.Websearch.Enabled = true
					return nil
				}); err != nil {
					return fmt.Errorf("%s: %w", i18n.T("cmd.config.save_failed"), err)
				}
				fmt.Fprintf(os.Stdout, "%s %s\n",
					theme.SuccessStyle.Render(theme.IconSuccess),
					i18n.T("cmd.config.websearch_enabled"))
				fmt.Fprintf(os.Stdout, "  %s\n", i18n.T("cmd.config.websearch_deploy_hint"))

			case "disable":
				if err := config.Update(func(c *config.Config) error {
					c.Websearch.Enabled = false
					return nil
				}); err != nil {
					return fmt.Errorf("%s: %w", i18n.T("cmd.config.save_failed"), err)
				}
				fmt.Fprintf(os.Stdout, "%s %s\n",
					theme.SuccessStyle.Render(theme.IconSuccess),
					i18n.T("cmd.config.websearch_disabled"))

			case "status":
				cfg, err := config.Load()
				if err != nil {
					return fmt.Errorf("%s: %w", i18n.T("cmd.config.load_failed"), err)
				}
				status := i18n.T("cmd.config.websearch_off")
				if cfg.Websearch.Enabled {
					status = i18n.T("cmd.config.websearch_on")
				}
				fmt.Fprintf(os.Stdout, "%s\n",
					i18n.Tf("cmd.config.websearch_status", status))

			default:
				return errors.New(i18n.Tf("cmd.config.websearch_invalid_action", action))
			}
			return nil
		},
	}
}
