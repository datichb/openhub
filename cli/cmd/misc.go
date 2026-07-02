package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/tui/common"
)

var conventionsCmd = &cobra.Command{
	Use:   "conventions",
	Short: "Affiche les conventions du projet",
	RunE: func(cmd *cobra.Command, args []string) error {
		a := MustApp()
		projectID, _ := cmd.Flags().GetString("project")
		project, err := resolveProject(a, projectID)
		if err != nil {
			return err
		}

		// Look for convention files
		conventions := []string{
			".editorconfig", "CONVENTIONS.md", "CONTRIBUTING.md",
			".eslintrc.json", ".prettierrc", "rustfmt.toml",
			"pyproject.toml", ".golangci.yml",
		}

		fmt.Fprintf(a.IO.Out, "%s Conventions de %s\n\n",
			common.Title.Render("oh conventions"), project.Name)

		found := false
		for _, f := range conventions {
			path := filepath.Join(project.Path, f)
			if _, err := os.Stat(path); err == nil {
				fmt.Fprintf(a.IO.Out, "  %s %s\n", common.SuccessStyle.Render(common.IconSuccess), f)
				found = true
			}
		}

		if !found {
			fmt.Fprintln(a.IO.Out, common.Subtitle.Render("  Aucun fichier de convention trouvé."))
		}
		return nil
	},
}

var beadsCmd = &cobra.Command{
	Use:   "beads",
	Short: "Gestion des tickets beads",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Delegate to bd command
		bdArgs := append([]string{}, args...)
		bdCmd := exec.Command("bd", bdArgs...)
		bdCmd.Stdin = os.Stdin
		bdCmd.Stdout = os.Stdout
		bdCmd.Stderr = os.Stderr
		return bdCmd.Run()
	},
}

var serviceCmd = &cobra.Command{
	Use:   "service",
	Short: "Gestion des services MCP projet",
	RunE: func(cmd *cobra.Command, args []string) error {
		a := MustApp()
		projectID, _ := cmd.Flags().GetString("project")
		project, err := resolveProject(a, projectID)
		if err != nil {
			return err
		}

		// Read opencode.json for MCP config
		configPath := filepath.Join(project.Path, "opencode.json")
		data, err := os.ReadFile(configPath)
		if err != nil {
			fmt.Fprintln(a.IO.Out, common.Subtitle.Render("  Aucune configuration MCP."))
			return nil
		}

		fmt.Fprintf(a.IO.Out, "%s Services MCP de %s\n\n",
			common.Title.Render("oh service"), project.Name)

		// Simple display of MCP section
		content := string(data)
		if strings.Contains(content, "mcpServers") || strings.Contains(content, "mcp") {
			fmt.Fprintf(a.IO.Out, "  %s Configuration MCP détectée dans opencode.json\n",
				common.SuccessStyle.Render(common.IconSuccess))
		} else {
			fmt.Fprintln(a.IO.Out, common.Subtitle.Render("  Aucun serveur MCP configuré."))
		}
		return nil
	},
}

var upgradeCmd = &cobra.Command{
	Use:   "upgrade",
	Short: "Met à jour oh et/ou opencode",
	RunE: func(cmd *cobra.Command, args []string) error {
		a := MustApp()
		fmt.Fprintln(a.IO.Out, common.Title.Render("  oh upgrade  "))
		fmt.Fprintln(a.IO.Out)
		fmt.Fprintf(a.IO.Out, "  oh:       %s\n", Version)
		fmt.Fprintf(a.IO.Out, "  opencode: %s\n", a.Config.Opencode.Version)
		fmt.Fprintln(a.IO.Out)
		fmt.Fprintln(a.IO.Out, common.Subtitle.Render("  Pour mettre à jour oh: brew upgrade datichb/openhub/oh"))
		fmt.Fprintln(a.IO.Out, common.Subtitle.Render("  Pour mettre à jour opencode: oh config set opencode.version <version>"))
		return nil
	},
}

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Alias pour upgrade",
	RunE:  upgradeCmd.RunE,
}

func init() {
	rootCmd.AddCommand(conventionsCmd)
	conventionsCmd.Flags().StringP("project", "j", "", "ID du projet")

	rootCmd.AddCommand(beadsCmd)

	rootCmd.AddCommand(serviceCmd)
	serviceCmd.Flags().StringP("project", "j", "", "ID du projet")

	rootCmd.AddCommand(upgradeCmd)
	rootCmd.AddCommand(updateCmd)
}
