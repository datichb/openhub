---
name: websearch-performance-research
description: Protocole de recherche web pour les audits de performance — Core Web Vitals, benchmarks, patterns d'optimisation.
---

# WebSearch Performance Research — Performance Auditor Protocol

**Version**: 1.0.0  
**Target**: Performance auditors (performance-auditor)  
**Extends**: `skills/shared/websearch-usage.md`

## Purpose

This skill provides a specialized protocol for using WebSearch to discover **performance optimization techniques**, **benchmarks**, and **best practices** during performance audits. It complements profiling data with industry research and current trends.

## When to Use Performance Research

### Trigger Conditions
Use WebSearch for performance research when:
1. **Performance Bottleneck Identified**: After profiling shows slow code path
2. **Framework/Library Optimization**: Seeking best practices for specific tech stack
3. **Benchmark Comparison**: Validating if observed metrics are acceptable
4. **Alternative Solutions**: Finding faster libraries or patterns
5. **Emerging Techniques**: Discovering new optimization strategies (2025-2026)

### Priority Targets
Focus performance research on:
- **Frontend frameworks**: React, Vue, Angular, Svelte (rendering, hydration)
- **Backend frameworks**: Express, Fastify, Hapi, NestJS (request handling)
- **Databases**: Query optimization, indexing strategies, connection pooling
- **Build tools**: Webpack, Vite, esbuild, Turbopack (bundle size, build time)
- **Runtime**: Node.js, Deno, Bun performance characteristics

## Performance Search Query Patterns

### Pattern 1: Framework-Specific Optimization
```
✅ "React 18 performance optimization large lists"
✅ "Next.js 14 bundle size reduction techniques"
✅ "Vue 3 reactivity performance best practices"
✅ "Svelte compiler optimization 2026"
```

### Pattern 2: Library Benchmarks
```
✅ "axios vs fetch vs got performance benchmark 2026"
✅ "Express vs Fastify vs Hapi benchmark Node.js 20"
✅ "date-fns vs dayjs vs Luxon bundle size comparison"
✅ "Lodash vs Ramda performance 2026"
```

### Pattern 3: Technique-Specific
```
✅ "React virtual scrolling performance"
✅ "Node.js streams memory efficiency"
✅ "Postgres JSONB indexing performance"
✅ "Redis caching strategy best practices"
```

### Pattern 4: Problem-Solution
```
✅ "slow API response Node.js troubleshooting"
✅ "React rendering performance fix"
✅ "database query optimization N+1 problem"
✅ "Webpack bundle size too large solution"
```

## Performance Metrics & Benchmarks

### Key Metrics to Research
When searching for benchmarks, focus on:

#### Frontend Metrics
- **FCP (First Contentful Paint)**: Target <1.8s (good), <3s (acceptable)
- **LCP (Largest Contentful Paint)**: Target <2.5s (good), <4s (acceptable)
- **TTI (Time to Interactive)**: Target <3.8s (good), <7.3s (acceptable)
- **CLS (Cumulative Layout Shift)**: Target <0.1 (good), <0.25 (acceptable)
- **Bundle Size**: <100KB initial JS (gzipped), <500KB total
- **Lighthouse Score**: >90 (good), >50 (acceptable)

#### Backend Metrics
- **Request Latency**: P50 <100ms, P95 <500ms, P99 <1s
- **Throughput**: Requests per second (RPS) for given hardware
- **Database Query Time**: <50ms (simple), <200ms (complex)
- **Memory Usage**: <512MB (small app), <2GB (medium), <8GB (large)
- **CPU Usage**: <50% average, <80% peak

### Search Query Examples
```
✅ "acceptable API response time 2026"
✅ "web vitals thresholds Google 2026"
✅ "Node.js Express throughput benchmark"
✅ "React component render time acceptable range"
```

## Workflow Integration

### Step 1: Profiling (Pre-Search)
Gather performance data locally:
```bash
# Frontend profiling
- Chrome DevTools Performance tab
- Lighthouse CI
- Web Vitals extension
- Bundle analyzer (webpack-bundle-analyzer)

# Backend profiling
- Node.js --inspect + Chrome DevTools
- clinic.js (flame graphs, bubbleprof)
- 0x (flame graphs)
- autocannon (load testing)

# Database profiling
- PostgreSQL EXPLAIN ANALYZE
- MySQL EXPLAIN
- MongoDB .explain("executionStats")
```

