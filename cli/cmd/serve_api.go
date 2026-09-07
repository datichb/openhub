package cmd

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/opencode"
)

// ── API Handlers ─────────────────────────────────────────────────────────────

// handleOpenCodeStats handles GET /api/v1/opencode/stats?period=7d|30d|all
func handleOpenCodeStats(a *app.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		period := r.URL.Query().Get("period")
		if period == "" {
			period = "7d"
		}

		db, err := opencode.OpenStatsDB()
		if err != nil || db == nil {
			writeJSON(w, map[string]interface{}{
				"available": false,
				"message":   "Opencode DB non disponible",
			})
			return
		}
		defer db.Close()

		stats, err := opencode.PeriodStats(db, period)
		if err != nil {
			writeError(w, err, http.StatusInternalServerError)
			return
		}

		writeJSON(w, map[string]interface{}{
			"available": true,
			"period":    period,
			"stats":     stats,
		})
	}
}

// handleTeamBoard handles GET /api/v1/team/board?project=X
func handleTeamBoard(a *app.App) http.HandlerFunc {
	type boardColumn struct {
		Name   string      `json:"name"`
		Claims interface{} `json:"claims"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		repo := getTeamRepoSafe(a)
		if repo == nil {
			writeJSON(w, map[string]interface{}{
				"available": false,
				"message":   "Aucune équipe configurée",
			})
			return
		}

		project := r.URL.Query().Get("project")
		claims, err := repo.ListClaims(project)
		if err != nil {
			writeError(w, err, http.StatusInternalServerError)
			return
		}

		// Group claims by status into 5 kanban columns
		columns := []string{"TODO", "IN_PROGRESS", "REVIEW", "BLOCKED", "DONE"}
		grouped := make(map[string][]interface{})
		for _, col := range columns {
			grouped[col] = []interface{}{}
		}

		for _, c := range claims {
			status := strings.ToUpper(c.Status)
			// Normalize common status variants
			switch {
			case status == "IN_PROGRESS" || status == "IN PROGRESS" || status == "PROGRESS":
				status = "IN_PROGRESS"
			case status == "TODO" || status == "PLANNED" || status == "":
				status = "TODO"
			case status == "REVIEW" || status == "IN_REVIEW":
				status = "REVIEW"
			case status == "BLOCKED":
				status = "BLOCKED"
			case status == "DONE" || status == "COMPLETED":
				status = "DONE"
			}
			if _, ok := grouped[status]; !ok {
				status = "TODO" // fallback
			}
			grouped[status] = append(grouped[status], c)
		}

		result := make([]boardColumn, 0, len(columns))
		for _, col := range columns {
			result = append(result, boardColumn{
				Name:   col,
				Claims: grouped[col],
			})
		}

		writeJSON(w, map[string]interface{}{
			"available": true,
			"columns":   result,
		})
	}
}

// handleTeamEvents handles GET /api/v1/team/events?limit=50&project=X
func handleTeamEvents(a *app.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repo := getTeamRepoSafe(a)
		if repo == nil {
			writeJSON(w, map[string]interface{}{
				"available": false,
				"message":   "Aucune équipe configurée",
			})
			return
		}

		limitStr := r.URL.Query().Get("limit")
		limit := 50
		if limitStr != "" {
			if n, err := strconv.Atoi(limitStr); err == nil && n > 0 && n <= 200 {
				limit = n
			}
		}

		project := r.URL.Query().Get("project")
		events, err := repo.ListEventsLimited(project, limit)
		if err != nil {
			writeError(w, err, http.StatusInternalServerError)
			return
		}

		writeJSON(w, map[string]interface{}{
			"available": true,
			"events":   events,
		})
	}
}

// handleTeamMembers handles GET /api/v1/team/members
func handleTeamMembers(a *app.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repo := getTeamRepoSafe(a)
		if repo == nil {
			writeJSON(w, map[string]interface{}{
				"available": false,
				"message":   "Aucune équipe configurée",
			})
			return
		}

		members, err := repo.ListMembers()
		if err != nil {
			writeError(w, err, http.StatusInternalServerError)
			return
		}

		writeJSON(w, map[string]interface{}{
			"available": true,
			"members":  members,
		})
	}
}

// handleCostChart handles GET /api/v1/chart/costs?period=30d
// Returns an SVG sparkline of daily costs.
func handleCostChart(a *app.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		period := r.URL.Query().Get("period")
		if period == "" {
			period = "30d"
		}

		db, err := opencode.OpenStatsDB()
		if err != nil || db == nil {
			w.Header().Set("Content-Type", "image/svg+xml")
			fmt.Fprint(w, emptySVG("DB non disponible"))
			return
		}
		defer db.Close()

		costs, err := opencode.DailyCosts(db, period)
		if err != nil || len(costs) == 0 {
			w.Header().Set("Content-Type", "image/svg+xml")
			fmt.Fprint(w, emptySVG("Aucune donnée"))
			return
		}

		w.Header().Set("Content-Type", "image/svg+xml")
		w.Header().Set("Cache-Control", "no-cache")
		fmt.Fprint(w, costSparklineSVG(costs))
	}
}

// ── SVG Generation ───────────────────────────────────────────────────────────

func costSparklineSVG(costs []opencode.DayCost) string {
	const (
		width   = 300
		height  = 60
		padX    = 5
		padY    = 5
		strokeW = "1.5"
	)

	innerW := float64(width - 2*padX)
	innerH := float64(height - 2*padY)

	// Find min/max for normalization
	var maxCost float64
	for _, dc := range costs {
		if dc.Cost > maxCost {
			maxCost = dc.Cost
		}
	}
	if maxCost == 0 {
		maxCost = 1 // avoid division by zero
	}

	// Build polyline points
	n := len(costs)
	points := make([]string, 0, n)
	for i, dc := range costs {
		x := padX + int(math.Round(float64(i)/float64(n-1)*innerW))
		if n == 1 {
			x = padX + int(innerW/2)
		}
		y := padY + int(math.Round((1-dc.Cost/maxCost)*innerH))
		points = append(points, fmt.Sprintf("%d,%d", x, y))
	}

	// Total cost for display
	var total float64
	for _, dc := range costs {
		total += dc.Cost
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 %d %d" width="%d" height="%d">`, width, height, width, height))
	sb.WriteString(`<rect width="100%" height="100%" fill="transparent"/>`)

	// Polyline
	sb.WriteString(fmt.Sprintf(`<polyline fill="none" stroke="#6366f1" stroke-width="%s" stroke-linecap="round" stroke-linejoin="round" points="%s"/>`,
		strokeW, strings.Join(points, " ")))

	// Fill area under curve
	fillPoints := strings.Join(points, " ") + fmt.Sprintf(" %d,%d %d,%d", width-padX, height-padY, padX, height-padY)
	sb.WriteString(fmt.Sprintf(`<polyline fill="rgba(99,102,241,0.1)" stroke="none" points="%s"/>`, fillPoints))

	// Total label
	sb.WriteString(fmt.Sprintf(`<text x="%d" y="12" font-family="monospace" font-size="10" fill="#888">$%.2f total</text>`, width-padX, total))

	sb.WriteString(`</svg>`)
	return sb.String()
}

