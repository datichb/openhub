// Package wiki is the oh MCP server `wiki` (modernisation LD-J), run by the
// tool for each session location (`oh mcp serve wiki`) in the bundles built
// from a workflow for a project that has docs/wiki/. Tools reach the model
// as wiki_<tool>:
//
//   - wiki_index    the index of the wiki, the god nodes computed by oh and
//     the proposals waiting;
//   - wiki_read     {page[, section]}: a page, or one of its sections;
//   - wiki_propose  {page, content, tag[, source, section]}: queues a
//     proposal; oh sets its author (the agent it sees), the user validates
//     it on the card of the commit checkpoint and oh applies it;
//   - wiki_pending  the proposals waiting (of the project, or the session);
//   - wiki_apply    {ids}: applies proposals (documentarian only);
//   - wiki_check    checks the wiki (frontmatter, links, tags, freshness,
//     god nodes).
//
// The server holds no state: everything goes to the oh daemon, for the
// session that calls the tool. Without the daemon, a proposal is not
// recorded and the agent keeps it in its report.
package wiki

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/datichb/openhub/cli/internal/daemon"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/mcp/protocol"
	mcpworkflow "github.com/datichb/openhub/cli/internal/mcp/workflow"
	wikisvc "github.com/datichb/openhub/cli/internal/services/wiki"
	"github.com/datichb/openhub/cli/internal/sessionspec"
)

// Backend is the oh daemon as seen from the MCP server.
type Backend interface {
	WikiIndex(ctx context.Context, toolSession string) (daemon.WikiIndex, error)
	WikiPage(ctx context.Context, toolSession, page, section string) (daemon.WikiPage, error)
	WikiPropose(ctx context.Context, req daemon.WikiProposeRequest) (daemon.WikiProposal, error)
	WikiPending(ctx context.Context, toolSession string, sessionOnly bool) ([]daemon.WikiProposal, error)
	WikiApply(ctx context.Context, toolSession string, ids []string) ([]wikisvc.Result, error)
	WikiCheck(ctx context.Context, toolSession string) (wikisvc.CheckReport, error)
}

// Serve runs the server on stdio against the oh daemon, with the token of
// the server group found in its environment.
func Serve() error {
	c := daemon.NewClient(daemon.DefaultPaths()).WithGroupToken(os.Getenv(sessionspec.EnvGroupToken))
	return New(c).Serve()
}

// pendingShown bounds the proposals listed by wiki_pending.
const pendingShown = 40

// New returns the MCP server backed by b.
func New(b Backend) *protocol.Server {
	s := protocol.NewServer("oh-wiki", "1.0.0")
	h := handlers{b: b}
	str := func(key string) map[string]any { return map[string]any{"type": "string", "description": i18n.T(key)} }
	s.RegisterTool(protocol.Tool{
		Name:        sessionspec.WikiToolIndex,
		Description: i18n.T("cmd.mcp.wiki.index_desc"),
		InputSchema: map[string]any{"type": "object", "properties": map[string]any{}},
	}, h.index)
	s.RegisterTool(protocol.Tool{
		Name:        sessionspec.WikiToolRead,
		Description: i18n.T("cmd.mcp.wiki.read_desc"),
		InputSchema: map[string]any{"type": "object", "properties": map[string]any{
			"page": str("cmd.mcp.wiki.page"), "section": str("cmd.mcp.wiki.section_read"),
		}, "required": []string{"page"}},
	}, h.read)
	s.RegisterTool(protocol.Tool{
		Name:        sessionspec.WikiToolPropose,
		Description: i18n.T("cmd.mcp.wiki.propose_desc"),
		InputSchema: map[string]any{"type": "object", "properties": map[string]any{
			"page": str("cmd.mcp.wiki.page"), "section": str("cmd.mcp.wiki.section"), "content": str("cmd.mcp.wiki.content"),
			"tag":    map[string]any{"type": "string", "enum": wikisvc.Tags, "description": i18n.T("cmd.mcp.wiki.tag")},
			"source": str("cmd.mcp.wiki.source"),
		}, "required": []string{"page", "content", "tag"}},
	}, h.propose)
	s.RegisterTool(protocol.Tool{
		Name:        sessionspec.WikiToolPending,
		Description: i18n.T("cmd.mcp.wiki.pending_desc"),
		InputSchema: map[string]any{"type": "object", "properties": map[string]any{
			"session_only": map[string]any{"type": "boolean", "description": i18n.T("cmd.mcp.wiki.session_only")},
		}},
	}, h.pending)
	s.RegisterTool(protocol.Tool{
		Name:        sessionspec.WikiToolApply,
		Description: i18n.T("cmd.mcp.wiki.apply_desc"),
		InputSchema: map[string]any{"type": "object", "properties": map[string]any{
			"ids": map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": i18n.T("cmd.mcp.wiki.ids")},
		}, "required": []string{"ids"}},
	}, h.apply)
	s.RegisterTool(protocol.Tool{
		Name:        sessionspec.WikiToolCheck,
		Description: i18n.T("cmd.mcp.wiki.check_desc"),
		InputSchema: map[string]any{"type": "object", "properties": map[string]any{}},
	}, h.check)
	return s
}

