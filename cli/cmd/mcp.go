package cmd

import (
	"fmt"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/mcp/figma"
	"github.com/datichb/openhub/cli/internal/mcp/gitlab"
	"github.com/datichb/openhub/cli/internal/mcp/gslides"
	"github.com/datichb/openhub/cli/internal/tui/common"
)

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Gestion des serveurs MCP intégrés",
}

func init() {
	rootCmd.AddCommand(mcpCmd)
	mcpCmd.AddCommand(mcpServeCmd())
	mcpCmd.AddCommand(mcpListCmd())
}

func mcpServeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "serve <name>",
		Short: "Lance un serveur MCP en mode stdio",
		Long:  "Lance un serveur MCP intégré qui communique via stdin/stdout (JSON-RPC). Utilisé par opencode.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			switch name {
			case "figma":
				return figma.Serve()
			case "gitlab":
				return gitlab.Serve()
			case "gslides":
				return gslides.Serve()
			default:
				return fmt.Errorf("serveur MCP inconnu: %q (disponibles: figma, gitlab, gslides)", name)
			}
		},
	}
	return cmd
}

func mcpListCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "Liste les serveurs MCP disponibles",
		Run: func(cmd *cobra.Command, args []string) {
			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "NOM\tDESCRIPTION\tCOMMANDE")
			fmt.Fprintf(w, "%s\tFigma API (fichiers, nodes, styles)\t%s\n",
				"figma", common.Subtitle.Render("oh mcp serve figma"))
			fmt.Fprintf(w, "%s\tGitLab API (projets, issues, MRs)\t%s\n",
				"gitlab", common.Subtitle.Render("oh mcp serve gitlab"))
			fmt.Fprintf(w, "%s\tGoogle Slides API (présentations)\t%s\n",
				"gslides", common.Subtitle.Render("oh mcp serve gslides"))
			w.Flush()
		},
	}
}
