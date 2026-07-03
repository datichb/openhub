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

var skillsCmd = &cobra.Command{
	Use:   "skills",
	Short: "Gestion des skills",
}

func init() {
	rootCmd.AddCommand(skillsCmd)
	skillsCmd.AddCommand(skillsListCmd())
}

func skillsListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "Liste les skills disponibles",
		RunE: func(cmd *cobra.Command, args []string) error {
			a := MustApp()

			hubDir := findHubDir()
			if hubDir == "" {
				return fmt.Errorf("%s", i18n.T("cmd.project.hub_not_found"))
			}

			skillsDir := filepath.Join(hubDir, "skills")
			if _, err := os.Stat(skillsDir); os.IsNotExist(err) {
				jsonOut, _ := cmd.Flags().GetBool("json")
				if jsonOut {
					return json.NewEncoder(os.Stdout).Encode([]struct{}{})
				}
				fmt.Fprintln(a.IO.Out, common.Subtitle.Render(i18n.T("cmd.skills.none")))
				return nil
			}

			var skills []skillInfo
			err := filepath.WalkDir(skillsDir, func(path string, d os.DirEntry, walkErr error) error {
				if walkErr != nil || d.IsDir() {
					return walkErr
				}
				if filepath.Ext(path) != ".md" {
					return nil
				}
				rel, _ := filepath.Rel(skillsDir, path)
				parts := strings.SplitN(rel, string(os.PathSeparator), 2)
				category := ""
				name := strings.TrimSuffix(filepath.Base(path), ".md")
				if len(parts) > 1 {
					category = parts[0]
				}
			info, err := d.Info()
			if err != nil {
				return nil
			}
			skills = append(skills, skillInfo{
					name:     name,
					category: category,
					file:     rel,
					size:     info.Size(),
				})
				return nil
			})
			if err != nil {
				return fmt.Errorf("walking skills directory: %w", err)
			}

			jsonOut, _ := cmd.Flags().GetBool("json")
			if jsonOut {
				type skillJSON struct {
					Name     string `json:"name"`
					Category string `json:"category"`
					File     string `json:"file"`
					Size     int64  `json:"size"`
				}
				out := make([]skillJSON, len(skills))
				for i, s := range skills {
					out[i] = skillJSON{Name: s.name, Category: s.category, File: s.file, Size: s.size}
				}
				return json.NewEncoder(os.Stdout).Encode(out)
			}

			if len(skills) == 0 {
				fmt.Fprintln(a.IO.Out, common.Subtitle.Render(i18n.T("cmd.skills.none")))
				return nil
			}

			w := tabwriter.NewWriter(a.IO.Out, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, i18n.T("cmd.skills.list.header"))
			for _, s := range skills {
				fmt.Fprintf(w, "%s\t%s\t%s\n", s.name, s.category, s.file)
			}
			w.Flush()
			fmt.Fprintf(a.IO.Out, "\n%s\n", common.Subtitle.Render(i18n.Tf("cmd.skills.list.count", len(skills))))
			return nil
		},
	}

	cmd.Flags().Bool("json", false, "Output in JSON format")
	return cmd
}

type skillInfo struct {
	name     string
	category string
	file     string
	size     int64
}
