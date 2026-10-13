package wiki

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/daemon"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
	wikisvc "github.com/datichb/openhub/cli/internal/services/wiki"
)

type fakeBackend struct {
	sessions []string
	proposed []daemon.WikiProposeRequest
	applied  []string
	err      error
}

func (f *fakeBackend) WikiIndex(_ context.Context, s string) (daemon.WikiIndex, error) {
	f.sessions = append(f.sessions, s)
	return daemon.WikiIndex{Title: "Atlas", Sections: []string{"Stack critique", "God nodes"},
		Pages:    []daemon.WikiPageInfo{{Path: "index.md", Title: "Atlas", Updated: "2026-10-01", Confidence: "mixed"}, {Path: "business/orders.md", Title: "Commandes"}},
		GodNodes: []wikisvc.GodNode{{Concept: "Commande", Pages: []string{"business/orders.md", "technical/architecture.md"}, Criticality: "Critique"}},
		Pending:  3,
		Session:  []daemon.WikiProposal{{ID: "wp-2", WikiProposal: domain.WikiProposal{Seq: 2, Agent: "reviewer", Page: "index.md", Tag: "DÉDUIT", Status: "pending", Content: "a\nb"}}},
	}, f.err
}

func (f *fakeBackend) WikiPage(_ context.Context, s, page, section string) (daemon.WikiPage, error) {
	f.sessions = append(f.sessions, s+"/"+page+"/"+section)
	return daemon.WikiPage{Page: page, Text: "## " + section}, f.err
}

func (f *fakeBackend) WikiPropose(_ context.Context, req daemon.WikiProposeRequest) (daemon.WikiProposal, error) {
	f.proposed = append(f.proposed, req)
	if f.err != nil {
		return daemon.WikiProposal{}, f.err
	}
	return daemon.WikiProposal{ID: "wp-4", WikiProposal: domain.WikiProposal{Seq: 4, Agent: "developer", Page: "technical/stack.md"}}, nil
}

func (f *fakeBackend) WikiPending(_ context.Context, s string, only bool) ([]daemon.WikiProposal, error) {
	f.sessions = append(f.sessions, s)
	if only {
		return nil, f.err
	}
	return []daemon.WikiProposal{{ID: "wp-1", WikiProposal: domain.WikiProposal{Seq: 1, Agent: "auditor", Page: "index.md", Section: "Zones d'ombre",
		Tag: "INCERTAIN", Status: "pending", Content: strings.Repeat("é", 150)}}}, f.err
}

func (f *fakeBackend) WikiApply(_ context.Context, s string, ids []string) ([]wikisvc.Result, error) {
	f.applied = append(f.applied, ids...)
	return []wikisvc.Result{{ID: "wp-1", Page: "index.md"}, {ID: "wp-9", Err: "not found"}}, f.err
}

func (f *fakeBackend) WikiCheck(_ context.Context, s string) (wikisvc.CheckReport, error) {
	return wikisvc.CheckReport{Pages: 2, Issues: []wikisvc.Issue{
		{Page: "a.md", Line: 3, Kind: wikisvc.IssueBrokenLink, Severity: wikisvc.SeverityError, Message: "lien"},
		{Page: "index.md", Kind: wikisvc.IssueGodNodeMissing, Severity: wikisvc.SeverityWarning, Message: "gn", Fixable: true},
	}}, f.err
}

type rpcResult struct {
	Result struct {
		Tools []struct {
			Name        string         `json:"name"`
			InputSchema map[string]any `json:"inputSchema"`
		} `json:"tools"`
		Content []struct{ Text string } `json:"content"`
		IsError bool                    `json:"isError"`
	} `json:"result"`
}

func call(t *testing.T, b Backend, method, params string) rpcResult {
	t.Helper()
	in := `{"jsonrpc":"2.0","id":1,"method":"` + method + `","params":` + params + "}\n"
	pr, pw := io.Pipe()
	go func() {
		_ = New(b).ServeIO(context.Background(), strings.NewReader(in), pw)
		_ = pw.Close()
	}()
	line, err := bufio.NewReader(pr).ReadBytes('\n')
	require.NoError(t, err)
	var out rpcResult
	require.NoError(t, json.Unmarshal(line, &out), string(line))
	return out
}

const meta = `"_meta":{"ai.tool/sessionID":"ses_child"}`

