package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/i18n"
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
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "Liste les agents disponibles",
		RunE: func(cmd *cobra.Command, args []string) error {
			a := MustApp()

			hubDir := findHubDir()
			if hubDir == "" {
				return fmt.Errorf("%s", i18n.T("cmd.project.hub_not_found"))
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
				info, err := entry.Info()
				if err != nil {
					continue
				}
				agents = append(agents, agentInfo{
					name: name,
					file: entry.Name(),
					size: info.Size(),
				})
			}

			jsonOut, _ := cmd.Flags().GetBool("json")
			if jsonOut {
				type agentJSON struct {
					Name string `json:"name"`
					File string `json:"file"`
					Size int64  `json:"size"`
				}
				out := make([]agentJSON, len(agents))
				for i, ag := range agents {
					out[i] = agentJSON{Name: ag.name, File: ag.file, Size: ag.size}
				}
				return json.NewEncoder(os.Stdout).Encode(out)
			}

			if len(agents) == 0 {
				fmt.Fprintln(a.IO.Out, common.Subtitle.Render(i18n.T("cmd.agent.none")))
				return nil
			}

			w := tabwriter.NewWriter(a.IO.Out, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, i18n.T("cmd.agent.list.header"))
			for _, ag := range agents {
				fmt.Fprintf(w, "%s\t%s\t%s\n", ag.name, ag.file, i18n.Tf("cmd.agent.list.bytes", ag.size))
			}
			w.Flush()
			fmt.Fprintf(a.IO.Out, "\n%s\n", common.Subtitle.Render(i18n.Tf("cmd.agent.list.count", len(agents))))
			return nil
		},
	}

	cmd.Flags().Bool("json", false, "Output in JSON format")
	return cmd
}

type agentInfo struct {
	name string
	file string
	size int64
}
