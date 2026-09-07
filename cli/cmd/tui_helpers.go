package cmd

import (
	"context"
	"log/slog"
	"strings"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/beads"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/opencode"
	"github.com/datichb/openhub/cli/internal/tui/v2/shell"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
)

// resolveActiveProject returns the currently selected project or the first one.
func resolveActiveProject(a *app.App) (*domain.Project, error) {
	ctx := context.Background()
	return resolveProject(ctx, a, "")
}

// makeResolveTeamFunc returns a ResolveTeamFunc for use in team views.
// It resolves the effective team config for the currently active project
// (project-level override → hub fallback) each time it is called.
func makeResolveTeamFunc(a *app.App) views.ResolveTeamFunc {
	return func() views.TeamResolution {
		project, _ := resolveActiveProject(a)
		tc := resolvedTeamConfig(a, project)
		return views.TeamResolution{
			Enabled:   tc.Enabled,
			StateRepo: tc.StateRepo,
			StatePath: tc.StatePath,
			MemberID:  tc.MemberID,
		}
	}
}

// findOpencodeOrToast checks that the opencode binary exists, shows a toast if not.
// Returns an error if the binary is not found.
func findOpencodeOrToast() (string, error) {
	bin, err := opencode.FindBinary()
	if err != nil && tuiShell != nil {
		tuiShell.ShowToast("opencode non trouvé", shell.ToastError)
	}
	return bin, err
}


// initBeadsForActiveProject initialises beads for the currently active project.
// Shows a confirmation modal before proceeding.
func initBeadsForActiveProject(a *app.App) {
	if tuiShell == nil {
		return
	}
	path := resolveActiveProjectPath(a)
	if path == "" {
		tuiShell.ShowToastMsg("Aucun projet actif", false)
		return
	}
	// Resolve project ID for the prefix
	var id, name string
	if ap := tuiShell.ActiveProject(); ap != nil {
		id = ap.ID
		name = ap.Name
	} else if p, err := resolveActiveProject(a); err == nil && p != nil {
		id = p.ID
		name = p.Name
	}
	initBeadsForProject(a, id, name, path)
}

// initBeadsForProject shows a confirmation modal then initialises beads for the
// given project (path, id/name as prefix).
func initBeadsForProject(_ *app.App, id, name, path string) {
	if tuiShell == nil {
		return
	}
	if beads.IsInitialized(path) {
		tuiShell.ShowToastMsg("Board déjà initialisé pour "+name, true)
		tuiShell.NavigateTo("board")
		return
	}

	label := name
	if label == "" {
		label = path
	}

	tuiShell.ShowSelectModal(
		"Initialiser le board pour "+label+" ?",
		[]views.SelectOption{
			{Label: "Oui, initialiser beads", Value: "yes"},
			{Label: "Annuler", Value: "no"},
		}, "",
		func(value string) {
			if value != "yes" {
				return
			}
			prefix := id
			if prefix == "" {
				prefix = strings.ToLower(strings.ReplaceAll(name, " ", "-"))
			}
			if err := beads.Init(path, prefix); err != nil {
				tuiShell.ShowToastMsg("Erreur init beads: "+err.Error(), false)
				return
			}
			tuiShell.ShowToastMsg("Board initialisé pour "+label, true)
			tuiShell.NavigateTo("board")
		},
	)
}

