---
name: websearch-design-patterns
description: Protocole de recherche web pour les agents design — patterns UI/UX, composants, accessibilité, design systems de référence.
---

# WebSearch Design Patterns — Design Agent Protocol

**Version**: 1.0.0  
**Target**: Design agents (ux-designer, ui-designer)  
**Extends**: `skills/shared/websearch-usage.md`

## Purpose

This skill provides a specialized protocol for using WebSearch to discover **UI/UX patterns**, **design trends**, **accessibility guidelines**, and **component examples** during design and prototyping phases. It enables evidence-based design decisions grounded in current best practices.

## When to Use Design Pattern Research

### Trigger Conditions
Use WebSearch for design research when:
1. **Component Design**: Designing complex UI components (datepickers, modals, forms)
2. **Pattern Discovery**: Unknown how to approach a UX problem
3. **Accessibility Standards**: Verifying WCAG compliance requirements
4. **Trend Research**: Understanding current design language trends
5. **Inspiration Gathering**: Finding reference examples for visual design
6. **Responsive Design**: Mobile-first patterns and breakpoint strategies

### Design Phase Integration
```
UX Designer Phase:
→ Research interaction patterns
→ WebSearch: User flows, wireframe examples, accessibility patterns

UI Designer Phase:
→ Research visual patterns
→ WebSearch: Color systems, typography, component libraries, Figma resources
```

## Design Pattern Query Patterns

### Pattern 1: Component Patterns
```
✅ "dashboard sidebar navigation pattern 2026"
✅ "multi-step form UX best practices"
✅ "data table pagination patterns"
✅ "mobile navigation drawer patterns"
✅ "date range picker UX design"
```

### Pattern 2: Interaction Patterns
```
✅ "infinite scroll vs pagination UX research"
✅ "drag and drop UI patterns accessibility"
✅ "loading state best practices 2026"
✅ "error message design patterns"
✅ "empty state design examples"
```

### Pattern 3: Accessibility Research
```
✅ "WCAG 2.2 color contrast requirements"
✅ "accessible modal dialog patterns"
✅ "screen reader navigation best practices"
✅ "keyboard navigation patterns web app"
✅ "focus management React accessibility"
```

### Pattern 4: Visual Design Trends
```
✅ "dashboard UI design trends 2026"
✅ "SaaS landing page design patterns"
✅ "dark mode design system 2026"
✅ "glassmorphism UI trends"
✅ "minimalist dashboard design examples"
```

### Pattern 5: Design Systems
```
✅ "design system color palette generation"
✅ "design tokens best practices 2026"
✅ "Figma design system setup guide"
✅ "component library documentation examples"
✅ "spacing scale design system"
```

## Evaluation Criteria for Design Decisions

When researching design patterns, evaluate on:

### 1. Usability
```
Search for:
- User research findings
- A/B test results
- Nielsen Norman Group articles
- Baymard Institute studies

Questions:
- Is the pattern intuitive for users?
- Does it reduce cognitive load?
- Is it familiar or innovative?
```

### 2. Accessibility (WCAG 2.2)
```
Search for:
- "WCAG 2.2 {component} requirements"
- "{pattern} accessibility best practices"
- "screen reader {component} support"

Minimum standards:
✅ Level AA compliance (required)
✅ Keyboard navigable
✅ Sufficient color contrast (4.5:1 text, 3:1 UI elements)
✅ Screen reader compatible
✅ Focus indicators visible
```

### 3. Responsiveness
```
Search for:
- "{pattern} mobile responsive design"
- "responsive {component} breakpoints"
- "mobile-first {pattern}"

Breakpoint standards:
- Mobile: 320px - 768px
- Tablet: 768px - 1024px
- Desktop: 1024px+
```

### 4. Performance Impact
```
Search for:
- "{pattern} performance implications"
- "heavy animation performance cost"
- "image optimization web design 2026"

Considerations:
- Animation impact on main thread
- Image size and loading strategies
- Third-party embeds cost
```

### 5. Design System Fit
```
Questions:
- Does this pattern exist in Material Design / Ant Design / Chakra UI?
- Can it be built with existing design tokens?
- Does it align with brand guidelines?
```

