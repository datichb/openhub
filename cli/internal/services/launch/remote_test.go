package launch

import (
	"strings"
	"testing"
)

func TestRemoteTickets(t *testing.T) {
	cases := []struct {
		v    any
		want []string
	}{
		{"bd-1", []string{"bd-1"}},
		{"bd-1, bd-2,", []string{"bd-1", "bd-2"}},
		{[]any{"bd-3", 4}, []string{"bd-3"}},
		{nil, nil},
	}
	for _, c := range cases {
		got := RemoteTickets(map[string]any{"ticket": c.v}, "ticket")
		if strings.Join(got, ",") != strings.Join(c.want, ",") {
			t.Errorf("%v: got %v want %v", c.v, got, c.want)
		}
	}
	if RemoteTickets(map[string]any{"ticket": "x"}, "") != nil {
		t.Error("no ticket input")
	}
}

func TestHTTPSRepoURL(t *testing.T) {
	for in, want := range map[string]string{
		"git@gitlab.com:acme/team-state.git":  "https://gitlab.com/acme/team-state.git",
		"https://gitlab.com/acme/team-state":  "https://gitlab.com/acme/team-state.git",
		"ssh://git@git.corp:2222/acme/ts.git": "https://git.corp/acme/ts.git",
		"":                                    "",
	} {
		if got := HTTPSRepoURL(in); got != want {
			t.Errorf("HTTPSRepoURL(%q) = %q, want %q", in, got, want)
		}
	}
}
