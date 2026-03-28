---
title: "Scaffold Command Design and Template Engine"
type: adr
id: "0005"
status: Proposed
superseded_by: ""
prd: "project-scaffolding"
created: 2026-03-27
updated: 2026-03-27
tickets: []
---

# ADR-0005: Scaffold Command Design and Template Engine

## Status
Proposed

## Context

The scaffolding PRD defines two commands — `rig init` (interactive setup + scaffold) and `rig scaffold` (scaffold only). These commands are the delivery mechanism for all scaffolded output: `CLAUDE.md`, review skills, and documentation templates. This ADR covers the command implementation, interactive questionnaire, template engine, and file generation strategy.

### Constraints
- Rig is a Go CLI built with Cobra — new commands must follow existing patterns in `cmd/`
- `.rig.yml` is parsed by `internal/config/` — the questionnaire must produce a valid config
- Generated files must be deterministic given the same `.rig.yml` + questionnaire answers
- `--defaults` must work without any user interaction (CI/automation use case)
- No network calls or AI invocations — scaffolding must complete in under 5 seconds

## Decision

### Command Structure

Two new Cobra commands in `cmd/`:

```
cmd/
├── init.go        # rig init — questionnaire + .rig.yml generation + scaffold
├── scaffold.go    # rig scaffold — scaffold only (requires .rig.yml)
```

`rig init` orchestrates three steps:
1. Run interactive questionnaire (or skip with `--defaults`)
2. Generate `.rig.yml` from answers (or use existing if user declines overwrite)
3. Call the scaffold function (shared with `rig scaffold`)

`rig scaffold` calls the scaffold function directly, reading `.rig.yml` for language config and prompting for follow-up questions (JVM sub-language, TypeScript, package manager) unless `--defaults` is passed.

The scaffold function lives in a new package `internal/scaffold/` — not in `cmd/` — so both commands share the same generation logic.

### Interactive Questionnaire

Use Go's standard `fmt.Scan` / `bufio.Scanner` for input — no external TUI library. The questionnaire is simple enough (multi-select, single-select, free text) that a TUI framework would be over-engineering.

**Input types:**
- **Multi-select** (languages): Display numbered list, user types space-separated numbers or toggles with space. Simpler alternative: display list, user types comma-separated numbers (e.g., `1,4` for Node.js and Java/JVM).
- **Single-select** (JVM language, build system, shell, package manager): Single character input with default in brackets (e.g., `(j)ava, (k)otlin, or (s)cala? [j]`).
- **Yes/no** (TypeScript): `(y/n) [y]`
- **Free text** (project description): Single line, optional.

**Flow:**
1. Language selection (multi-select)
2. Per-language follow-ups (only for selected languages):
   - Java/JVM → JVM language + build system
   - Node.js → TypeScript? + package manager
   - Python → package manager
   - Go, Rust, Ruby → no follow-ups
3. Shell selection
4. Project description (optional)

**`--defaults` behaviour:** Skip all prompts, use: Java for JVM, TypeScript yes, npm for Node, pip for Python, zsh for shell, empty project description.

### Questionnaire Output: ScaffoldConfig

The questionnaire produces a `ScaffoldConfig` struct that captures choices not already in `.rig.yml`:

```go
package scaffold

// ScaffoldConfig holds questionnaire answers that go beyond .rig.yml.
// .rig.yml captures language + version + build systems.
// ScaffoldConfig captures sub-choices needed for CLAUDE.md and skill generation.
type ScaffoldConfig struct {
    ProjectDescription string

    // JVM sub-language (only relevant when java is in .rig.yml)
    JVMLanguage string // "java", "kotlin", "scala"

    // Node.js options
    TypeScript bool

    // Resolved from .rig.yml + questionnaire for template rendering
    Languages []LanguageConfig
}

type LanguageConfig struct {
    Name        string // "go", "node", "python", "java", "kotlin", "scala", "rust", "ruby"
    BuildSystem string // "gradle", "maven", "sbt", "npm", "yarn", "pnpm", "pip", "poetry", "uv", "pipenv", "cargo", "bundler"
    TypeScript  bool   // only for node
}
```

This struct is passed to all generators (CLAUDE.md, skills, templates). The JVM sub-language distinction matters because `.rig.yml` only has `java` as a language key — the Kotlin/Scala choice affects which test framework, linter, and property-based testing library appear in the generated output (per ADR-0002).

### Template Engine

Use Go's `text/template` (already a dependency via Dockerfile generation in `internal/dockerfile/`). Each generated file has a corresponding `.tmpl` template.

Templates live in `internal/scaffold/templates/`, embedded via `go:embed`:

```
internal/scaffold/
├── scaffold.go              # Main scaffold function
├── config.go                # ScaffoldConfig struct
├── questionnaire.go         # Interactive prompts
├── templates/
│   ├── claude_md.go.tmpl    # CLAUDE.md template
│   ├── skill_architect.go.tmpl
│   ├── skill_tester.go.tmpl
│   ├── skill_security.go.tmpl
│   ├── skill_ba.go.tmpl
│   ├── prd_template.go.tmpl
│   ├── adr_template.go.tmpl
│   └── ticket_template.go.tmpl
```

Template data is the `ScaffoldConfig` plus resolved tool recommendations from ADR-0002. The templates use `{{if}}` blocks to include/exclude language-specific sections.

Why `text/template` over alternatives:
- Already used in the project (`internal/dockerfile/template.go`)
- No external dependency
- Sufficient for the complexity level (conditional sections, loops over languages)
- `go:embed` for template files follows the pattern established by ADR-0001 Phase 0

### File Generation Strategy

The scaffold function generates files in this order:

