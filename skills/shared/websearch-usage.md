# WebSearch Usage — Best Practices

**Version**: 1.0.0  
**Target**: All agents with `permission.websearch = allow`  
**Provider**: Exa AI (hosted by OpenCode, no API key required)

## Purpose

This skill provides best practices for using the `websearch` tool effectively in openhub agents. WebSearch enables agents to discover current information, documentation, CVEs, best practices, and design trends beyond the training data cutoff.

## Prerequisites

### Hub-level Activation
WebSearch must be enabled in `openhub/opencode.json`:

```json
{
  "permission": {
    "websearch": "allow",
    "webfetch": "allow"
  }
}
```

### Agent-level Permission
Each agent must declare WebSearch permission in its frontmatter:

```yaml
permission:
  websearch: allow
  webfetch: allow
```

## When to Use WebSearch vs WebFetch

### Use `websearch` for:
- **Discovery**: Finding information, documentation, or resources
- **Research**: Exploring current events, trends, or ecosystem changes
- **CVE/Advisory Lookup**: Searching for security vulnerabilities
- **Best Practices**: Finding up-to-date patterns, techniques, or recommendations
- **Exploration**: Discovering libraries, tools, or frameworks

### Use `webfetch` for:
- **Retrieval**: Getting content from a specific known URL
- **Documentation Reading**: Fetching official docs after discovering the URL
- **Verification**: Retrieving a specific page to confirm details

### Workflow: Search → Fetch
```
1. websearch: "React Server Components best practices 2026"
   → Returns: [url1, url2, url3, ...]
   
2. webfetch: url1
   → Returns: Full page content in markdown
   
3. Analyze and synthesize findings
```

## Search Query Best Practices

### 1. Be Specific and Contextual
❌ Bad:  `"authentication"`  
✅ Good: `"Next.js 14 authentication with NextAuth v5 best practices"`

❌ Bad:  `"react performance"`  
✅ Good: `"React 18 performance optimization techniques for large lists"`

### 2. Include Version Numbers
When searching for framework/library-specific info:
- `"Django 5.0 async views tutorial"`
- `"Vue 3 Composition API patterns"`
- `"TypeScript 5.4 new features"`

### 3. Use Time-Sensitive Keywords
For recent information:
- `"CVE 2026"` or `"CVE-2026-*"`
- `"React trends 2026"`
- `"latest webpack configuration"`

### 4. Combine Technology Stack Terms
For stack-specific research:
- `"AWS Lambda Node.js 20 cold start optimization"`
- `"Postgres 16 JSONB indexing performance"`
- `"Kubernetes 1.29 security best practices"`

### 5. Use Domain-Specific Terminology
For professional results:
- `"OAuth 2.1 authorization code flow security advisories"`
- `"WCAG 2.2 contrast ratio requirements"`
- `"OWASP Top 10 2025 mitigation strategies"`

## Example Queries by Use Case

### Security Research (Auditors)
```
websearch("CVE-2026 Express.js")
websearch("npm package lodash security advisories 2026")
websearch("OWASP injection prevention Node.js")
websearch("GitHub security advisory GHSA-2026")
```

### Documentation Discovery (Planning)
```
websearch("Tailwind CSS v4 documentation")
websearch("Next.js 15 App Router migration guide")
websearch("Prisma 6 schema reference")
```

### Stack Research (Planning)
```
websearch("headless CMS comparison 2026 Sanity Contentful Strapi")
websearch("React state management Zustand vs Jotai benchmarks")
websearch("serverless database PlanetScale vs Neon vs Supabase")
```

### Design Patterns (Design)
```
websearch("dashboard UI design patterns 2026")
websearch("mobile navigation best practices accessibility")
websearch("dark mode color palette design system")
```

### Performance Research (Auditors)
```
websearch("Next.js bundle size optimization techniques")
websearch("React rendering performance profiling tools")
websearch("PostgreSQL query optimization indexing strategies")
```

## Rate Limits and Constraints

### Hosted Service
- WebSearch is **hosted by OpenCode** (Exa AI MCP service)
- **No API key required** — authentication handled automatically
- Subject to OpenCode's rate limits (details TBD)

### Best Practices to Avoid Rate Limits
1. **Batch related queries**: Search once with comprehensive terms instead of multiple narrow searches
2. **Cache results**: If you found relevant URLs, use `webfetch` directly on subsequent runs
3. **Prioritize quality over quantity**: 1-3 targeted searches > 10 vague searches
4. **Use search operators**: Leverage Exa's advanced search syntax (if supported)

## Error Handling

