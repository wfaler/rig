---
title: "Documentation Template Design"
type: adr
id: "0004"
status: Proposed
superseded_by: ""
prd: "project-scaffolding"
created: 2026-03-27
updated: 2026-03-27
tickets: []
---

# ADR-0004: Documentation Template Design

## Status
Proposed

## Context

The scaffolding PRD requires that `rig scaffold` generates documentation templates enforcing a PRD → ADR → Ticket hierarchy. This ADR records the specific template designs — front matter schemas, section structures, and inline guidance — that determine the generated output.

Templates serve two audiences:
1. **Humans** scanning for structure and filling in sections
2. **AI agents** parsing for actionable instructions and structured metadata

The templates must enable programmatic search, filtering, and traceability (e.g., "show me all tickets derived from ADR-0001"). The front matter is also indexed by the search/MCP server (ADR-0001) with boosted field weights.

## Decision

### Front Matter Schema

All documents use YAML front matter with structured metadata. Field names and allowed values are standardised across document types.

#### PRD Front Matter

```yaml
---
title: "[Feature Name]"
type: prd
status: Draft                    # Draft | Under Review | Approved | Superseded
feature: "[feature-slug]"       # Short identifier, used to link ADRs and tickets
created: YYYY-MM-DD
updated: YYYY-MM-DD
adrs: []                        # ADR IDs that implement this PRD, e.g. ["0001", "0002"]
---
```

#### ADR Front Matter

```yaml
---
title: "[Decision Title]"
type: adr
id: "NNNN"                      # Sequential ADR number
status: Proposed                 # Proposed | Accepted | Deprecated | Superseded
superseded_by: ""               # ADR ID if superseded
prd: "[feature-slug]"           # Which PRD this implements
created: YYYY-MM-DD
updated: YYYY-MM-DD
tickets: []                     # Ticket IDs derived from this ADR
---
```

#### Ticket Front Matter

```yaml
---
title: "[Ticket Title]"
type: ticket
id: "[PROJ-NNNN]"               # Ticket identifier
status: Open                    # Open | In Progress | Done | Blocked
adr: "NNNN"                    # ADR this ticket derives from
adr_step: ""                   # Which implementation phase/step
priority: Medium                # Critical | High | Medium | Low
created: YYYY-MM-DD
updated: YYYY-MM-DD
depends_on: []                  # Other ticket IDs that must complete first
---
```

### Template Structures

#### PRD Template (`docs/templates/prd-template.md`)

```markdown
# [Feature Name] — Product Requirements Document

## Vision
<!-- One paragraph: what does this feature enable? Why does it matter? -->

## Problem Statement
<!-- What pain point does this solve? Who experiences it? What happens if we don't solve it? -->

## Target Users
<!-- Who benefits? Be specific about roles and contexts. -->

## Requirements
<!-- High-level capabilities. Each requirement should be testable but not prescriptive
     about implementation. Use "The system should..." language. Avoid specifying technologies,
     algorithms, or data structures — those decisions belong in the ADR. -->

### Must Have
<!-- Requirements without which the feature is not viable -->

### Should Have
<!-- Requirements that significantly enhance value but aren't blocking -->

### Could Have
<!-- Nice-to-haves that can be deferred -->

## Success Criteria
<!-- How do we know this feature is working? Measurable outcomes, not implementation details. -->

## Open Questions
<!-- Unresolved decisions that need input before proceeding to ADR -->
```

**Guidance:** A PRD describes *what* and *why*, never *how*. If you're naming specific technologies, algorithms, or data structures, you're writing an ADR, not a PRD.

#### ADR Template (`docs/templates/adr-template.md`)

```markdown
# ADR-NNNN: [Decision Title]

## Status
[Proposed | Accepted | Deprecated | Superseded]

## Context
<!-- What is the issue? What constraints exist?
     What forces are at play (performance, cost, team expertise, timeline)? -->

## Decision
<!-- What did we decide? Be specific about technologies, patterns, data structures,
     and interfaces. Include enough detail that each component or change can be
     turned into a ticket with minimal ambiguity.

     Structure this section with sub-headings for each significant decision.
     For each decision, explain what it is and how it works — not just that we chose it. -->

## Alternatives Considered
<!-- What else did we evaluate? Why did we reject it? Be honest about trade-offs. -->

## Consequences
<!-- What are the implications? Include both positive and negative.
     What becomes easier? What becomes harder? What new constraints are introduced? -->

## Implementation Plan
<!-- Ordered phases or steps. Each step should map to one or more tickets.
     A step is ready to become a ticket when:
     - The scope is unambiguous (someone could implement it without asking clarifying questions)
     - The acceptance criteria are clear
     - Dependencies on other steps are explicit -->
```

**Guidance:** An ADR describes *how* and records *why this way and not another*. The implementation plan should be detailed enough that each step can become a ticket without further architectural discussion.

#### Ticket Template (`docs/templates/ticket-template.md`)

```markdown
# [Ticket Title]

## Description
<!-- What needs to be built or changed? Be specific. Reference the ADR for context,
     but include enough detail that someone can implement this without reading the ADR. -->

## Acceptance Criteria
<!-- Checkboxes. Each criterion is independently verifiable.
     Include both functional criteria and quality criteria. -->

- [ ] [Functional: what the code does]
- [ ] [Functional: edge cases handled]
- [ ] Unit tests covering all public functions
- [ ] Integration tests for external dependencies (if applicable)
- [ ] Property-based tests for functions with broad input ranges (if applicable)
- [ ] Mutation testing passes (80%+ kill rate for changed code)
- [ ] Linting passes with no new suppressions
- [ ] Documentation updated (if user-facing)

## Scope Boundaries
<!-- What is explicitly NOT part of this ticket? Prevents scope creep. -->
```

**Guidance:** A ticket is the atomic unit of work. It should be implementable in a single PR by one developer (human or AI). The acceptance criteria are a contract — the ticket is done when all boxes are checked.

## Alternatives Considered

### No front matter (plain markdown)
Rejected. Without structured metadata, documents can't be programmatically searched, filtered, or linked. The search/MCP server (ADR-0001) relies on front matter for indexing.

### JSON front matter instead of YAML
Rejected. YAML is more readable for humans writing documentation. Markdown parsers universally support YAML front matter.

### Separate metadata files instead of front matter
Rejected. Keeping metadata in the same file as content ensures they stay in sync. Separate files create a maintenance burden and can drift.

## Consequences

- All project documentation follows a consistent structure
- Front matter enables programmatic traceability: PRD → ADR → Ticket links can be validated automatically
- Templates include inline guidance comments that teach the hierarchy — developers learn the workflow by reading the templates
- The search/MCP server can index documents by type, status, and relationships using front matter fields

## Implementation Plan

1. **Create template files** — generate `docs/templates/prd-template.md`, `adr-template.md`, `ticket-template.md` with front matter and section structure
2. **Wire into scaffold generator** — templates are always generated regardless of language config
3. **Add front matter validation** — optional linting step that checks for required fields and valid cross-references
