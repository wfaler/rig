---
title: "Project Scaffolding"
type: prd
status: Draft
feature: "project-scaffolding"
created: 2026-03-27
updated: 2026-03-27
adrs: ["0002", "0003", "0004", "0005"]
---

# Project Scaffolding — Product Requirements Document

## Vision

When a developer runs `rig scaffold`, they should get a complete project quality framework tailored to their specific stack: language-specific development guidelines for AI agents, review skills that catch bugs and architectural issues without writing code, and documentation templates that enforce a clear hierarchy from product intent to implementable tickets.

The scaffolding adapts to the project. A Scala project with SBT gets different testing guidance, linting rules, and build commands than a Java project with Gradle or a Python project with Poetry. The generated skills reference the project's actual tools and conventions, not generic boilerplate.

The goal is that an AI agent working inside a rig container has everything it needs to write high-quality code, review its own work, and produce well-structured documentation — without the developer having to set any of this up manually.

## Problem Statement

Developers using AI coding agents face two recurring problems:

1. **AI agents don't know your standards.** Without explicit guidance, agents write code that compiles but doesn't follow project conventions — wrong test frameworks, missing edge cases, no property-based tests, lax linting. The developer spends time correcting what the agent should have done right the first time.

2. **Documentation is unstructured.** Teams write PRDs, ADRs, and tickets in ad-hoc formats. The relationship between them is unclear. PRDs are too detailed (they become specs) or too vague (they can't inform architecture). ADRs lack enough detail to derive tickets. Tickets are ambiguous. AI agents asked to "write an ADR" produce something, but it doesn't fit the team's workflow.

Rig already solves the environment problem. This feature solves the process problem.

## Commands

### `rig init` — Interactive Project Setup

`rig init` becomes an interactive questionnaire that gathers all the information needed to generate both `.rig.yml` and the full project scaffold in one flow. The questionnaire replaces the current template-dumping behaviour.

**Language selection** (multi-select):
```
Languages (space to toggle, enter to confirm):
  [x] Node.js
  [ ] Python
  [ ] Go
  [x] Java/JVM
  [ ] Rust
  [ ] Ruby
```

**Follow-up questions per language** (only for selected languages):

When **Java/JVM** is selected:
```
JVM language: (j)ava, (k)otlin, or (s)cala? [j]
Build system: (g)radle, (m)aven, or (s)bt? [g]
```
This determines test framework (JUnit 5 vs Kotest vs ScalaTest/MUnit), property-based testing library (jqwik vs Kotest property testing vs ScalaCheck), lint tooling (Checkstyle vs Detekt vs Scalafix/WartRemover), and build commands.

When **Node.js** is selected:
```
TypeScript? (y/n) [y]
Package manager: (n)pm, (y)arn, or (p)npm? [n]
```

When **Python** is selected:
```
Package manager: (p)ip, p(o)etry, (u)v, or p(i)penv? [p]
```

**Common questions:**
```
Shell: (z)sh, (b)ash, or (f)ish? [z]
Brief project description (for AI agent context):
```

The project description populates skill personas so they reference "the Acme payment gateway" rather than "the project."

After the questionnaire, `rig init` generates `.rig.yml` (populated from answers) and immediately runs the scaffold step — generating `CLAUDE.md`, skills, and templates. All in one command.

Questions have sensible defaults (shown in brackets) — pressing Enter accepts the default. `rig init --defaults` skips the questionnaire entirely and generates everything with defaults.

### `rig scaffold` — Scaffold an Existing Project

`rig scaffold` runs the scaffolding step on an existing project that already has `.rig.yml`. This is useful when:

- A project was created before the scaffolding feature existed
- `.rig.yml` was written by hand without running `rig init`
- The scaffold files were deleted and need regenerating

`rig scaffold` reads the existing `.rig.yml` to determine languages and build systems, then asks the same follow-up questions that `rig init` would (JVM sub-language, TypeScript, etc.) before generating the scaffold files. `rig scaffold --defaults` skips the questionnaire.

See [ADR-0005](../adr/0005-scaffold-command-design.md) for command implementation, questionnaire design, template engine, and file generation strategy.

## What Gets Scaffolded

`rig scaffold` generates the following, adapted to the answers above:

### 1. `CLAUDE.md` — Language-Specific Development Guidelines

A `CLAUDE.md` tailored to the languages and build systems configured in `.rig.yml`. This is the primary mechanism for teaching AI agents how to work in the project.

Content varies by language — each language section covers test framework, property-based testing library, integration testing (testcontainers), VCR/HTTP recording, mutation testing, linting, and formatting with specific tool recommendations. See [ADR-0002](../adr/0002-language-tool-recommendations.md) for the specific tool choices per language and rationale.

**Supported languages:** Go, Node.js/TypeScript, Python, Java, Kotlin, Scala, Rust, Ruby.

**Common to all languages:**
- Testing strategy hierarchy: unit → integration → property-based → mutation
- Test quality checklist (edge cases, error paths, independence)
- Store VCR cassettes in `testdata/` adjacent to test files
- Mutation testing target: 80%+ kill rate

### 2. Claude Skills (`.claude/skills/`)

Four read-only skills that analyse code and documentation without modifying anything. All report findings but never modify files, make commits, or run builds.

Each skill has a clear, non-overlapping scope:
- `/tester` — Does the code do what the specs say? Are tests comprehensive? Are there property-based testing opportunities?
- `/architect` — Is the design sound? Will it scale? Will it survive failures? Are there structural problems?
- `/security-auditor` — Can it be exploited? Are there injection, traversal, or supply chain risks?
- `/business-analyst` — Are PRDs, ADRs, and tickets at the right level of abstraction? Are they consistent, unambiguous, and traceable?

**Stack adaptation:** Each skill is generated with the project's specific context baked in — the project description, languages, test frameworks, build commands, and lint tools. A Scala project's `/architect` references SBT tasks and ScalaCheck; a Go project's references `go test` and `rapid`. Skills are not generic templates — they are tailored guidance documents.

See [ADR-0003](../adr/0003-review-skill-design.md) for the detailed design of each skill — personas, review checklists, output formats, and severity levels.

### 3. Documentation Templates

Templates that enforce the PRD → ADR → Ticket hierarchy. The system should generate three template files:

- **`docs/templates/prd-template.md`** — Product Requirements Document template
- **`docs/templates/adr-template.md`** — Architecture Decision Record template
- **`docs/templates/ticket-template.md`** — Implementation Ticket template

All templates must use YAML front matter with structured metadata enabling programmatic search, filtering, and traceability (e.g., "show me all tickets derived from ADR-0001", "what PRDs are still in Draft?"). The front matter is also indexed by the search/MCP server (ADR-0001) with boosted field weights.

Each template must include inline guidance comments that teach the document hierarchy — a developer reading the templates should understand the PRD → ADR → Ticket workflow without external explanation.

See [ADR-0004](../adr/0004-documentation-templates.md) for the specific template designs, front matter schemas, and section structures.

### 4. Directory Structure Created

```
project/
├── .rig.yml                              # Already generated
├── CLAUDE.md                             # Language-specific dev guidelines
├── .claude/
│   └── skills/
│       ├── tester/
│       │   └── SKILL.md                  # Correctness, coverage, property testing
│       ├── architect/
│       │   └── SKILL.md                  # Design, scalability, reliability
│       ├── security-auditor/
│       │   └── SKILL.md                  # Vulnerabilities, injection, supply chain
│       └── business-analyst/
│           └── SKILL.md                  # PRD/ADR/ticket scope, consistency, completeness
└── docs/
    └── templates/
        ├── prd-template.md
        ├── adr-template.md
        └── ticket-template.md
```

## Behaviour

### `rig init` Flow

1. Run interactive questionnaire (or use `--defaults`)
2. Generate `.rig.yml` from answers
3. Run scaffold step (same as `rig scaffold`)

If `.rig.yml` already exists, ask: `".rig.yml already exists. Overwrite? (y/n) [n]"`. If declined, skip to scaffold step using the existing `.rig.yml`.

### `rig scaffold` Prerequisite

`rig scaffold` requires `.rig.yml` to exist. If it doesn't, print: `No .rig.yml found. Run 'rig init' first.`

### File Conflict Handling

- If a file already exists, **do not overwrite**. Skip it and print a message: `Skipping CLAUDE.md (already exists)`
- Exception: if `CLAUDE.md` exists but doesn't contain language-specific sections for the configured languages, offer to append the missing sections
- Templates are always generated (they're in `docs/templates/` which is unlikely to conflict)

Note: `CLAUDE.md` is a composite file — the scaffolding feature generates language-specific development guidelines (ADR-0002), and the search/MCP feature (ADR-0001 Phase 2) appends search guidance. Both features use append-if-missing semantics to avoid overwriting each other's sections.

### Language Detection

The generated `CLAUDE.md` content is determined by the languages configured in `.rig.yml` combined with the answers to the interactive questionnaire. If `.rig.yml` has `java` configured and the user selects Kotlin, the output references Kotest, Detekt, and Kotest property testing — not JUnit 5, Checkstyle, and jqwik.

### No Languages Configured

If `.rig.yml` has no languages (empty config), generate `CLAUDE.md` with the common testing strategy section only (no language-specific guidance). The skills and templates are always generated regardless of language config.

### Non-Interactive Mode

Both `rig init --defaults` and `rig scaffold --defaults` skip the questionnaire and use defaults for all choices (Java for JVM, TypeScript for Node, pytest for Python, pip for Python package manager). Useful for CI or automation.

## Non-Goals

- **No code generation.** This feature generates guidance documents and templates, not application code, test scaffolds, or CI configs.
- **No enforcement.** The generated files are guidance, not gates. They don't configure CI pipelines, pre-commit hooks, or build-time checks. That's the developer's responsibility.
- **No dependency installation.** The CLAUDE.md recommends specific test frameworks and linters, but doesn't install them. The developer adds them to their project's dependency file.

## Implementation Sequencing

The scaffolding feature spans five ADRs with dependencies between them. The implementation order is:

### Phase 1: Foundation (ADR-0005)
**Scaffold command and template engine.** This is the prerequisite for all other work — it creates the `internal/scaffold/` package, the `rig init` and `rig scaffold` commands, the questionnaire, and the `text/template`-based file generation infrastructure. Nothing else can ship without this.

### Phase 2: Simplest output first (ADR-0004)
**Documentation templates.** Templates are static markdown files with no language-specific variation. They exercise the scaffold infrastructure end-to-end with minimal complexity — good for validating the file generation, conflict handling, and directory creation before tackling dynamic content.

### Phase 3: Language-specific output (ADR-0002)
**CLAUDE.md generation.** This is the first dynamically generated output — the template engine must handle conditional language sections, build system variations, and the JVM sub-language distinction. Depends on ADR-0005 for the template infrastructure and `ScaffoldConfig`.

### Phase 4: Skills (ADR-0003)
**Review skill generation.** Skills are more complex templates — they need the project description, language-specific tool references, and stack-adapted personas. Depends on ADR-0005 for template infrastructure and shares the `ScaffoldConfig` with ADR-0002.

### Independent: Search & MCP (ADR-0001)
**Search and MCP server.** ADR-0001 is independent of the scaffolding feature — it modifies the markdown server, not the scaffold commands. However, ADR-0001 Phase 2 (MCP auto-configuration) adds to `rig init` by generating `.mcp.json` and appending search guidance to `CLAUDE.md`. Phase 2 should be implemented after ADR-0005 so it can use the scaffold infrastructure's append-if-missing pattern.

```
ADR-0005 (scaffold commands)
    │
    ├── ADR-0004 (templates) ── no further dependencies
    │
    ├── ADR-0002 (CLAUDE.md) ── no further dependencies
    │
    └── ADR-0003 (skills) ── no further dependencies

ADR-0001 (search/MCP) ── independent, but Phase 2 builds on ADR-0005
```

## Success Criteria

- `rig init` with `--defaults` completes in under 5 seconds (no network calls, no AI invocations)
- `rig scaffold` on an existing project generates the same output as `rig init` would have
- A JVM project configured with Scala + SBT gets ScalaCheck, ScalaTest/MUnit, Scalafix, and WartRemover guidance — not JUnit and Checkstyle
- An AI agent reading the generated `CLAUDE.md` produces code that follows the specified testing strategy on the first attempt (unit tests, edge cases, property-based tests where appropriate)
- `/tester` identifies specific property-based testing opportunities with named properties and code examples in the project's property-testing library
- `/architect` references the project's PRD/ADRs when identifying architectural concerns, and uses the project's actual build commands
- `/security-auditor` adapts its audit categories to the project's stack (e.g., doesn't look for SQL injection in a project with no database)
- The PRD → ADR → Ticket template hierarchy is self-documenting — a developer reading the templates understands the workflow without external explanation
