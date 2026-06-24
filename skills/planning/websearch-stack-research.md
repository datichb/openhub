---
name: websearch-stack-research
description: "Protocole de recherche WebSearch spécialisé pour les agents de planification — découverte et évaluation de stacks technologiques, librairies, frameworks et patterns architecturaux. Utiliser quand le pathfinder détecte une stack incomplète ou non documentée, quand le planner doit choisir entre plusieurs librairies, ou quand l'onboarder identifie des lacunes technologiques. Couvre : comparaisons de librairies, critères d'évaluation (popularité, maintenance, TypeScript, bundle size, licence), patterns de requêtes ciblées, gestion des résultats biaisés ou obsolètes. Mots-clés : library comparison, framework selection, migration planning, tech stack, npm downloads, GitHub stars, bundle size, ecosystem trends."
version: 1.0.0
target: planning-agents
bucket: B
extends: skills/shared/websearch-usage.md
---

# WebSearch Stack Research — Planning Agent Protocol

**Version**: 1.0.0  
**Target**: Planning agents (pathfinder, onboarder, planner)  
**Extends**: `skills/shared/websearch-usage.md`

## Purpose

This skill provides a specialized protocol for using WebSearch to discover and evaluate **technology stacks**, **libraries**, **frameworks**, and **architectural patterns** during project planning and scouting phases. It enables data-driven technology recommendations based on current ecosystem trends.

## When to Use Stack Research

### Trigger Conditions
Use WebSearch for stack research when:
1. **Project Initialization**: Pathfinder detects incomplete or undocumented tech stack
2. **Technology Decision**: Choosing between multiple libraries/frameworks
3. **Migration Planning**: Evaluating alternatives to legacy tech
4. **Feature Implementation**: Unknown how to implement new requirements
5. **Best Practices Discovery**: Learning patterns for unfamiliar technology

### Planning Phase Integration
```
Pathfinder Phase:
→ Discover existing tech stack
→ WebSearch: Missing documentation, migration guides

Onboarder Phase:
→ Identify knowledge gaps
→ WebSearch: Official docs, setup guides, tutorials

Planner Phase:
→ Design features with uncertain implementation
→ WebSearch: Library comparisons, architecture patterns
```

## Stack Research Query Patterns

### Pattern 1: Library Comparison
```
✅ "React state management comparison Zustand Redux Jotai 2026"
✅ "Next.js authentication library comparison NextAuth Clerk Auth.js"
✅ "Node.js ORM comparison Prisma TypeORM Drizzle"
✅ "headless CMS comparison Sanity Contentful Strapi 2026"
```

### Pattern 2: Documentation Discovery
```
✅ "Tailwind CSS official documentation"
✅ "Prisma schema reference guide"
✅ "Next.js 15 App Router migration guide"
✅ "TypeScript 5.4 documentation"
```

### Pattern 3: Ecosystem Trends
```
✅ "React ecosystem trends 2026"
✅ "serverless framework comparison AWS Lambda 2026"
✅ "Jamstack hosting comparison Vercel Netlify Cloudflare Pages"
✅ "monorepo tools comparison Turborepo Nx pnpm workspaces"
```

### Pattern 4: Best Practices & Patterns
```
✅ "Next.js project structure best practices 2026"
✅ "microservices architecture Node.js patterns"
✅ "GraphQL API design best practices"
✅ "React component design patterns 2026"
```

### Pattern 5: Integration & Setup
```
✅ "integrate Stripe Next.js App Router"
✅ "setup Tailwind CSS with Next.js 14"
✅ "configure ESLint Prettier TypeScript"
✅ "deploy Next.js Vercel environment variables"
```

## Evaluation Criteria for Stack Decisions

When researching libraries/frameworks, evaluate on:

### 1. Popularity & Adoption
```
Search indicators:
- "most popular {category} 2026"
- "npm downloads {library}"
- "GitHub stars {library}"

Thresholds:
✅ HIGH: >1M weekly npm downloads, >20K GitHub stars
✅ MEDIUM: 100K-1M downloads, 5K-20K stars
⚠ LOW: <100K downloads, <5K stars (higher risk)
```

