package bundle

import (
	"os"
	"path/filepath"
	"sort"

	"github.com/datichb/openhub/cli/internal/bricks"
)

// Per-agent skill catalogue (modernisation L2). Each agent may load only
// the skills it can reach:
//
//   - its native skills;
//   - the deliverable skills named in its composed prompt (sections that do
//     not invite it to load a skill aside, invitingText);
//   - the stack skills of the project when its frontmatter sets
//     stack_skills (agents that write or judge code);
//   - for the entry agent, the workflow `skills.extra` and the skills named
//     in the prompt template;
//   - for an agent that sets skills_from_callers, the skills its callers
//     name in their prompt, their own native skills aside (a caller names
//     those for itself);
//
// and the `requires:` closure of these, then, transitively, the deliverable
// skills named (invitingText) by an on-demand skill of the catalogue, the
// skills inlined in its prompt aside. A [SKILL:…] tag invites the agent
// that receives the delegation prompt (callee, entry agent of the prompt
// template), not the one writing it; a stack skill enters only through the
// detected stack. The bundle ships the union of the catalogues.

// agentCatalogue collects the on-demand skill refs of an agent.
type agentCatalogue struct {
	refs []string
	seen map[string]bool
	// named are the deliverable skills named in the prompt, offered to the
	// callees that accept the skills of their callers.
	named []string
	// natives are the native skill refs of the agent.
	natives map[string]bool
	// inlined are the IDs of the skills inlined in the prompt.
	inlined     map[string]bool
	fromCallers bool
	// env is the session of the agent (L3); skip and keep drop the skills
	// whose when: fails for it (keep also applies the session filters).
	env  whenEnv
	skip func(*skillDoc) bool
	keep func(string) bool
}

func newAgentCatalogue(inlined map[string]bool, fromCallers bool) *agentCatalogue {
	return &agentCatalogue{seen: map[string]bool{}, natives: map[string]bool{}, inlined: inlined, fromCallers: fromCallers}
}

func (c *agentCatalogue) addNatives(refs []string) {
	for _, r := range refs {
		c.natives[r] = true
	}
	c.add(refs...)
}

func (c *agentCatalogue) add(refs ...string) {
	for _, r := range refs {
		if !c.seen[r] {
			c.seen[r] = true
			c.refs = append(c.refs, r)
		}
	}
}

// docs returns the closure of the refs of the agent, the skills whose
// when: fails for it aside (inlined skills included).
func (c *agentCatalogue) docs(l *skillLoader, denied skillDenied) ([]*skillDoc, error) {
	return l.closureFor(c.refs, denied, c.skip)
}

// skillIDs returns the catalogue of the agent: the IDs of the closure of its
// refs, the inlined skills aside, sorted. Never nil (an empty catalogue
// means no skill).
func (c *agentCatalogue) skillIDs(l *skillLoader, denied skillDenied) ([]string, error) {
	docs, err := c.docs(l, denied)
	if err != nil {
		return nil, err
	}
	ids := []string{}
	seen := map[string]bool{}
	for _, d := range docs {
		if !c.inlined[d.ID] && !seen[d.ID] {
			seen[d.ID] = true
			ids = append(ids, d.ID)
		}
	}
	sort.Strings(ids)
	return ids, nil
}

// expand adds, until nothing changes, the deliverable skills named in the
// on-demand skills of the catalogue (sections that do not invite agent
// aside, blocks whose when: fails for it aside): the agent loading a skill
// may be told to load another one.
func (c *agentCatalogue) expand(l *skillLoader, denied skillDenied, index map[string]string, agent string, agents map[string]bool) error {
	scanned := map[string]bool{}
	for {
		docs, err := c.docs(l, denied)
		if err != nil {
			return err
		}
		added := false
		for _, d := range docs {
			if scanned[d.Ref] || c.inlined[d.ID] {
				continue
			}
			scanned[d.Ref] = true
			body := filterFor(d.Ref, string(d.body()), c.env)
			for _, ref := range selfMentionedSkills(invitingText(body, agent, agents), index) {
				m, err := l.load(ref)
				if err != nil || c.inlined[m.ID] || c.seen[ref] || bricks.IsStackSkill(ref) || !c.keep(ref) || !deliverable(l, ref, denied) {
					continue
				}
				c.add(ref)
				added = true
			}
		}
		if !added {
			return nil
		}
	}
}

// addCallerSkills gives the agents that accept them the skills their callers
// name.
func addCallerSkills(cats map[string]*agentCatalogue, callers map[string][]string) {
	for id, c := range cats {
		if !c.fromCallers {
			continue
		}
		for _, caller := range callers[id] {
			cc := cats[caller]
			if cc == nil {
				continue
			}
			for _, ref := range cc.named {
				if !cc.natives[ref] && (c.keep == nil || c.keep(ref)) {
					c.add(ref)
				}
			}
		}
	}
}

// promptTemplate returns the template of the workflow prompt: the request's
// PromptTemplate, else the inline text, else the hub file of the template
// ("" when there is none or it cannot be read: a team template is given by
// the caller).
func (req Request) promptTemplate() string {
	if req.PromptTemplate != "" {
		return req.PromptTemplate
	}
	p := req.Spec.Prompt
	switch {
	case p == nil:
		return ""
	case p.Text != "":
		return p.Text
	case p.Template != "" && filepath.IsLocal(filepath.FromSlash(p.Template)):
		data, err := os.ReadFile(filepath.Join(req.HubDir, "workflows", filepath.FromSlash(p.Template)))
		if err == nil {
			return string(data)
		}
	}
	return ""
}
