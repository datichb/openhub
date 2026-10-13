package wiki

import (
	"context"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/datichb/openhub/cli/internal/domain"
)

// Metrics of the wiki (modernisation LD-I), computed by oh from the pages
// and the queue, shown by the TUI (project wiki view) and `oh wiki stats`:
//
//   - coverage of the domains: the domains are the pages business/<d>.md
//     and the business pages the index maps (its domains section); a
//     domain is covered when its page exists and holds at least one
//     tagged enrichment;
//   - median age of the pages: days since their `updated` date;
//   - share of INCERTAIN among the tagged enrichments;
//   - proposals waiting in the queue (pending, accepted not applied yet).

// Metrics are the figures of a wiki.
type Metrics struct {
	Pages int `json:"pages"`
	// ByKind counts the pages by kind (layout.go).
	ByKind map[string]int `json:"by_kind"`

	Domains        int      `json:"domains"`
	DomainsCovered int      `json:"domains_covered"`
	Uncovered      []string `json:"uncovered,omitempty"` // relative to docs/wiki/

	// MedianAgeDays is the median age of the dated pages (Dated of them).
	MedianAgeDays int    `json:"median_age_days"`
	Dated         int    `json:"dated"`
	Oldest        string `json:"oldest,omitempty"`
	OldestDays    int    `json:"oldest_days,omitempty"`

	Tags map[string]int `json:"tags"` // CONFIRMÉ, DÉDUIT, INCERTAIN
	// Pending and Accepted are the open proposals of the project (-1:
	// unknown, no queue).
	Pending  int `json:"pending"`
	Accepted int `json:"accepted"`

	// Synced is the commit of the last doc-sync (index.md `synced:`).
	Synced string `json:"synced,omitempty"`
}

// Coverage is the share of covered domains (0 to 1; 0 without domain).
func (m Metrics) Coverage() float64 {
	if m.Domains == 0 {
		return 0
	}
	return float64(m.DomainsCovered) / float64(m.Domains)
}

// TaggedEnrichments is the number of tagged enrichments.
func (m Metrics) TaggedEnrichments() int {
	n := 0
	for _, v := range m.Tags {
		n += v
	}
	return n
}

// UncertainShare is the share of INCERTAIN among the tagged enrichments.
func (m Metrics) UncertainShare() float64 {
	n := m.TaggedEnrichments()
	if n == 0 {
		return 0
	}
	return float64(m.Tags[TagUncertain]) / float64(n)
}

// Metrics computes the figures of a loaded wiki (the queue aside).
func (w *Wiki) Metrics(now time.Time) Metrics {
	m := Metrics{Pages: len(w.Pages), ByKind: map[string]int{}, Tags: map[string]int{}, Pending: -1, Accepted: -1}
	today := day(now)
	var ages []int
	domains := map[string]bool{}
	for _, p := range w.Pages {
		m.ByKind[PageKind(p.Path)]++
		if domainPage(p.Path) {
			domains[p.Path] = true
		}
		if t, ok := p.Frontmatter.UpdatedAt(); ok {
			age := int(today.Sub(t).Hours() / 24)
			if age < 0 {
				age = 0
			}
			ages = append(ages, age)
			if age > m.OldestDays || m.Oldest == "" {
				m.Oldest, m.OldestDays = p.Path, age
			}
		}
		for tag, n := range pageTags(p.Text) {
			m.Tags[tag] += n
		}
	}
	if idx, ok := w.Page(IndexPage); ok {
		m.Synced = strings.TrimSpace(idx.Frontmatter.Synced)
		for _, d := range mappedDomains(idx) {
			domains[d] = true
		}
	}
	for d := range domains {
		m.Domains++
		p, ok := w.Page(d)
		if ok && len(pageTags(p.Text)) > 0 {
			m.DomainsCovered++
			continue
		}
		m.Uncovered = append(m.Uncovered, d)
	}
	sort.Strings(m.Uncovered)
	m.Dated = len(ages)
	if len(ages) > 0 {
		sort.Ints(ages)
		mid := len(ages) / 2
		m.MedianAgeDays = ages[mid]
		if len(ages)%2 == 0 {
			m.MedianAgeDays = (ages[mid-1] + ages[mid]) / 2
		}
	}
	return m
}

// pageTags counts the tagged enrichments of a page by tag.
func pageTags(text string) map[string]int {
	out := map[string]int{}
	EachLine(text, func(_ int, line string) {
		if m := tagLineRe.FindStringSubmatch(strings.TrimSpace(line)); m != nil {
			out[m[1]]++
		}
	})
	return out
}

// mappedDomains returns the business pages the domains section of the
// index links to (relative to docs/wiki/).
func mappedDomains(idx *Page) []string {
	var out []string
	in := false
	EachLine(idx.Text, func(_ int, line string) {
		if strings.HasPrefix(line, "## ") {
			h := strings.ToLower(line[3:])
			in = strings.Contains(h, "domaine") || strings.Contains(h, "domain")
			return
		}
		if !in {
			return
		}
		for _, l := range linkRe.FindAllStringSubmatch(stripCode(line), -1) {
			if t := linkTarget(path.Dir(idx.Path), l[2]); domainPage(t) {
				out = append(out, t)
			}
		}
	})
	return out
}

// Metrics returns the figures of the wiki of the project at root, with its
// queue.
func (s *Service) Metrics(ctx context.Context, root, projectID string) (Metrics, error) {
	w, err := Load(root)
	if err != nil {
		return Metrics{}, err
	}
	m := w.Metrics(s.now())
	if s.Store != nil && projectID != "" {
		open, err := s.Open(ctx, projectID, "")
		if err != nil {
			return m, err
		}
		m.Pending, m.Accepted = 0, 0
		for _, p := range open {
			if p.Status == domain.WikiAccepted {
				m.Accepted++
			} else {
				m.Pending++
			}
		}
	}
	return m, nil
}