// fetchBoardTicketsForPath loads and normalises tickets from the beads system
// for the given project path. Returns nil (and no error) when bd is not installed,
// when the project has no tickets yet, or when beads is not initialized.
func fetchBoardTicketsForPath(projectPath string) []views.BoardTicket {
	if projectPath == "" {
		return nil
	}
	if err := beads.Available(); err != nil {
		return nil // bd not installed — board shows empty, no crash
	}
	if !beads.IsInitialized(projectPath) {
		return nil // no .beads/ — board will show the init invite screen
	}
	raw, err := beads.ListAll(projectPath)
	if err != nil {
		slog.Warn("board: failed to list beads tickets", "path", projectPath, "error", err)
		return nil
	}
	out := make([]views.BoardTicket, 0, len(raw))
	for _, t := range raw {
		// Epics are containers, not actionable items — exclude from the board.
		if strings.EqualFold(t.Type, "epic") {
			continue
		}
		out = append(out, views.BoardTicket{
			ID:          t.ID,
			Title:       t.Title,
			Status:      boardNormalizeStatus(t.Status),
			Priority:    t.Priority,
			Type:        t.Type,
			ExternalRef: beads.ExternalRefForTicket(t),
		})
	}
	return out
}

// boardNormalizeStatus maps beads status values to the board column statuses.
func boardNormalizeStatus(s string) string {
	switch strings.ToLower(s) {
	case "todo", "to_do", "backlog", "open":
		return "todo"
	case "in_progress", "in-progress", "doing", "wip":
		return "in_progress"
	case "review", "in_review", "in-review":
		return "review"
	case "done", "completed", "closed", "cancelled":
		return "done"
	case "blocked", "stuck":
		return "blocked"
	default:
		return "todo"
	}
}

// resolveActiveProjectPath returns the path of the active project.
// Prefers the shell's active project; falls back to the first registered project.
// Returns "" if no project is available (safe to call with a minimal App in tests).
func resolveActiveProjectPath(a *app.App) string {
	if tuiShell != nil {
		if ap := tuiShell.ActiveProject(); ap != nil {
			return ap.Path
		}
	}
	if a == nil || a.Projects == nil {
		return ""
	}
	// Non-interactive fallback: pick the first active project without prompting.
	ctx := context.Background()
	projects, err := a.Projects.List(ctx, domain.ProjectStatusActive)
	if err != nil || len(projects) == 0 {
		return ""
	}
	return projects[0].Path
}

// resolveProviderCreds populates provider credentials into opts.
func resolveProviderCreds(a *app.App, project *domain.Project, opts *opencode.StartOpts) {
	prov := project.Provider
	if prov == "" {
		prov = a.Config.Opencode.DefaultProvider
	}
	opts.Provider = prov

	if a.Secrets == nil {
		slog.Warn("secrets store non disponible — provider credentials non résolus",
			"provider", prov,
			"hint", "vérifiez que le keychain est accessible ou définissez OH_PASSPHRASE")
		return
	}

	ctx := context.Background()
	switch prov {
	case "bedrock":
		token, _ := a.Secrets.Get(ctx, "openhub.provider.bedrock.token."+project.ID)
		if token == "" {
			token, _ = a.Secrets.Get(ctx, "openhub.provider.bedrock.token")
		}
		if token == "" {
			slog.Warn("bedrock token non trouvé",
				"hint", "oh secrets set openhub.provider.bedrock.token <bearer-token>")
		}
		opts.BearerToken = token
		opts.AWSProfile = a.Config.Provider.Bedrock.AWSProfile
		opts.AWSRegion = a.Config.Provider.Bedrock.AWSRegion
	case "anthropic":
		key, _ := a.Secrets.Get(ctx, "openhub.provider.anthropic.token."+project.ID)
		if key == "" {
			key, _ = a.Secrets.Get(ctx, "openhub.provider.anthropic.token")
		}
		if key == "" {
			slog.Warn("anthropic API key non trouvée",
				"hint", "oh secrets set openhub.provider.anthropic.token <api-key>")
		}
		opts.APIKey = key
	case "openrouter":
		key, _ := a.Secrets.Get(ctx, "openhub.provider.openrouter.token."+project.ID)
		if key == "" {
			key, _ = a.Secrets.Get(ctx, "openhub.provider.openrouter.token")
		}
		if key == "" {
			slog.Warn("openrouter API key non trouvée",
				"hint", "oh secrets set openhub.provider.openrouter.token <api-key>")
		}
		opts.APIKey = key
	}
}

