package cmd

import (
	"fmt"
	"strings"

	"github.com/datichb/openhub/cli/internal/bundle"
	"github.com/datichb/openhub/cli/internal/i18n"
)

// maxRecapModelGroups is the number of other models the second row of the
// recap names; the others are counted.
const maxRecapModelGroups = 2

// recapModelRows are the model rows of the launch recap: the model the
// session really uses (the entry agent's, else the default of the bundle),
// then, when other agents of the bundle run another model, one row that
// groups them by model. At most two rows; none when the bundle names no model.
func recapModelRows(r *bundle.Report) [][2]string {
	modelOf := func(a bundle.AgentReport) string {
		if a.Model != "" {
			return a.Model
		}
		return r.DefaultModel
	}
	main := r.DefaultModel
	for _, a := range r.Agents {
		if a.Entry {
			main = modelOf(a)
			break
		}
	}
	if main == "" {
		return nil
	}
	rows := [][2]string{{i18n.T("m1.recap_model"), main}}

	var order []string
	groups := map[string][]string{}
	for _, a := range r.Agents {
		if m := modelOf(a); !a.Entry && m != "" && m != main {
			if _, ok := groups[m]; !ok {
				order = append(order, m)
			}
			groups[m] = append(groups[m], a.ID)
		}
	}
	if len(order) == 0 {
		return rows
	}
	parts := make([]string, 0, maxRecapModelGroups+1)
	for i, m := range order {
		if i == maxRecapModelGroups {
			parts = append(parts, i18n.Tf("m1.recap_model_more", len(order)-maxRecapModelGroups))
			break
		}
		parts = append(parts, fmt.Sprintf("%s (%s)", m, strings.Join(groups[m], ", ")))
	}
	return append(rows, [2]string{i18n.T("m1.recap_model_others"), strings.Join(parts, " · ")})
}