## Workflow Integration

### UX Designer → WebSearch Workflow
```
1. UX DESIGNER: Identify interaction challenge
   → Problem: "Users confused by multi-step checkout flow"

2. WEBSEARCH: Research patterns
   Query: "multi-step checkout UX best practices 2026"
   Findings:
   - Show progress indicator (Baymard study: 23% cart abandonment reduction)
   - Allow backward navigation
   - Save progress automatically

3. WEBSEARCH: Accessibility requirements
   Query: "WCAG multi-step form accessibility"
   Requirements:
   - ARIA live regions for step updates
   - Keyboard navigation between steps
   - Error summary at top

4. UX DESIGNER: Wireframe solution
   → Include: Step indicator, back button, auto-save, error handling
   → Cite sources in design doc
```

### UI Designer → WebSearch Workflow
```
1. UI DESIGNER: Implement visual design for feature
   → Task: "Design dashboard data visualization cards"

2. WEBSEARCH: Gather inspiration
   Query: "dashboard data card design examples 2026"
   Sources:
   - Dribbble dashboard shots
   - SaaS dashboard screenshot collections
   - Design system card components (Ant Design, Material)

3. WEBSEARCH: Design system guidance
   Query: "card component design tokens Figma"
   → Fetch: Figma design system card structure examples

4. WEBSEARCH: Accessibility check
   Query: "accessible card component WCAG"
   → Ensure: Semantic HTML, sufficient contrast, keyboard focus

5. UI DESIGNER: Create Figma component
   → Variants: Default, hover, focus, disabled
   → Document: Usage guidelines, accessibility notes
   → Link: Research sources in Figma description
```

## Design Research Sources (Priority Order)

### 1. Authoritative UX Research
- **Nielsen Norman Group**: https://www.nngroup.com (research-backed patterns)
- **Baymard Institute**: https://baymard.com (e-commerce UX research)
- **Smashing Magazine**: https://www.smashingmagazine.com (design articles)
- **A List Apart**: https://alistapart.com (web standards, accessibility)

### 2. Accessibility Guidelines
- **WCAG 2.2**: https://www.w3.org/WAI/WCAG22/quickref/ (official standards)
- **WebAIM**: https://webaim.org (accessibility resources)
- **Inclusive Components**: https://inclusive-components.design (accessible patterns)
- **A11y Project**: https://www.a11yproject.com (accessibility checklist)

### 3. Component Libraries (Reference)
- **Material Design 3**: https://m3.material.io (Google design system)
- **Ant Design**: https://ant.design (comprehensive component library)
- **Chakra UI**: https://chakra-ui.com (accessible React components)
- **Radix UI**: https://www.radix-ui.com (unstyled primitives)
- **shadcn/ui**: https://ui.shadcn.com (Tailwind + Radix examples)

### 4. Visual Inspiration
- **Dribbble**: https://dribbble.com (UI shots, trends)
- **Behance**: https://www.behance.net (case studies, full projects)
- **Awwwards**: https://www.awwwards.com (award-winning web design)
- **Mobbin**: https://mobbin.com (mobile app design patterns)
- **UI Garage**: https://uigarage.net (UI pattern library)

### 5. Design Systems
- **Design Systems Repo**: https://designsystemsrepo.com (curated design systems)
- **Adele**: https://adele.uxpin.com (design system gallery)
- **Component Gallery**: https://component.gallery (component examples)

## Design Pattern Research Template