func emptySVG(msg string) string {
	return fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 300 60" width="300" height="60">
<text x="150" y="35" font-family="monospace" font-size="11" fill="#888" text-anchor="middle">%s</text>
</svg>`, msg)
}

// ── Sessions with cost info (for the Sessions+Costs panel) ───────────────────

// handleSessionsWithCost handles GET /api/v1/opencode/sessions?limit=20
// Returns recent opencode sessions with cost data.
func handleOpenCodeSessions(a *app.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		limitStr := r.URL.Query().Get("limit")
		limit := 20
		if limitStr != "" {
			if n, err := strconv.Atoi(limitStr); err == nil && n > 0 && n <= 100 {
				limit = n
			}
		}

		db, err := opencode.OpenStatsDB()
		if err != nil || db == nil {
			writeJSON(w, map[string]interface{}{
				"available": false,
				"message":   "Opencode DB non disponible",
			})
			return
		}
		defer db.Close()

		sessions, err := opencode.RecentSessions(db, limit)
		if err != nil {
			writeError(w, err, http.StatusInternalServerError)
			return
		}

		// Serialize for frontend
		type sessionView struct {
			ID        string  `json:"id"`
			Title     string  `json:"title"`
			Model     string  `json:"model"`
			Cost      float64 `json:"cost"`
			TokensIn  int64   `json:"tokens_in"`
			TokensOut int64   `json:"tokens_out"`
			CreatedAt string  `json:"created_at"`
		}

		views := make([]sessionView, 0, len(sessions))
		for _, s := range sessions {
			views = append(views, sessionView{
				ID:        s.ID,
				Title:     s.Title,
				Model:     s.Model,
				Cost:      s.Cost,
				TokensIn:  s.TokensInput,
				TokensOut: s.TokensOutput,
				CreatedAt: s.TimeCreated.Format("2006-01-02 15:04"),
			})
		}

		data, _ := json.Marshal(map[string]interface{}{
			"available": true,
			"sessions":  views,
		})
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Write(data)
	}
}
