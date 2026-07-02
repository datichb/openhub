package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/opencode"
	"github.com/datichb/openhub/cli/internal/prompt"
	"github.com/datichb/openhub/cli/internal/tui/common"
)

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Lance une session opencode",
	Long: `Prépare le contexte projet puis lance opencode.
Détecte automatiquement le projet si vous êtes dans un répertoire enregistré.`,
	RunE: runStart,
}

func init() {
	rootCmd.AddCommand(startCmd)
	startCmd.Flags().StringP("agent", "a", "", "Agent à utiliser")
	startCmd.Flags().StringP("prompt", "p", "", "Prompt initial")
	startCmd.Flags().StringP("provider", "P", "", "Provider LLM (bedrock, anthropic, openai)")
	startCmd.Flags().StringP("project", "j", "", "ID du projet (détection auto sinon)")
	startCmd.Flags().StringP("resume", "r", "", "Reprendre une session existante (ID)")
	startCmd.Flags().Bool("dev", false, "Mode développement")
}

func runStart(cmd *cobra.Command, args []string) error {
	a := MustApp()

	// --- Ensure opencode is installed ---
	if err := ensureOpencode(a); err != nil {
		return err
	}

	// --- Compatibility warning ---
	if ocVersion, err := opencode.Version(); err == nil {
		compat := opencode.CheckCompatibility(Version, ocVersion)
		if !compat.Compatible {
			fmt.Fprintf(a.IO.Out, "%s %s\n",
				common.WarningStyle.Render(common.IconWarning),
				compat.Warning)
		}
	}

	// --- Resume mode ---
	resumeID, _ := cmd.Flags().GetString("resume")
	if resumeID != "" {
		fmt.Fprintf(a.IO.Out, "%s Reprise de la session %s\n",
			common.SuccessStyle.Render(common.IconArrow), resumeID)
		return opencode.Exec(opencode.StartOpts{
			ResumeSessionID: resumeID,
		})
	}

	// --- Resolve project ---
	projectID, _ := cmd.Flags().GetString("project")
	project, err := resolveProject(a, projectID)
	if err != nil {
		return err
	}

	// --- Resolve provider + bearer token ---
	provider, _ := cmd.Flags().GetString("provider")
	if provider == "" {
		provider = "bedrock" // default
	}

	var bearerToken string
	if provider == "bedrock" && a.Secrets != nil {
		token, _ := a.Secrets.Get("bedrock-token-" + project.ID)
		if token == "" {
			token, _ = a.Secrets.Get("bedrock-token-default")
		}
		bearerToken = token
	}

	// --- Detect stack and build context ---
	stack := prompt.DetectStack(project.Path)
	agent, _ := cmd.Flags().GetString("agent")
	userPrompt, _ := cmd.Flags().GetString("prompt")

	// --- Display summary ---
	fmt.Fprintln(a.IO.Out)
	fmt.Fprintf(a.IO.Out, "%s %s\n", common.Title.Render("oh start"), common.Subtitle.Render(project.Name))
	fmt.Fprintln(a.IO.Out)
	fmt.Fprintf(a.IO.Out, "  Projet:   %s (%s)\n", project.Name, project.Path)
	fmt.Fprintf(a.IO.Out, "  Langage:  %s\n", displayOrDefault(stack.Language, project.Language))
	if stack.Framework != "" {
		fmt.Fprintf(a.IO.Out, "  Framework: %s\n", stack.Framework)
	}
	fmt.Fprintf(a.IO.Out, "  Provider: %s\n", provider)
	if agent != "" {
		fmt.Fprintf(a.IO.Out, "  Agent:    %s\n", agent)
	}
	if bearerToken != "" {
		fmt.Fprintf(a.IO.Out, "  Token:    %s\n", common.SuccessStyle.Render(common.IconSuccess+" configuré"))
	}
	fmt.Fprintln(a.IO.Out)

	// --- Launch ---
	fmt.Fprintf(a.IO.Out, "%s Lancement d'opencode...\n\n",
		common.SuccessStyle.Render(common.IconArrow))

	return opencode.Exec(opencode.StartOpts{
		ProjectPath: project.Path,
		ProjectID:   project.ID,
		Agent:       agent,
		Prompt:      userPrompt,
		Provider:    provider,
		BearerToken: bearerToken,
	})
}