### 2. Maintenance & Community
```
Search indicators:
- "{library} active development 2026"
- "{library} community support"
- "{library} issues response time"

Red flags:
❌ Last commit >6 months ago
❌ Many open critical issues
❌ Maintainer burnout announcements
```

### 3. Documentation Quality
```
Search:
- "{library} documentation"
- "{library} getting started guide"
- "{library} tutorials 2026"

Quality signs:
✅ Official docs with search
✅ TypeScript examples
✅ Migration guides
✅ API reference
✅ Video tutorials
```

### 4. TypeScript Support
```
Search:
- "{library} TypeScript support"
- "{library} type definitions"

Tiers:
✅ EXCELLENT: Written in TS, full type coverage
✅ GOOD: DefinitelyTyped types maintained
⚠ POOR: No types, community types incomplete
```

### 5. Bundle Size
```
Search:
- "{library} bundle size"
- "bundlephobia {library}"
- "{library} vs {alternative} size comparison"

Thresholds (gzipped):
✅ SMALL: <10KB
✅ MEDIUM: 10-50KB
⚠ LARGE: >50KB (justify if necessary)
```

### 6. Performance
```
Search:
- "{library} performance benchmark 2026"
- "{library} vs {alternative} speed comparison"

Look for:
- Independent benchmarks (not vendor-published)
- Real-world scenarios (not micro-benchmarks)
- Recent data (2024-2026)
```

### 7. License
```
Search:
- "{library} license"

Acceptable for most projects:
✅ MIT, Apache 2.0, ISC, BSD
⚠ GPL (requires legal review)
❌ Proprietary (requires purchase)
```

## Workflow Integration

### Pathfinder → WebSearch Workflow
```
1. PATHFINDER: Analyze existing codebase
   → Detected: Next.js 13, React 18, unknown state management

2. WEBSEARCH: Identify missing pieces
   Query: "Next.js 13 state management detection patterns"
   → Learn: Look for Redux, Zustand, Jotai imports

3. PATHFINDER: Re-scan with new patterns
   → Detected: Zustand in use

4. WEBSEARCH: Document Zustand usage
   Query: "Zustand best practices 2026"
   → Fetch: Official docs for handoff

5. HANDOFF: Complete tech stack map
   → State: Zustand (docs: https://zustand-demo.pmnd.rs/)
```

### Onboarder → WebSearch Workflow
```
1. ONBOARDER: Review project requirements
   → Gap: "How do we handle authentication?"

2. WEBSEARCH: Research options
   Query: "Next.js authentication library comparison 2026"
   Results:
   - NextAuth.js (most popular, complex)
   - Clerk (easiest, paid tiers)
   - Auth.js (NextAuth v5, beta)

3. WEBSEARCH: Deep dive on top candidate
   Query: "NextAuth.js Next.js 14 App Router setup"
   → Fetch: Official migration guide

4. ONBOARDER: Document setup steps
   → Include: Auth provider setup, session handling, protected routes
```

### Planner → WebSearch Workflow
```
1. PLANNER: Design feature "Real-time notifications"
   → Uncertainty: How to implement?

2. WEBSEARCH: Discover solutions
   Query: "real-time notifications Next.js 2026 WebSocket vs SSE"
   Options:
   - Pusher (hosted, easy)
   - Ably (hosted, feature-rich)
   - Socket.io (self-hosted, complex)
   - Server-Sent Events (native, simple)

3. WEBSEARCH: Evaluate best fit
   Query: "Server-Sent Events Next.js App Router example"
   → Fetch: Tutorial with code examples

4. PLANNER: Include in implementation plan
   → Approach: SSE with EventSource API
   → Rationale: Native browser support, simpler than WebSockets for one-way notifications
   → Reference: [SSE guide URL]
```

## Library Comparison Template

When comparing libraries, use this structure:

