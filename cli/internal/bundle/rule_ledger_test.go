package bundle

import (
	"bufio"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/bricks"
	"github.com/datichb/openhub/cli/internal/workflow"
)

// Rule ledger (modernisation P0.1): no normative rule an agent sees leaves
// the content without a justification while the prompts are trimmed.
//
// For every agent of the shipped workflows (built without project), the
// ledger collects the text it can see: its own body, the skills inlined in
// its prompt and the on-demand skills it can reach (reachableSkills: its
// catalogue of L2, natives, skills it names, requires closure), over all
// the workflows that ship it. The generated blocks of the prompt (absent skills, session role,
// project instructions) are left out. Every normative line (JAMAIS,
// TOUJOURS, NE … PAS, DOIT, OBLIGATOIRE, ❌, ✅, ⛔, MUST, NEVER, interdit),
// outside code fences, is pinned in testdata/rules/<agent>.tsv:
//
//	<skill or agent>\t<section title>\t<normalised text>
//
// A rule is removed when its text no longer appears anywhere in what the
// agent sees (a rule moved to another section or skill the agent still
// reaches, unchanged, needs nothing). Each removed rule is justified by a
// line of testdata/rules/<agent>.moved (written by hand, never by -update):
//
//	<normalised text, as in the .tsv>\t<justification>
//
// with one of these justifications:
//
//	moved:<target>    the rule now lives elsewhere (skill or agent ID first,
//	                  e.g. "moved:dev-standards-git" or "moved:reviewer §Sortie")
//	guard:<guard>     a guard of oh enforces it (e.g. "guard:gitguard.go push deny")
//	merged:<text>     folded into another rule (its new wording)
//	dropped:<reason>  removed on purpose (why)
//
// Lines starting with # and blank lines of a .moved are ignored. The test
// fails when a pinned rule is removed without a .moved entry, when a .moved
// entry names a rule the agent still sees (obsolete entry), or when the
// .tsv is out of date. Once the .moved entries are written,
// go test ./internal/bundle -run TestRuleLedger -update rewrites the .tsv
// files (only those whose removals are all justified).

type ledgerRule struct{ source, section, text string }

func (r ledgerRule) line() string { return r.source + "\t" + r.section + "\t" + r.text }

// wb is a non-word character (letters of every script are word characters,
// unlike in \b).
const wb = `[^\p{L}\p{N}_]`

var (
	ruleMarkerRe = regexp.MustCompile(`(?i)(?:^|` + wb + `)(?:jamais|toujours|doit|doivent|must|never|` +
		`ne` + wb + `(?:[^.\n]{0,40}` + wb + `)?pas|n['’](?:[^.\n]{0,40}` + wb + `)?pas)(?:` + wb + `|$)` +
		`|obligatoire|interdit|❌|✅|⛔`)
	ruleListMarkerRe = regexp.MustCompile(`^(?:[-*+]|\d+[.)]|>|#{1,6}|\[[ xX]\])(?:\s+|$)`)
	htmlCommentRe    = regexp.MustCompile(`<!--.*?-->`)
	ledgerHeadingRe  = regexp.MustCompile(`^#{1,6}\s+(.+)$`)
	spacesRe         = regexp.MustCompile(`\s+`)
)

const ledgerIntro = "(intro)"

// normalizeRule strips the list markers, blockquotes and heading marks, joins
// the cells of a table row with " | " and collapses the spaces.
func normalizeRule(line string) string {
	s := strings.TrimSpace(htmlCommentRe.ReplaceAllString(line, ""))
	if strings.HasPrefix(s, "|") {
		cells := strings.Split(strings.Trim(s, "|"), "|")
		kept := cells[:0]
		for _, c := range cells {
			if c = strings.TrimSpace(c); c != "" {
				kept = append(kept, c)
			}
		}
		s = strings.Join(kept, " | ")
	}
	for {
		loc := ruleListMarkerRe.FindStringIndex(s)
		if loc == nil {
			break
		}
		s = strings.TrimSpace(s[loc[1]:])
	}
	return strings.TrimSpace(spacesRe.ReplaceAllString(s, " "))
}

