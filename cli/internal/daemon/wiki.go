package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
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

// WikiProposeRequest is the body of POST /v1/wiki/propose.
type WikiProposeRequest struct {
	ToolSession string `json:"tool_session"`
	Page        string `json:"page"`
	Section     string `json:"section,omitempty"`
	Content     string `json:"content"`
	Tag         string `json:"tag"`
	Source      string `json:"source,omitempty"`
}

// WikiApplyRequest is the body of POST /v1/wiki/apply.
type WikiApplyRequest struct {
	ToolSession string   `json:"tool_session"`
	IDs         []string `json:"ids"`
}

// WikiProposal is a stored proposal.
type WikiProposal struct {
	ID string `json:"id"`
	domain.WikiProposal
}

// WikiPageInfo is a page of the index.
type WikiPageInfo struct {
	Path       string `json:"path"`
	Title      string `json:"title,omitempty"`
	Updated    string `json:"updated,omitempty"`
	Confidence string `json:"confidence,omitempty"`
}

// WikiIndex answers GET /v1/wiki/index.
type WikiIndex struct {
	Title    string         `json:"title,omitempty"`
	Sections []string       `json:"sections,omitempty"`
	Pages    []WikiPageInfo `json:"pages"`
	GodNodes []wiki.GodNode `json:"god_nodes,omitempty"`
	Pending  int            `json:"pending"`
	NoIndex  bool           `json:"no_index,omitempty"`
	Skipped  []string       `json:"skipped,omitempty"`
	Session  []WikiProposal `json:"session_proposals,omitempty"`
}

// WikiPage answers GET /v1/wiki/page.
type WikiPage struct {
	Page    string `json:"page"`
	Section string `json:"section,omitempty"`
	Text    string `json:"text"`
}

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
		errors.Is(err, wiki.ErrBadRef):
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
		Proposal: wiki.Proposal{Page: req.Page, Section: req.Section, Content: req.Content, Tag: req.Tag, Source: req.Source}})
	if err != nil {
		d.wikiErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, wikiView(p))
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

// WikiIndex returns the index of the wiki of the session of a tool session.
func (c *Client) WikiIndex(ctx context.Context, toolSession string) (WikiIndex, error) {
	var out WikiIndex
	err := c.do(ctx, http.MethodGet, "/wiki/index?tool_session="+url.QueryEscape(toolSession), nil, &out)
	return out, err
}

// WikiPage reads a page of the wiki (or one of its sections).
func (c *Client) WikiPage(ctx context.Context, toolSession, page, section string) (WikiPage, error) {
	q := url.Values{"tool_session": {toolSession}, "page": {page}}
	if section != "" {
		q.Set("section", section)
	}
	var out WikiPage
	err := c.do(ctx, http.MethodGet, "/wiki/page?"+q.Encode(), nil, &out)
	return out, err
}

// WikiPropose queues a proposal.
func (c *Client) WikiPropose(ctx context.Context, req WikiProposeRequest) (WikiProposal, error) {
	var out WikiProposal
	err := c.do(ctx, http.MethodPost, "/wiki/propose", req, &out)
	return out, err
}

// WikiPending lists the open proposals of the project (or of the session).
func (c *Client) WikiPending(ctx context.Context, toolSession string, sessionOnly bool) ([]WikiProposal, error) {
	q := url.Values{"tool_session": {toolSession}}
	if sessionOnly {
		q.Set("scope", "session")
	}
	var out []WikiProposal
	err := c.do(ctx, http.MethodGet, "/wiki/pending?"+q.Encode(), nil, &out)
	return out, err
}

// WikiApply applies proposals (documentarian only).
func (c *Client) WikiApply(ctx context.Context, toolSession string, ids []string) ([]wiki.Result, error) {
	var out []wiki.Result
	err := c.do(ctx, http.MethodPost, "/wiki/apply", WikiApplyRequest{ToolSession: toolSession, IDs: ids}, &out)
	return out, err
}

// WikiCheck checks the wiki of the session, restricted to a change when
// opts restricts it (LD-H, LD-D).
func (c *Client) WikiCheck(ctx context.Context, toolSession string, opts wiki.CheckOptions) (wiki.CheckReport, error) {
	q := url.Values{"tool_session": {toolSession}}
	for _, p := range opts.Paths {
		q.Add("path", p)
	}
	if opts.Base != "" {
		q.Set("base", opts.Base)
	}
	if opts.Head != "" {
		q.Set("head", opts.Head)
	}
	if opts.SinceSync {
		q.Set("since_sync", "1")
	}
	var out wiki.CheckReport
	err := c.do(ctx, http.MethodGet, "/wiki/check?"+q.Encode(), nil, &out)
	return out, err
}
