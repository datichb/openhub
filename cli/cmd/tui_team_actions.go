package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/opencode"
	"github.com/datichb/openhub/cli/internal/teamstate"
	"github.com/datichb/openhub/cli/internal/tui/v2/shell"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
)

// ─────────────────────────────────────────────────────────────────────────────
// Team Init action
// ─────────────────────────────────────────────────────────────────────────────

func actionTeamInit() {
	if tuiShell == nil {
		return
	}

	a := MustApp()

	tuiShell.ShowInputModal("Git remote URL du team-state", "", func(remote string) {
		if remote == "" {
			return
		}
		tuiShell.ShowInputModal("Votre member ID", "", func(memberID string) {
			if memberID == "" {
				return
			}
			tuiShell.ShowInputModal("Nom d'affichage", "", func(displayName string) {
				if displayName == "" {
					return
				}
				roleOptions := []views.SelectOption{
					{Label: "Lead", Value: "lead"},
					{Label: "Développeur", Value: "dev"},
					{Label: "Reviewer", Value: "reviewer"},
				}
				tuiShell.ShowSelectModal("Rôle", roleOptions, "dev", func(role string) {
					tuiShell.ShowToast("Initialisation team...", shell.ToastInfo)
					ctx := tuiShell.Context()

					go func() {
						select {
						case <-ctx.Done():
							return
						default:
						}
						err := runTeamInitFromTUI(a, remote, memberID, displayName, role)
						tuiShell.App().QueueUpdateDraw(func() {
							if err != nil {
								tuiShell.ShowToast("Team init échoué: "+truncateErr(err), shell.ToastError)
							} else {
								tuiShell.ShowToast("Team initialisé ! Redémarrez le TUI pour les nouvelles options.", shell.ToastSuccess)
							}
						})
					}()
				})
			})
		})
	})
}

func runTeamInitFromTUI(a *app.App, remote, memberID, displayName, role string) error {
	statePath := a.Config.Team.StatePath
	if statePath == "" {
		statePath = config.DefaultTeamStatePath()
	}

	repo := teamstate.NewRepo(remote, statePath)

	ctx := context.Background()
	if err := repo.EnsureReady(ctx); err != nil {
		return fmt.Errorf("cloning team-state: %w", err)
	}

	if err := repo.InitStructure(ctx); err != nil {
		return fmt.Errorf("init structure: %w", err)
	}

	member := teamstate.Member{
		ID:          memberID,
		DisplayName: displayName,
		Role:        role,
		DefaultMode: "semi-auto",
	}
	if repo.HasMember(memberID) {
		if err := repo.UpdateMember(member); err != nil {
			return fmt.Errorf("update member: %w", err)
		}
	} else {
		if err := repo.AddMember(member); err != nil {
			return fmt.Errorf("add member: %w", err)
		}
	}

	if err := repo.CommitAndPush(ctx, "team: init "+memberID, "members.toml"); err != nil {
		return fmt.Errorf("commit: %w", err)
	}

	vip := configViper()
	vip.Set("team.enabled", true)
	vip.Set("team.state_repo", remote)
	vip.Set("team.state_path", statePath)
	vip.Set("team.member_id", memberID)
	if err := vip.WriteConfigAs(config.ConfigPath()); err != nil {
		return fmt.Errorf("writing hub.toml: %w", err)
	}

	a.Config.Team.Enabled = true
	a.Config.Team.StateRepo = remote
	a.Config.Team.StatePath = statePath
	a.Config.Team.MemberID = memberID

	return nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Takeover brief enrichment
// ─────────────────────────────────────────────────────────────────────────────

func runTakeoverEnrich(a *app.App, project, ticketID string) error {
	repo := teamstate.NewRepo(a.Config.Team.StateRepo, a.Config.Team.StatePath)

	content, err := repo.ReadBrief(project, ticketID)
	if err != nil {
		return fmt.Errorf("reading brief: %w", err)
	}

	enriched, err := opencode.RunHeadless(opencode.HeadlessOpts{
		Agent:  "brief-enricher",
		Prompt: content,
	})
	if err != nil {
		return fmt.Errorf("enrichment: %w", err)
	}

	briefsDir := filepath.Join(repo.Path(), "projects", project, "takeover-briefs")
	entries, _ := os.ReadDir(briefsDir)
	var latestBase string
	for _, e := range entries {
		name := e.Name()
		if len(name) > len(ticketID)+1 && name[:len(ticketID)] == ticketID &&
			strings.HasSuffix(name, ".md") && !strings.HasSuffix(name, ".enriched.md") {
			latestBase = strings.TrimSuffix(name, ".md")
		}
	}
	if latestBase == "" {
		latestBase = ticketID
	}

	enrichedContent := fmt.Sprintf("# Takeover Brief (enrichi): %s\n\n%s", ticketID, enriched)
	enrichedFile := filepath.Join(briefsDir, latestBase+".enriched.md")
	if err := os.WriteFile(enrichedFile, []byte(enrichedContent), 0o644); err != nil {
		return fmt.Errorf("writing enriched brief: %w", err)
	}

	relPath := "projects/" + project + "/takeover-briefs/" + latestBase + ".enriched.md"
	_ = repo.CommitAndPush(context.Background(), fmt.Sprintf("takeover: enriched brief for %s/%s", project, ticketID), relPath)

	return nil
}