// extractRules returns the normative lines of a markdown text, outside code
// fences, with the title of the heading above them.
func extractRules(source, text string) []ledgerRule {
	var out []ledgerRule
	section := ledgerIntro
	fence := false
	for _, ln := range strings.Split(text, "\n") {
		if strings.HasPrefix(strings.TrimSpace(ln), "```") {
			fence = !fence
			continue
		}
		if fence || strings.HasPrefix(ln, "> Skill `") || strings.HasPrefix(ln, "> Skills absentes") {
			continue
		}
		heading := ledgerHeadingRe.FindStringSubmatch(ln)
		if ruleMarkerRe.MatchString(ln) {
			if s := normalizeRule(ln); len([]rune(s)) > 8 {
				out = append(out, ledgerRule{source: source, section: section, text: s})
			}
		}
		if heading != nil {
			section = normalizeRule(heading[1])
		}
	}
	return out
}

type ledgerSource struct{ source, text string }

// promptSources splits a composed agent prompt into the agent body and its
// inlined skills, without the generated blocks appended by the build.
func promptSources(agent, body string) []ledgerSource {
	locs := inlinedSkillRe.FindAllStringSubmatchIndex(body, -1)
	tailFrom := 0
	if len(locs) > 0 {
		tailFrom = locs[len(locs)-1][0]
	}
	end := len(body)
	for _, m := range inlinedTail {
		if j := strings.Index(body[tailFrom:], m); j >= 0 && tailFrom+j < end {
			end = tailFrom + j
		}
	}
	first := end
	if len(locs) > 0 {
		first = locs[0][0]
	}
	out := []ledgerSource{{agent, body[:first]}}
	for i, l := range locs {
		stop := end
		if i+1 < len(locs) {
			stop = locs[i+1][0]
		}
		out = append(out, ledgerSource{body[l[2]:l[3]], body[l[0]:stop]})
	}
	return out
}

// agentLedgers collects the rules of every agent over the shipped workflows:
// the agent first, then the skills by ID, each in reading order.
func agentLedgers(t *testing.T) map[string][]ledgerRule {
	t.Helper()
	hub, cat, env := shippedWorkflows(t)
	type acc struct {
		seen    map[ledgerRule]bool
		sources map[string][]ledgerRule
	}
	all := map[string]*acc{}
	for _, ref := range cat.Refs() {
		r, diags := workflow.Check(cat, ref, nil, env)
		require.False(t, diags.HasErrors(), "%v", diags)
		b, err := Build(Request{HubDir: hub, OutDir: t.TempDir(), Spec: r.Spec})
		require.NoError(t, err)
		reach := reachableSkills(t, hub, r.Spec, b, "")
		for _, a := range b.Spec.Agents {
			ac := all[a.ID]
			if ac == nil {
				ac = &acc{seen: map[ledgerRule]bool{}, sources: map[string][]ledgerRule{}}
				all[a.ID] = ac
			}
			sources := promptSources(a.ID, a.Body)
			for _, sk := range reach[a.ID] {
				data, err := os.ReadFile(filepath.Join(b.Dir, skillsDir, sk.ID, "SKILL.md"))
				require.NoError(t, err)
				_, body := bricks.SplitFrontmatter(data)
				sources = append(sources, ledgerSource{sk.ID, string(body)})
			}
			for _, s := range sources {
				for _, rule := range extractRules(s.source, s.text) {
					if !ac.seen[rule] {
						ac.seen[rule] = true
						ac.sources[s.source] = append(ac.sources[s.source], rule)
					}
				}
			}
		}
	}
	out := map[string][]ledgerRule{}
	for agent, ac := range all {
		ids := make([]string, 0, len(ac.sources))
		for id := range ac.sources {
			if id != agent {
				ids = append(ids, id)
			}
		}
		sort.Strings(ids)
		rules := ac.sources[agent]
		for _, id := range ids {
			rules = append(rules, ac.sources[id]...)
		}
		out[agent] = rules
	}
	return out
}

var movedKinds = []string{"moved:", "guard:", "merged:", "dropped:"}

// readMoved returns the justification of each removed rule by its text.
func readMoved(name string) (map[string]string, error) {
	out := map[string]string{}
	f, err := os.Open(name)
	if errors.Is(err, os.ErrNotExist) {
		return out, nil
	}
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1<<20)
	var bad []string
	for n := 1; sc.Scan(); n++ {
		line := sc.Text()
		if strings.TrimSpace(line) == "" || strings.HasPrefix(line, "#") {
			continue
		}
		text, why, ok := strings.Cut(line, "\t")
		valid := ok && text != "" && !strings.Contains(why, "\t")
		if valid {
			valid = false
			for _, k := range movedKinds {
				if v, found := strings.CutPrefix(why, k); found && strings.TrimSpace(v) != "" {
					valid = true
				}
			}
		}
		if !valid {
			bad = append(bad, filepath.Base(name)+":"+strconv.Itoa(n)+": "+line)
			continue
		}
		out[text] = why
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	if len(bad) > 0 {
		return nil, errors.New("malformed .moved lines (<text>\\t<moved:|guard:|merged:|dropped:><value>):\n" + strings.Join(bad, "\n"))
	}
	return out, nil
}

