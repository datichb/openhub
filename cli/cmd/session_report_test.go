package cmd

import (
	"bytes"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n/i18ntest"
	"github.com/datichb/openhub/cli/internal/services/report"
)

func TestPrintSessionReports(t *testing.T) {
	i18ntest.Set(t, "fr")
	at := time.Date(2026, 10, 13, 9, 5, 0, 0, time.Local)
	var b bytes.Buffer
	printSessionReports(&b, "ses_a", []domain.Report{
		{Seq: 1, Agent: "developer", Title: "Fait\x1b[2J", Bytes: 820, CreatedAt: at, Sections: []string{"Changements par fichier", "Tests"}},
		{Seq: 2, Title: "Review", Bytes: 2150, CreatedAt: at},
	})
	out := b.String()
	for _, want := range []string{"ID", "AUTEUR", "rp-1", "developer", "820 o", "09:05", "Changements par fichier · Tests", "rp-2", "?", "2.1 Ko", "oh session report ses_a <rp-id>"} {
		assert.Contains(t, out, want)
	}
	assert.NotContains(t, out, "\x1b")

	b.Reset()
	printSessionReports(&b, "ses_a", nil)
	assert.Contains(t, b.String(), "Aucun rapport")

	assert.Contains(t, reportError("x", report.ErrBadID).Error(), "rp-<numéro>")
	assert.Contains(t, reportError("rp-9", domain.ErrNotFound).Error(), "rp-9 introuvable")
	assert.Contains(t, reportError("rp-1", report.ErrAltered).Error(), "modifié hors d'oh")
}
