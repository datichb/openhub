package cmd

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/datichb/openhub/cli/internal/daemon/api"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n/i18ntest"
	sessionsvc "github.com/datichb/openhub/cli/internal/services/session"
	"github.com/datichb/openhub/cli/internal/storage/sqlite"
	"github.com/datichb/openhub/cli/internal/teamstate"
)

// A12: cobra errors, unknown sessions and the daemon refusal are translated.
func TestLocalizeError(t *testing.T) {
	i18ntest.Set(t, "fr")
	cases := map[string]error{
		"option courte inconnue : 'j' dans -j":   errors.New("unknown shorthand flag: 'j' in -j"),
		"option inconnue : --nope":               errors.New("unknown flag: --nope"),
		`commande inconnue "session approve"`:    errors.New("unknown command \"session approve\" for \"oh\"\n\nDid you mean this?\n\tsession\n"),
		"Vouliez-vous dire : session ?":          errors.New("unknown command \"sesion\" for \"oh\"\n\nDid you mean this?\n\tsession\n"),
		"1 argument(s) attendu(s), 0 reçu(s)":    errors.New("accepts 1 arg(s), received 0"),
		"Des serveurs d'outil sont actifs (2)":   &api.APIError{Method: "POST", Path: "/shutdown", Status: 409, Message: "tool servers alive: 2 (use force)"},
		"Des serveurs d'outil sont actifs :":     &api.APIError{Method: "POST", Path: "/shutdown", Status: 409, Message: "sessions are running (use force)"},
		"lancez cette commande dans un terminal": errors.New("open /dev/tty: device not configured"),
		// N15
		"occupé par pid 42": fmt.Errorf("pull: %w", &teamstate.BusyError{Path: "/t", Holder: "pid 42 (oh tui) since 10:00:00"}),
		// N14
		"version plus récente d'oh (migrations v999":                                     fmt.Errorf("running migrations: %w", &sqlite.SchemaNewerError{Unknown: []int{999}, Known: 42}),
		"oh repair --restore /b/oh.db.v41.bak":                                           &sqlite.MigrationError{Version: 42, Backup: "/b/oh.db.v41.bak", Err: errors.New("boom")},
		"La migration v42 de la base de données a échoué : boom\nDiagnostic : oh doctor": &sqlite.MigrationError{Version: 42, Err: errors.New("boom")},
	}
	for want, err := range cases {
		if got := localizeError(err); !strings.Contains(got, want) {
			t.Errorf("localizeError(%q) = %q, want %q", err, got, want)
		}
	}
	err := sessionRefError("zzz", errors.Join(errors.New("session zzz"), domain.ErrNotFound))
	if err == nil || !strings.Contains(err.Error(), "Session introuvable : zzz") {
		t.Fatalf("sessionRefError = %v", err)
	}
	if err := sessionRefError("ses", sessionsvc.ErrAmbiguous); err == nil || !strings.Contains(err.Error(), "ambiguë") {
		t.Fatalf("sessionRefError = %v", err)
	}
	if got := localizeError(errors.New("plain")); got != "plain" {
		t.Fatalf("other errors unchanged: %q", got)
	}
}
