package bundle

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/datichb/openhub/cli/internal/sessionspec"
)

// Annexes are the files a skill points to (templates, references…). Skills
// declare them in their frontmatter, relative to the skills/ directory:
//
//	annexes: [templates/review-report-format.md]
//
// In the bundle the annexes are copied next to the skill:
// skills/<id>/<annex path>.
//
//   - On-demand skill: the tool gives the skill's base directory, so the
//     relative path written in the skill works as is.
//   - Inlined skill (agent body): references to the annex are rewritten to
//     sessionspec.BundleRootVar + "/skills/<id>/<annex path>", expanded by
//     the adapter with the actual bundle root (which can differ inside a
//     container). The bundle hash covers the unexpanded text.

// annexRef matches the annex paths written in the skill prose.
var annexRef = regexp.MustCompile(`templates/[A-Za-z0-9._-]+\.md`)

// annexFile is one file to copy into the bundle.
type annexFile struct {
	Rel string // path under skills/<id>/
	Src string // absolute source path
}

// annexes returns the declared annexes of a skill (an annex outside skills/
// or missing is an error).
func (l *skillLoader) annexes(d *skillDoc) ([]annexFile, error) {
	if d.Source == "" {
		return nil, nil // generated skill
	}
	hubSkills := filepath.Join(l.hubDir, "skills")
	out := make([]annexFile, 0, len(d.Front.Annexes))
	for _, rel := range d.Front.Annexes {
		clean := path.Clean(filepath.ToSlash(rel))
		if path.IsAbs(clean) || clean == "." || strings.HasPrefix(clean, "../") || clean == ".." {
			return nil, fmt.Errorf("bundle: skill %s: annex %q must be a path under skills/", d.Ref, rel)
		}
		src := filepath.Join(hubSkills, filepath.FromSlash(clean))
		if st, err := os.Stat(src); err != nil || st.IsDir() {
			return nil, fmt.Errorf("bundle: skill %s: annex %s not found", d.Ref, rel)
		}
		out = append(out, annexFile{Rel: clean, Src: src})
	}
	return out, nil
}

// writeAnnexes copies the annexes of a skill to <skillsDir>/<id>/.
func writeAnnexes(skillsDir string, d *skillDoc, files []annexFile) error {
	for _, f := range files {
		dst := filepath.Join(skillsDir, d.ID, filepath.FromSlash(f.Rel))
		if _, err := os.Stat(dst); err == nil {
			continue // same skill inlined by several agents (one source per id)
		}
		data, err := os.ReadFile(f.Src)
		if err != nil {
			return fmt.Errorf("bundle: reading annex %s of %s: %w", f.Rel, d.Ref, err)
		}
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(dst, data, bundleFileMode); err != nil {
			return err
		}
	}
	return nil
}

// inlineAnnexRefs rewrites the annex references of an inlined skill body to
// their location in the bundle (relative to BundleRootVar).
func inlineAnnexRefs(body []byte, d *skillDoc, files []annexFile) []byte {
	if len(files) == 0 {
		return body
	}
	rels := make([]string, 0, len(files))
	for _, f := range files {
		rels = append(rels, f.Rel)
	}
	sort.Slice(rels, func(i, j int) bool { return len(rels[i]) > len(rels[j]) })
	text := string(body)
	for _, rel := range rels {
		re := regexp.MustCompile(`(^|[^A-Za-z0-9._/-])` + regexp.QuoteMeta(rel))
		text = re.ReplaceAllString(text, "${1}"+sessionspec.BundleRootVar+"/skills/"+d.ID+"/"+rel)
	}
	return []byte(text)
}