type handlers struct{ b Backend }

func text(s string) *protocol.ToolResult {
	return &protocol.ToolResult{Content: []protocol.ContentBlock{{Type: "text", Text: s}}}
}

func session(ctx context.Context) (string, error) {
	id := mcpworkflow.ToolSession(ctx)
	if id == "" {
		return "", errors.New(i18n.T("cmd.mcp.workflow.no_session"))
	}
	return id, nil
}

// notRecorded reports that the daemon cannot serve the wiki (stopped, wiki
// turned off, a daemon without the route, a session unknown to oh).
func notRecorded(err error) bool {
	if errors.Is(err, daemon.ErrNotRunning) {
		return true
	}
	var api *daemon.APIError
	return errors.As(err, &api) && (api.Status == http.StatusServiceUnavailable || api.Status == http.StatusNotFound && strings.HasPrefix(api.Message, "404 "))
}

func unavailable(err error) error {
	if notRecorded(err) {
		return errors.New(i18n.T("cmd.mcp.wiki.unavailable"))
	}
	var api *daemon.APIError
	if errors.As(err, &api) && api.Message != "" {
		return errors.New(api.Message)
	}
	return err
}

func (h handlers) index(ctx context.Context, _ json.RawMessage) (*protocol.ToolResult, error) {
	id, err := session(ctx)
	if err != nil {
		return nil, err
	}
	ix, err := h.b.WikiIndex(ctx, id)
	if err != nil {
		return nil, unavailable(err)
	}
	var b strings.Builder
	if ix.NoIndex {
		b.WriteString(i18n.T("cmd.mcp.wiki.no_index") + "\n")
	} else {
		fmt.Fprintf(&b, "# %s\n", firstText(ix.Title, wikisvc.IndexPage))
		if len(ix.Sections) > 0 {
			fmt.Fprintf(&b, "%s\n", i18n.Tf("cmd.mcp.wiki.sections", strings.Join(ix.Sections, " · ")))
		}
	}
	fmt.Fprintf(&b, "\n%s\n", i18n.Tf("cmd.mcp.wiki.pages", len(ix.Pages)))
	for _, p := range ix.Pages {
		line := "- " + p.Path
		if p.Title != "" {
			line += " : " + p.Title
		}
		var meta []string
		if p.Updated != "" {
			meta = append(meta, p.Updated)
		}
		if p.Confidence != "" {
			meta = append(meta, p.Confidence)
		}
		if len(meta) > 0 {
			line += " (" + strings.Join(meta, ", ") + ")"
		}
		b.WriteString(line + "\n")
	}
	if len(ix.GodNodes) > 0 {
		fmt.Fprintf(&b, "\n%s\n\n", i18n.T("cmd.mcp.wiki.god_nodes"))
		for _, g := range ix.GodNodes {
			fmt.Fprintf(&b, "- %s : %s (%s)\n", g.Concept, strings.Join(g.Pages, ", "), g.Criticality)
		}
	}
	fmt.Fprintf(&b, "\n%s\n", i18n.Tf("cmd.mcp.wiki.pending_count", ix.Pending, len(ix.Session)))
	for _, p := range ix.Session {
		b.WriteString(proposalLine(p) + "\n")
	}
	return text(strings.TrimRight(b.String(), "\n")), nil
}

func (h handlers) read(ctx context.Context, params json.RawMessage) (*protocol.ToolResult, error) {
	var in struct {
		Page    string `json:"page"`
		Section string `json:"section"`
	}
	if err := json.Unmarshal(params, &in); err != nil || strings.TrimSpace(in.Page) == "" {
		return nil, errors.New(i18n.T("cmd.mcp.wiki.page_required"))
	}
	id, err := session(ctx)
	if err != nil {
		return nil, err
	}
	p, err := h.b.WikiPage(ctx, id, strings.TrimSpace(in.Page), strings.TrimSpace(in.Section))
	if err != nil {
		return nil, unavailable(err)
	}
	return text(p.Text), nil
}