1. **`CLAUDE.md`** — language-specific development guidelines
2. **`.claude/skills/*/SKILL.md`** — four review skills
3. **`docs/templates/*.md`** — PRD, ADR, ticket templates

**File conflict handling:**
- If a file exists, skip it and print: `Skipping CLAUDE.md (already exists)`
- Exception: if `CLAUDE.md` exists but is missing sections for configured languages, offer to append
- Templates in `docs/templates/` are always generated (low conflict risk)
- Skills in `.claude/skills/` follow the same skip-if-exists rule

**Directory creation:** Create directories as needed (`docs/templates/`, `.claude/skills/*/`). Use `os.MkdirAll` — idempotent, no error if directory exists.

**Output feedback:** Print each generated file path as it's created:
```
Created CLAUDE.md
Created .claude/skills/architect/SKILL.md
Created .claude/skills/tester/SKILL.md
Created .claude/skills/security-auditor/SKILL.md
Created .claude/skills/business-analyst/SKILL.md
Created docs/templates/prd-template.md
Created docs/templates/adr-template.md
Created docs/templates/ticket-template.md
```

### `rig init` .rig.yml Generation

When `rig init` generates `.rig.yml`, it maps questionnaire answers to the config structure:

- Selected languages → `languages:` section with default versions (`"lts"` for Node, `"latest"` for others)
- Build systems → `build_systems:` under each language (from follow-up questions or defaults)
- Shell → `shell:` field
- Ports, env, code_server, markdown_server → not asked in questionnaire, use defaults

If `.rig.yml` already exists:
```
.rig.yml already exists. Overwrite? (y/n) [n]
```
If declined, proceed to scaffold using the existing `.rig.yml`.

### `rig scaffold` .rig.yml Reading

`rig scaffold` reads `.rig.yml` via the existing config parser in `internal/config/`. It then prompts for follow-up questions not captured in `.rig.yml`:

- If `java` is in languages → ask JVM sub-language (java/kotlin/scala)
- If `node` is in languages → ask TypeScript (y/n)
- Python package manager and Node package manager are already in `.rig.yml` under `build_systems`

With `--defaults`, skip all prompts and use defaults.

### Integration with ADR-0001 (Search/MCP)

ADR-0001 Phase 2 adds MCP auto-configuration to `rig init`:
- Generate `.mcp.json` with the rig-docs MCP server entry
- Append a search guidance section to `CLAUDE.md`

This is additive — the scaffold function generates the base `CLAUDE.md`, and ADR-0001 Phase 2 adds the search section. The append logic (check if section exists, append if not) is handled by the scaffold function's file conflict handling, extended to support section-level merging for `CLAUDE.md`.

**Sequencing:** ADR-0001 Phase 2 should be implemented after this ADR, so it can build on the scaffold infrastructure. The `CLAUDE.md` template should include a placeholder or extension point for additional sections from other features.

## Alternatives Considered

### External TUI library (bubbletea, survey, promptui)
Rejected. The questionnaire has ~5-8 prompts with simple input types. A TUI framework adds a dependency, increases binary size, and introduces a learning curve for contributors — all for a feature that `fmt.Scan` handles adequately. If the questionnaire grows significantly more complex (e.g., interactive file tree, real-time validation), reconsider.

### Generating files without templates (string concatenation)
Rejected. The current codebase already suffered from this approach with the markdown server (Go string constants with embedded JS/CSS). Templates provide separation of concerns, editor support, and easier maintenance.

### Storing ScaffoldConfig in a separate file (e.g., `.rig-scaffold.yml`)
Rejected. The JVM sub-language and TypeScript choices are only needed at generation time. Once `CLAUDE.md` and skills are generated, the choices are baked into the output. Storing them adds a file to maintain with no ongoing value. If regeneration is needed, `rig scaffold` re-asks the questions.

### Template files as external assets (not embedded)
Rejected. Embedding via `go:embed` keeps the binary self-contained — no need to ship template files alongside the binary or worry about file paths at runtime. This follows the pattern established by ADR-0001 Phase 0 for JS/CSS files.

## Consequences

- `rig init` becomes the single entry point for new projects: questionnaire → config → scaffold
- `rig scaffold` enables retrofitting existing projects without re-running `rig init`
- The `internal/scaffold/` package provides a clean extension point for future scaffolded output
- Template-based generation is deterministic and testable (given the same `ScaffoldConfig`, output is identical)
- The `ScaffoldConfig` struct captures the gap between `.rig.yml` (what's persisted) and the full context needed for generation (JVM sub-language, TypeScript)
- `CLAUDE.md` is a composite file with contributions from multiple features (scaffold base + ADR-0001 search section) — the append-if-missing pattern handles this

## Implementation Plan

1. **Create `internal/scaffold/` package** — `ScaffoldConfig` struct, scaffold function signature, directory/file creation utilities
2. **Implement questionnaire** — language multi-select, per-language follow-ups, shell, project description; `--defaults` bypass
3. **Implement `cmd/scaffold.go`** — Cobra command that reads `.rig.yml`, runs follow-up questions, calls scaffold function
4. **Implement `cmd/init.go`** — Cobra command that runs full questionnaire, generates `.rig.yml`, calls scaffold function
5. **Implement template rendering** — `go:embed` templates, `text/template` execution with `ScaffoldConfig` data
6. **Implement CLAUDE.md generation** — language-specific sections from ADR-0002 tool tables
7. **Implement skill generation** — four SKILL.md files from ADR-0003 specifications
8. **Implement template generation** — PRD/ADR/ticket templates from ADR-0004 designs
9. **Add tests** — unit tests for questionnaire parsing, template rendering, file conflict handling; integration tests for full scaffold flow
