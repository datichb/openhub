package teamstate

import (
	"fmt"
	"strings"
)

// RenderTemplateBrief generates a human-readable Markdown summary from raw brief data.
// This is generated WITHOUT any LLM call — pure Go template.
func RenderTemplateBrief(brief *TakeoverBrief) string {
	var b strings.Builder

	// Header
	b.WriteString(fmt.Sprintf("# Takeover Brief: %s\n\n", brief.Meta.TicketID))
	b.WriteString(fmt.Sprintf("**Transfere de** %s → %s | **Date** %s | **Raison** %s\n\n",
		brief.Meta.TransferredFrom,
		brief.Meta.TransferredTo,
		brief.Meta.TransferDate.Format("2006-01-02"),
		brief.Meta.Reason))

	if brief.Meta.StaleDays > 0 {
		b.WriteString(fmt.Sprintf("> Ticket inactif depuis %d jours\n\n", brief.Meta.StaleDays))
	}

	// Activity
	b.WriteString("## Activite\n\n")
	if brief.Activity.SessionsCount > 0 {
		b.WriteString(fmt.Sprintf("- %d session(s)", brief.Activity.SessionsCount))
		if !brief.Activity.FirstSession.IsZero() && !brief.Activity.LastSession.IsZero() {
			b.WriteString(fmt.Sprintf(" (%s → %s)",
				brief.Activity.FirstSession.Format("02 Jan"),
				brief.Activity.LastSession.Format("02 Jan")))
		}
		b.WriteString("\n")
		if brief.Activity.TotalDurationMinutes > 0 {
			hours := brief.Activity.TotalDurationMinutes / 60
			mins := brief.Activity.TotalDurationMinutes % 60
			if hours > 0 {
				b.WriteString(fmt.Sprintf("- Duree totale : ~%dh%02dm\n", hours, mins))
			} else {
				b.WriteString(fmt.Sprintf("- Duree totale : ~%dm\n", mins))
			}
		}
	} else {
		b.WriteString("- Aucune session enregistree\n")
	}

	// Git
	if brief.Git.Branch != "" || brief.Git.CommitsCount > 0 {
		b.WriteString(fmt.Sprintf("- %d commit(s)", brief.Git.CommitsCount))
		if brief.Git.Branch != "" {
			b.WriteString(fmt.Sprintf(" sur branche `%s`", brief.Git.Branch))
		}
		b.WriteString("\n")
	}
	b.WriteString("\n")

	// Files
	if len(brief.Git.FilesModified) > 0 || len(brief.Git.FilesCreated) > 0 {
		b.WriteString("## Fichiers principaux\n\n")
		b.WriteString("| Fichier | Action | Lignes |\n")
		b.WriteString("|---------|--------|--------|\n")

		for _, f := range brief.Git.FilesModified {
			b.WriteString(fmt.Sprintf("| `%s` | modifie | +%d/-%d |\n",
				f.Path, f.Additions, f.Deletions))
		}
		for _, f := range brief.Git.FilesCreated {
			b.WriteString(fmt.Sprintf("| `%s` | cree | +%d |\n",
				f.Path, f.Additions))
		}
		b.WriteString("\n")
	}

	// Last state
	if brief.Git.LastCommitMessage != "" {
		b.WriteString("## Dernier etat connu\n\n")
		b.WriteString(fmt.Sprintf("- Dernier commit : \"%s\"", brief.Git.LastCommitMessage))
		if !brief.Git.LastCommitDate.IsZero() {
			b.WriteString(fmt.Sprintf(" (%s)", brief.Git.LastCommitDate.Format("02 Jan 15:04")))
		}
		b.WriteString("\n")
		if len(brief.Events) > 0 {
			b.WriteString(fmt.Sprintf("- Derniere session : \"%s\"\n", brief.Events[0].Summary))
		}
		b.WriteString("\n")
	}

	// Event history
	if len(brief.Events) > 0 {
		b.WriteString("## Historique sessions\n\n")
		// Events are newest-first, reverse for chronological display
		for i := len(brief.Events) - 1; i >= 0; i-- {
			e := brief.Events[i]
			b.WriteString(fmt.Sprintf("%d. **%s** — %s\n",
				len(brief.Events)-i,
				e.Timestamp.Format("02 Jan"),
				e.Summary))
		}
		b.WriteString("\n")
	}

	// Footer
	b.WriteString("---\n")
	b.WriteString("*Brief genere automatiquement par template. Utiliser `oh takeover-brief enrich ")
	b.WriteString(brief.Meta.TicketID)
	b.WriteString("` pour une version enrichie par IA.*\n")

	return b.String()
}