// ensureOpencode checks that the opencode binary is available.
// If not found, prompts the user to install it via Homebrew or auto-download.
func ensureOpencode(a *app.App) error {
	_, err := opencode.FindBinary()
	if err == nil {
		return nil // already installed
	}

	fmt.Fprintf(a.IO.Out, "%s opencode non trouvé.\n\n",
		common.WarningStyle.Render(common.IconWarning))

	var choice string
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Comment installer opencode ?").
				Options(
					huh.NewOption("brew install anomalyco/tap/opencode (recommandé)", "brew"),
					huh.NewOption("Télécharger automatiquement", "download"),
					huh.NewOption("Annuler", "cancel"),
				).
				Value(&choice),
		),
	)
	if err := form.Run(); err != nil {
		return fmt.Errorf("sélection annulée")
	}

	switch choice {
	case "brew":
		fmt.Fprintf(a.IO.Out, "\n  Exécutez: %s\n\n",
			common.Bold.Render("brew install anomalyco/tap/opencode"))
		return fmt.Errorf("opencode requis — installez-le puis relancez oh start")
	case "download":
		return downloadOpencode(a)
	default:
		return fmt.Errorf("opencode requis pour lancer une session")
	}
}

// downloadOpencode downloads and installs the opencode binary.
func downloadOpencode(a *app.App) error {
	installDir := a.Config.Opencode.InstallDir
	version := a.Config.Opencode.Version

	if version == "" {
		version = "latest"
	}

	fmt.Fprintf(a.IO.Out, "%s Téléchargement d'opencode...\n",
		common.SuccessStyle.Render(common.IconArrow))

	var lastPercent int
	_, err := opencode.Download(version, installDir, func(downloaded, total int64) {
		if total > 0 {
			percent := int(downloaded * 100 / total)
			if percent != lastPercent && percent%5 == 0 {
				lastPercent = percent
				fmt.Fprintf(a.IO.Out, "\r  Progression: %d%% (%d/%d MB)",
					percent, downloaded/1024/1024, total/1024/1024)
			}
		}
	})
	if err != nil {
		return fmt.Errorf("échec du téléchargement: %w", err)
	}

	fmt.Fprintln(a.IO.Out) // newline after progress
	fmt.Fprintf(a.IO.Out, "%s opencode installé avec succès.\n\n",
		common.SuccessStyle.Render(common.IconSuccess))
	return nil
}

// resolveProject finds the project to use. Priority:
// 1. --project flag (explicit ID)
// 2. Current directory detection
// 3. Interactive selection if multiple projects exist
func resolveProject(a *app.App, projectID string) (*domain.Project, error) {
	// Explicit ID
	if projectID != "" {
		p, err := a.Projects.Get(projectID)
		if err != nil {
			if errors.Is(err, domain.ErrNotFound) {
				return nil, fmt.Errorf("projet %q introuvable", projectID)
			}
			return nil, err
		}
		return p, nil
	}

	// Auto-detect from cwd
	cwd, _ := os.Getwd()
	projects, err := a.Projects.List(domain.ProjectStatusActive)
	if err != nil {
		return nil, err
	}

	if len(projects) == 0 {
		return nil, fmt.Errorf("aucun projet enregistré. Lancez `oh init` ou `oh project add`")
	}

	// Check if cwd matches a project
	for i, p := range projects {
		absPath, _ := filepath.Abs(p.Path)
		if absPath == cwd || isSubPath(cwd, absPath) {
			return &projects[i], nil
		}
	}

	// If only one project, use it
	if len(projects) == 1 {
		return &projects[0], nil
	}

	// Interactive selection
	var selectedID string
	options := make([]huh.Option[string], len(projects))
	for i, p := range projects {
		label := fmt.Sprintf("%s (%s)", p.Name, p.Language)
		options[i] = huh.NewOption(label, p.ID)
	}

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Choisir un projet").
				Options(options...).
				Value(&selectedID),
		),
	)
	if err := form.Run(); err != nil {
		return nil, err
	}

	for i, p := range projects {
		if p.ID == selectedID {
			return &projects[i], nil
		}
	}
	return nil, fmt.Errorf("projet non trouvé")
}

func displayOrDefault(detected, fallback string) string {
	if detected != "" {
		return detected
	}
	if fallback != "" {
		return fallback
	}
	return "—"
}
