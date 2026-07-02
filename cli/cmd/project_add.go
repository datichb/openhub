package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/huh"
	"github.com/google/uuid"
	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/tui/common"
)

func projectAddCmd() *cobra.Command {
	var (
		name     string
		path     string
		language string
		tracker  string
	)

	cmd := &cobra.Command{
		Use:     "add",
		Aliases: []string{"register"},
		Short:   "Enregistre un nouveau projet",
		Long:    "Enregistre un projet dans le hub. Si aucun flag n'est fourni, lance un wizard interactif.",
		RunE: func(cmd *cobra.Command, args []string) error {
			a := MustApp()

			// If no flags provided, run interactive form
			if name == "" && path == "" {
				return runProjectAddInteractive(a)
			}

			// Non-interactive mode
			if name == "" {
				return fmt.Errorf("--name est requis en mode non-interactif")
			}
			if path == "" {
				path = "."
			}

			absPath, err := filepath.Abs(path)
			if err != nil {
				return fmt.Errorf("resolving path: %w", err)
			}

			return doCreateProject(a, name, absPath, language, tracker)
		},
	}

	cmd.Flags().StringVarP(&name, "name", "n", "", "Nom du projet")
	cmd.Flags().StringVarP(&path, "path", "p", "", "Chemin du projet (défaut: répertoire courant)")
	cmd.Flags().StringVarP(&language, "language", "l", "", "Langage principal")
	cmd.Flags().StringVarP(&tracker, "tracker", "t", "", "Issue tracker (github, gitlab, jira, linear)")

	return cmd
}

func runProjectAddInteractive(a *app.App) error {
	var (
		name     string
		path     string
		language string
		tracker  string
	)

	cwd, _ := os.Getwd()

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Nom du projet").
				Description("Un identifiant court et lisible").
				Value(&name).
				Validate(func(s string) error {
					if strings.TrimSpace(s) == "" {
						return fmt.Errorf("le nom est requis")
					}
					return nil
				}),

			huh.NewInput().
				Title("Chemin du projet").
				Description("Répertoire racine du projet").
				Value(&path).
				Placeholder(cwd),

			huh.NewSelect[string]().
				Title("Langage principal").
				Options(
					huh.NewOption("Go", "go"),
					huh.NewOption("TypeScript", "typescript"),
					huh.NewOption("Python", "python"),
					huh.NewOption("Rust", "rust"),
					huh.NewOption("Java", "java"),
					huh.NewOption("Autre", "other"),
				).
				Value(&language),

			huh.NewSelect[string]().
				Title("Issue tracker").
				Options(
					huh.NewOption("Aucun", ""),
					huh.NewOption("GitHub Issues", "github"),
					huh.NewOption("GitLab Issues", "gitlab"),
					huh.NewOption("Jira", "jira"),
					huh.NewOption("Linear", "linear"),
				).
				Value(&tracker),
		),
	)

	if err := form.Run(); err != nil {
		return err
	}

	if path == "" {
		path = cwd
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("resolving path: %w", err)
	}

	return doCreateProject(a, name, absPath, language, tracker)
}

func doCreateProject(a *app.App, name, absPath, language, tracker string) error {
	// Check path exists
	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		return fmt.Errorf("le répertoire n'existe pas: %s", absPath)
	}

	id := generateProjectID(name)
	now := time.Now()

	p := &domain.Project{
		ID:        id,
		Name:      name,
		Path:      absPath,
		Language:  language,
		Tracker:   tracker,
		Status:    domain.ProjectStatusActive,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := a.Projects.Create(p); err != nil {
		return fmt.Errorf("creating project: %w", err)
	}

	fmt.Fprintf(a.IO.Out, "%s Projet %s enregistré (%s)\n",
		common.SuccessStyle.Render(common.IconSuccess),
		common.Bold.Render(name),
		absPath,
	)
	return nil
}

func generateProjectID(name string) string {
	slug := strings.ToLower(strings.TrimSpace(name))
	slug = strings.ReplaceAll(slug, " ", "-")
	// Remove non-alphanumeric chars except dashes
	var clean strings.Builder
	for _, r := range slug {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			clean.WriteRune(r)
		}
	}
	slug = clean.String()
	if len(slug) > 32 {
		slug = slug[:32]
	}
	short := uuid.New().String()[:8]
	return slug + "-" + short
}