```markdown
## Design Pattern Research: {Component/Pattern Name}

**Use Case**: {Specific design challenge}  
**Date**: 2026-05-29  
**Designer**: {Name}

### Problem Statement
{Describe the UX/UI challenge requiring research}

Example: "Users need to filter a large dataset (10K+ items) by multiple criteria without leaving the page or sacrificing performance."

### Research Conducted

**Query 1**: "data table filter patterns UX best practices"  
**Top Sources**:
- Nielsen Norman Group: Filtering patterns (https://...)
- Material Design: Data tables (https://...)

**Key Findings**:
- Multi-select filters reduce time-to-result by 35% (NNG study)
- Inline filtering > separate filter modal (lower cognitive load)
- Show count of results for each filter option (transparency)

**Query 2**: "accessible filter component WCAG 2.2"  
**Accessibility Requirements**:
- Use <fieldset> + <legend> for filter groups
- ARIA live region for result count updates
- Keyboard shortcuts for common filters (optional enhancement)
- Clear all filters button (required)

### Design Decision

**Selected Pattern**: Inline multi-select filters with result count preview

**Rationale**:
1. Research shows 35% efficiency gain (NNG)
2. Familiar pattern (used by Airbnb, Amazon, LinkedIn)
3. Meets WCAG 2.2 AA with proper markup
4. Can be built with existing design tokens

**Visual Reference**:
- Dribbble example: [URL]
- Material Design spec: [URL]

### Accessibility Checklist
- [x] Keyboard navigable
- [x] Screen reader announcements (ARIA live region)
- [x] Sufficient color contrast (checked with WebAIM tool)
- [x] Focus indicators visible
- [x] Mobile responsive (tested breakpoints)

### Implementation Notes
- Component: `FilterPanel.tsx`
- Design tokens: spacing-4, color-primary, border-radius-md
- Figma link: [Component page URL]
- Dependencies: None (native HTML/CSS)

### Sources
1. Nielsen Norman Group - Filtering Patterns: https://...
2. Material Design - Data Tables: https://...
3. WebAIM Contrast Checker: https://...
4. WCAG 2.2 Filterform Success Criteria: https://...
```

## Common Design Challenges & Search Strategies

### Challenge 1: Mobile Navigation
```
Problem: Designing navigation for mobile app with 20+ menu items

Search Strategy:
1. "mobile navigation patterns 2026 many items"
2. "hamburger menu vs bottom tab bar UX research"
3. "nested navigation mobile best practices"

Expected Findings:
- Hybrid approach (primary tabs + hamburger for secondary)
- Nielsen research on menu discoverability
- Accessibility considerations (reachability zones)
```

### Challenge 2: Form Design
```
Problem: Long registration form (15+ fields) with high abandonment rate

Search Strategy:
1. "long form UX best practices reduce abandonment"
2. "multi-step form vs single page form UX research"
3. "form field design WCAG accessibility"

Expected Findings:
- Multi-step reduces perceived effort
- Show progress indicator (reduces uncertainty)
- Inline validation improves completion rate
- WCAG: Error identification, labels, instructions
```

### Challenge 3: Data Visualization
```
Problem: Display complex financial data in dashboard

Search Strategy:
1. "financial dashboard data visualization best practices"
2. "chart types comparison data visualization"
3. "accessible data visualization WCAG"

Expected Findings:
- Bar charts for comparison, line for trends
- Avoid pie charts (hard to compare)
- Color-blind safe palettes (use patterns too)
- Provide data table alternative (accessibility)
```

### Challenge 4: Loading States
```
Problem: API calls take 2-5 seconds, users think app is frozen

Search Strategy:
1. "loading state UX patterns 2026"
2. "skeleton screen vs spinner UX research"
3. "perceived performance optimization techniques"

Expected Findings:
- Skeleton screens reduce perceived wait time
- Progress indicators for >2 second operations
- Optimistic UI updates (show action immediately)
- WebSearch: "Perceived performance by Luke Wroblewski"
```

### Challenge 5: Dark Mode
```
Problem: Implement dark mode that's WCAG compliant and visually appealing

Search Strategy:
1. "dark mode design system best practices 2026"
2. "dark mode color contrast WCAG"
3. "dark mode design tokens Figma"

Expected Findings:
- Not pure black (#000), use #121212 or similar (reduces eye strain)
- Maintain 4.5:1 contrast for text
- Invert luminance, not colors (brand colors stay recognizable)
- Test with system preference (prefers-color-scheme)
```

## Rate Limit Strategy

Design research can involve many visual references. Optimize:

### Front-load Pattern Research
```
Early project phase:
✅ "SaaS dashboard design patterns 2026"
✅ "design system examples 2026"
✅ "web app UX patterns library"
→ Gather broad inspiration once
→ Save URLs for later reference
```