### Common Errors
```
Error: "WebSearch tool not available"
→ Check: permission.websearch = allow in opencode.json
→ Check: permission.websearch = allow in agent frontmatter

Error: "Rate limit exceeded"
→ Wait and retry
→ Reduce search frequency
→ Use webfetch for known URLs instead

Error: "No results found"
→ Broaden search terms
→ Remove overly specific version numbers
→ Try alternative phrasing
```

### Graceful Degradation
If WebSearch fails:
1. Log the error clearly for debugging
2. Fallback to training data knowledge (with caveat about potential staleness)
3. Suggest manual research to the user with specific keywords
4. Continue with the task using available information

Example:
```
❌ WebSearch failed for "CVE-2026-12345"
✓ Continuing analysis with known patterns...
⚠ Note: Unable to verify latest CVE details — recommend manual check at https://cve.mitre.org
```

## Integration with Agent Workflows

### Auditor Workflow
```
1. Static Analysis → Identify potential issues
2. WebSearch → "CVE {package} {version}"
3. WebFetch → Retrieve CVE details from official advisories
4. Report → Include CVE IDs, severity, mitigation steps
```

### Planning Workflow
```
1. Requirements Analysis → Identify unknown libraries/patterns
2. WebSearch → "library comparison {feature} {tech stack}"
3. WebFetch → Documentation URLs for top candidates
4. Recommendation → Suggest best-fit library with rationale
```

### Design Workflow
```
1. Design Requirements → Identify UI/UX needs
2. WebSearch → "{component type} design patterns {year}"
3. WebFetch → Dribbble, Behance, or design system examples
4. Mockup → Create design inspired by current trends
```

## Security Considerations

### Trusted Sources
Prefer results from:
- Official documentation (e.g., `react.dev`, `nextjs.org`)
- Security advisories (e.g., `cve.mitre.org`, `nvd.nist.gov`, `github.com/advisories`)
- Reputable blogs (e.g., `web.dev`, `css-tricks.com`, `smashingmagazine.com`)

### Verification
- **Always verify** information from multiple sources if critical
- **Cross-reference** CVE IDs with official databases
- **Check publication date** — prefer recent articles (last 1-2 years)

### Privacy
- **Do not search for**:
  - User-specific data or PII
  - Project-specific credentials or secrets
  - Proprietary code or internal architecture details
- **Keep queries generic** and industry-standard

## Output Format

When reporting WebSearch results, always include:
1. **Query used**: Exact search string for reproducibility
2. **Top results**: 2-3 most relevant URLs with brief description
3. **Key findings**: Synthesized insights (not copy-paste)
4. **Confidence**: High/Medium/Low based on source authority

Example report:
```
## Research: Next.js 15 App Router Best Practices

**Query**: "Next.js 15 App Router best practices 2026"

**Top Sources**:
- https://nextjs.org/docs/app — Official Next.js documentation (High authority)
- https://vercel.com/blog/next-15 — Vercel blog announcement (High authority)

**Key Findings**:
- Server Components by default (reduces client bundle by ~40%)
- New caching strategy with fetch() API changes
- Recommended folder structure: app/(routes)/[dynamic]/page.tsx

**Confidence**: High (official sources, recent publication)
```

## Testing WebSearch Locally

### In OpenCode TUI
```bash
# From a project with WebSearch enabled
cd /path/to/project
OPENCODE_ENABLE_EXA=1 opencode

# In conversation:
> Can you search for "React 19 new features"?
```

### Via Agent Deployment
```bash
# Deploy agents with WebSearch support
./oc.sh deploy my-project

# Launch agent
cd /path/to/my-project
oc start auditor security
```

## Troubleshooting

### WebSearch Not Available
```bash
# Check hub config
cat openhub/opencode.json | jq '.permission.websearch'

# Check agent config
cat agents/auditor/security-auditor.md | grep -A5 'permission:'

# Test with env override
OPENCODE_ENABLE_EXA=1 oc start auditor security
```

### Results Not Relevant
- Refine query with more specific terms
- Add year/version constraints
- Use quotation marks for exact phrases: `"React Server Components"`
- Add exclusion terms (if supported): `best practices -tutorial`

### Performance Issues
- Limit searches to 1-2 per session
- Cache URLs and reuse with `webfetch`
- Avoid searching during rate-limited periods

---

## Summary

**DO**:
- ✅ Use specific, contextual queries
- ✅ Combine websearch (discovery) + webfetch (retrieval)
- ✅ Include versions, years, and technical terms
- ✅ Verify critical information from multiple sources
- ✅ Report queries and sources transparently

**DON'T**:
- ❌ Search for PII, secrets, or proprietary data
- ❌ Use vague or overly broad queries
- ❌ Over-rely on a single search result
- ❌ Exceed rate limits with redundant searches
- ❌ Forget to handle errors gracefully

**Remember**: WebSearch is a **discovery tool** for current information. Always validate findings and maintain transparency about sources.
