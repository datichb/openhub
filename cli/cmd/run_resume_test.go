package cmd

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/storage/sqlite"
)

func TestResumeIntent(t *testing.T) {
	st, err := sqlite.Open(filepath.Join(t.TempDir(), "oh.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	prefs := sqlite.NewPreferenceStore(st)
	a := (&app.App{}).WithPreferences(prefs, prefs)
	ctx := context.Background()

	if _, ok := loadResumeIntent(ctx, a, "p1", "ses_1"); ok {
		t.Fatal("no intent yet")
	}
	saveResumeIntent(ctx, a, "p1", "ses_1", resumeIntent{Workflow: "feature", Inputs: map[string]string{"request": "export CSV"}, Mode: "manuel"})
	got, ok := loadResumeIntent(ctx, a, "p1", "ses_1")
	if !ok || got.Workflow != "feature" || got.Inputs["request"] != "export CSV" || got.Mode != "manuel" {
		t.Fatalf("intent = %+v (%v)", got, ok)
	}
	if _, ok := loadResumeIntent(ctx, a, "p2", "ses_1"); ok {
		t.Fatal("intents are per project")
	}
	if cmd := resumeCommand(*got); cmd != "oh run feature -i request='export CSV'" {
		t.Fatalf("command = %q", cmd)
	}
	forgetResumeIntent(ctx, a, "p1", "ses_1")
	if _, ok := loadResumeIntent(ctx, a, "p1", "ses_1"); ok {
		t.Fatal("intent forgotten")
	}
}