### Use Known Design Systems
Instead of searching each component:
```
❌ Search: "button design patterns"
❌ Search: "input field design patterns"
❌ Search: "modal design patterns"

✅ Fetch once: Material Design 3 component library
✅ Fetch once: Ant Design documentation
→ Reference these systems for all components
```

### Batch Accessibility Research
```
✅ "WCAG 2.2 quick reference checklist"
✅ webfetch("https://www.w3.org/WAI/WCAG22/quickref/")
→ Comprehensive accessibility guide
→ Reference for all components, no need to search per component
```

## Error Handling

### Scenario 1: Conflicting Design Advice
```
Source A: "Always use hamburger menu for mobile"
Source B: "Hamburger menus hurt discoverability"

Action:
1. Check publication dates (UX evolves, recent = better)
2. Check study methodology (A/B tests > opinions)
3. Check context (content-heavy sites ≠ task-based apps)
4. Report: "Mixed guidance, recommend user testing with our audience"
```

### Scenario 2: Inaccessible Trend
```
Trend: Glassmorphism (blurred backgrounds)
Problem: Low contrast, WCAG violation

Action:
1. WebSearch: "glassmorphism accessibility issues"
2. Find: Contrast issues confirmed by WebAIM
3. Decision: Use glassmorphism for decorative elements only, not interactive UI
4. Document: "Visual trend adopted with accessibility constraints"
```

### Scenario 3: No Research Backing
```
Client request: "I saw this on Dribbble, let's do that"
Problem: Novel pattern with no UX research

Action:
1. WebSearch: "{pattern} usability testing results"
2. If no results: Note lack of research
3. Recommend: "Innovative pattern, but unproven. Suggest user testing before full implementation"
4. Alternative: Propose similar proven pattern as safer option
```

## Integration with Figma

### Research → Figma Workflow
```
1. WebSearch: Gather references
   → Save URLs in Figma file description or Miro board

2. Create Figma mood board
   → Import screenshots of inspiring examples
   → Label with source URLs

3. Design components
   → Reference mood board
   → Annotate accessibility requirements from research

4. Document usage
   → Include WCAG guidelines from WebSearch
   → Link to research sources
   → Provide implementation notes for developers
```

### Figma Plugin Recommendations
If searching for Figma resources:
```
✅ "Figma design system templates 2026"
✅ "Figma accessibility plugins WCAG"
✅ "Figma component library best practices"
```

## Best Practices Summary

**DO**:
- ✅ Research patterns before designing (avoid reinventing)
- ✅ Verify accessibility with WCAG 2.2 standards
- ✅ Cite sources in design documentation
- ✅ Test designs against research findings
- ✅ Gather quantitative data (A/B tests, studies) over opinions
- ✅ Check mobile patterns separately (≠ desktop patterns)
- ✅ Include dark mode considerations
- ✅ Save inspiration URLs for future reference

**DON'T**:
- ❌ Copy designs without understanding rationale
- ❌ Follow trends blindly (test against usability/accessibility)
- ❌ Ignore research that contradicts your preference
- ❌ Design without accessibility research
- ❌ Use Dribbble as sole source (visual ≠ usable)
- ❌ Skip responsive design research
- ❌ Trust vendor marketing as research (biased)

**Remember**: Great design is **evidence-based**. WebSearch provides access to user research, accessibility standards, and proven patterns. Always validate findings with user testing when possible.

---

## Design Research Checklist

Before finalizing a design:
- [ ] Researched similar patterns from authoritative sources (NNG, Material, etc.)
- [ ] Verified WCAG 2.2 AA compliance requirements
- [ ] Checked responsive behavior patterns (mobile/tablet/desktop)
- [ ] Reviewed accessibility patterns (keyboard nav, screen readers, contrast)
- [ ] Gathered visual references (Dribbble, Behance, component libraries)
- [ ] Evaluated performance implications (animations, images, embeds)
- [ ] Checked design system alignment (tokens, components, spacing)
- [ ] Documented rationale and sources in Figma/design doc
- [ ] Included implementation notes for developers
- [ ] Planned user testing validation (if novel pattern)

If all checkboxes pass → Ship design with confidence.