// readLedgerTSV returns the pinned rule lines of an agent (nil when absent).
func readLedgerTSV(t *testing.T, name string) ([]ledgerRule, bool) {
	t.Helper()
	data, err := os.ReadFile(name)
	if errors.Is(err, os.ErrNotExist) {
		return nil, false
	}
	require.NoError(t, err)
	var out []ledgerRule
	for _, line := range strings.Split(strings.TrimSuffix(string(data), "\n"), "\n") {
		if line == "" {
			continue
		}
		f := strings.SplitN(line, "\t", 3)
		require.Len(t, f, 3, "%s: %q", name, line)
		out = append(out, ledgerRule{f[0], f[1], f[2]})
	}
	return out, true
}

func renderLedger(rules []ledgerRule) string {
	var b strings.Builder
	for _, r := range rules {
		b.WriteString(r.line())
		b.WriteByte('\n')
	}
	return b.String()
}

func TestRuleLedger(t *testing.T) {
	hub := repoHub(t)
	current := agentLedgers(t)
	dir := filepath.Join("testdata", "rules")
	agents := map[string]bool{}
	for a := range current {
		agents[a] = true
	}
	pinned, err := filepath.Glob(filepath.Join(dir, "*.tsv"))
	require.NoError(t, err)
	for _, p := range pinned {
		agents[strings.TrimSuffix(filepath.Base(p), ".tsv")] = true
	}
	targets := hubSkillIndex(hub, nil)
	files, err := bricks.FindAgentFiles(hub)
	require.NoError(t, err)
	for _, rules := range current {
		for _, r := range rules {
			targets[r.source] = r.source // generated workflow skills
		}
	}
	if *updateGolden {
		require.NoError(t, os.MkdirAll(dir, 0o755))
	}
	for _, agent := range sortedKeys(agents) {
		t.Run(agent, func(t *testing.T) {
			tsv := filepath.Join(dir, agent+".tsv")
			moved, err := readMoved(filepath.Join(dir, agent+".moved"))
			require.NoError(t, err)
			sees := map[string]bool{}
			for _, r := range current[agent] {
				sees[r.text] = true
			}
			golden, exists := readLedgerTSV(t, tsv)
			var lost []string
			for _, r := range golden {
				if !sees[r.text] && moved[r.text] == "" {
					lost = append(lost, r.line())
				}
			}
			var obsolete, unknown []string
			for _, text := range sortedKeys(moved) {
				if sees[text] {
					obsolete = append(obsolete, text+"\t"+moved[text])
				}
				if target, ok := strings.CutPrefix(moved[text], "moved:"); ok {
					id := strings.FieldsFunc(target, func(r rune) bool { return r == ' ' || r == '#' || r == '§' })[0]
					if _, isSkill := targets[id]; !isSkill && files[id] == "" {
						unknown = append(unknown, text+"\t"+moved[text])
					}
				}
			}
			assert.Empty(t, lost, "%s: rules removed from what %s sees: justify each one in %s.moved "+
				"(<text>\\t<moved:|guard:|merged:|dropped:>…, see rule_ledger_test.go)", agent, agent, agent)
			assert.Empty(t, obsolete, "%s.moved: obsolete entries, %s still sees these rules", agent, agent)
			assert.Empty(t, unknown, "%s.moved: moved: names no hub skill or agent", agent)
			want := renderLedger(current[agent])
			if *updateGolden {
				if len(lost) > 0 || len(obsolete) > 0 || len(unknown) > 0 {
					return
				}
				if len(current[agent]) == 0 {
					require.NoError(t, os.Remove(tsv))
					return
				}
				require.NoError(t, os.WriteFile(tsv, []byte(want), 0o644))
				return
			}
			require.True(t, exists, "missing %s (go test ./internal/bundle -run TestRuleLedger -update)", tsv)
			if len(lost) > 0 || renderLedger(golden) == want {
				return
			}
			pinnedLines := map[string]bool{}
			for _, r := range golden {
				pinnedLines[r.line()] = true
			}
			wantLines := map[string]bool{}
			var diff []string
			for _, r := range current[agent] {
				wantLines[r.line()] = true
				if !pinnedLines[r.line()] {
					diff = append(diff, "+ "+r.line())
				}
			}
			for _, r := range golden {
				if !wantLines[r.line()] {
					diff = append(diff, "- "+r.line())
				}
			}
			if len(diff) == 0 {
				diff = append(diff, "(rule order changed)")
			}
			t.Errorf("%s is out of date (go test ./internal/bundle -run TestRuleLedger -update):\n%s", tsv, strings.Join(diff, "\n"))
		})
	}
}

