package wiki

import (
	"path"
	"strings"
)

// Layout of the wiki (modernisation LD-F, doc-wiki-protocol): a single
// place for what the project knows.
//
//	docs/wiki/
//	  index.md          map of the wiki (required)
//	  glossary.md       terms of the domain
//	  technical/        architecture, stack, tests, conventions, review rules
//	  business/         a page per domain (business/index.md: the map)
//	  specs/            specifications (written by the spec workflow)
//	  decisions/        ADR, and decisions-rejetees.md
//	  postmortems/      a page per incident
//
// wiki_check reports a page out of this layout, and reads the narrative
// folders (specs, decisions, postmortems) as decisions rather than
// observations: their pages need no confidence tag.

// Pages and folders of the layout, relative to Dir.
const (
	GlossaryPage          = "glossary.md"
	TechnicalDir          = "technical"
	BusinessDir           = "business"
	SpecsDir              = "specs"
	DecisionsDir          = "decisions"
	PostmortemsDir        = "postmortems"
	RejectedDecisionsPage = DecisionsDir + "/decisions-rejetees.md"
)

// LayoutDirs are the folders of the layout.
var LayoutDirs = []string{TechnicalDir, BusinessDir, SpecsDir, DecisionsDir, PostmortemsDir}

// Kinds of page (PageKind).
const (
	KindIndex      = "index"
	KindGlossary   = "glossary"
	KindTechnical  = "technical"
	KindBusiness   = "business"
	KindSpec       = "spec"
	KindDecision   = "decision"
	KindPostmortem = "postmortem"
	KindOther      = "other" // out of the layout
)

// PageKind returns the kind of a page of the wiki (relative to Dir).
func PageKind(rel string) string {
	switch rel {
	case IndexPage:
		return KindIndex
	case GlossaryPage:
		return KindGlossary
	}
	top, _, nested := strings.Cut(rel, "/")
	if !nested {
		return KindOther
	}
	switch top {
	case TechnicalDir:
		return KindTechnical
	case BusinessDir:
		return KindBusiness
	case SpecsDir:
		return KindSpec
	case DecisionsDir:
		return KindDecision
	case PostmortemsDir:
		return KindPostmortem
	}
	return KindOther
}

// narrative reports whether a page records decisions or events rather
// than observations of the code (a spec, an ADR, a postmortem): a
// confidence tag is not required.
func narrative(rel string) bool {
	switch PageKind(rel) {
	case KindSpec, KindDecision, KindPostmortem:
		return true
	}
	return false
}

// domainPage reports whether a page documents a business domain
// (business/<domain>.md, the map aside).
func domainPage(rel string) bool {
	return PageKind(rel) == KindBusiness && path.Base(rel) != IndexPage
}
