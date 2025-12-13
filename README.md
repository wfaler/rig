# Devbox

A CLI tool that creates dockerized development sandboxes for AI agents (Claude, Gemini, Codex, GitHub CLI).

## Features

- Persistent Docker containers with language runtimes and build tools
- Support for Go, Node, Python, Java, Rust, and Ruby
- AI agent CLIs pre-installed (Claude, Gemini, Codex, GitHub CLI)
- Docker-in-Docker support for testcontainers
- Optional code-server (VS Code in browser) with language extensions
- Automatic image rebuilding when configuration changes

## Installation

```bash
go install github.com/wfaler/devbox@latest
```

Or build from source:

```bash
git clone https://github.com/wfaler/devbox.git
cd devbox
make build
```

## Quick Start

```bash
# Initialize a new workspace
devbox init

# Edit .assistant.yml to configure your environment
# Then start an AI agent session
devbox claude    # or gemini, codex, gh

# Or just open a bash shell
devbox bash
```

## Configuration

Create a `.assistant.yml` file in your project directory:

```yaml
languages:
  node:
    version: "lts"
    build_system: npm
  python:
    version: "3.12"
    build_system: poetry

ports:
  - "8080:8080"
  - "3000"

env:
  API_KEY: "${API_KEY}"

code_server: true  # Optional: VS Code in browser
```

See [REQUIREMENTS.md](REQUIREMENTS.md) for full configuration options.

## Development

### Prerequisites

- Go 1.22+
- Docker
- Make (optional)

### Setup

```bash
git clone https://github.com/wfaler/devbox.git
cd devbox
make deps
```

### Common Tasks

```bash
make build      # Build the binary
make test       # Run tests
make test-v     # Run tests with verbose output
make lint       # Run linter (requires golangci-lint)
make fmt        # Format code
make clean      # Remove build artifacts
make install    # Install to $GOPATH/bin
```

### Project Structure

```
devbox/
├── main.go                      # Entry point
├── cmd/                         # CLI commands (Cobra)
│   ├── root.go                  # Root command
│   ├── init.go                  # devbox init
│   ├── agent.go                 # devbox [claude|gemini|codex|gh]
│   └── bash.go                  # devbox bash
├── internal/
│   ├── config/                  # YAML config parsing and validation
│   ├── docker/                  # Docker SDK client wrapper
│   ├── dockerfile/              # Dockerfile template generation
│   └── project/                 # Project utilities (naming, hashing)
├── REQUIREMENTS.md              # Full specification
└── Makefile
```

### Running Tests

```bash
# Run all tests
make test

# Run tests with coverage
make coverage

# Run specific package tests
go test ./internal/config/...
```

### Adding a New Language

1. Add the language to `SupportedLanguages` in `internal/config/config.go`
2. Add build systems to `BuildSystemsForLanguage` in `internal/config/config.go`
3. Add install function in `internal/dockerfile/languages.go`
4. Add VS Code extensions to `VSCodeExtensionsForLanguage` in `internal/dockerfile/languages.go`
5. Add tests

### Adding a New AI Agent

1. Add the agent name to `validAgents` map in `cmd/agent.go`
2. Add description to `agentDescriptions` map in `cmd/agent.go`
3. Add npm package to the Dockerfile template in `internal/dockerfile/template.go`

## License

MIT
