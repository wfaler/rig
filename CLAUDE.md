# Rig — Development Guidelines

## Project Overview

Rig creates isolated Docker containers pre-configured with language runtimes, build tools, and AI coding assistants. It is a Go CLI built with Cobra, using the Docker SDK to build images and manage containers.

## Build & Run

```bash
make build          # Build optimised binary
make test           # Run all tests
make test-v         # Verbose tests
make test-race      # Tests with race detector
make coverage       # HTML coverage report
make lint           # Run golangci-lint
make fmt            # Format code
```

## Architecture

```
cmd/                    # CLI commands (Cobra). Entry point: main.go
internal/config/        # YAML config parsing (.rig.yml)
internal/docker/        # Docker SDK wrapper (image build, container lifecycle)
internal/dockerfile/    # Dockerfile generation from config
  scripts/              # Embedded JS/CSS for markdown server (go:embed)
internal/project/       # Project-level utilities (naming, hashing)
```

Key patterns:
- Dockerfile content is generated from Go templates (`BaseTemplate` in `template.go`)
- JS/CSS for the markdown server lives in `internal/dockerfile/scripts/` as standalone files, embedded into the binary via `//go:embed` (see `embed.go`)
- Extra files (like `rig-md-server.js`) are added to the Docker build context via `BuildContext.ExtraFiles` and `COPY`'d into the image
- Config is parsed from `.rig.yml` with sensible defaults (markdown server enabled by default, zsh shell, etc.)

## Testing Strategy

### Unit Tests

Every package must have unit tests. Use table-driven tests with `t.Run()` subtests. Use `testify/assert` for assertions and `testify/require` for preconditions that should halt the test on failure.

```go
func TestFoo(t *testing.T) {
    tests := []struct {
        name     string
        input    string
        expected string
    }{
        {"descriptive name", "input", "expected"},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result := Foo(tt.input)
            assert.Equal(t, tt.expected, result)
        })
    }
}
```

### Integration Tests

Use [testcontainers-go](https://github.com/testcontainers/testcontainers-go) for any test that requires Docker, external services (databases, message queues), or end-to-end container lifecycle validation. Tag integration tests with `//go:build integration` so they can be run separately.

### Property-Based Testing

Use [rapid](https://github.com/flyingmutant/rapid) for property-based tests. Apply property-based testing where inputs have broad valid ranges and outputs should satisfy universal invariants. Good candidates in this project:

- **Config parsing**: arbitrary valid/invalid YAML should never panic; parsed configs should always satisfy structural invariants (e.g., port numbers in valid range, no nil maps)
- **Dockerfile generation**: any valid config should produce a Dockerfile that starts with `FROM` and ends with `CMD`; generated output should never contain empty `RUN` statements
- **Project naming/hashing**: name generation should be deterministic (same input → same output); hashes should be stable and never empty
- **Port mapping**: any port spec should either parse successfully or return a clear error; parsed ports should never overlap

Do not force property-based tests where example-based tests are clearer. If the property is just "output equals this specific value", use a regular test.

### Third-Party Dependencies & VCR

When tests interact with external services or APIs, record interactions using [go-vcr](https://github.com/dnaeon/go-vcr) or equivalent HTTP record/replay. This ensures:

- Tests are deterministic and fast (no network calls in CI)
- Tests don't break when third-party services are unavailable
- Recorded cassettes serve as documentation of expected API behaviour

Store cassettes in `testdata/` directories adjacent to the test files.

### Mutation Testing

Run [go-mutesting](https://github.com/zimmski/go-mutesting) or [gremlins](https://github.com/go-gremlins/gremlins) to verify test suite effectiveness. Mutation testing catches tests that pass without actually validating behaviour (e.g., assertions on the wrong variable, missing edge cases).

Target a mutation kill rate above 80%. When a mutant survives, either:
- Add a test that catches the mutation, or
- Document why the mutated code path is unreachable or irrelevant

### Test Quality Checklist

Before considering a change complete:

- [ ] All new public functions have unit tests
- [ ] Edge cases are covered (empty inputs, nil values, boundary conditions)
- [ ] Error paths are tested, not just happy paths
- [ ] Tests are independent — no shared mutable state between tests
- [ ] No test relies on execution order

## Linting & Static Analysis

Use `golangci-lint` with strict configuration. The linter should enforce:

- **errcheck**: all errors must be handled, never silently discarded
- **govet**: catch common mistakes (printf format strings, struct tag issues)
- **staticcheck**: advanced static analysis (deprecated APIs, unreachable code)
- **gosimple**: simplify code where possible
- **unused**: no dead code
- **ineffassign**: no ineffectual assignments
- **misspell**: catch typos in comments and strings
- **gocritic**: opinionated but useful checks (unnecessary type conversions, etc.)
- **revive**: extensible linter replacing golint

Run `make lint` before committing. CI should fail on any lint violation — no `//nolint` directives without an accompanying comment explaining why.

## Code Style

- Follow standard Go conventions (`gofmt`, `go vet`)
- Prefer returning errors over panicking
- Use descriptive variable names; single-letter variables only in short scopes (loop indices, receiver names)
- Keep functions short and focused — if a function needs a comment explaining what a block does, extract it
- No global mutable state outside of `func init()` or package-level constants
