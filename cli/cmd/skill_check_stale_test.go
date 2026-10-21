package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/datichb/openhub/cli/internal/bundle"
	"github.com/datichb/openhub/cli/internal/i18n/i18ntest"
)

func staleHub(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for rel, content := range files {
		p := filepath.Join(dir, "skills", rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func stackMD(name, extra string) string {
	return "---\nname: " + name + "\ndescription: Utiliser pour " + name + "\n" + extra + "---\n# " + name + "\n"
}

func TestRunSkillStale(t *testing.T) {
	i18ntest.Set(t, "fr")
	today := time.Date(2026, 10, 13, 9, 0, 0, 0, time.UTC)
	hub := staleHub(t, map[string]string{
		"developer/stacks/dev-standards-react.md":  stackMD("dev-standards-react", "versions:\n  react: \"19.3\"\n  react-compiler: \"1.0\"\nreviewed: 2026-05-01\n"),
		"developer/stacks/dev-standards-golang.md": stackMD("dev-standards-golang", "versions: {go: \"1.27\"}\nreviewed: 2026-10-12\n"),
		"developer/stacks/dev-standards-jest.md":   stackMD("dev-standards-jest", ""),
	})

	var out bytes.Buffer
	if err := runSkillStale(&out, hub, today, staleOptions{maxAge: "90d"}, false); err != nil {
		t.Fatalf("without --strict: %v", err)
	}
	text := out.String()
	for _, want := range []string{
		"3 vérifiées, revue tous les 90 jours au plus, aujourd'hui 2026-10-13",
		"developer/stacks/dev-standards-jest : métadonnées de fraîcheur absentes ou inutilisables (versions, reviewed)",
		"developer/stacks/dev-standards-react : revue le 2026-05-01 (il y a 165 jours), couvre react 19.3, react-compiler 1.0",
		"2 skill(s) à revoir : oh run skill-refresh",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("output lacks %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, "dev-standards-golang") {
		t.Errorf("a fresh skill is not listed:\n%s", text)
	}

	out.Reset()
	err := runSkillStale(&out, hub, today, staleOptions{maxAge: "90d", strict: true}, true)
	var staleErr *staleSkillsError
	if !errors.As(err, &staleErr) || staleErr.n != 2 {
		t.Fatalf("--strict: want 2 skills to review, got %v", err)
	}
	if err.Error() != "2 skill(s) de stack à revoir" {
		t.Errorf("message = %q", err.Error())
	}
	var rep bundle.FreshnessReport
	if err := json.Unmarshal(out.Bytes(), &rep); err != nil {
		t.Fatalf("--json: %v\n%s", err, out.String())
	}
	if rep.MaxAgeDays != 90 || rep.Checked != 3 || len(rep.Skills) != 2 || rep.Skills[0].Status != bundle.FreshnessMissing || rep.Skills[1].AgeDays != 165 {
		t.Errorf("report = %+v", rep)
	}

	out.Reset()
	if err := runSkillStale(&out, hub, today, staleOptions{maxAge: "26w", strict: true}, false); !errors.As(err, &staleErr) || staleErr.n != 1 {
		t.Fatalf("182 days: only the skill without metadata, got %v", err)
	}

	out.Reset()
	if err := runSkillStale(&out, hub, today, staleOptions{maxAge: "3 mois"}, false); err == nil {
		t.Error("an unreadable max age is an error")
	}
}

func TestRunSkillStaleAllFresh(t *testing.T) {
	i18ntest.Set(t, "en")
	hub := staleHub(t, map[string]string{
		"developer/stacks/dev-standards-golang.md": stackMD("dev-standards-golang", "versions: {go: \"1.27\"}\nreviewed: 2026-10-12\n"),
		"shared/other.md":                          stackMD("other", ""),
	})
	var out bytes.Buffer
	if err := runSkillStale(&out, hub, time.Date(2026, 10, 13, 0, 0, 0, 0, time.UTC), staleOptions{maxAge: "90", strict: true}, false); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "Stack skills: all 1 reviewed within the last 90 days.") {
		t.Errorf("output = %q", out.String())
	}
}
