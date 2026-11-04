package cmd

import (
	"context"
	"errors"
	"log/slog"
	"regexp"
	"strings"
	"time"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/beads"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/runsvc"
	"github.com/datichb/openhub/cli/internal/safego"
	"github.com/datichb/openhub/cli/internal/teamstate"
	"github.com/datichb/openhub/cli/internal/tui/v2/shell"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
)

// The actions of the card menu of the team board that need more than a launch
// form: where the ticket of a card lives, the import of a GitLab issue into
// Beads, the claim of the card when the work starts, the instruction to the
// running session.

// tuiCardClaim is the card whose work a launch starts: the launch moves its
// claim in progress, within the limit of work in progress.
type tuiCardClaim struct {
	// Project and Ticket name the claim (the team-state project of the card).
	Project, Ticket string
	// Imported is the Beads ticket the issue of the card was imported as and
	// the launch runs on: the card is the claim, not that ticket.
	Imported string
}

// jiraKey is the shape of a Jira ticket key (SRU-12); the ID of a Beads
// ticket has a lowercase prefix and a hash.
var jiraKey = regexp.MustCompile(`^[A-Z][A-Z0-9_]+-\d+$`)

// boardCardSource tells where the ticket of a card lives: a number is an issue
// where the tracker is GitLab, a Jira key is Jira's, anything else is a Beads
// ticket. For a GitLab issue it also returns its reference (« #693 »).
func boardCardSource(a *app.App, ticket views.TicketContext) (source views.CardSource, ref string) {
	if ref = boardIssueRef(a, ticket); ref != "" {
		return views.CardGitLab, ref
	}
	if jiraKey.MatchString(ticket.ID) || strings.Trim(ticket.ID, "0123456789") == "" {
		return views.CardOther, ""
	}
	return views.CardBeads, ""
}

// boardCardProject is the project of a card: its own, else the active one.
func boardCardProject(a *app.App, ticket views.TicketContext) *domain.Project {
	if ticket.ProjectID != "" && a.Projects != nil {
		if p, err := a.Projects.Get(context.Background(), ticket.ProjectID); err == nil && p != nil {
			return p
		}
	}
	p, _ := resolveActiveProject(a)
	return p
}

// boardCardPath is the directory of the project of a card ("" when unknown).
func boardCardPath(a *app.App, ticket views.TicketContext) string {
	if ticket.ProjectPath != "" {
		return ticket.ProjectPath
	}
	if p := boardCardProject(a, ticket); p != nil {
		return p.Path
	}
	return ""
}

// boardCanImport reports whether the issue of a card can become a Beads ticket:
// Beads is installed and initialised in the project of the card.
func boardCanImport(a *app.App, ticket views.TicketContext) bool {
	path := boardCardPath(a, ticket)
	return path != "" && beads.Available() == nil && beads.IsInitialized(path)
}

// cardLaunchRequest is the launch form of a workflow on a card: the request
// and the inputs prefilled, the claim of the card to start when the launch
// starts the work.
func cardLaunchRequest(l views.CardLaunch, ticket views.TicketContext) tuiLaunchRequest {
	req := boardLaunchRequest(l.Workflow, ticket, tuiLaunchRequest{})
	prefill := map[string]string{}
	if l.Request != "" {
		input := boardRequestInput
		if tuiStartWiring != nil {
			input = tuiStartWiring.requestInput(l.Workflow)
		}
		prefill[input] = l.Request
	}
	for k, v := range l.Prefill {
		prefill[k] = v
	}
	if len(prefill) > 0 {
		req.Prefill = prefill
	}
	if l.Start {
		req.Card = &tuiCardClaim{Project: ticket.Project, Ticket: ticket.ID}
	}
	return req
}

// boardLaunchCard opens the launch form of a workflow on a card of the team
// board: the request or the inputs prefilled, the issue imported into Beads
// first when asked, the claim of the card to move in progress when the launch
// starts the work.
func boardLaunchCard(a *app.App, l views.CardLaunch, ticket views.TicketContext) {
	req := cardLaunchRequest(l, ticket)
	if !l.Import {
		openLaunchForm(a, req)
		return
	}
	sh := tuiShell
	if sh == nil {
		return
	}
	ref := "#" + ticket.ID
	safego.Go(func() {
		id, created, err := importCardIssue(a, ticket)
		queueOnShell(sh, func() {
			if err != nil {
				sh.ShowToast(i18n.Tf("v53f.card.import_failed", ref, localizeError(err)), shell.ToastError)
				return
			}
			key := "v53f.card.import_reused"
			if created {
				key = "v53f.card.imported"
			}
			sh.ShowToast(i18n.Tf(key, ref, id), shell.ToastSuccess)
			req.Tickets, req.AtOptions = []string{id}, true
			if req.Card != nil {
				req.Card.Imported = id
			}
			openLaunchForm(a, req)
		})
	})
}

// importCardIssue returns the Beads ticket of the GitLab issue of a card,
// created when the project has none (the external ref gitlab-<iid> links
// them): launching twice on the same issue works on the same ticket.
func importCardIssue(a *app.App, ticket views.TicketContext) (id string, created bool, err error) {
	path := boardCardPath(a, ticket)
	if path == "" {
		return "", false, errors.New(i18n.T("v53f.card.no_project"))
	}
	if !beads.IsInitialized(path) {
		return "", false, errors.New(i18n.T("v53f.card.no_beads"))
	}
	description := "Imported from the GitLab issue #" + ticket.ID + " (" + beads.BuildExternalRef(beads.ProviderGitLab, ticket.ID) + ")."
	if extract := strings.TrimSpace(ticket.Description); extract != "" {
		description += "\n\n" + extract
	}
	return beads.ImportFromTracker(path, beads.ProviderGitLab, ticket.ID, ticket.Title, description)
}

