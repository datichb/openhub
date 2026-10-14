package specdoc

import (
	"reflect"
	"testing"
)

const goodSpec = `---
updated: 2026-10-14
confidence: mixed
agents: [planner]
---

# Spec — Percent

## Objectif
Calculer un pourcentage.

## Glossaire
- **Part** : la valeur mesurée (` + "`part`" + `)
- **Total** : la référence (` + "`total`" + `)

## Critères d'acceptation
- **CA-1** — total nul : erreur · Test : ` + "`TestPercent_ZeroTotal`" + `
- **CA-2** — part > total : résultat plafonné à 100
  · Test : ` + "`go test -run TestPercent_Cap ./...`" + `
- **CA-3** — part ou total négatif : erreur
  - Test : TestPercent_Negative
  - Exemple : Percent(-1, 10)

` + "```markdown\n- CA-9 exemple sans test\n```" + `

## Découpage
- **T1** — Percent et ses erreurs · Couvre : CA-1, CA-3
  - Agent prévu : developer (backend)
- **T2** — plafond · Couvre : CA-2
`

func TestCheckGood(t *testing.T) {
	if got := Check([]byte(goodSpec), true); len(got) != 0 {
		t.Fatalf("problems = %+v", got)
	}
	if got := IDs([]byte(goodSpec)); !reflect.DeepEqual(got, []string{"CA-1", "CA-2", "CA-3"}) {
		t.Fatalf("ids = %v", got)
	}
}

func TestCheckTables(t *testing.T) {
	spec := `# Spec
## Glossary
| Term | Definition |
|---|---|
| Loan | a book lent to a member |

## Acceptance criteria
| ID | Criterion | Test |
|----|-----------|------|
| AC-1 | zero total fails | TestZero |
| AC-2 | capped at 100 | à définir |

## Breakdown
| Slice | Covers |
|---|---|
| T1 | AC-1 |
| T2 | AC-2, AC-7 |
| T3 | docs only |
`
	want := []Problem{
		{CriterionNoTest, "AC-2"},
		{SliceUnknownCriterion, "AC-7"},
		{SliceNoCriterion, "T3 | docs only |"},
	}
	if got := Check([]byte(spec), true); !reflect.DeepEqual(got, want) {
		t.Fatalf("problems = %+v", got)
	}
}

func TestCheckProblems(t *testing.T) {
	cases := map[string]struct {
		spec      string
		breakdown bool
		want      []Problem
	}{
		"empty":          {"# Spec\n", true, []Problem{{NoGlossary, ""}, {NoCriteria, ""}, {NoBreakdown, ""}}},
		"structure only": {"# Spec\n## Glossaire\n- **A** : a\n## Critères d'acceptation\n- CA-1 x · Test : TestX\n", false, nil},
		"empty glossary": {"## Glossaire\nRien.\n## Critères\n- CA-1 x · Test : TestX\n", false, []Problem{{EmptyGlossary, ""}}},
		"no test": {"## Glossaire\n- a\n## Critères d'acceptation\n- CA-1 erreur si total nul\n- CA-2 x · Test : ?\n- CA-3 y · Test :\n", false,
			[]Problem{{CriterionNoTest, "CA-1"}, {CriterionNoTest, "CA-2"}, {CriterionNoTest, "CA-3"}}},
		"no id": {"## Glossaire\n- a\n## Critères d'acceptation\n- total nul · Test : TestZero\n- CA-1 x · Test : TestX\n", false,
			[]Problem{{CriterionNoID, "total nul · Test : TestZero"}}},
		"duplicate": {"## Glossaire\n- a\n## Critères d'acceptation\n- CA-1 x · Test : TestX\n- CA-1 y · Test : TestY\n", false,
			[]Problem{{CriterionDuplicate, "CA-1"}}},
		"grouped": {"## Glossaire\n- a\n## Critères d'acceptation\n- Règle du total\n  - CA-1 x · Test : TestX\n", false, nil},
		"subsections": {"## Glossaire\n- a\n## Critères d'acceptation\n### Erreurs\n- CA-1 x · Test : TestX\n### Plafond\n- CA-2 y · Test : TestY\n## Découpage\n- T1 · Couvre : CA-1\n", true,
			[]Problem{{CriterionUncovered, "CA-2"}}},
		"no criteria":     {"## Glossaire\n- a\n## Critères d'acceptation\n\n", false, []Problem{{NoCriteria, ""}}},
		"empty breakdown": {"## Glossaire\n- a\n## Critères d'acceptation\n- CA-1 x · Test : T\n## Découpage\nÀ venir.\n", true, []Problem{{NoBreakdown, ""}}},
		"nbsp":            {"## Glossaire\n- a\n## Critères d'acceptation\n- CA-1 x · Test\u00a0: TestX\n", false, nil},
	}
	for name, c := range cases {
		if got := Check([]byte(c.spec), c.breakdown); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: problems = %+v, want %+v", name, got, c.want)
		}
	}
}
