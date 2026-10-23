package daemon

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/services/wiki"
	"github.com/datichb/openhub/cli/internal/sessionspec"
)

// Routes of the wiki MCP server of the project (modernisation LD-J),
// authenticated like the workflow routes (token of the server group, the
// session must belong to it). The wiki is the one of the location of the
// oh session (its worktree), the queue the one of its project. The author
// of a proposal is the agent the daemon saw in the calling tool session;
// wiki_apply is refused to any agent but the documentarian, whatever the
// rules of the session.

// wikiRequestMax bounds the body of the wiki POST routes (a proposal is
// bounded by the WikiService, after cleaning).
const wikiRequestMax = 64 << 10

func wikiView(p domain.WikiProposal) WikiProposal { return WikiProposal{ID: p.ID(), WikiProposal: p} }

// wikiSession returns the oh session of an authorized wiki request
// (written error otherwise).
func (d *Daemon) wikiSession(w http.ResponseWriter, r *http.Request, toolSession string) (*domain.Session, string, bool) {
	if d.opts.Wiki == nil || d.opts.Sessions == nil {
		writeErr(w, http.StatusServiceUnavailable, i18n.T("cmd.daemon.wiki.off"))
		return nil, "", false
	}
	if toolSession == "" {
		writeErr(w, http.StatusBadRequest, "tool_session is required")
		return nil, "", false
	}
	root, ok := d.authorizeWorkflow(w, r, toolSession)
	if !ok {
		return nil, "", false
	}
	sess, err := d.opts.Sessions.Get(r.Context(), root)
	if err != nil {
		writeErr(w, http.StatusNotFound, err.Error())
		return nil, "", false
	}
	return sess, root, true
}

func (d *Daemon) wikiErr(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, wiki.ErrBadPage), errors.Is(err, wiki.ErrBadTag), errors.Is(err, wiki.ErrNoSource), errors.Is(err, wiki.ErrEmpty),
		errors.Is(err, wiki.ErrTooLarge), errors.Is(err, wiki.ErrSessionData), errors.Is(err, wiki.ErrSymlink), errors.Is(err, wiki.ErrBadID),
		errors.Is(err, wiki.ErrBadRef), errors.Is(err, wiki.ErrReplaceLine), errors.Is(err, wiki.ErrReplaceStructure),
		errors.Is(err, wiki.ErrReplaceContent), errors.Is(err, wiki.ErrReplaceSame), errors.Is(err, wiki.ErrStale):
		writeErr(w, http.StatusUnprocessableEntity, err.Error())
	case errors.Is(err, wiki.ErrNoWiki), errors.Is(err, wiki.ErrNoPage), errors.Is(err, wiki.ErrNoSection), errors.Is(err, domain.ErrNotFound):
		writeErr(w, http.StatusNotFound, err.Error())
	case errors.Is(err, domain.ErrWikiLimit):
		writeErr(w, http.StatusInsufficientStorage, i18n.Tf("cmd.daemon.wiki.full", wiki.MaxPerSession, wiki.MaxPerProject))
	default:
		writeErr(w, http.StatusInternalServerError, err.Error())
	}
}

func (d *Daemon) handleWikiIndex(w http.ResponseWriter, r *http.Request) {
	sess, root, ok := d.wikiSession(w, r, r.URL.Query().Get("tool_session"))
	if !ok {
		return
	}
	wk, err := wiki.Load(sess.LaunchPath)
	if err != nil {
		d.wikiErr(w, err)
		return
	}
	ix := wk.Index()
	out := WikiIndex{Title: ix.Title, Sections: ix.Sections, GodNodes: ix.GodNodes, NoIndex: !ix.Present, Skipped: wk.Skipped, Pages: []WikiPageInfo{}}
	for _, p := range wk.Pages {
		out.Pages = append(out.Pages, WikiPageInfo{Path: p.Path, Title: p.Title, Updated: p.Frontmatter.Updated, Confidence: p.Frontmatter.Confidence})
	}
	if open, err := d.opts.Wiki.Open(r.Context(), sess.ProjectID, ""); err == nil {
		out.Pending = len(open)
		for _, p := range open {
			if p.SessionID == root {
				out.Session = append(out.Session, wikiView(p))
			}
		}
	}
	writeJSON(w, http.StatusOK, out)
}

func (d *Daemon) handleWikiPage(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	sess, _, ok := d.wikiSession(w, r, q.Get("tool_session"))
	if !ok {
		return
	}
	page, section := q.Get("page"), strings.TrimSpace(q.Get("section"))
	text, err := wiki.ReadPage(sess.LaunchPath, page, section)
	if err != nil {
		d.wikiErr(w, err)
		return
	}
	rel, _ := wiki.PagePath(page)
	writeJSON(w, http.StatusOK, WikiPage{Page: rel, Section: section, Text: text})
}