```markdown
## Library Comparison: {Category}

**Use Case**: {Specific requirement}  
**Date**: 2026-05-29  
**Search Query**: "{your query}"

### Options Evaluated

#### Option 1: {Library A}
- **Popularity**: {downloads/stars}
- **Bundle Size**: {size} KB gzipped
- **TypeScript**: ✅ Full support / ⚠ Partial / ❌ None
- **Documentation**: [URL]
- **Last Updated**: {date}
- **Pros**:
  - {pro 1}
  - {pro 2}
- **Cons**:
  - {con 1}
  - {con 2}
- **Best For**: {scenario}

#### Option 2: {Library B}
...

#### Option 3: {Library C}
...

### Recommendation

**Selected**: {Library X}

**Rationale**:
1. {Reason 1 with data}
2. {Reason 2 with data}
3. {Reason 3 with data}

**Trade-offs Accepted**:
- {Trade-off 1}
- {Trade-off 2}

**Implementation Notes**:
- Install: `npm install {library}`
- Setup: {brief overview}
- Documentation: [URL]

**Sources**:
- Comparison article: [URL]
- Official docs: [URL]
- Benchmark: [URL]
```

## Ecosystem-Specific Search Strategies

### React Ecosystem
```
Common searches:
- "React UI component library comparison 2026" (MUI, Chakra, Radix, shadcn/ui)
- "React form library comparison 2026" (React Hook Form, Formik, Final Form)
- "React animation library comparison" (Framer Motion, React Spring, GSAP)
- "React testing library best practices" (Testing Library, Enzyme, Cypress)
```

### Next.js Ecosystem
```
Common searches:
- "Next.js database integration 2026" (Prisma, Drizzle, Supabase)
- "Next.js deployment options comparison" (Vercel, Netlify, AWS Amplify)
- "Next.js CMS integration" (Sanity, Contentful, Strapi, Payload)
- "Next.js monitoring tools" (Sentry, LogRocket, Datadog)
```

### Node.js Backend
```
Common searches:
- "Node.js framework comparison 2026" (Express, Fastify, Hapi, NestJS)
- "Node.js validation library" (Zod, Yup, Joi, AJV)
- "Node.js job queue library" (Bull, BullMQ, Bee-Queue)
- "Node.js logging library" (Winston, Pino, Bunyan)
```

### Database Selection
```
Common searches:
- "database comparison SQL vs NoSQL 2026"
- "PostgreSQL vs MySQL vs MongoDB use cases"
- "serverless database comparison PlanetScale Neon Supabase"
- "database migration tool comparison Prisma Drizzle TypeORM"
```

## Handling Incomplete or Biased Results

### Vendor Bias
```
Problem: Search returns vendor marketing pages only

Solutions:
1. Add "independent comparison" to query
   ✅ "independent headless CMS comparison 2026"
2. Search for community discussions
   ✅ "Reddit {library A} vs {library B}"
3. Look for academic or research sources
   ✅ "research paper {technology}"
```

### Outdated Information
```
Problem: Top results are from 2020-2022

Solutions:
1. Add year filter
   ✅ "React state management 2025-2026"
2. Search for "latest" or "current"
   ✅ "latest React patterns 2026"
3. Check official changelogs
   ✅ "React 18 new features"
```

### Missing Context
```
Problem: Generic comparison without use-case specificity

Solutions:
1. Add your specific requirements
   ✅ "React form library large multi-step forms"
2. Add scale indicators
   ✅ "database choice small startup vs enterprise"
3. Add technical constraints
   ✅ "real-time library serverless AWS Lambda"
```

## Rate Limit Strategy

Stack research can be extensive. Optimize with:

### Progressive Refinement
```
Round 1: Broad Overview
✅ "Next.js full-stack architecture 2026"
→ Get landscape understanding

Round 2: Targeted Deep Dives
✅ "Next.js authentication NextAuth vs Clerk"
✅ "Next.js database Prisma vs Drizzle"
→ Compare top 2-3 candidates per category

Round 3: Implementation Details (WebFetch)
✅ webfetch("https://next-auth.js.org/getting-started")
→ Fetch official documentation for selected choices
```

### Parallel Research Categories
If you need to research multiple categories, batch them:
```
Single query approach:
✅ "Next.js tech stack recommendations 2026 database authentication deployment"
→ Get overview of all categories in one search
→ Follow up with targeted queries only where needed
```

