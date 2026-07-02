package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/tui/common"
)

var agentCmd = &cobra.Command{
	Use:   "agent",
	Short: "Gestion des agents",
}

func init() {
	rootCmd.AddCommand(agentCmd)
	agentCmd.AddCommand(agentListCmd())
}

func agentListCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "Liste les agents disponibles",
		RunE: func(cmd *cobra.Command, args []string) error {
			a := MustApp()

			hubDir := findHubDir()
			if hubDir == "" {
				return fmt.Errorf("impossible de trouver le répertoire hub")
			}

			agentsDir := filepath.Join(hubDir, "agents")
			entries, err := os.ReadDir(agentsDir)
			if err != nil {
				return fmt.Errorf("reading agents directory: %w", err)
			}

			var agents []agentInfo
			for _, entry := range entries {
				if entry.IsDir() || filepath.Ext(entry.Name()) != ".md" {
					continue
				}
				name := strings.TrimSuffix(entry.Name(), ".md")
				info, _ := entry.Info()
				agents = append(agents, agentInfo{
					name: name,
					file: entry.Name(),
					size: info.Size(),
				})
			}

			if len(agents) == 0 {
				fmt.Fprintln(a.IO.Out, common.Subtitle.Render("Aucun agent trouvé."))
				return nil
			}

			w := tabwriter.NewWriter(a.IO.Out, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "AGENT\tFICHIER\tTAILLE")
			for _, ag := range agents {
				fmt.Fprintf(w, "%s\t%s\t%d octets\n", ag.name, ag.file, ag.size)
			}
			w.Flush()
			fmt.Fprintf(a.IO.Out, "\n%s\n", common.Subtitle.Render(fmt.Sprintf("%d agent(s)", len(agents))))
			return nil
		},
	}
}

type agentInfo struct {
	name string
	file string
	size int64
}
