---
title: "Language-Specific Tool Recommendations for CLAUDE.md"
type: adr
id: "0002"
status: Proposed
superseded_by: ""
prd: "project-scaffolding"
created: 2026-03-27
updated: 2026-03-27
tickets: []
---

# ADR-0002: Language-Specific Tool Recommendations for CLAUDE.md

## Status
Proposed

## Context

The scaffolding PRD requires that `rig scaffold` generates a `CLAUDE.md` tailored to the project's languages and build systems. The generated file is the primary mechanism for teaching AI agents how to work in the project — which test frameworks to use, how to lint, what property-based testing library to reach for.

This ADR records the specific tool choices per language and the rationale behind each. These choices determine the content of the generated `CLAUDE.md` and should be updated as the ecosystem evolves.

### Constraints
- Tools must be well-maintained and widely adopted in their ecosystem
- Each category (test, property-based, integration, VCR, mutation, lint, format) should have one clear recommendation per language to avoid ambiguity for AI agents
- Tools must work inside a Docker container (debian:bookworm-slim) without special hardware

## Decision

### Tool Selection Per Language

#### Go
| Category | Tool | Rationale |
|----------|------|-----------|
| Test framework | `go test` + `testify/assert` + `testify/require` | Standard; `testify` is the most widely used assertion library |
| Property-based | `rapid` (github.com/flyingmutant/rapid) | Modern, fast, better shrinking than `gopter`; actively maintained |
| Integration | `testcontainers-go` | Official testcontainers port; tagged with `//go:build integration` |
| VCR | `go-vcr` (github.com/dnaeon/go-vcr) | Most popular Go HTTP recorder; cassettes in `testdata/` |
| Mutation | `gremlins` (github.com/go-gremlins/gremlins) | Actively maintained successor to `go-mutesting` |
| Lint | `golangci-lint` (strict: errcheck, govet, staticcheck, gosimple, unused, ineffassign, misspell, gocritic, revive) | De facto standard meta-linter |
| Format | `gofmt` | Standard, non-negotiable |

#### Node.js / TypeScript
| Category | Tool | Rationale |
|----------|------|-----------|
| Test framework | Vitest (preferred) or Jest | Vitest is faster, native ESM; Jest as fallback for existing projects |
| Property-based | `fast-check` | Most actively maintained JS/TS property testing library |
| Integration | `testcontainers-node` | Official testcontainers port |
| VCR | `nock` | Most popular HTTP recording/replay for Node |
| Mutation | `stryker-mutator` | Only serious JS/TS mutation testing framework |
| Lint | `eslint` (with `@typescript-eslint` if TypeScript) | Industry standard |
| Format | `prettier` | Industry standard |

#### Python
| Category | Tool | Rationale |
|----------|------|-----------|
| Test framework | `pytest` | De facto standard; superior to unittest |
| Property-based | `hypothesis` | Best-in-class; far ahead of alternatives |
| Integration | `testcontainers-python` | Official testcontainers port |
| VCR | `vcrpy` or `responses` | `vcrpy` for full recording; `responses` for simpler mocking |
| Mutation | `mutmut` | Most actively maintained Python mutation tester |
| Lint | `ruff` (strict config) | Replaces flake8, isort, pycodestyle; dramatically faster |
| Type checking | `mypy` (strict mode) | Most widely adopted; `pyright` is an alternative |

#### Java
| Category | Tool | Rationale |
|----------|------|-----------|
| Test framework | JUnit 5 + AssertJ | Industry standard; AssertJ provides fluent assertions |
| Property-based | `jqwik` | Most mature Java property testing; JUnit 5 native |
| Integration | `testcontainers-java` | Official testcontainers (original implementation) |
| VCR | WireMock | Most popular Java HTTP mock/record server |
| Mutation | `pitest` | Only serious Java mutation testing framework |
| Lint | Checkstyle (strict) + SpotBugs + ErrorProne | Complementary: style, bugs, common mistakes |
| Build | Gradle (`./gradlew test`, `./gradlew check`) or Maven (`mvn test`, `mvn verify`) | As configured in `.rig.yml` |