func TestExtractRules(t *testing.T) {
	text := strings.Join([]string{
		"Intro : tu ne modifies pas le code du projet.",
		"## Ce que tu NE fais PAS",
		"- ❌ Modifier un fichier",
		"  * Tu **dois** toujours   relire.",
		"1. Lance les tests, JAMAIS sans eux.",
		"> ⚠️ Ne jamais pousser.",
		"- [ ] Le ticket n'est pas clos.",
		"Cette chaîne a un pas de trois.", // « ne » inside a word
		"Il ne faut rien oublier.",        // no « pas »
		"Tu n’as pas le droit.",
		"| Action | Règle |",
		"|---|---|",
		"| Commit | ✅ après validation |",
		"```bash",
		"git push # JAMAIS",
		"```",
		"### Sortie <!-- capability: optional -->",
		"Le format est OBLIGATOIRE.",
		"court ❌",
		"> Skill `x` : incluse ci-dessous, ne la charge pas.",
	}, "\n")
	got := extractRules("agent", text)
	var lines []string
	for _, r := range got {
		lines = append(lines, r.section+" / "+r.text)
	}
	assert.Equal(t, []string{
		"(intro) / Intro : tu ne modifies pas le code du projet.",
		"(intro) / Ce que tu NE fais PAS",
		"Ce que tu NE fais PAS / ❌ Modifier un fichier",
		"Ce que tu NE fais PAS / Tu **dois** toujours relire.",
		"Ce que tu NE fais PAS / Lance les tests, JAMAIS sans eux.",
		"Ce que tu NE fais PAS / ⚠️ Ne jamais pousser.",
		"Ce que tu NE fais PAS / Le ticket n'est pas clos.",
		"Ce que tu NE fais PAS / Tu n’as pas le droit.",
		"Ce que tu NE fais PAS / Commit | ✅ après validation",
		"Sortie / Le format est OBLIGATOIRE.",
	}, lines)
	for _, r := range got {
		assert.Equal(t, "agent", r.source)
	}
}

func TestPromptSources(t *testing.T) {
	body := "corps ❌\n\n---\n\n" + inlinedSkillHeading("a") + "aaaa\n\n---\n\n" + inlinedSkillHeading("b") + "bb" +
		absentSkillsNote([]string{"c"}) + contractHeading + " à partir du workflow `w`."
	got := promptSources("agent", body)
	require.Len(t, got, 3)
	assert.Equal(t, ledgerSource{"agent", "corps ❌\n\n---\n\n"}, got[0])
	assert.Equal(t, ledgerSource{"a", inlinedSkillHeading("a") + "aaaa\n\n---\n\n"}, got[1])
	assert.Equal(t, ledgerSource{"b", inlinedSkillHeading("b") + "bb"}, got[2])
	assert.Equal(t, []ledgerSource{{"agent", "seul"}}, promptSources("agent", "seul"+contractHeading+" à partir du workflow `w`."))
}

func TestReadMoved(t *testing.T) {
	dir := t.TempDir()
	name := filepath.Join(dir, "a.moved")
	got, err := readMoved(name)
	require.NoError(t, err)
	assert.Empty(t, got)

	require.NoError(t, os.WriteFile(name, []byte("# commentaire\n\nrègle un\tmoved:dev-standards-git\nrègle deux\tdropped:doublon\n"), 0o644))
	got, err = readMoved(name)
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"règle un": "moved:dev-standards-git", "règle deux": "dropped:doublon"}, got)

	require.NoError(t, os.WriteFile(name, []byte("règle\tremoved:x\nsans tabulation\nvide\tguard:\n"), 0o644))
	_, err = readMoved(name)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "a.moved:1:")
	assert.Contains(t, err.Error(), "a.moved:2:")
	assert.Contains(t, err.Error(), "a.moved:3:")
}
