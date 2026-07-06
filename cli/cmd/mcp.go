package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/i18n"
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
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			return []string{"figma", "gitlab", "gslides"}, cobra.ShellCompDirectiveNoFileComp
		},
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
				return fmt.Errorf("%s", i18n.Tf("cmd.mcp.serve.unknown", name))
			}
		},
	}
	return cmd
}

func mcpListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "Liste les serveurs MCP disponibles",
		RunE: func(cmd *cobra.Command, args []string) error {
			jsonOut, _ := cmd.Flags().GetBool("json")
			if jsonOut {
				type mcpServer struct {
					Name        string `json:"name"`
					Description string `json:"description"`
					Command     string `json:"command"`
				}
				servers := []mcpServer{
					{Name: "figma", Description: i18n.T("cmd.mcp.list.figma_desc"), Command: "oh mcp serve figma"},
					{Name: "gitlab", Description: i18n.T("cmd.mcp.list.gitlab_desc"), Command: "oh mcp serve gitlab"},
					{Name: "gslides", Description: i18n.T("cmd.mcp.list.gslides_desc"), Command: "oh mcp serve gslides"},
				}
				return json.NewEncoder(os.Stdout).Encode(servers)
			}

			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, i18n.T("cmd.mcp.list.header"))
			fmt.Fprintf(w, "%s\t%s\t%s\n",
				"figma", i18n.T("cmd.mcp.list.figma_desc"), common.Subtitle.Render("oh mcp serve figma"))
			fmt.Fprintf(w, "%s\t%s\t%s\n",
				"gitlab", i18n.T("cmd.mcp.list.gitlab_desc"), common.Subtitle.Render("oh mcp serve gitlab"))
			fmt.Fprintf(w, "%s\t%s\t%s\n",
				"gslides", i18n.T("cmd.mcp.list.gslides_desc"), common.Subtitle.Render("oh mcp serve gslides"))
			w.Flush()
			return nil
		},
	}

	cmd.Flags().Bool("json", false, "Output in JSON format")
	return cmd
}