func (h handlers) propose(ctx context.Context, params json.RawMessage) (*protocol.ToolResult, error) {
	// The author is never read from the call: oh knows who calls.
	var in struct {
		Page    string `json:"page"`
		Section string `json:"section"`
		Content string `json:"content"`
		Tag     string `json:"tag"`
		Source  string `json:"source"`
	}
	if err := json.Unmarshal(params, &in); err != nil || strings.TrimSpace(in.Page) == "" || strings.TrimSpace(in.Content) == "" || strings.TrimSpace(in.Tag) == "" {
		return nil, errors.New(i18n.T("cmd.mcp.wiki.propose_required"))
	}
	id, err := session(ctx)
	if err != nil {
		return nil, err
	}
	p, err := h.b.WikiPropose(ctx, daemon.WikiProposeRequest{ToolSession: id, Page: in.Page, Section: in.Section, Content: in.Content, Tag: in.Tag, Source: in.Source})
	if notRecorded(err) {
		return text(i18n.T("cmd.mcp.wiki.not_recorded")), nil
	}
	var api *daemon.APIError
	if errors.As(err, &api) && (api.Status == http.StatusUnprocessableEntity || api.Status == http.StatusInsufficientStorage || api.Status == http.StatusRequestEntityTooLarge) {
		return nil, errors.New(i18n.Tf("cmd.mcp.wiki.refused", api.Message))
	}
	if err != nil {
		return nil, unavailable(err)
	}
	return text(i18n.Tf("cmd.mcp.wiki.proposed", p.ID, firstText(p.Agent, "?"), p.Page, p.ID)), nil
}

func (h handlers) pending(ctx context.Context, params json.RawMessage) (*protocol.ToolResult, error) {
	var in struct {
		SessionOnly bool `json:"session_only"`
	}
	_ = json.Unmarshal(params, &in)
	id, err := session(ctx)
	if err != nil {
		return nil, err
	}
	list, err := h.b.WikiPending(ctx, id, in.SessionOnly)
	if err != nil {
		return nil, unavailable(err)
	}
	if len(list) == 0 {
		return text(i18n.T("cmd.mcp.wiki.pending_none")), nil
	}
	var b strings.Builder
	for i, p := range list {
		if i == pendingShown {
			fmt.Fprintf(&b, "%s\n", i18n.Tf("cmd.mcp.wiki.pending_more", len(list)-i))
			break
		}
		b.WriteString(proposalLine(p) + "\n")
	}
	return text(strings.TrimRight(b.String(), "\n")), nil
}

func (h handlers) apply(ctx context.Context, params json.RawMessage) (*protocol.ToolResult, error) {
	var in struct {
		IDs []string `json:"ids"`
	}
	if err := json.Unmarshal(params, &in); err != nil || len(in.IDs) == 0 {
		return nil, errors.New(i18n.T("cmd.mcp.wiki.ids_required"))
	}
	id, err := session(ctx)
	if err != nil {
		return nil, err
	}
	res, err := h.b.WikiApply(ctx, id, in.IDs)
	if err != nil {
		return nil, unavailable(err)
	}
	var b strings.Builder
	for _, r := range res {
		if r.Err != "" {
			fmt.Fprintf(&b, "%s\n", i18n.Tf("cmd.mcp.wiki.apply_failed", r.ID, r.Err))
			continue
		}
		fmt.Fprintf(&b, "%s\n", i18n.Tf("cmd.mcp.wiki.applied", r.ID, r.Page))
	}
	return text(strings.TrimRight(b.String(), "\n")), nil
}

func (h handlers) check(ctx context.Context, _ json.RawMessage) (*protocol.ToolResult, error) {
	id, err := session(ctx)
	if err != nil {
		return nil, err
	}
	rep, err := h.b.WikiCheck(ctx, id)
	if err != nil {
		return nil, unavailable(err)
	}
	return text(CheckText(rep, 60)), nil
}

// CheckText renders a check report, at most limit issues (0: all).
func CheckText(rep wikisvc.CheckReport, limit int) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s\n", i18n.Tf("cmd.mcp.wiki.check_summary", rep.Pages, rep.Errors(), len(rep.Issues)-rep.Errors()))
	for i, is := range rep.Issues {
		if limit > 0 && i == limit {
			fmt.Fprintf(&b, "%s\n", i18n.Tf("cmd.mcp.wiki.pending_more", len(rep.Issues)-i))
			break
		}
		loc := is.Page
		if is.Line > 0 {
			loc = fmt.Sprintf("%s:%d", is.Page, is.Line)
		}
		mark := "⚠"
		if is.Severity == wikisvc.SeverityError {
			mark = "✗"
		}
		fix := ""
		if is.Fixable {
			fix = " [--fix]"
		}
		fmt.Fprintf(&b, "%s %s · %s · %s%s\n", mark, loc, is.Kind, is.Message, fix)
	}
	return strings.TrimRight(b.String(), "\n")
}

// proposalLine renders a proposal on one line (its content is a datum
// written by an agent: its first line only, bounded).
func proposalLine(p daemon.WikiProposal) string {
	first, _, _ := strings.Cut(p.Content, "\n")
	if r := []rune(first); len(r) > 100 {
		first = string(r[:99]) + "…"
	}
	page := p.Page
	if p.Section != "" {
		page += " § " + p.Section
	}
	return fmt.Sprintf("- %s · %s · %s · %s · %s · « %s »", p.ID, p.Status, firstText(p.Agent, "?"), page, p.Tag, first)
}

func firstText(ss ...string) string {
	for _, s := range ss {
		if s != "" {
			return s
		}
	}
	return ""
}
