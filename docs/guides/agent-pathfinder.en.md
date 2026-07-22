> [Lire en français](agent-pathfinder.fr.md)

# Agent Pathfinder — User Guide

## 🎯 What is the Pathfinder agent?

The **Pathfinder** agent is a fast, flexible reconnaissance agent that explores a feature in 2-5 minutes and produces an actionable structured report.

Pathfinder fills the gap between lightweight exploration and full planning:

```
Simple ← Pathfinder → Planner → Complex
```

## 🚀 When to use Pathfinder?

### ✅ Use Pathfinder for:

- **Simple features**: "Add an email field to the profile"
- **Exploratory phase**: "See if we can integrate Stripe quickly"
- **POC/Prototype**: "Test the idea of a tagging system"
- **Quick estimation**: "How long would that take?"
- **Complexity uncertainty**: "Is it worth planning in detail?"

### 🎯 Use Planner for:

- **Complex features**: "Refactor the authentication system with OAuth + 2FA"
- **Design/audit signals**: "Analytics dashboard with optimized UX and performance audit"
- **Production-critical**: "Migrate the PostgreSQL database to MongoDB"
- **Multi-agent**: Feature requiring designers, auditors, architects
- **Detailed planning**: Need for fully enriched Beads tickets

## 📊 How does Pathfinder work?

### Pathfinder workflow (2-5 min)

```
1. Understand the request (30 sec)
   ↓
2. Explore the context (2-3 min)
   - Key files/modules
   - Existing Beads tickets
   - Reusable patterns
   ↓
3. Estimate complexity (1 min)
   - XS / S / M / L / XL
   - Complexity factors
   ↓
4. Structure a draft (1 min)
   - Epic + estimated tickets
   ↓
5. Identify risks & signals (30 sec)
   ↓
6. Recommend (30 sec)
   - ✅ Direct processing OR
   - 🎯 Escalate to planner
```

### Complexity scale

| Size | Estimated tickets | Total duration | Examples | Pathfinder recommendation |
|------|-------------------|----------------|----------|--------------------------|
| **XS** | 1 task | < 1h | Add a field, change a color | ✅ Direct |
| **S** | 1-2 tickets | 1-3h | Simple form, CRUD endpoint | ✅ Direct |
| **M** | 3-5 tickets | 0.5-1 day | Tagging system, advanced filter | ⚠️ User's choice |
| **L** | 6-10 tickets | 1-3 days | OAuth auth, analytics dashboard | 🎯 Escalate to planner |
| **XL** | 10+ tickets | 1+ week | Auth overhaul, DB migration | 🎯 Escalate to planner |

**Factors that increase complexity (+1 level):**
- ⚠️ Design signals (UX/UI)
- ⚠️ Audit signals (security, performance, GDPR, accessibility)
- ⚠️ Multiple dependencies (>3 related tickets)
- ⚠️ Data migration
- ⚠️ Multi-module impact (>3 modules affected)

## 💼 Use cases

### Example 1: Simple feature (XS) → Direct processing

**Request:**
```
"I'd like to add a phone field to the user profile"
```

**Pathfinder result (1 min):**
- **Complexity:** S (2 tickets, ~1h15)
- **Structure:**
  1. DB migration + User model (~30min)
  2. Input in ProfileForm + validation (~45min)
- **Recommendation:** ✅ **Direct** - Simple feature, immediate processing

**Next step:**
→ Pathfinder → orchestrator-dev → Direct implementation

---

### Example 2: Complex feature (L) → Escalate to planner

**Request:**
```
"I'd like to implement a real-time notifications system"
```

**Pathfinder result (3 min):**
- **Complexity:** L (6 tickets, ~14h)
- **Signals detected:**
  - ✅ Architecture (new system, WebSocket vs SSE choice)
  - ✅ UX/UI (NotificationCenter design needed)
  - ✅ Performance (persistent connections, scaling)
- **Recommendation:** 🎯 **Escalate** - High complexity + strong signals

**Next step:**
→ Pathfinder → Planner (with full handoff) → 7-phase planning

---

### Example 3: Complexity uncertainty → User's choice

**Request:**
```
"See if we can integrate a Stripe payment system"
```

**Pathfinder result (2 min):**
- **Complexity:** M (4-5 tickets, ~6-8h)
- **Signals detected:**
  - ⚠️ Security (sensitive payments, PCI-DSS)
  - ⚠️ Architecture (webhook handlers, error handling)
- **Recommendation:** ⚠️ **User's choice**
  - Option 1: Quick POC with orchestrator-dev
  - Option 2: Full planning with planner (security audit recommended)

**Next step:**
→ User decides based on context (POC vs Production)

## 🎨 Pathfinder report format

Pathfinder produces a structured markdown report:

```markdown
# 🔍 Pathfinder Report

**Feature:** [Name]
**Complexity:** [XS|S|M|L|XL]
**Date:** [timestamp]

## 📝 Quick context
[2-3 sentences of understanding]

## 🔎 Exploration (2-3 min)
- Key files identified
- Existing Beads tickets
- Reusable patterns

## 🎯 Proposed structure (draft)
- Suggested epic
- Estimated tickets (1, 2, 3...)

## ❓ Open questions
- Categorized questions

## ⚠️ Identified risks
- Level + description

## 🚦 Detected signals
| Signal | Status | Details |

## 🎯 Recommendation
✅ Direct OR 🎯 Escalate (with justification)

## 📦 Handoff to planner (if escalating)
[Full section for transmission to the planner]
```

**The report is usable by:**
- 👤 **The user** (direct reading, informed decision)
- 🤖 **orchestrator-dev** (context for direct implementation)
- 🤖 **planner** (full handoff if escalating)

