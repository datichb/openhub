package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

var completionCmd = &cobra.Command{
	Use:   "completion [bash|zsh|fish|powershell]",
	Short: "Génère le script d'autocomplétion pour votre shell",
	Long: `Pour charger l'autocomplétion:

Bash:
  $ source <(oh completion bash)
  # Pour charger à chaque session:
  $ oh completion bash > /etc/bash_completion.d/oh

Zsh:
  $ source <(oh completion zsh)
  # Pour charger à chaque session:
  $ oh completion zsh > "${fpath[1]}/_oh"

Fish:
  $ oh completion fish | source
  # Pour charger à chaque session:
  $ oh completion fish > ~/.config/fish/completions/oh.fish

PowerShell:
  PS> oh completion powershell | Out-String | Invoke-Expression
`,
	DisableFlagsInUseLine: true,
	ValidArgs:             []string{"bash", "zsh", "fish", "powershell"},
	Args:                  cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
	Run: func(cmd *cobra.Command, args []string) {
		switch args[0] {
		case "bash":
			rootCmd.GenBashCompletion(os.Stdout)
		case "zsh":
			rootCmd.GenZshCompletion(os.Stdout)
		case "fish":
			rootCmd.GenFishCompletion(os.Stdout, true)
		case "powershell":
			rootCmd.GenPowerShellCompletionWithDesc(os.Stdout)
		}
	},
}

func init() {
	rootCmd.AddCommand(completionCmd)
}
