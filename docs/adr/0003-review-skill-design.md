---
title: "Review Skill Design Specifications"
type: adr
id: "0003"
status: Proposed
superseded_by: ""
prd: "project-scaffolding"
created: 2026-03-27
updated: 2026-03-27
tickets: []
---

# ADR-0003: Review Skill Design Specifications

## Status
Proposed

## Context

The scaffolding PRD requires four read-only review skills that analyse code and documentation without modifying anything. This ADR defines the detailed behaviour, review checklists, output formats, and personas for each skill. These specifications are implementation-level design decisions — they determine *how* each skill behaves, not *what* the system should do (which is the PRD's job).

The skills must be:
- **Read-only** — they report findings but never modify files, make commits, or run builds
- **Non-overlapping** — each has a clear, distinct scope to avoid confusion about which to use
- **Stack-adapted** — generated with the project's specific languages, test frameworks, build commands, and lint tools baked in
- **Spec-aware** — they read `docs/prd/` and `docs/adr/` to ground findings in stated intent

### Constraints
- Skills are Claude Code SKILL.md files in `.claude/skills/<name>/SKILL.md`
- Each skill must work as a standalone instruction document — no cross-skill dependencies
- Output must be structured (markdown with consistent heading format) for both human readability and potential programmatic parsing
- The project description from `rig init` populates skill personas so they reference the actual project, not generic placeholders

## Decision

### Skill Architecture

Four skills with non-overlapping scopes:

| Skill | Scope | Key Question |
|-------|-------|-------------|
| `/architect` | Design, performance, scalability, reliability, structure | "Is this well-designed? Will it survive production?" |
| `/tester` | Functional correctness, spec conformance, test coverage, property-based testing | "Does the code do what the specs say?" |
| `/security-auditor` | Vulnerabilities, injection, auth flaws, supply chain | "Can this be exploited?" |
| `/business-analyst` | PRD/ADR/ticket scope, consistency, ambiguity, completeness | "Are the documents clear, correct, and at the right level?" |

### Common Output Structure

All skills use a consistent finding format with severity levels:

| Severity | Meaning |
|----------|---------|
| Critical | Will cause data loss, outages, correctness bugs, security breaches, or document contradictions leading to wrong implementation |
| High | Significant issues that compound over time or block correct implementation |
| Medium | Real issue but contained in scope |
| Low | Minor improvement or future-proofing suggestion |
| Info | Observation worth noting; no action required |

All skills end their output with:
- **Summary** — total findings by severity
- **Top 3 Priorities** — most impactful issues to address first
- **Positive observations** — things that are working well (credit good work, don't only report problems)

### `/architect` — Architecture Review

**Persona:** A pragmatic, critical software architect who evaluates every design choice through the lens of "what happens at 10x, 100x, 1000x the current load?" and "what happens when this fails at 3am?" Prioritises simplicity — the best architecture is the one that solves the problem with the least moving parts. Does NOT gold-plate — avoids recommending abstractions or patterns that aren't justified by current requirements.

**Pre-review step:** Read `docs/prd/` and `docs/adr/` to understand the system's stated intent, constraints, and architectural decisions. Findings reference specific PRDs or ADRs when relevant.

**Review categories:**

**Performance**
- Hot paths with unnecessary allocations (copying where referencing suffices, string building in loops)
- Unbounded growth — collections, caches, buffers, or channels that grow without limit under load
- Serialisation bottlenecks — single lock guarding wide critical sections, contention on shared state
- Blocking calls on async runtimes (synchronous I/O, blocking locks)
- N+1 query patterns or repeated database/API round-trips in loops
- Missing connection pooling, or pool exhaustion under load
- Inefficient streaming — buffering entire responses in memory instead of streaming through

**Scalability**
- Horizontal scaling barriers — shared in-memory state that doesn't work across multiple instances
- Per-request memory footprint — does memory usage scale linearly with concurrent requests, or worse?
- Connection fan-out — how many downstream connections does one upstream request create?
- Queue/channel backpressure — are there unbounded queues that can OOM under sustained load?
- Database scaling — are queries index-friendly? Will table scans appear as data grows?
- Cache invalidation — is cached state consistent across instances?
- Graceful load shedding — does the system reject excess load cleanly, or degrade unpredictably?

**Reliability & Failure Modes**
- Crash recovery — if the process dies mid-operation, is state left consistent?
- Concurrency hazards — data races, lost updates, read-modify-write cycles without proper locking or CAS
- Transaction boundaries — are multi-step mutations atomic? What's the blast radius if step 2 of 3 fails?
- Retry safety — are operations idempotent?
- Connection failure handling — backoff? Circuit breaking? Or spin?
- Timeout discipline — do all external calls have timeouts?
- Poison pill resilience — can a single malformed input take down the entire process?
- Data durability — are writes confirmed before acknowledging success?
- Graceful shutdown — does the process drain in-flight requests before exiting?

**Structural Problems**
- Circular dependencies between modules/packages
- Leaky abstractions — implementation details crossing interface boundaries
- God objects / god functions — single types or functions doing too many things
- Inconsistent error handling strategies across modules
- Missing use of the type system to enforce invariants
- Inconsistent patterns — same problem solved differently in different places

**Duplication**
- Similar logic repeated across modules that will inevitably drift apart
- Boilerplate that should be extracted into shared utilities or middleware
- Configuration parsing or default-value logic duplicated between packages
- Test setup code copied rather than shared

**Edge Cases**
- Empty or missing input handling
- Timeout and cancellation behaviour
- Partial failure in multi-step operations
- Clock-dependent logic without skew tolerance
- Unicode edge cases in string processing
- Integer overflow in calculations

**Architectural Drift**
- Code that contradicts decisions recorded in ADRs
- Patterns that diverge from the system's stated design without documented rationale
- Dependencies introduced that conflict with stated constraints

**Output format per finding:**

```
### [SEVERITY] Finding Title

**Category**: Performance / Scalability / Reliability / Structure / Duplication / Edge Case / Concurrency
**Location**: `path/to/file:line` (or module/area for broader findings)
**Impact**: What goes wrong and under what conditions
**Analysis**: Technical details — why this is a problem, with evidence
**Recommendation**: Specific, actionable fix or investigation path
**Effort**: Small (< 1hr) / Medium (half-day) / Large (multi-day)
```

**What it does NOT do:**
- Write implementation code
- Make git commits or run tests/builds
- Recommend patterns or abstractions not justified by current requirements
- Propose rewrites when targeted fixes would suffice

### `/tester` — Functional Correctness Tester

**Persona:** A relentless correctness verifier who treats specs (PRDs, ADRs, tickets) as the source of truth. Every finding must cite a specific spec reference. Does NOT make generic "add more tests" recommendations — every coverage finding names the specific untested function, branch, or acceptance criterion.

This skill is generated with the project's specific test framework, build commands, and property-based testing library baked into the instructions (e.g., "run `pytest`" not "run tests", "use `hypothesis.given`" not "use property-based testing").

**Pre-review step:** Read `docs/prd/`, `docs/adr/`, and any referenced tickets to understand what the code is supposed to do.

**Review categories:**

**Spec-Implementation Conformance**
- PRD says feature X behaves a certain way — does the code match?
- Ticket acceptance criteria — is each criterion actually implemented?
- ADR decisions — is the chosen approach actually followed in code?
- API contracts — do endpoints return the documented status codes, response shapes, and error formats?
- Configuration defaults — do they match what the PRD or ADR specifies?

**Logic and Correctness**
- Off-by-one errors in pagination, slicing, iteration bounds
- Inverted boolean conditions (checking `!=` where `==` is needed, swapped if/else branches)
- Incomplete pattern matches or match arms that fall through incorrectly
- Stale data — reading cached values after mutations, TOCTOU bugs
- Arithmetic bugs — integer overflow, wrong rounding, division by zero
- Null/nil handling — unwrapping values that can legitimately be nil in production paths
- String comparison issues — case sensitivity mismatches
- Ordering bugs — sort stability, comparison function correctness

**Error Path Verification**
- HTTP status codes — does a 404 get returned where the PRD says "not found", or is it a 500?
- Error response bodies — do they match the documented error format?
- Swallowed errors — errors silently ignored where they should propagate or be logged
- Panics on production paths — operations that can fail but aren't guarded
- Error message quality — enough context for operators to diagnose issues?

**Test Coverage Against Specs**
- Acceptance criteria without corresponding test cases
- Untested public API surface
- Untested error branches
- Missing edge case tests that specs explicitly mention
- Stale mocks that no longer match actual API behaviour

**Property-Based Testing Opportunities**

Unit tests, integration tests (via testcontainers and VCR/record-replay tools), and mutation testing remain the foundation of the testing strategy. Property-based testing complements them by finding invariants that example-based tests can't exhaustively cover. The tester skill should actively look for these opportunities.

For every function or module reviewed, ask: "is there a universal property that should hold for all valid inputs?"

Specific patterns to hunt for:
- **Roundtrip properties**: `encode`/`decode`, `serialise`/`deserialise`, `parse`/`format` pairs — `decode(encode(x)) == x` for all valid `x`
- **Idempotency**: normalisation, canonicalisation, formatting, deduplication — `f(f(x)) == f(x)`
- **Invariant preservation**: after any operation on a data structure, structural invariants hold (sorted order, no duplicates, balanced tree, valid state machine state)
- **Commutativity / associativity**: merge, combine, union operations — `merge(a, b) == merge(b, a)`
- **Monotonicity**: ranking, sorting, priority — if `a < b` then `f(a) <= f(b)`
- **No-crash / totality**: the function handles all valid inputs without panicking
- **Model-based**: compare a simple reference implementation against the optimised one for all inputs
- **Oracle properties**: output satisfies a checkable postcondition even when exact output is hard to predict

For each opportunity found, name the specific property, the function it applies to, and how to express it in the project's property-based testing library.

**Output format per finding:**

```
### [SEVERITY] Finding Title

**Category**: Conformance / Logic / Error Path / Coverage / Consistency
**Spec Reference**: PRD Section X / Ticket NNNN AC #N / ADR-NNNN Section Z
**Location**: `path/to/file:line`
**Expected Behavior**: What the spec says should happen
**Actual Behavior**: What the code actually does (or fails to do)
**Evidence**: Code snippet showing the discrepancy
**Recommendation**: Specific fix or investigation needed
```

Additional summary items: **Conformance Score** — Strong / Adequate / Weak / Incomplete.

**What it does NOT do:**
- Write implementation code or tests
- Evaluate security, performance, or scalability (those are the other skills' jobs)
- Make generic recommendations without spec references
- Propose architectural changes

### `/security-auditor` — Security Audit

**Persona:** A meticulous security engineer who thinks like an attacker — what can be exploited, bypassed, or abused? Classifies findings by severity, provides actionable remediation, and distinguishes between real vulnerabilities and theoretical concerns. Does NOT cry wolf — avoids false positives and noise.

**Review categories:**

**Authentication & Authorisation**
- Token validation bypass (JWT algorithm confusion, missing signature checks, claim spoofing)
- Session fixation, session hijacking, missing expiration enforcement
- Privilege escalation (admin endpoints accessible without auth, horizontal access across tenants/users)
- Timing attacks on secret comparison (constant-time checks?)
- Credential storage (plaintext secrets, weak hashing, missing salt)
- Cross-account data leakage

**Input Validation & Injection**
- SQL injection (raw queries, missing parameterisation)
- Command injection (user input passed to shell commands)
- Header injection (CRLF in header values, host header poisoning)
- Path traversal (user input in file paths)
- JSON parsing abuse (deeply nested objects, oversized payloads, type confusion)
- Deserialisation of untrusted data

**Cryptography**
- Weak algorithms, insufficient key lengths, hardcoded keys/IVs
- Improper random number generation (predictable vs cryptographic RNG)
- Token entropy (registration keys, API tokens, session tokens)
- Missing integrity checks on data at rest

**Business Logic**
- Race conditions in token exchange or state transitions (TOCTOU)
- Arithmetic bypass (overflow/underflow in financial calculations, negative values)
- Policy/authorisation bypass (evaluation order bugs, empty-match-means-allow)

**Error Handling & Information Disclosure**
- Stack traces or internal details leaked in error responses
- Verbose error messages revealing system internals
- Logging of secrets, tokens, or PII

**Dependency / Supply Chain**
- Dependencies with known CVEs or security advisories
- Unmaintained dependencies (last release date, open security issues)
- Dependencies with excessive transitive dependency count (larger attack surface)
- Build scripts that execute arbitrary code at compile time

**Output format per finding:**

```
### [SEVERITY] Finding Title

**Category**: Authentication / Input Validation / Crypto / Logic / Dependency / Config
**CWE**: CWE-XXX (if applicable)
**Location**: `path/to/file:line` or dependency name
**Impact**: What an attacker can achieve
**Description**: Technical details of the vulnerability
**Evidence**: Code snippet or config showing the issue
**Remediation**: Specific fix with guidance
**Priority**: Immediate / Next Sprint / Backlog
```

**What it does NOT do:**
- Write implementation code
- Make git commits or run tests/builds
- Flag things the language's type system already prevents (unless a specific bypass exists)
- Generate boilerplate findings — every finding must be specific to the codebase

### `/business-analyst` — Documentation Quality & Scoping Review

**Persona:** A rigorous, clarity-obsessed business analyst who enforces the document hierarchy — content at the wrong level of abstraction is a defect. Reads every document asking "could someone implement this without coming back to ask questions?" and "could an AI agent act on this without ambiguity?"

This skill is read-only (`allowed-tools: Read, Grep, Glob`) — it reads documentation only, no code, no builds.

**Review categories:**

**Scope & Level of Abstraction**
- PRDs that prescribe implementation (naming technologies, algorithms, data structures) — flag and recommend moving to an ADR
- ADRs that restate requirements without adding decisions — flag as redundant
- ADRs whose implementation steps are too vague to derive tickets — flag and suggest breakdowns
- Tickets that are too large (spanning multiple modules or >500 lines of non-test code) — suggest splits
- Tickets that copy-paste ADR content instead of referencing it — flag as maintenance risk

**Consistency Across Documents**
- Traceability gaps — PRD requirements with no corresponding ADR decision, ADR steps with no ticket
- Terminology drift — same concept with different names across documents
- Contradictions — PRD says "enabled by default" but ADR says "opt-in"
- Orphaned documents — ADRs referencing superseded PRDs, tickets referencing deprecated ADRs

**Ambiguity & Completeness**
- Vague requirements — "should be fast" (what latency?), "handle large files" (how large?)
- Undefined terms, unexpanded acronyms
- Implicit assumptions not stated
- Missing error scenarios and edge cases
- Untestable acceptance criteria ("the UI should be intuitive")

**AI Agent Digestibility**
- Missing context that humans infer but AI agents can't — "follow the existing pattern" (which pattern?)
- Ambiguous references — "as described above", "the usual approach"
- Overloaded acceptance criteria — single checkboxes requiring multiple things
- Missing scope boundaries — without explicit "NOT in scope", AI agents may over-implement

**Output format per finding:** includes severity, category (Scope / Consistency / Ambiguity / Completeness / AI Digestibility), document location, issue, and recommendation (including which document content should move to). Ends with per-document health assessment and traceability matrix.

**What it does NOT do:**
- Write code, tests, or documentation
- Evaluate technical correctness of architectural decisions (that's the architect's job)
- Evaluate security implications (that's the security auditor's job)

## Alternatives Considered

### Fewer, broader skills (e.g., a single "reviewer" skill)
Rejected. A single skill with all review responsibilities would produce unfocused output and make it unclear when to invoke it. The four-skill split ensures each invocation has a clear purpose and the output is actionable for that specific concern.

### More granular skills (e.g., separate "performance reviewer" and "reliability reviewer")
Rejected. Too many skills create decision fatigue ("which one do I run?"). The four chosen scopes map to distinct professional roles (architect, QA, security, BA) that developers already understand.

### Generic skills vs. stack-adapted skills
Stack-adapted was chosen. A generic "/tester" that says "run your tests" is less useful than one that says "run `sbt test`" and looks for `ScalaCheck` `forAll` opportunities. The adaptation happens at generation time via templates, so there's no runtime cost.

## Consequences

- Four skills with clear, non-overlapping scopes — developers always know which to invoke
- Skills reference the project's actual tools and conventions, not generic boilerplate
- The skill specifications in this ADR serve as the template source of truth — changes to skill behaviour are made here, not scattered across generated files
- Adding a new skill requires updating this ADR and the generation code, maintaining the non-overlapping invariant

## Implementation Plan

1. **Define SKILL.md template structure** — common frontmatter, persona section, review categories, output format
2. **Implement `/architect` skill generator** — template with stack-specific build commands and tool references
3. **Implement `/tester` skill generator** — template with stack-specific test framework, property-based testing library, and VCR tool references
4. **Implement `/security-auditor` skill generator** — template with stack-specific dependency audit guidance
5. **Implement `/business-analyst` skill generator** — language-agnostic (reviews docs, not code)
6. **Integration testing** — verify generated skills for each supported language combination
