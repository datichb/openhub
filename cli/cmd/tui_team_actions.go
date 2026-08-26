package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/opencode"
	"github.com/datichb/openhub/cli/internal/teamstate"
	"github.com/datichb/openhub/cli/internal/tracker"
	"github.com/datichb/openhub/cli/internal/tui/v2/shell"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
)

// ─────────────────────────────────────────────────────────────────────────────
// Team Init action (hub-level)
// ─────────────────────────────────────────────────────────────────────────────

func actionTeamInit() {
	if tuiShell == nil {
		return
	}

	a := MustApp()

	// Étape 1 — Git remote URL (ShowInputModal, now has border+title like all modals)
	tuiShell.ShowInputModal("Étape 1 — Git remote URL du team-state", "", func(remote string) {
		if remote == "" {
			return
		}

		// showIdentityForm shows Étape 2 (or 3 if HTTPS) — identity form.
		showIdentityForm := func() {
			go func() {
				tuiShell.App().QueueUpdateDraw(func() {
					stepLabel := "Étape 2 — Identité"
					if teamstate.IsHTTPS(remote) {
						stepLabel = "Étape 3 — Identité"
					}
					tuiShell.ShowInlineForm(views.InlineFormConfig{
						Title: stepLabel,
						Fields: []views.FormField{
							{
								Key:      "member_id",
								Label:    "Member ID",
								Type:     views.FieldText,
								Required: true,
								Hint:     "Identifiant unique dans l'équipe (ex: benjamin, alice)",
							},
							{
								Key:   "display_name",
								Label: "Nom d'affichage",
								Type:  views.FieldText,
								Hint:  "Votre nom tel qu'il apparaîtra dans les events team",
							},
						{
							Key:     "role",
							Label:   "Rôle",
							Type:    views.FieldSelect,
							Default: "dev",
							Options: []views.SelectOption{
								{Label: "Lead", Value: "lead"},
								{Label: "Développeur", Value: "dev"},
								{Label: "Reviewer", Value: "reviewer"},
							},
							Hint: "← / → pour changer",
						},
					},
					OnSubmit: func(values map[string]string, _ map[string][]string) {
						memberID := values["member_id"]
						if memberID == "" {
							go func() {
								tuiShell.App().QueueUpdateDraw(func() {
									tuiShell.ShowToast("Member ID requis", shell.ToastError)
									})
								}()
								return
							}
							go func() {
								tuiShell.App().QueueUpdateDraw(func() {
									tuiShell.ShowToast("Initialisation de l'équipe...", shell.ToastInfo)
								})
							}()
							ctx := tuiShell.Context()
							go func() {
								select {
								case <-ctx.Done():
									return
								default:
								}
								err := runTeamInitFromTUI(a, remote, memberID, values["display_name"], values["role"])
								tuiShell.App().QueueUpdateDraw(func() {
									if err != nil {
										tuiShell.ShowToast("Initialisation échouée: "+err.Error(), shell.ToastError)
									} else {
										tuiShell.ShowToast("Équipe initialisée ! Redémarrez le TUI pour les nouvelles options.", shell.ToastSuccess)
									}
								})
							}()
						},
						OnCancel: func() {
							go func() {
								tuiShell.App().QueueUpdateDraw(func() {
									tuiShell.ShowToast("Annulé", shell.ToastInfo)
								})
							}()
						},
					})
				})
			}()
		}

		// Étape 2 (HTTPS only) — credentials, then identity
		if teamstate.IsHTTPS(remote) {
			go func() {
				tuiShell.App().QueueUpdateDraw(func() {
					collectCredentialsForInit(a, remote, showIdentityForm)
				})
			}()
		} else {
			showIdentityForm()
		}
	})
}

