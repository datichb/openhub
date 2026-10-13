package bundle

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"
)

// TeamSkillPrefix prefixes the refs of the team catalogue skills in the
// problems of CheckTeamSkills.
const TeamSkillPrefix = "team:"

// teamSkillKeys are the frontmatter keys of a team skill read by oh besides
// the hub ones: extends: hub:<ref> (replaces a hub skill, never delivered).
var teamSkillKeys = []string{"extends"}

// CheckTeamSkills applies the unknown_key rule of CheckSkills, and its
// description rules as warnings (long_description: error past the spec
// limit only; description_not_trigger), to the skills
// of a team catalogue (catalogDir/skills/**/*.md, annexes of templates/
// aside). Refs are prefixed with TeamSkillPrefix; a missing folder is no
// problem.
func CheckTeamSkills(catalogDir string) ([]SkillProblem, error) {
	var problems []SkillProblem
	root := filepath.Join(catalogDir, "skills")
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return fs.SkipAll
			}
			return err
		}
		if d.IsDir() {
			if d.Name() == "templates" && filepath.Dir(path) == root {
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(path) != ".md" || strings.HasPrefix(d.Name(), ".") {
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
			problems = append(problems, problem(SkillInvalidFrontmatter, TeamSkillPrefix+ref, perr.Error()))
			return nil
		}
		if desc := doc.Front.Description; strings.TrimSpace(desc) != "" && !isTrigger(desc) {
			problems = append(problems, problem(SkillNotTrigger, TeamSkillPrefix+ref, ""))
		}
		if n := utf8.RuneCountInString(doc.Front.Description); n > maxHubDescriptionLen {
			p := problem(SkillLongDescription, TeamSkillPrefix+ref, strconv.Itoa(n))
			p.Error = n > maxSkillDescriptionLen
			problems = append(problems, p)
		}
		for _, k := range doc.Keys {
			if k != "bucket" && !slices.Contains(deliveredSkillKeys, k) && !slices.Contains(hubSkillKeys, k) && !slices.Contains(teamSkillKeys, k) {
				problems = append(problems, problem(SkillUnknownKey, TeamSkillPrefix+ref, k))
			}
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walking team skills: %w", err)
	}
	return problems, nil
}