// cardTeam is the team-state of the project of a launch and the member who
// launches; false when the project has no team (nothing is claimed then).
func cardTeam(a *app.App, project *domain.Project) (cardClaimStore, string, bool) {
	if project == nil || !teamEnabledForProject(a, project) || a.Config.ActiveTeam().MemberID == "" {
		return nil, "", false
	}
	tc := resolvedTeamConfig(a, project)
	statePath := tc.StatePath
	if statePath == "" {
		statePath = defaultTeamStatePath(a)
	}
	repo := teamstate.NewRepo(tc.StateRepo, statePath)
	if !repo.IsCloned() {
		return nil, "", false
	}
	return repo, a.Config.ActiveTeam().MemberID, true
}

// cardClaimProject is the project the claim of a card is under.
func cardClaimProject(card *tuiCardClaim, project *domain.Project) string {
	if card.Project != "" {
		return card.Project
	}
	return project.ID
}

// cardLaunchRoom refuses (*teamstate.InProgressLimitError) a launch that
// starts the work on a card when that goes past the limit of tickets the
// member has in progress. A card the member already works on, or someone
// else's, starts nothing new. Without a team there is nothing to check.
func cardLaunchRoom(ctx context.Context, a *app.App, project *domain.Project, card *tuiCardClaim) error {
	repo, member, ok := cardTeam(a, project)
	if !ok {
		return nil
	}
	return checkCardRoom(ctx, repo, a, member, cardClaimProject(card, project), card.Ticket)
}

func checkCardRoom(ctx context.Context, repo wipStore, a *app.App, member, projectID, ticketID string) error {
	c, err := repo.GetClaim(ctx, projectID, ticketID)
	switch {
	case err != nil && !errors.Is(err, teamstate.ErrClaimNotFound):
		return nil //nolint:nilerr // an unreadable team state never blocks the work
	case err != nil, c.ClaimedBy == "":
		// No claim, or one nobody holds (the pool): the launch takes it.
	case c.ClaimedBy != member:
		return nil // somebody else's: the launch starts nothing for the member
	default:
		var board teamstate.BoardConfig
		if cfg, cerr := repo.LoadConfig(ctx); cerr == nil && cfg != nil {
			board = cfg.Board
		}
		if c.Status != board.InitialStatus() && c.Status != teamstate.ClaimStatusPlanned {
			return nil // already started or in review
		}
	}
	return checkInProgressRoom(ctx, repo, a.Config.Tracker, member, teamstate.ClaimRef{Project: projectID, TicketID: ticketID})
}

// cardClaimStore is what moving the claim of a card in progress asks of the
// team-state.
type cardClaimStore interface {
	ticketClaimStore
	teamstate.ClaimReader
}

// startCardClaim moves the claim of a card in progress for member, or takes
// it when nobody holds it (the pool) or when the card has no claim yet; the
// session that starts is recorded on it. It returns a note ("" when nothing
// changed).
func startCardClaim(ctx context.Context, repo cardClaimStore, projectID, member, ticketID string) string {
	if c, err := repo.GetClaim(ctx, projectID, ticketID); err == nil && c.ClaimedBy == "" {
		var board teamstate.BoardConfig
		if cfg, cerr := repo.LoadConfig(ctx); cerr == nil && cfg != nil {
			board = cfg.Board
		}
		if err := repo.ClaimPoolTicket(ctx, projectID, ticketID, member, board.DefaultWorkStatus()); err != nil {
			return i18n.Tf("v53f.card.claim_failed", ticketID, localizeError(err))
		}
		if _, _, err := repo.AdoptClaimDurable(ctx, projectID, ticketID, member, runsvc.StartedSession(ctx)); err != nil {
			slog.Warn("team claim not tied to the session", "ticket", ticketID, "error", err)
		}
		return i18n.Tf("cmd.run.claim.taken", projectID, ticketID)
	}
	return startTicketClaim(ctx, repo, projectID, member, ticketID)
}

// claimCardAtLaunch moves the card of a started launch in progress. The
// session is already running: a claim that fails is told, never an error of
// the launch.
func claimCardAtLaunch(ctx context.Context, a *app.App, project *domain.Project, card *tuiCardClaim, sessionID string) string {
	repo, member, ok := cardTeam(a, project)
	if !ok {
		return ""
	}
	ctx = runsvc.WithStartedSession(ctx, sessionID)
	return startCardClaim(ctx, repo, cardClaimProject(card, project), member, card.Ticket)
}

// boardActiveSession returns the open session that works on a card: the one
// its claim names, when it is not finished.
func boardActiveSession(ticket views.TicketContext) string {
	return tuiSess.openSession(ticket.SessionID)
}

// boardInstruct sends an instruction to the session of a card, off the event
// loop, and tells the result.
func boardInstruct(ticket views.TicketContext, sessionID, text string) {
	sh, sess := tuiShell, tuiSess
	if sh == nil || sess == nil {
		return
	}
	safego.Go(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		err := sess.Send(ctx, sessionID, text)
		queueOnShell(sh, func() {
			if err != nil {
				sh.ShowToast(i18n.Tf("v53f.card.instruct_failed", err), shell.ToastError)
				return
			}
			sh.ShowToast(i18n.Tf("v53f.card.instruct_sent", ticket.ID, sessionID), shell.ToastSuccess)
		})
	})
}