### Leverage Known Resources
For common stacks, go direct to known comparisons:
```
✅ webfetch("https://2024.stateofjs.com/en-US/libraries/")
✅ webfetch("https://risingstars.js.org/2025/en")
✅ webfetch("https://npmtrends.com/{lib1}-vs-{lib2}")
→ Annual surveys and trend reports cover many comparisons
```

## Error Handling

### Scenario 1: No Clear Winner
```
Research shows: All options have similar trade-offs

Action:
1. Default to most popular (lower risk, more resources)
2. Document: "No significant difference found, chose {X} based on community size"
3. Plan: "Can migrate to alternative if needs change"
```

### Scenario 2: All Options Have Deal-Breakers
```
Research shows: Every library has a critical limitation for our use case

Action:
1. Re-evaluate requirements (are they all necessary?)
2. Search for hybrid solutions: "{feature} custom implementation"
3. Consider building minimal custom solution
4. Document trade-offs clearly for stakeholder decision
```

### Scenario 3: Conflicting Recommendations
```
Source A: "Use {X} for {use case}"
Source B: "Avoid {X}, use {Y} instead"

Action:
1. Check publication dates (recent wins)
2. Check source authority (official > blog)
3. Check context (scale, constraints, etc.)
4. Report: "Mixed opinions found, recommending {X} because {reason}"
```

## Integration with Documentation

### Handoff Format
After research, include in handoff document:

```markdown
## Technology Stack

### Core Framework
- **Next.js 14** (App Router)
  - Docs: https://nextjs.org/docs
  - Chosen for: SSR, file-based routing, Vercel ecosystem
  - Version: 14.2.0 (latest stable)

### State Management
- **Zustand 4.5**
  - Docs: https://zustand-demo.pmnd.rs/
  - Chosen for: Simplicity, TypeScript support, small bundle (3KB)
  - Alternative considered: Redux (rejected due to boilerplate complexity)

### Database
- **Prisma 5** + PostgreSQL 16
  - Docs: https://www.prisma.io/docs
  - Chosen for: Type-safe ORM, excellent DX, migration tooling
  - Alternative considered: Drizzle (rejected due to smaller community)

### Authentication
- **NextAuth.js 5** (Auth.js)
  - Docs: https://authjs.dev/
  - Chosen for: OAuth providers, session handling, Next.js integration
  - Setup guide: [Internal doc link]

### Research Sources
- Stack comparison: https://example.com/nextjs-stack-2026
- Community recommendations: https://reddit.com/r/nextjs/...
- Benchmark data: https://github.com/user/comparison-repo
```

## Best Practices Summary

**DO**:
- ✅ Research before committing to a library (avoid costly migrations)
- ✅ Evaluate multiple options (minimum 2-3 candidates)
- ✅ Check publication dates (prefer 2024-2026 sources)
- ✅ Verify with official documentation
- ✅ Consider bundle size, TypeScript support, maintenance status
- ✅ Document rationale for choices (for future reference)
- ✅ Include sources in handoff documents

**DON'T**:
- ❌ Choose based on hype alone (viral tweets != good fit)
- ❌ Ignore bundle size (frontend performance matters)
- ❌ Select unmaintained libraries (future security risk)
- ❌ Over-engineer (YAGNI principle applies)
- ❌ Trust vendor comparisons uncritically (biased)
- ❌ Skip TypeScript support check (if using TS)
- ❌ Forget to document trade-offs

**Remember**: Stack decisions have long-term consequences. Invest time in research during planning to avoid expensive migrations later. WebSearch enables data-driven, current, and well-informed technology choices.

---

## Stack Research Checklist

Before recommending a library/framework:
- [ ] Identified 2-3 candidate options
- [ ] Checked popularity (npm downloads, GitHub stars)
- [ ] Verified active maintenance (recent commits, issue response)
- [ ] Reviewed documentation quality
- [ ] Confirmed TypeScript support (if applicable)
- [ ] Checked bundle size (<50KB preferred)
- [ ] Found performance benchmarks (if performance-critical)
- [ ] Verified license compatibility
- [ ] Read community feedback (Reddit, GitHub discussions)
- [ ] Documented rationale and trade-offs
- [ ] Included official documentation links in handoff

If all checkboxes pass → Recommend with confidence.