// collectCredentialsForInit shows the "Étape 2 — Authentification" select + credentials
// form for hub-level team init (same flow as team configure, reused here).
func collectCredentialsForInit(_ *app.App, remote string, afterCredentials func()) {
	authOptions := []views.SelectOption{
		{Label: "Oui, fournir un token", Value: "provide"},
		{Label: "Déjà configuré (skip)", Value: "skip"},
		{Label: "Non, accès public", Value: "public"},
	}
	tuiShell.ShowSelectModal("Étape 2 — Authentification", authOptions, "provide", func(authChoice string) {
		if authChoice == "provide" {
			go func() {
				tuiShell.App().QueueUpdateDraw(func() {
					tuiShell.ShowInlineForm(views.InlineFormConfig{
						Title: "Étape 2 — Credentials",
						Fields: []views.FormField{
							{
								Key:     "username",
								Label:   "Username",
								Type:    views.FieldText,
								Default: "oauth2",
								Hint:    "GitLab PAT : oauth2 · GitLab Project Token : nom du token · GitHub : votre username",
							},
							{
								Key:   "token",
								Label: "Token d'accès",
								Type:  views.FieldPassword,
								Hint:  "Stocké dans votre keychain système, jamais dans oh",
							},
						},
						OnSubmit: func(values map[string]string, _ map[string][]string) {
							username := values["username"]
							if username == "" {
								username = "oauth2"
							}
							token := values["token"]
							if token == "" {
								go func() {
									tuiShell.App().QueueUpdateDraw(func() {
										tuiShell.ShowToast("Token requis", shell.ToastError)
									})
								}()
								return
							}
						go func() {
							_ = teamstate.EnsureCredentialHelper(remote)
							if err := teamstate.ConfigureCredential(tuiShell.Context(), remote, username, token); err != nil {
								tuiShell.App().QueueUpdateDraw(func() {
									tuiShell.ShowToast("Erreur configuration credential : "+err.Error(), shell.ToastError)
								})
								return
							}
							// Sequential: toast first, then next modal.
							// A single goroutine with sleep prevents the race that causes
							// a hard TUI freeze when two QueueUpdateDraw calls run in the
							// same draw cycle.
							time.Sleep(50 * time.Millisecond)
							tuiShell.App().QueueUpdateDraw(func() {
								tuiShell.ShowToast("Credential configuré — les pulls/pushs utiliseront ce token automatiquement", shell.ToastSuccess)
							})
							time.Sleep(50 * time.Millisecond)
							tuiShell.App().QueueUpdateDraw(func() {
								afterCredentials()
							})
						}()
						},
						OnCancel: func() {
							go func() {
								tuiShell.App().QueueUpdateDraw(func() {
									tuiShell.ShowToast("Annulé", shell.ToastInfo)
								})
							}()
						},
					})
				})
			}()
		} else {
			go func() {
				tuiShell.App().QueueUpdateDraw(func() {
					afterCredentials()
				})
			}()
		}
	})
}