#### Kotlin
| Category | Tool | Rationale |
|----------|------|-----------|
| Test framework | Kotest (preferred) or JUnit 5 + kotlin.test | Kotest is idiomatic Kotlin; JUnit 5 for existing projects |
| Property-based | Kotest property testing (`forAll`, `checkAll`) | Integrated with Kotest; no separate dependency needed |
| Integration | `testcontainers-java` | Same library; works with Kotlin |
| VCR | WireMock or MockWebServer | WireMock for recording; MockWebServer for OkHttp projects |
| Mutation | `pitest` (Kotlin plugin) | Only option; requires Kotlin plugin |
| Lint | `detekt` (strict config) + `ktlint` | `detekt` for analysis; `ktlint` for formatting |

#### Scala
| Category | Tool | Rationale |
|----------|------|-----------|
| Test framework | MUnit (preferred) or ScalaTest | MUnit is simpler, faster; ScalaTest for existing projects |
| Property-based | `ScalaCheck` (`forAll`, generators, shrinking) | The original property-based testing library; Scala ecosystem standard |
| Integration | `testcontainers-scala` | Scala wrapper around testcontainers-java |
| VCR | WireMock | Works well with Scala; no Scala-specific alternative needed |
| Mutation | `stryker4s` | Only Scala mutation testing framework |
| Lint | `scalafix` rules + `WartRemover` (strict) | Complementary: `scalafix` for rewrites; `WartRemover` for forbidden patterns |
| Format | `scalafmt` | Standard, non-negotiable |
| Build | SBT (`sbt test`, `sbt scalafixAll`) or Gradle/Maven | As configured in `.rig.yml` |

#### Rust
| Category | Tool | Rationale |
|----------|------|-----------|
| Test framework | Built-in `#[test]`, `assert_eq!` / `assert!` | No external framework needed |
| Property-based | `proptest` | More ergonomic than `quickcheck`; better shrinking |
| Integration | `tests/` directory | Rust convention for integration tests |
| VCR | `mockito` or `wiremock` | `wiremock` for HTTP recording; `mockito` for general mocking |
| Mutation | `cargo-mutants` | Actively maintained; easy to use |
| Lint | `clippy` — `#![deny(clippy::all, clippy::pedantic)]` | Standard; pedantic catches real issues |
| Format | `rustfmt` | Standard, non-negotiable |

#### Ruby
| Category | Tool | Rationale |
|----------|------|-----------|
| Test framework | RSpec | De facto standard for Ruby |
| Property-based | `rantly` or `hypothesis-ruby` | Limited ecosystem; `rantly` is more mature |
| Integration | `testcontainers-ruby` | Official testcontainers port |
| VCR | `vcr` gem + `webmock` | The original VCR library; name comes from here |
| Mutation | `mutant` | Most capable Ruby mutation tester |
| Lint | `rubocop` (strict config) | De facto standard |

### Common Standards (All Languages)

These are language-agnostic quality standards included in every generated `CLAUDE.md`:

- **Testing strategy hierarchy**: unit → integration → property-based → mutation
- **Test quality checklist**: edge cases, error paths, test independence
- **VCR cassettes**: stored in `testdata/` adjacent to test files
- **Mutation testing target**: 80%+ kill rate

## Alternatives Considered

### Single "recommended" tool per category vs. multiple options
We provide a primary recommendation with an alternative where ecosystems are split (e.g., Vitest vs Jest, MUnit vs ScalaTest). The primary is used for new projects; the alternative is acknowledged for existing projects that already use it. This avoids AI agents being confused by open-ended "choose one of these five options" lists.

### Including build tool setup instructions
Rejected. The CLAUDE.md recommends tools but does not install them (per the scaffolding PRD's non-goals). Build tool setup is the developer's responsibility.

## Consequences

- Each language has a clear, opinionated set of tool recommendations
- AI agents receive unambiguous guidance — "use X" rather than "consider X, Y, or Z"
- Tool choices can be updated in this ADR without modifying the PRD
- The generated CLAUDE.md content is deterministic given a language + build system combination

## Implementation Plan

1. **Implement Go template generation** — template that produces the Go section of CLAUDE.md from the tool table above
2. **Implement Node.js/TypeScript template** — with TypeScript conditional sections
3. **Implement Python template** — with package manager variations
4. **Implement JVM templates** — Java/Kotlin/Scala with build system variations
5. **Implement Rust template**
6. **Implement Ruby template**
7. **Implement common standards section** — appended to all generated CLAUDE.md files
