package bundle

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/datichb/openhub/cli/internal/bricks"
)

// Freshness of the stack skills (modernisation R13). A stack skill
// (bricks.IsStackSkill) carries two hub-only frontmatter fields, never
// delivered to the tool:
//
//	versions:            the versions the skill covers, by technology
//	  react: "19.3"      (key: lowercase technology, value: the version
//	  react-compiler: "1.0"  described, "12+" for a minimum only)
//	reviewed: 2026-10-13 the date of its last review against the releases
//
// CheckSkills requires them on every stack skill and checks their shape;
// CheckFreshness (`oh skill check --stale`) lists the skills whose review is
// older than a maximum age.

// DefaultMaxAgeDays is the default maximum age of a review: a quarter. The
// stacks of the hub publish a minor version every one to three months
// (Kubernetes every four, Go every six), so a quarterly review catches every
// release cycle, and the monthly check (`oh skill check --stale`, CI) spreads
// the work: about a third of the stacks per month.
const DefaultMaxAgeDays = 90

// reviewedLayout is the date format of reviewed:.
const reviewedLayout = "2006-01-02"

var versionKeyPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9.+-]*$`)

// checkFreshnessFields reports the freshness problems of a hub skill: a
// stack skill without versions: or reviewed:, and malformed values on any
// skill that sets them.
func checkFreshnessFields(d *skillDoc, now time.Time) []SkillProblem {
	var problems []SkillProblem
	hasVersions := slices.Contains(d.Keys, "versions")
	hasReviewed := slices.Contains(d.Keys, "reviewed")
	if bricks.IsStackSkill(d.Ref) {
		var missing []string
		if !hasVersions {
			missing = append(missing, "versions")
		}
		if !hasReviewed {
			missing = append(missing, "reviewed")
		}
		if len(missing) > 0 {
			problems = append(problems, problem(SkillMissingFreshness, d.Ref, strings.Join(missing, ", ")))
		}
	}
	if hasVersions {
		if detail := versionsProblem(d.Front.Versions); detail != "" {
			problems = append(problems, problem(SkillInvalidFreshness, d.Ref, detail))
		}
	}
	if hasReviewed {
		if _, err := parseReviewed(d.Front.Reviewed, now); err != nil {
			problems = append(problems, problem(SkillInvalidFreshness, d.Ref, err.Error()))
		}
	}
	return problems
}

// versionsProblem describes what is wrong with versions:, "" when nothing.
func versionsProblem(v map[string]string) string {
	if len(v) == 0 {
		return "versions: empty"
	}
	for _, k := range sortedKeys(v) {
		if !versionKeyPattern.MatchString(k) {
			return fmt.Sprintf("versions: key %q (lowercase technology)", k)
		}
		if strings.TrimSpace(v[k]) == "" {
			return fmt.Sprintf("versions: %s has no version", k)
		}
	}
	return ""
}

// parseReviewed reads reviewed: (YYYY-MM-DD, not after now).
func parseReviewed(s string, now time.Time) (time.Time, error) {
	t, err := time.Parse(reviewedLayout, strings.TrimSpace(s))
	if err != nil {
		return time.Time{}, fmt.Errorf("reviewed: %q is not a YYYY-MM-DD date", s)
	}
	if t.After(day(now)) {
		return time.Time{}, fmt.Errorf("reviewed: %s is in the future", s)
	}
	return t, nil
}

// day is the calendar date of t, at midnight UTC.
func day(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

// FreshnessStatus is the state of a skill in a freshness report.
type FreshnessStatus string

const (
	FreshnessOK      FreshnessStatus = "ok"
	FreshnessStale   FreshnessStatus = "stale"   // reviewed more than the maximum age ago
	FreshnessMissing FreshnessStatus = "missing" // no usable versions: or reviewed:
)

// SkillFreshness is one skill of a freshness report.
type SkillFreshness struct {
	Ref      string            `json:"ref"`
	Status   FreshnessStatus   `json:"status"`
	Reviewed string            `json:"reviewed,omitempty"`
	AgeDays  int               `json:"age_days,omitempty"`
	Versions map[string]string `json:"versions,omitempty"`
	// Detail says what is missing or malformed (missing).
	Detail string `json:"detail,omitempty"`
}

// FreshnessReport is the result of CheckFreshness.
type FreshnessReport struct {
	Today      string `json:"today"`
	MaxAgeDays int    `json:"max_age_days"`
	// Checked is the number of skills checked: the stack skills and the
	// other skills that carry versions: or reviewed:.
	Checked int `json:"checked"`
	// Skills are the skills to review (stale or missing), missing first,
	// then the oldest review first.
	Skills []SkillFreshness `json:"skills"`
}

// CheckFreshness lists the skills of the hub to review on today: the stack
// skills, and the other skills that carry freshness metadata, whose review
// is older than maxAgeDays or which lack usable metadata.
func CheckFreshness(hubDir string, today time.Time, maxAgeDays int) (*FreshnessReport, error) {
	rep := &FreshnessReport{Today: day(today).Format(reviewedLayout), MaxAgeDays: maxAgeDays, Skills: []SkillFreshness{}}
	root := filepath.Join(hubDir, "skills")
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		if d.IsDir() {
			if d.Name() == "templates" && filepath.Dir(path) == root {
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(path) != ".md" {
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		ref := filepath.ToSlash(strings.TrimSuffix(rel, ".md"))
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		doc, perr := parseSkill(ref, path, content)
		if perr != nil {
			if bricks.IsStackSkill(ref) {
				rep.Checked++
				rep.Skills = append(rep.Skills, SkillFreshness{Ref: ref, Status: FreshnessMissing, Detail: perr.Error()})
			}
			return nil
		}
		if f, ok := skillFreshness(doc, today, maxAgeDays); ok {
			rep.Checked++
			if f.Status != FreshnessOK {
				rep.Skills = append(rep.Skills, f)
			}
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walking skills: %w", err)
	}
	sort.SliceStable(rep.Skills, func(i, j int) bool {
		a, b := rep.Skills[i], rep.Skills[j]
		if (a.Status == FreshnessMissing) != (b.Status == FreshnessMissing) {
			return a.Status == FreshnessMissing
		}
		if a.AgeDays != b.AgeDays {
			return a.AgeDays > b.AgeDays
		}
		return a.Ref < b.Ref
	})
	return rep, nil
}

// skillFreshness is the freshness of a skill; ok is false for a skill that
// is neither a stack skill nor carries freshness metadata.
func skillFreshness(d *skillDoc, today time.Time, maxAgeDays int) (SkillFreshness, bool) {
	hasVersions := slices.Contains(d.Keys, "versions")
	hasReviewed := slices.Contains(d.Keys, "reviewed")
	if !bricks.IsStackSkill(d.Ref) && !hasVersions && !hasReviewed {
		return SkillFreshness{}, false
	}
	f := SkillFreshness{Ref: d.Ref, Reviewed: strings.TrimSpace(d.Front.Reviewed), Versions: d.Front.Versions}
	var missing []string
	if !hasVersions {
		missing = append(missing, "versions")
	} else if p := versionsProblem(d.Front.Versions); p != "" {
		missing = append(missing, p)
	}
	var reviewed time.Time
	if !hasReviewed {
		missing = append(missing, "reviewed")
	} else if t, err := parseReviewed(d.Front.Reviewed, today); err != nil {
		missing = append(missing, err.Error())
	} else {
		reviewed = t
	}
	if len(missing) > 0 {
		f.Status, f.Detail = FreshnessMissing, strings.Join(missing, ", ")
		return f, true
	}
	f.AgeDays = int(day(today).Sub(reviewed).Hours() / 24)
	f.Status = FreshnessOK
	if f.AgeDays > maxAgeDays {
		f.Status = FreshnessStale
	}
	return f, true
}

var maxAgePattern = regexp.MustCompile(`^(\d+)([dw]?)$`)

// ParseMaxAge reads a maximum age in days: "90", "90d" or "12w".
func ParseMaxAge(s string) (int, error) {
	m := maxAgePattern.FindStringSubmatch(strings.TrimSpace(strings.ToLower(s)))
	if m == nil {
		return 0, fmt.Errorf("max age %q: a number of days (90, 90d) or of weeks (12w)", s)
	}
	n, err := strconv.Atoi(m[1])
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("max age %q: a positive number", s)
	}
	if m[2] == "w" {
		n *= 7
	}
	return n, nil
}