func runTeamInitFromTUI(a *app.App, remote, memberID, displayName, role string) error {
	statePath := a.Config.ActiveTeam().StatePath
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

	// Update in-memory config to reflect the newly configured team.
	// If Teams already has entries, update the first enabled one;
	// otherwise, append a new entry (fresh setup).
	newTeam := config.TeamConfig{
		ID:        config.RepoNameFromRemote(remote),
		Enabled:   true,
		StateRepo: remote,
		StatePath: statePath,
		MemberID:  memberID,
	}
	if len(a.Config.Teams) > 0 {
		a.Config.Teams[0] = newTeam
	} else {
		a.Config.Teams = append(a.Config.Teams, newTeam)
	}
	// Clear legacy field to avoid stale data
	a.Config.Team = config.TeamConfig{}

	// Persist via config.Save (produces [[teams]] format, not legacy [team])
	if err := config.Save(a.Config); err != nil {
		return fmt.Errorf("writing hub.toml: %w", err)
	}

	return nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Team Configure action (project-level)
// ─────────────────────────────────────────────────────────────────────────────

// actionTeamConfigure opens a modal chain to fully configure the team for the
// currently active project:
//   - inherit  → use hub team as-is
//   - custom   → full setup (clone, init, add/update member, persist)
//   - disabled → opt out for this project
func actionTeamConfigure() {
	if tuiShell == nil {
		return
	}

	a := MustApp()

	project := tuiShell.ActiveProject()
	if project == nil {
		tuiShell.ShowToast("Aucun projet actif — sélectionnez un projet d'abord", shell.ToastError)
		return
	}

	hubTeam := a.Config.ActiveTeam()
	hubMember := getHubMemberInfo(a)

	// ── Étape 1 : choose mode ─────────────────────────────────────────────
	var modeOptions []views.SelectOption
	if hubTeam.Enabled {
		modeOptions = []views.SelectOption{
			{
				Label: fmt.Sprintf("Hériter du hub (%s, membre : %s)", hubTeam.StateRepo, hubTeam.MemberID),
				Value: domain.ProjectTeamModeInherit,
			},
			{Label: "Configuration personnalisée pour ce projet", Value: domain.ProjectTeamModeCustom},
			{Label: "Pas de team pour ce projet", Value: domain.ProjectTeamModeDisabled},
		}
	} else {
		modeOptions = []views.SelectOption{
			{Label: "Pas de team pour ce projet", Value: domain.ProjectTeamModeDisabled},
			{Label: "Configurer une team custom pour ce projet", Value: domain.ProjectTeamModeCustom},
		}
	}

	tuiShell.ShowSelectModal("Team pour ce projet", modeOptions, domain.ProjectTeamModeInherit, func(mode string) {
		switch mode {
		case domain.ProjectTeamModeDisabled:
			go func() {
				tuiShell.App().QueueUpdateDraw(func() {
					applyProjectTeamConfig(a, project.ID, &domain.ProjectTeamConfig{Mode: domain.ProjectTeamModeDisabled})
				})
			}()

		case domain.ProjectTeamModeInherit:
			go func() {
				tuiShell.App().QueueUpdateDraw(func() {
					applyProjectTeamConfig(a, project.ID, nil)
				})
			}()

		case domain.ProjectTeamModeCustom:
			go func() {
				tuiShell.App().QueueUpdateDraw(func() {
					tuiShell.ShowInputModal("Git remote URL du team-state", "", func(customRepo string) {
						if customRepo == "" {
							go func() {
								tuiShell.App().QueueUpdateDraw(func() {
									tuiShell.ShowToast("URL annulée", shell.ToastInfo)
								})
							}()
							return
						}

						// ── Credentials (HTTPS only) ─────────────────────────────
						if teamstate.IsHTTPS(customRepo) {
							go func() {
								tuiShell.App().QueueUpdateDraw(func() {
									collectCredentialsThenMember(a, project.ID, customRepo, hubMember)
								})
							}()
						} else {
							// SSH — no credentials needed
							go func() {
								tuiShell.App().QueueUpdateDraw(func() {
									continueCustomFlowWithMember(a, project.ID, customRepo, hubMember)
								})
							}()
						}
					})
				})
			}()
		}
	})
}

// collectCredentialsThenMember handles the HTTPS credential collection flow
// before continuing with the member identity step.
func collectCredentialsThenMember(a *app.App, projectID, customRepo string, hubMember hubMemberInfo) {
	authOptions := []views.SelectOption{
		{Label: "Oui, fournir un token", Value: "provide"},
		{Label: "Déjà configuré (skip)", Value: "skip"},
		{Label: "Non, accès public", Value: "public"},
	}
	tuiShell.ShowSelectModal("Étape 1 — Authentification", authOptions, "provide", func(authChoice string) {
		if authChoice == "provide" {
			go func() {
				tuiShell.App().QueueUpdateDraw(func() {
					tuiShell.ShowInlineForm(views.InlineFormConfig{
						Title: "Étape 2 — Credentials",
						Fields: []views.FormField{
							{
								Key:     "username",
								Label:   "Username",
								Type:    views.FieldText,
								Default: "oauth2",
								Hint:    "GitLab PAT : oauth2 · GitLab Project Token : nom du token · GitHub : votre username",
							},
							{
								Key:   "token",
								Label: "Token d'accès",
								Type:  views.FieldPassword,
								Hint:  "Stocké dans votre keychain système, jamais dans oh",
							},
						},
						OnSubmit: func(values map[string]string, _ map[string][]string) {
							username := values["username"]
							if username == "" {
								username = "oauth2"
							}
							token := values["token"]
							if token == "" {
								go func() {
									tuiShell.App().QueueUpdateDraw(func() {
										tuiShell.ShowToast("Token requis", shell.ToastError)
									})
								}()
								return
							}
						go func() {
							_ = teamstate.EnsureCredentialHelper(customRepo)
							if err := teamstate.ConfigureCredential(tuiShell.Context(), customRepo, username, token); err != nil {
								tuiShell.App().QueueUpdateDraw(func() {
									tuiShell.ShowToast("Erreur configuration credential : "+err.Error(), shell.ToastError)
								})
								return
							}
							// Sequential with sleep — same pattern as collectCredentialsForInit.
							time.Sleep(50 * time.Millisecond)
							tuiShell.App().QueueUpdateDraw(func() {
								tuiShell.ShowToast("Credential configuré — les pulls/pushs utiliseront ce token automatiquement", shell.ToastSuccess)
							})
							time.Sleep(50 * time.Millisecond)
							tuiShell.App().QueueUpdateDraw(func() {
								continueCustomFlowWithMember(a, projectID, customRepo, hubMember)
							})
						}()
						},
						OnCancel: func() {
							go func() {
								tuiShell.App().QueueUpdateDraw(func() {
									tuiShell.ShowToast("Annulé", shell.ToastInfo)
								})
							}()
						},
					})
				})
			}()
		} else {
			go func() {
				tuiShell.App().QueueUpdateDraw(func() {
					continueCustomFlowWithMember(a, projectID, customRepo, hubMember)
				})
			}()
		}
	})
}

// continueCustomFlowWithMember continues the custom team setup flow
// after credentials have been configured (or skipped). It handles the
// identity selection step (reuse hub member or create new).
func continueCustomFlowWithMember(a *app.App, projectID, customRepo string, hubMember hubMemberInfo) {
	if hubMember.MemberID != "" {
		reuseLabel := fmt.Sprintf("Utiliser l'existant (%s — %s)", hubMember.MemberID, hubMember.DisplayName)
		identityOptions := []views.SelectOption{
			{Label: reuseLabel, Value: "reuse"},
			{Label: "Créer un nouveau membre", Value: "new"},
		}
		tuiShell.ShowSelectModal("Identité dans ce repo", identityOptions, "reuse", func(choice string) {
			if choice == "reuse" {
				runCustomSetupAndApply(a, projectID, customRepo,
					hubMember.MemberID, hubMember.DisplayName, hubMember.Role)
			} else {
				go func() {
					tuiShell.App().QueueUpdateDraw(func() {
						collectNewMemberAndApply(a, projectID, customRepo)
					})
				}()
			}
		})
	} else {
		collectNewMemberAndApply(a, projectID, customRepo)
	}
}

// collectNewMemberAndApply shows a single identity form (Member ID + Nom + Rôle)
// and calls runCustomSetupAndApply once submitted.
func collectNewMemberAndApply(a *app.App, projectID, customRepo string) {
	tuiShell.ShowInlineForm(views.InlineFormConfig{
		Title: "Étape 3 — Identité",
		Fields: []views.FormField{
			{
				Key:      "member_id",
				Label:    "Member ID",
				Type:     views.FieldText,
				Required: true,
				Hint:     "Identifiant unique dans l'équipe (ex: benjamin, alice)",
			},
			{
				Key:   "display_name",
				Label: "Nom d'affichage",
				Type:  views.FieldText,
				Hint:  "Votre nom tel qu'il apparaîtra dans les events team",
			},
			{
				Key:     "role",
			Label:   "Rôle",
			Type:    views.FieldSelect,
			Default: "dev",
			Options: []views.SelectOption{
				{Label: "Lead", Value: "lead"},
				{Label: "Développeur", Value: "dev"},
				{Label: "Reviewer", Value: "reviewer"},
			},
			Hint: "← / → pour changer",
		},
	},
	OnSubmit: func(values map[string]string, _ map[string][]string) {
		memberID := values["member_id"]
		if memberID == "" {
			go func() {
				tuiShell.App().QueueUpdateDraw(func() {
					tuiShell.ShowToast("Member ID requis", shell.ToastError)
				})
			}()
				return
			}
			runCustomSetupAndApply(a, projectID, customRepo, memberID, values["display_name"], values["role"])
		},
		OnCancel: func() {
			go func() {
				tuiShell.App().QueueUpdateDraw(func() {
					tuiShell.ShowToast("Annulé", shell.ToastInfo)
				})
			}()
		},
	})
}

// runCustomSetupAndApply runs the full custom team setup in a single sequential
// goroutine and persists the result in the DB.
//
// We use a single goroutine with a small sleep before the first QueueUpdateDraw.
// This guarantees the event loop has finished the button-handler draw cycle
// (RemovePage + SetFocus) before we attempt to add a new overlay (toast).
// Using two parallel goroutines both calling QueueUpdateDraw causes a race that
// hard-freezes tview because the second goroutine can enqueue its callback before
// the first draw cycle completes, leading to two concurrent page mutations.
func runCustomSetupAndApply(a *app.App, projectID, customRepo, memberID, displayName, role string) {
	ctx := tuiShell.Context()

	go func() {
		// Wait for the event loop to finish the current button-handler draw cycle.
		time.Sleep(50 * time.Millisecond)

		select {
		case <-ctx.Done():
			return
		default:
		}

		// Show the "in progress" toast — QueueUpdateDraw blocks this goroutine
		// until the toast is rendered, guaranteeing ordering.
		tuiShell.App().QueueUpdateDraw(func() {
			tuiShell.ShowToast("Initialisation de l'équipe pour ce projet...", shell.ToastInfo)
		})

		// Run the actual setup (git clone/init/member/push) — blocking, network I/O.
		result, err := runTeamCustomSetup(a, customRepo, memberID, displayName, role)

		// Show the result toast.
		tuiShell.App().QueueUpdateDraw(func() {
			if err != nil {
				tuiShell.ShowToast("Erreur d'initialisation de l'équipe : "+err.Error(), shell.ToastError)
				return
			}

			effectiveMemberID := memberID
			if effectiveMemberID == "" {
				effectiveMemberID = a.Config.ActiveTeam().MemberID
			}

			tc := &domain.ProjectTeamConfig{
				Mode:      domain.ProjectTeamModeCustom,
				StateRepo: customRepo,
				StatePath: result.StatePath,
				MemberID:  effectiveMemberID,
			}
			applyProjectTeamConfig(a, projectID, tc)
		})

		// Surface pull warning as a separate toast (must not share the same
		// QueueUpdateDraw as applyProjectTeamConfig to avoid double page mutation).
		if result.PullWarning != "" {
			time.Sleep(50 * time.Millisecond)
			tuiShell.App().QueueUpdateDraw(func() {
				tuiShell.ShowToast(result.PullWarning, shell.ToastWarning)
			})
		}
	}()
}

// applyProjectTeamConfig persists the team config override for a project in the DB.
//
// IMPORTANT: This function MUST be called from within the tview event loop
// (e.g. from a QueueUpdateDraw callback or a tview handler). It calls ShowToast
// directly — never via QueueUpdateDraw — to avoid a nested-QueueUpdateDraw deadlock.
// (QueueUpdateDraw blocks on an unbuffered done-channel; calling it from inside a
// running QueueUpdateDraw callback deadlocks because the event loop cannot drain the
// queue while executing the current callback.)
func applyProjectTeamConfig(a *app.App, projectID string, tc *domain.ProjectTeamConfig) {
	ctx := context.Background()
	p, err := a.Projects.Get(ctx, projectID)
	if err != nil {
		// Direct call — we're already on the event loop.
		tuiShell.ShowToast("Erreur : projet introuvable", shell.ToastError)
		return
	}

	p.TeamConfig = tc
	if err := a.Projects.Update(ctx, p); err != nil {
		tuiShell.ShowToast("Erreur : "+err.Error(), shell.ToastError)
		return
	}

	modeLabel := "hérité du hub"
	if tc != nil {
		modeLabel = tc.Mode
	}
	tuiShell.ShowToast(
		fmt.Sprintf("Équipe configurée : %s — redéployez pour appliquer", modeLabel),
		shell.ToastSuccess,
	)
}

// ─────────────────────────────────────────────────────────────────────────────
// Takeover brief enrichment
// ─────────────────────────────────────────────────────────────────────────────

func runTakeoverEnrich(a *app.App, project, ticketID string) error {
	repo := teamstate.NewRepo(a.Config.ActiveTeam().StateRepo, a.Config.ActiveTeam().StatePath)

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

	relPath := filepath.Join("projects", project, "takeover-briefs", latestBase+".enriched.md")
	ctx := tuiShell.Context()
	_ = repo.CommitAndPush(ctx, fmt.Sprintf("takeover: enriched brief for %s/%s", project, ticketID), relPath)

	return nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Sync Tracker action (omnibar + view touche 's')
// ─────────────────────────────────────────────────────────────────────────────

// actionSyncTracker is the omnibar action for "sync tracker".
func actionSyncTracker() {
	if tuiShell == nil {
		return
	}
	a := MustApp()
	ctx := tuiShell.Context()

	tuiShell.ShowToast("Synchronisation en cours...", shell.ToastInfo)

	go func() {
		select {
		case <-ctx.Done():
			return
		default:
		}
		result, err := runSyncTrackerForTUI(a, ctx)
		select {
		case <-ctx.Done():
			return
		default:
		}
		tuiShell.App().QueueUpdateDraw(func() {
			if err != nil {
				tuiShell.ShowToast("✗ Sync: "+err.Error(), shell.ToastError)
				return
			}
			content := formatSyncResultModal(result)
			tuiShell.ShowScrollableModal("Résultat sync tracker", content, []views.ModalAction{
				{Label: "OK", Callback: func() {}},
			})
		})
	}()
}

// runSyncTrackerForTUI executes the tracker sync and returns a result for display.
func runSyncTrackerForTUI(a *app.App, ctx context.Context) (*views.SyncTrackerResult, error) {
	// Resolve team config
	tc := resolvedTeamConfig(a, nil)
	if !tc.Enabled {
		return nil, fmt.Errorf("équipe non configurée")
	}

	repo := teamstate.NewRepo(tc.StateRepo, tc.StatePath)
	if !repo.IsCloned() {
		return nil, fmt.Errorf("team-state non cloné — lancez 'team init'")
	}

	// Load team config
	teamCfg, err := repo.LoadConfig()
	if err != nil {
		return nil, fmt.Errorf("chargement config: %w", err)
	}
	if teamCfg.Tracker.Type == "" {
		return nil, fmt.Errorf("tracker non configuré — utilisez 'g' dans la vue Config équipe")
	}

	// Build credential source
	credSrc := buildCredentialSource(a, teamCfg.MCP)
	trackerType := tracker.Type(teamCfg.Tracker.Type)

	creds, err := tracker.ResolveCredentials(ctx, credSrc, trackerType)
	if err != nil {
		return nil, fmt.Errorf("credentials manquants — activez %s dans Settings: %w", teamCfg.Tracker.Type, err)
	}

	t, err := tracker.New(creds)
	if err != nil {
		return nil, fmt.Errorf("initialisation tracker: %w", err)
	}

	engine := tracker.NewEngine(t, repo, teamCfg.Tracker, config.HubDir())

	// Pull before sync
	_ = repo.Pull(ctx)

	// Run sync
	syncResult, err := engine.Run(ctx)
	if err != nil {
		return nil, fmt.Errorf("sync: %w", err)
	}

	// Convert to view-friendly result
	result := &views.SyncTrackerResult{
		ClaimsCreated: syncResult.ClaimsCreated,
		ClaimsUpdated: syncResult.ClaimsUpdated,
		LabelsPushed:  syncResult.LabelsPushed,
	}
	for _, p := range syncResult.Projects {
		result.Projects = append(result.Projects, fmt.Sprintf("%s: %d fetched, %d created, %d updated", p.ProjectID, p.IssuesFetched, p.ClaimsCreated, p.ClaimsUpdated))
	}
	for _, w := range syncResult.Warnings {
		result.Warnings = append(result.Warnings, w.Message)
	}
	for _, e := range syncResult.Errors {
		result.Errors = append(result.Errors, e.Error())
	}

	return result, nil
}

func formatSyncResultModal(r *views.SyncTrackerResult) string {
	if r == nil {
		return "Aucun résultat"
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Claims créés:      %d\n", r.ClaimsCreated))
	sb.WriteString(fmt.Sprintf("Claims mis à jour: %d\n", r.ClaimsUpdated))
	sb.WriteString(fmt.Sprintf("Labels poussés:    %d\n", r.LabelsPushed))

	if len(r.Projects) > 0 {
		sb.WriteString("\nProjets:\n")
		for _, p := range r.Projects {
			sb.WriteString(fmt.Sprintf("  %s\n", p))
		}
	}
	if len(r.Warnings) > 0 {
		sb.WriteString("\nWarnings:\n")
		for _, w := range r.Warnings {
			sb.WriteString(fmt.Sprintf("  ⚠ %s\n", w))
		}
	}
	if len(r.Errors) > 0 {
		sb.WriteString("\nErreurs:\n")
		for _, e := range r.Errors {
			sb.WriteString(fmt.Sprintf("  ✗ %s\n", e))
		}
	}
	return sb.String()
}