### Step 2: Identify Optimization Targets
Prioritize by impact:
```
HIGH IMPACT (search first):
- Code paths in hot loop (>1000 calls/sec)
- Render-blocking operations (FCP/LCP)
- Large bundle contributors (>50KB)
- Slow database queries (>200ms)

MEDIUM IMPACT:
- Moderate frequency operations (100-1000 calls/sec)
- Non-critical renders (below-fold content)
- Medium bundles (10-50KB)

LOW IMPACT:
- Rare operations (<100 calls/sec)
- Background tasks
- Small utilities (<10KB)
```

### Step 3: Execute Performance Research
For each HIGH impact target:
```
1. Search for best practices:
   "React useCallback useMemo performance best practices 2026"

2. Search for benchmarks:
   "React re-render optimization benchmark"

3. Search for alternatives:
   "React virtualization library comparison 2026"
   (react-window vs react-virtuoso vs @tanstack/virtual)
```

### Step 4: Validate Findings
For each optimization technique found:
```
EXTRACT:
- Technique name (e.g., "React.memo for expensive components")
- Performance improvement (e.g., "40% render time reduction")
- Implementation complexity (Low/Medium/High)
- Trade-offs (e.g., "increased memory usage")
- Compatibility (React version, browser support)

VALIDATE:
- Is the benchmark from a reputable source? (web.dev, official docs, research papers)
- Is the data recent? (2024-2026 preferred)
- Is the scenario comparable to ours? (similar scale, stack, constraints)

FETCH DETAILS:
- Use webfetch on detailed guides or official documentation
- Look for code examples and implementation guides
```

### Step 5: Implement & Measure
After research, implement and verify:
```
1. Baseline measurement (before optimization)
2. Implement optimization based on research
3. Post-optimization measurement
4. Compare: Did we achieve expected improvement?
5. If yes → Report success + source research
   If no → Document why (different context, outdated info)
```

## Common Performance Patterns to Research

### React Performance
```
Search Topics:
- "React.memo vs useMemo vs useCallback when to use"
- "React lazy loading components code splitting"
- "React Context performance optimization"
- "React key prop performance impact"
- "React useTransition useDeferredValue 2026"
```

### Next.js Performance
```
Search Topics:
- "Next.js Image optimization best practices"
- "Next.js dynamic imports performance"
- "Next.js ISR vs SSR vs SSG performance comparison"
- "Next.js bundle size optimization 2026"
- "Next.js Server Components performance"
```

### Node.js Performance
```
Search Topics:
- "Node.js event loop blocking detection"
- "Node.js worker threads when to use"
- "Node.js cluster mode performance"
- "Node.js async/await vs Promises performance"
- "Node.js 20 performance improvements"
```

### Database Performance
```
Search Topics:
- "PostgreSQL index optimization strategies"
- "MongoDB aggregation pipeline performance"
- "N+1 query problem solution Prisma"
- "database connection pooling best practices"
- "SQL query optimization common mistakes"
```

### Bundle Optimization
```
Search Topics:
- "Webpack tree shaking configuration 2026"
- "Vite code splitting strategies"
- "import cost analysis tools"
- "moment.js alternatives smaller bundle"
- "dynamic imports best practices"
```

## Reporting Performance Research

### Research Finding Template
```markdown
## Performance Optimization: [Component/Feature]

### Issue Identified
**Location**: src/components/DataTable.tsx  
**Metric**: LCP 4.2s (slow), render time 850ms  
**Impact**: User-facing, blocking interaction  
**Priority**: HIGH

### Research Conducted
**Query**: "React large table virtualization performance 2026"  
**Sources**:
- web.dev React performance guide (authoritative, 2026)
- TanStack Virtual documentation (official, v3.0.0)
- React virtualization benchmark (GitHub, 2025)

### Key Findings
1. **Virtualization Recommended**: Render only visible rows
   - Expected improvement: 70-90% render time reduction
   - Libraries: @tanstack/virtual, react-window, react-virtuoso

2. **Benchmark Data** (source: github.com/user/benchmark):
   - 10,000 rows without virtualization: 1200ms render
   - 10,000 rows with @tanstack/virtual: 150ms render
   - 88% improvement confirmed

3. **Implementation Complexity**: Medium
   - Estimated effort: 4-6 hours
   - Breaking changes: None
   - Dependencies: Add @tanstack/virtual (15KB gzipped)

### Recommendation
Implement @tanstack/virtual for DataTable component.
Expected outcome: LCP <2.5s (good), render time <200ms

### Implementation Notes
- Use `useVirtualizer` hook with row height 48px
- Maintain accessibility (keyboard navigation, screen readers)
- Test with 10K+ rows dataset
- Measure before/after with Lighthouse

### References
- https://tanstack.com/virtual/latest/docs/framework/react/react-virtual
- https://web.dev/virtualize-long-lists-react-window/
```