func TestWikiTools(t *testing.T) {
	i18n.SetLocale("fr")
	out := call(t, &fakeBackend{}, "tools/list", `{}`)
	var names []string
	for _, tl := range out.Result.Tools {
		names = append(names, tl.Name)
	}
	assert.ElementsMatch(t, []string{"index", "read", "propose", "pending", "apply", "check"}, names, "wiki_<tool> for the model")

	b := &fakeBackend{}
	out = call(t, b, "tools/call", `{"name":"index","arguments":{},`+meta+`}`)
	require.False(t, out.Result.IsError, out.Result.Content)
	txt := out.Result.Content[0].Text
	assert.Contains(t, txt, "# Atlas")
	assert.Contains(t, txt, "- business/orders.md : Commandes")
	assert.Contains(t, txt, "- Commande : business/orders.md, technical/architecture.md (Critique)")
	assert.Contains(t, txt, "3 dans le projet, 1 de cette session")
	assert.Contains(t, txt, "- wp-2 · pending · reviewer · index.md · DÉDUIT · « a »", "first line only")
	assert.Equal(t, []string{"ses_child"}, b.sessions)

	out = call(t, b, "tools/call", `{"name":"read","arguments":{"page":"technical/stack.md","section":"Go"},`+meta+`}`)
	require.False(t, out.Result.IsError)
	assert.Equal(t, "## Go", out.Result.Content[0].Text)

	// The author is never read from the call.
	out = call(t, b, "tools/call", `{"name":"propose","arguments":{"page":"technical/stack.md","content":"zod","tag":"DÉDUIT","agent":"documentarian"},`+meta+`}`)
	require.False(t, out.Result.IsError, out.Result.Content)
	assert.Equal(t, []daemon.WikiProposeRequest{{ToolSession: "ses_child", Page: "technical/stack.md", Content: "zod", Tag: "DÉDUIT"}}, b.proposed)
	assert.Contains(t, out.Result.Content[0].Text, "wp-4 en file (auteur developer")
	out = call(t, b, "tools/call", `{"name":"propose","arguments":{"page":"x.md","content":" ","tag":"DÉDUIT"},`+meta+`}`)
	assert.True(t, out.Result.IsError)
	out = call(t, b, "tools/call", `{"name":"propose","arguments":{"page":"x.md","content":"x","tag":"DÉDUIT"}}`)
	assert.True(t, out.Result.IsError, "no session")

	out = call(t, b, "tools/call", `{"name":"pending","arguments":{},`+meta+`}`)
	assert.Contains(t, out.Result.Content[0].Text, "wp-1 · pending · auditor · index.md § Zones d'ombre · INCERTAIN · « "+strings.Repeat("é", 99)+"… »")
	out = call(t, b, "tools/call", `{"name":"pending","arguments":{"session_only":true},`+meta+`}`)
	assert.Equal(t, "Aucune proposition en attente.", out.Result.Content[0].Text)

	out = call(t, b, "tools/call", `{"name":"apply","arguments":{"ids":["wp-1","wp-9"]},`+meta+`}`)
	require.False(t, out.Result.IsError)
	assert.Equal(t, "wp-1 : appliquée dans docs/wiki/index.md\nwp-9 : non appliquée (not found)", out.Result.Content[0].Text)
	out = call(t, b, "tools/call", `{"name":"apply","arguments":{"ids":[]},`+meta+`}`)
	assert.True(t, out.Result.IsError)

	out = call(t, b, "tools/call", `{"name":"check","arguments":{},`+meta+`}`)
	assert.Equal(t, "2 pages, 1 erreur(s), 1 avertissement(s).\n✗ a.md:3 · broken_link · lien\n⚠ index.md · god_node_missing · gn [--fix]", out.Result.Content[0].Text)
}

func TestWikiWithoutDaemon(t *testing.T) {
	i18n.SetLocale("fr")
	b := &fakeBackend{err: daemon.ErrNotRunning}
	out := call(t, b, "tools/call", `{"name":"propose","arguments":{"page":"x.md","content":"x","tag":"DÉDUIT"},`+meta+`}`)
	require.False(t, out.Result.IsError, "not an error: the agent keeps it in its report")
	assert.Contains(t, out.Result.Content[0].Text, "non enregistrée")
	out = call(t, b, "tools/call", `{"name":"index","arguments":{},`+meta+`}`)
	assert.True(t, out.Result.IsError)
	assert.Contains(t, out.Result.Content[0].Text, "n'est pas servi")

	b.err = &daemon.APIError{Status: http.StatusUnprocessableEntity, Message: wikisvc.ErrSessionData.Error()}
	out = call(t, b, "tools/call", `{"name":"propose","arguments":{"page":"x.md","content":"x","tag":"DÉDUIT"},`+meta+`}`)
	assert.True(t, out.Result.IsError)
	assert.Contains(t, out.Result.Content[0].Text, "refusée par oh")
	b.err = &daemon.APIError{Status: http.StatusForbidden, Message: "wiki_apply est réservé au documentarian"}
	out = call(t, b, "tools/call", `{"name":"apply","arguments":{"ids":["wp-1"]},`+meta+`}`)
	assert.True(t, out.Result.IsError)
	assert.Contains(t, out.Result.Content[0].Text, "réservé au documentarian")
}