func (d *Daemon) handleWikiPropose(w http.ResponseWriter, r *http.Request) {
	var req WikiProposeRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, wikiRequestMax)).Decode(&req); err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			writeErr(w, http.StatusRequestEntityTooLarge, wiki.ErrTooLarge.Error())
			return
		}
		writeErr(w, http.StatusBadRequest, "tool_session, page, content and tag are required")
		return
	}
	sess, root, ok := d.wikiSession(w, r, req.ToolSession)
	if !ok {
		return
	}
	agent := d.callerAgent(r.Context(), root, req.ToolSession)
	p, err := d.opts.Wiki.Propose(r.Context(), wiki.Filed{ProjectID: sess.ProjectID, SessionID: root, ToolSession: req.ToolSession, Agent: agent,
		Root:     sess.LaunchPath,
		Proposal: wiki.Proposal{Page: req.Page, Section: req.Section, Content: req.Content, Tag: req.Tag, Source: req.Source, Replaces: req.Replaces}})
	if err != nil {
		d.wikiErr(w, err)
		return
	}
	view := wikiView(p)
	if prior, err := d.opts.Wiki.PriorRefusal(r.Context(), p); err == nil && prior != nil {
		view.PreviouslyRefused = &WikiRefusal{ID: prior.ID(), Reason: prior.Reason, At: prior.DecidedAt}
	}
	writeJSON(w, http.StatusCreated, view)
}

func (d *Daemon) handleWikiPending(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	sess, root, ok := d.wikiSession(w, r, q.Get("tool_session"))
	if !ok {
		return
	}
	scope := ""
	if q.Get("scope") == "session" {
		scope = root
	}
	list, err := d.opts.Wiki.Open(r.Context(), sess.ProjectID, scope)
	if err != nil {
		d.wikiErr(w, err)
		return
	}
	out := make([]WikiProposal, 0, len(list))
	for _, p := range list {
		out = append(out, wikiView(p))
	}
	writeJSON(w, http.StatusOK, out)
}

// handleWikiRefused lists the recent refusals of the project (or of the
// session): what the agents must not propose again unchanged.
func (d *Daemon) handleWikiRefused(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	sess, root, ok := d.wikiSession(w, r, q.Get("tool_session"))
	if !ok {
		return
	}
	scope := ""
	if q.Get("scope") == "session" {
		scope = root
	}
	list, err := d.opts.Wiki.Refused(r.Context(), sess.ProjectID, scope)
	if err != nil {
		d.wikiErr(w, err)
		return
	}
	out := make([]WikiProposal, 0, len(list))
	for _, p := range list {
		out = append(out, wikiView(p))
	}
	writeJSON(w, http.StatusOK, out)
}

func (d *Daemon) handleWikiApply(w http.ResponseWriter, r *http.Request) {
	var req WikiApplyRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, wikiRequestMax)).Decode(&req); err != nil || len(req.IDs) == 0 {
		writeErr(w, http.StatusBadRequest, "tool_session and ids are required")
		return
	}
	sess, root, ok := d.wikiSession(w, r, req.ToolSession)
	if !ok {
		return
	}
	if d.opts.WikiNoApply {
		writeErr(w, http.StatusForbidden, i18n.T("cmd.daemon.wiki.apply_remote"))
		return
	}
	agent := d.callerAgent(r.Context(), root, req.ToolSession)
	if agent != sessionspec.WikiApplier {
		if agent == "" {
			agent = "?"
		}
		writeErr(w, http.StatusForbidden, i18n.Tf("cmd.daemon.wiki.apply_forbidden", agent))
		return
	}
	if len(req.IDs) > wiki.MaxPerSession {
		req.IDs = req.IDs[:wiki.MaxPerSession]
	}
	writeJSON(w, http.StatusOK, d.opts.Wiki.ApplyIDs(r.Context(), sess.LaunchPath, sess.ProjectID, req.IDs, agent))
}

func (d *Daemon) handleWikiCheck(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	sess, _, ok := d.wikiSession(w, r, q.Get("tool_session"))
	if !ok {
		return
	}
	paths := q["path"]
	if len(paths) > wiki.MaxScopePaths {
		paths = paths[:wiki.MaxScopePaths]
	}
	opts := wiki.CheckOptions{Paths: paths, Base: strings.TrimSpace(q.Get("base")), Head: strings.TrimSpace(q.Get("head")), SinceSync: q.Get("since_sync") == "1"}
	rep, err := wiki.CheckWith(r.Context(), sess.LaunchPath, opts, nil)
	if err != nil {
		d.wikiErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, rep)
}
