package gitlabapi

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseRef(t *testing.T) {
	tests := []struct {
		in   string
		want Ref
		ok   bool
	}{
		{"#623", Ref{Kind: RefIssue, IID: 623, Text: "#623"}, true},
		{" #623 ", Ref{Kind: RefIssue, IID: 623, Text: "#623"}, true},
		{"!12", Ref{Kind: RefMR, IID: 12, Text: "!12"}, true},
		{"https://gitlab.example.com/group/app/-/issues/623",
			Ref{Kind: RefIssue, IID: 623, Project: "group/app", Host: "gitlab.example.com", Text: "https://gitlab.example.com/group/app/-/issues/623"}, true},
		{"https://gitlab.com/group/sub/app/-/merge_requests/70#note_5",
			Ref{Kind: RefMR, IID: 70, Project: "group/sub/app", Host: "gitlab.com", Text: "https://gitlab.com/group/sub/app/-/merge_requests/70"}, true},
		{"https://gitlab.com/group/app/-/work_items/9?foo=1",
			Ref{Kind: RefIssue, IID: 9, Project: "group/app", Host: "gitlab.com", Text: "https://gitlab.com/group/app/-/work_items/9"}, true},
		// Not GitLab references.
		{"623", Ref{}, false},
		{"bd-12", Ref{}, false},
		{"proj-a1b.3", Ref{}, false},
		{"#", Ref{}, false},
		{"#0", Ref{}, false},
		{"#12a", Ref{}, false},
		{"https://github.com/o/r/issues/12", Ref{}, false},
		{"https://github.com/o/r/pull/12", Ref{}, false},
		{"https://gitlab.com/group/app/-/issues", Ref{}, false},
		{"#1 #2", Ref{}, false},
		{"corrige #623", Ref{}, false},
		{"", Ref{}, false},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			got, ok := ParseRef(tt.in)
			assert.Equal(t, tt.ok, ok)
			if tt.ok {
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestFindRefs(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want []string
	}{
		{"none", "ajoute un export CSV", nil},
		{"issue in a sentence", "implémente le ticket #623 pour la compta", []string{"#623"}},
		{"issue at the end", "voir #623.", []string{"#623"}},
		{"in parentheses", "export (#623, !70)", []string{"#623", "!70"}},
		{"duplicates once", "#12 puis encore #12", []string{"#12"}},
		{"order kept", "!5 et #3", []string{"!5", "#3"}},
		{"url and short form", "https://gitlab.example.com/g/app/-/issues/623 (#623)",
			[]string{"https://gitlab.example.com/g/app/-/issues/623", "#623"}},
		{"markdown heading is not a reference", "# Titre\n## Sous-titre", nil},
		{"anchor of a word is not a reference", "page#12", nil},
		{"a letter after the digits", "#6a", nil},
		{"hex color of letters", "couleur #fff", nil},
		{"github url", "https://github.com/o/r/issues/12", nil},
		{"fragment of a url is not a reference", "https://gitlab.com/g/app/-/merge_requests/70#5 et !8",
			[]string{"https://gitlab.com/g/app/-/merge_requests/70", "!8"}},
		{"line start", "#623 à lire", []string{"#623"}},
		{"after a colon", "ticket:#623", []string{"#623"}},
		{"too many digits", "#1234567", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got []string
			for _, r := range FindRefs(tt.in) {
				got = append(got, r.Text)
			}
			assert.Equal(t, tt.want, got)
		})
	}
}
