// Package modelcheck validates the identifiers written by the model
// cascade commands (oh config model, Models view): agent, family and model.
package modelcheck

import (
	"errors"
	"sort"
	"strings"

	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/sessionspec"
)

// maxSuggestions is the number of close names an error proposes.
const maxSuggestions = 3

// Catalog is the effective agent catalogue (hub, team, project): the id of
// each agent and its family (folder of the agent).
type Catalog struct {
	agents map[string]string
}

// NewCatalog builds a catalogue from agent id → family.
func NewCatalog(agents map[string]string) Catalog { return Catalog{agents: agents} }

// Empty reports whether the catalogue knows no agent (unreadable hub): the
// identifiers cannot be checked then.
func (c Catalog) Empty() bool { return len(c.agents) == 0 }

// Agents returns the sorted agent ids.
func (c Catalog) Agents() []string {
	out := make([]string, 0, len(c.agents))
	for id := range c.agents {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

// Families returns the sorted families of the agents.
func (c Catalog) Families() []string {
	seen := map[string]bool{}
	var out []string
	for _, f := range c.agents {
		if f != "" && !seen[f] {
			seen[f] = true
			out = append(out, f)
		}
	}
	sort.Strings(out)
	return out
}

// CheckAgent refuses an agent id absent from the catalogue.
func (c Catalog) CheckAgent(id string) error {
	if _, ok := c.agents[id]; ok {
		return nil
	}
	return withSuggestions(i18n.Tf("modelcheck.unknown_agent", id), id, c.Agents())
}

// CheckFamily refuses a family that no agent of the catalogue belongs to.
func (c Catalog) CheckFamily(name string) error {
	families := c.Families()
	for _, f := range families {
		if f == name {
			return nil
		}
	}
	return withSuggestions(i18n.Tf("modelcheck.unknown_family", name, strings.Join(families, ", ")), name, families)
}

// CheckModel refuses a model that is not written provider/model[#variant].
func CheckModel(model string) error {
	ref := sessionspec.ParseModelRef(model)
	if ref.Provider == "" || ref.Model == "" || strings.ContainsAny(model, " \t\r\n") ||
		(strings.Contains(model, "#") && ref.Variant == "") {
		return errors.New(i18n.Tf("modelcheck.bad_model", model))
	}
	return nil
}

func withSuggestions(msg, name string, candidates []string) error {
	if near := Near(name, candidates); len(near) > 0 {
		msg += " " + i18n.Tf("modelcheck.did_you_mean", strings.Join(near, ", "))
	}
	return errors.New(msg)
}

// Near returns the candidates close to name (substring or small edit
// distance), the closest first.
func Near(name string, candidates []string) []string {
	name = strings.ToLower(name)
	type scored struct {
		name  string
		score int
	}
	var found []scored
	limit := max(2, len(name)/3)
	for _, c := range candidates {
		lc := strings.ToLower(c)
		d := editDistance(name, lc)
		switch {
		case d <= limit:
			found = append(found, scored{c, d})
		case name != "" && (strings.Contains(lc, name) || strings.Contains(name, lc)):
			found = append(found, scored{c, limit + 1})
		}
	}
	sort.SliceStable(found, func(i, j int) bool { return found[i].score < found[j].score })
	var out []string
	for i := 0; i < len(found) && i < maxSuggestions; i++ {
		out = append(out, found[i].name)
	}
	return out
}

// editDistance is the Levenshtein distance between a and b.
func editDistance(a, b string) int {
	ra, rb := []rune(a), []rune(b)
	prev := make([]int, len(rb)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(ra); i++ {
		cur := make([]int, len(rb)+1)
		cur[0] = i
		for j := 1; j <= len(rb); j++ {
			cost := 1
			if ra[i-1] == rb[j-1] {
				cost = 0
			}
			cur[j] = min(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
		}
		prev = cur
	}
	return prev[len(rb)]
}
