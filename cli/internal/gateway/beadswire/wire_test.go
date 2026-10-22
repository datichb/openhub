package beadswire

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The fake bd in an image and the daemon that serves it may come from two
// different oh versions: the field names are the contract. This pins them,
// together with what is left out when empty (A-09).
func TestWireFieldNames(t *testing.T) {
	tests := []struct {
		name string
		v    any
		want string
	}{
		{"exec request", ExecRequest{Argv: []string{"show", "bd-1"}, Cwd: "/work", Stdin: []byte("hi"), GitHook: true, HookToken: "tok"},
			`{"argv":["show","bd-1"],"cwd":"/work","stdin":"aGk=","git_hook":true,"hook_token":"tok"}`},
		{"exec request without the optional fields", ExecRequest{Argv: []string{"ready"}, Cwd: "/work"},
			`{"argv":["ready"],"cwd":"/work"}`},
		{"exec response", ExecResponse{Stdout: []byte("out"), Stderr: []byte("err"), ExitCode: 2, Error: "boom"},
			`{"stdout":"b3V0","stderr":"ZXJy","exit_code":2,"error":"boom"}`},
		{"a success keeps the exit code", ExecResponse{Stdout: []byte("ok")},
			`{"stdout":"b2s=","exit_code":0}`},
		{"hook token request", HookTokenRequest{Token: "ohg_x", Argv: []string{"hooks", "run", "post-merge"}},
			`{"token":"ohg_x","argv":["hooks","run","post-merge"]}`},
		{"hook token response", HookTokenResponse{HookToken: "h1"}, `{"hook_token":"h1"}`},
		{"journal entry", JournalEntry{Time: time.Date(2026, 10, 22, 9, 30, 0, 0, time.UTC), Argv: []string{"create", "T"}, Cwd: "/w", Seq: 3, Stdin: []byte("d"), Placeholder: "tmp-1"},
			`{"time":"2026-10-22T09:30:00Z","argv":["create","T"],"cwd":"/w","seq":3,"stdin":"ZA==","placeholder":"tmp-1"}`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := json.Marshal(tc.v)
			require.NoError(t, err)
			assert.JSONEq(t, tc.want, string(got))
		})
	}
}

// What one side writes the other side reads back unchanged, including binary
// standard input and arguments with spaces and quotes.
func TestWireRoundTrip(t *testing.T) {
	stdin := []byte{0x00, 0xff, '\n', '"', 'é'}
	req := ExecRequest{Argv: []string{"update", "bd-9", "--notes", `a "quoted" note`, "-"}, Cwd: "/work/sp ace", Stdin: stdin, GitHook: true, HookToken: "h"}
	var gotReq ExecRequest
	roundTrip(t, req, &gotReq)
	assert.Equal(t, req, gotReq)

	resp := ExecResponse{Stdout: stdin, Stderr: []byte("warn"), ExitCode: 3}
	var gotResp ExecResponse
	roundTrip(t, resp, &gotResp)
	assert.Equal(t, resp, gotResp)

	entry := JournalEntry{Time: time.Date(2026, 10, 22, 9, 30, 0, 123, time.UTC), Argv: []string{"close", "bd-1"}, Cwd: "/w", Seq: 7, Stdin: stdin, Placeholder: "tmp"}
	var gotEntry JournalEntry
	roundTrip(t, entry, &gotEntry)
	assert.True(t, entry.Time.Equal(gotEntry.Time))
	gotEntry.Time = entry.Time
	assert.Equal(t, entry, gotEntry)
}

// A journal written by an older fake bd (before phase 5: no seq, stdin or
// placeholder) and a request with a field added later are still read.
func TestWireToleratesOlderAndNewerPeers(t *testing.T) {
	var old JournalEntry
	require.NoError(t, json.Unmarshal([]byte(`{"time":"2026-10-01T08:00:00Z","argv":["close","bd-2"],"cwd":"/w"}`), &old))
	assert.Equal(t, []string{"close", "bd-2"}, old.Argv)
	assert.Zero(t, old.Seq)
	assert.Nil(t, old.Stdin)
	assert.Empty(t, old.Placeholder)

	var req ExecRequest
	dec := json.NewDecoder(bytes.NewReader([]byte(`{"argv":["list"],"cwd":"/w","from_the_future":{"x":1}}`)))
	require.NoError(t, dec.Decode(&req), "a decoder without DisallowUnknownFields ignores a new field")
	assert.Equal(t, []string{"list"}, req.Argv)
	assert.False(t, req.GitHook)
	assert.Empty(t, req.HookToken, "no hook token: the command is checked as any other")
}

func roundTrip(t *testing.T, in, out any) {
	t.Helper()
	b, err := json.Marshal(in)
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(b, out))
}