## Rate Limit Strategy

Performance research can require extensive searches. Optimize:

### Bundle Searches by Category
Instead of:
```
❌ Search 1: "React performance"
❌ Search 2: "React memo"
❌ Search 3: "React useMemo"
```

Use:
```
✅ Search 1: "React optimization techniques memo useMemo useCallback 2026"
   → Comprehensive overview
✅ Search 2: "React performance benchmarks 2026"
   → Metrics and comparisons
```

### Use WebFetch for Known Resources
If you know authoritative sources:
```
✅ webfetch("https://web.dev/fast/")
✅ webfetch("https://react.dev/learn/render-and-commit")
✅ webfetch("https://nodejs.org/en/docs/guides/dont-block-the-event-loop")
```

### Prioritize Official Documentation
Search format:
```
✅ "React official performance guide"
   → Often better than searching generic terms
✅ "Next.js documentation Image optimization"
   → Targeted, authoritative
```

## Error Handling

### Scenario 1: Conflicting Advice
```
Source A: "Always use React.memo"
Source B: "React.memo is overused, avoid premature optimization"

Action:
1. Check publication dates (newer = better for evolving frameworks)
2. Check authority (official docs > blog posts)
3. Check context (large apps vs small apps)
4. Report: "Conflicting guidance found, recommend profiling-driven approach"
```

### Scenario 2: Outdated Benchmarks
```
Search: "React performance 2020"
Problem: React 18 introduced significant changes (Concurrent Mode, Automatic Batching)

Action:
1. Add year filter: "React 18 performance 2024-2026"
2. Verify React version compatibility
3. Note: "Benchmark may not apply to React 18+ Concurrent features"
```

### Scenario 3: Non-Reproducible Results
```
Benchmark claims: "90% improvement"
Our implementation: "10% improvement"

Action:
1. Check benchmark conditions (hardware, dataset size, browser)
2. Verify our implementation matches benchmark setup
3. Report: "Expected 90%, achieved 10% — likely due to [difference in context]"
4. Still valuable if 10% is meaningful
```

## Integration with Local Profiling

### Hybrid Approach
Combine WebSearch with local tools:

```
1. LOCAL PROFILING: Identify slow operations
   → Chrome DevTools: Component renders 500ms (slow)

2. WEBSEARCH: Research solutions
   → "React rendering performance optimization 2026"
   → Find: Use React.memo, useCallback, code splitting

3. LOCAL TESTING: Validate techniques
   → Implement React.memo
   → Measure: Now renders 150ms (70% faster)

4. REPORT: Cite research + local measurements
   → "Applied React.memo based on research from web.dev"
   → "Confirmed 70% render time reduction (500ms → 150ms)"
```

## Best Practices Summary

**DO**:
- ✅ Always profile locally BEFORE searching (understand the problem)
- ✅ Search for recent techniques (2024-2026)
- ✅ Validate findings with local measurements
- ✅ Cite sources and benchmark data
- ✅ Consider trade-offs (complexity vs performance gain)
- ✅ Focus on user-facing metrics (LCP, TTI, perceived performance)
- ✅ Report expected vs actual improvements

**DON'T**:
- ❌ Implement optimizations without measuring baseline
- ❌ Trust outdated benchmarks (>2 years old for fast-moving frameworks)
- ❌ Optimize for vanity metrics (micro-benchmarks without real impact)
- ❌ Apply every optimization found (risk over-engineering)
- ❌ Ignore complexity costs (harder maintenance vs marginal gains)
- ❌ Skip validation (measure before/after)

**Remember**: Performance optimization is **data-driven**. WebSearch provides techniques and benchmarks, but local profiling and measurement are critical for validating real-world impact.

---

## Performance Research Checklist

Before recommending an optimization:
- [ ] Local profiling data captured (baseline metrics)
- [ ] WebSearch conducted for best practices
- [ ] Benchmark data from reputable source (official docs, research papers)
- [ ] Publication date recent (2024-2026 preferred)
- [ ] Implementation complexity assessed (effort vs gain)
- [ ] Trade-offs documented (memory, complexity, compatibility)
- [ ] Expected improvement quantified (e.g., "30-50% reduction")
- [ ] Local validation planned (test, measure, compare)
- [ ] User impact clear (affects LCP, TTI, or other user-facing metric)

If all checkboxes pass → Recommend optimization with confidence.
