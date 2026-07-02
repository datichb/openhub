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

var skillsCmd = &cobra.Command{
	Use:   "skills",
	Short: "Gestion des skills",
}

func init() {
	rootCmd.AddCommand(skillsCmd)
	skillsCmd.AddCommand(skillsListCmd())
}

func skillsListCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "Liste les skills disponibles",
		RunE: func(cmd *cobra.Command, args []string) error {
			a := MustApp()

			hubDir := findHubDir()
			if hubDir == "" {
				return fmt.Errorf("impossible de trouver le répertoire hub")
			}

			skillsDir := filepath.Join(hubDir, "skills")
			if _, err := os.Stat(skillsDir); os.IsNotExist(err) {
				fmt.Fprintln(a.IO.Out, common.Subtitle.Render("Aucun skill trouvé."))
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
				info, _ := d.Info()
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

			if len(skills) == 0 {
				fmt.Fprintln(a.IO.Out, common.Subtitle.Render("Aucun skill trouvé."))
				return nil
			}

			w := tabwriter.NewWriter(a.IO.Out, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "SKILL\tCATÉGORIE\tFICHIER")
			for _, s := range skills {
				fmt.Fprintf(w, "%s\t%s\t%s\n", s.name, s.category, s.file)
			}
			w.Flush()
			fmt.Fprintf(a.IO.Out, "\n%s\n", common.Subtitle.Render(fmt.Sprintf("%d skill(s)", len(skills))))
			return nil
		},
	}
}

type skillInfo struct {
	name     string
	category string
	file     string
	size     int64
}
