package beads

import "testing"

func TestParseExternalRef(t *testing.T) {
	tests := []struct {
		input    string
		wantProv string
		wantID   string
	}{
		{"gitlab-693", "gitlab", "693"},
		{"gitlab-17", "gitlab", "17"},
		{"jira-MYAPP-42", "jira", "MYAPP-42"},
		{"jira-SRU-123", "jira", "SRU-123"},
		{"github-456", "github", "456"},
		{"", "", ""},
		{"unknown-123", "", ""},
		{"gitlab-", "", ""},   // no ID after prefix
		{"gitlab", "", ""},    // no dash
		{"GITLAB-42", "", ""}, // case-sensitive
	}
	for _, tt := range tests {
		prov, id := ParseExternalRef(tt.input)
		if prov != tt.wantProv || id != tt.wantID {
			t.Errorf("ParseExternalRef(%q) = (%q, %q), want (%q, %q)",
				tt.input, prov, id, tt.wantProv, tt.wantID)
		}
	}
}

func TestBuildExternalRef(t *testing.T) {
	tests := []struct {
		provider, id string
		want         string
	}{
		{"gitlab", "693", "gitlab-693"},
		{"jira", "MYAPP-42", "jira-MYAPP-42"},
		{"github", "123", "github-123"},
		{"", "123", ""},
		{"gitlab", "", ""},
	}
	for _, tt := range tests {
		got := BuildExternalRef(tt.provider, tt.id)
		if got != tt.want {
			t.Errorf("BuildExternalRef(%q, %q) = %q, want %q",
				tt.provider, tt.id, got, tt.want)
		}
	}
}

func TestExtractRefFromTitle(t *testing.T) {
	tests := []struct {
		title string
		want  string
	}{
		{"[gitlab-693] Fix login bug", "gitlab-693"},
		{"[#42] Some task", "#42"},
		{"[jira-MYAPP-42] Refactor auth", "jira-MYAPP-42"},
		{"No ref in this title", ""},
		{"", ""},
		{"[] empty brackets", ""},
		{"[a-b] minimal", "a-b"},
	}
	for _, tt := range tests {
		got := ExtractRefFromTitle(tt.title)
		if got != tt.want {
			t.Errorf("ExtractRefFromTitle(%q) = %q, want %q",
				tt.title, got, tt.want)
		}
	}
}

func TestExternalRefForTicket(t *testing.T) {
	// Structured field takes precedence
	ticket := Ticket{ExternalRef: "gitlab-693", Title: "[gitlab-999] Wrong ref in title"}
	if got := ExternalRefForTicket(ticket); got != "gitlab-693" {
		t.Errorf("ExternalRefForTicket with ExternalRef = %q, want %q", got, "gitlab-693")
	}

	// Fallback to title parsing
	ticket2 := Ticket{Title: "[gitlab-42] Fix bug"}
	if got := ExternalRefForTicket(ticket2); got != "gitlab-42" {
		t.Errorf("ExternalRefForTicket with title fallback = %q, want %q", got, "gitlab-42")
	}

	// No ref anywhere
	ticket3 := Ticket{Title: "Plain title"}
	if got := ExternalRefForTicket(ticket3); got != "" {
		t.Errorf("ExternalRefForTicket with no ref = %q, want %q", got, "")
	}
}