## 🔄 Full workflow with Pathfinder

### Case 1: Simple feature

```
User: "Add an email field to the profile"
      ↓
Orchestrator (heuristic: simplicity detected)
      ↓
Pathfinder (2 min)
      ↓
Report: ✅ Direct (S, 2 tickets, ~1h)
      ↓
orchestrator-dev
      ↓
Implementation
```

### Case 2: Complex feature

```
User: "Real-time notifications system"
      ↓
Orchestrator (heuristic: no clear signal)
      ↓
Pathfinder (3 min)
      ↓
Report: 🎯 Escalate (L, arch/UX/perf signals)
      ↓
User: "OK, escalate to planner"
      ↓
Planner (7-phase workflow with pathfinder handoff)
      ↓
Enriched Beads tickets + routing
```

### Case 3: Direct planner (obvious complexity)

```
User: "Complete overhaul of the authentication system with OAuth + 2FA"
      ↓
Orchestrator (heuristic: keyword "overhaul" detected)
      ↓
Direct Planner (no need for pathfinder)
      ↓
Enriched Beads tickets
```

## 🎛️ Orchestrator routing heuristic

The orchestrator automatically chooses between Pathfinder and Planner based on criteria:

### → Pathfinder (quick)

- **Keywords**: "simple", "quick", "add", "modify", "quick scan", "pathfinder"
- **Exploration**: "explore", "see if", "POC", "prototype"
- **Default** if no clear signal

### → Planner (full)

- **Keywords**: "overhaul", "system", "architecture", "migration"
- **Signals**: "UX", "design", "security", "performance", "audit"
- **Obvious complexity**

### → User question

- **Doubt**: mixed criteria
- **Ambiguity** about complexity

## 📚 Key differences: Pathfinder vs Planner

| Aspect | Pathfinder | Planner |
|--------|-------|---------|
| **Duration** | 2-5 min | 10-20 min |
| **Workflow** | Free and flexible | 7 rigid phases |
| **Model** | Claude Sonnet 4 | Claude Opus 4 |
| **Output** | Structured report | Enriched Beads tickets |
| **Depth** | Reconnaissance | Full analysis |
| **Delegation** | No (except documentarian) | Yes (designers, auditors) |
| **Escalation** | Can escalate to planner | Final point |
| **Usage** | Simple features, exploration | Complex features, production |

## 💡 Best practices

### ✅ Do

1. **Use Pathfinder first** if you don't know the complexity
2. **Read the full report** before deciding
3. **Follow Pathfinder's recommendation** (but you decide)
4. **Ask questions** if the report lacks information
5. **Escalate to planner** if complexity increases along the way

### ❌ Avoid

1. **Don't skip Pathfinder** for unknown features (time savings if simple)
2. **Don't ignore the signals** detected by Pathfinder
3. **Don't force direct processing** if Pathfinder recommends escalation
4. **Don't use Planner** for obviously simple features (waste of time)

## 🔧 Configuration

Pathfinder is configured in `~/.oh/hub.toml`:

```json
{
  "agent_models": {
    "families": {
      "planning": "claude-opus-4"
    },
    "agents": {
      "pathfinder": "claude-sonnet-4-6"
    }
  }
}
```

**Pathfinder uses Claude Sonnet 4.6 for:**
- Execution speed (2-5 min)
- Extended + Adaptive thinking
- 1M token context window
- Reduced cost vs Opus 4
- Optimal quality for reconnaissance

## 🆘 FAQ

### Q: Can I invoke Pathfinder directly?

**A:** Yes! You can explicitly ask:
- "Pathfinder this feature for me"
- "Do a quick scan of this idea"
- "Estimate the complexity quickly"

The orchestrator will automatically invoke Pathfinder.

### Q: Can Pathfinder create Beads tickets?

**A:** Yes, but it asks for confirmation first (`ask` permissions). Generally, Pathfinder recommends that orchestrator-dev or the planner do it instead.

### Q: What happens if I refuse the escalation?

**A:** You are free to refuse. Pathfinder will have provided sufficient context for orchestrator-dev to implement, even if the complexity is medium.

### Q: Can Pathfinder consult designers/auditors?

**A:** No, only the planner can delegate to specialized agents (designer, auditor-*, etc.). This is a reason to escalate if these signals are strong.

### Q: How do I force the planner without going through Pathfinder?

**A:** Ask explicitly:
- "Plan this feature completely"
- "In-depth analysis with the planner"
- "Detailed structure as Beads tickets"

The orchestrator will understand and invoke the planner directly.

### Q: Does Pathfinder replace the planner?

**A:** No, they are complementary:
- **Pathfinder** = quick reconnaissance, estimation, triage
- **Planner** = full analysis, enrichment, multi-agent coordination

Pathfinder can **escalate** to the planner, but does not replace it.

## 📖 Resources

- **Pathfinder Agent**: `/agents/planning/pathfinder.md`
- **Pathfinder Protocol**: `/skills/planning/pathfinder-protocol.md`
- **Handoff Format**: `/skills/planning/pathfinder-handoff-format.md`
- **Planner Agent**: `/agents/planning/planner.md`
- **Orchestrator**: `/agents/planning/orchestrator.md`

## 🎯 Summary

**Pathfinder** is your ally for:
- ⚡ Saving time on simple features
- 🔍 Quickly exploring an idea
- 📊 Estimating complexity before committing
- 🎯 Making an informed decision (direct or full planner)

**Golden rule:** *"When in doubt, start with Pathfinder — it will tell you if you need to escalate."*
