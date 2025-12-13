# Rig

**Instant, reproducible development environments for AI-assisted coding.**

Rig creates isolated Docker containers pre-configured with your language runtimes, build tools, and AI coding assistants. One command to enter a fully-equipped sandbox—no manual setup, no "works on my machine" issues.

## Why Rig?

- **Zero Setup** — Define your stack in YAML, run `rig`, and you're coding
- **AI Agents Ready** — Claude Code, Gemini CLI, OpenAI Codex and GitHub CLI pre-installed
- **VS Code in Browser** — Optional code-server with language extensions, auto-configured
- **Testcontainers Support** — Docker-in-Docker works out of the box
- **Persistent Sessions** — Your container persists between sessions; instant startup after first build
- **Auto-Rebuild** — Change your config, and the image rebuilds automatically

## Quick Start

```bash
# Install
go install github.com/wfaler/rig@latest

# Initialize a project
cd your-project
rig init

# Edit .rig.yml to your needs, then:
rig
```

That's it. You're in a container with your languages, tools, and AI assistants ready to go.

## Configuration

Create `.rig.yml` in your project root:

```yaml
languages:
  node:
    version: "lts"
    build_system: yarn
  python:
    version: "3.12"
    build_system: poetry

ports:
  - "3000"
  - "5432:5432"

env:
  API_KEY: "${API_KEY}"  # Expands from host environment

shell: zsh  # zsh with oh-my-zsh (default), bash, or fish

code_server:
  enabled: true
  theme: "Default Dark Modern"
  extensions:
    - github.copilot
```

### Supported Languages

| Language | Versions | Build Systems |
|----------|----------|---------------|
| **Node** | lts, latest, specific (e.g., "20") | npm, yarn, pnpm |
| **Python** | latest, specific (e.g., "3.12") | pip, poetry, pipenv |
| **Go** | latest, specific (e.g., "1.22") | built-in |
| **Java/Kotlin/Scala** | latest, specific (e.g., "21") | gradle, maven, sbt, ant |
| **Rust** | latest, specific (e.g., "1.75") | cargo |
| **Ruby** | latest, specific (e.g., "3.3") | bundler, gem |

### Code Server (VS Code in Browser)

Enable `code_server` to get a full VS Code experience in your browser:

```yaml
code_server:
  enabled: true
  port: 8080                    # default, auto-exposed
  theme: "Default Dark Modern"  # any VS Code theme
  extensions:                   # extensions to install
    - golang.go                 # Go
    - ms-python.python          # Python
    - github.copilot            # AI assistant
```

Run `rig init` to see all recommended extensions for each language.

## Commands

| Command | Description |
|---------|-------------|
| `rig` | Enter the container (builds if needed) |
| `rig init` | Create `.rig.yml` template |
| `rig rebuild` | Force clean rebuild of image |

## What's Inside

Every rig container includes:

- **AI Assistants**: Claude Code, Gemini CLI, GitHub CLI
- **Dev Tools**: git, curl, wget, jq, vim, build-essential
- **Docker CLI**: For testcontainers and Docker workflows
- **Version Managers**: Mise (polyglot) and SDKMAN (JVM)

## How It Works

1. **Config Hash** — Your `.rig.yml` is hashed to create a unique image tag
2. **Smart Builds** — Images only rebuild when config changes
3. **Persistent Containers** — Named `rig-<project>`, reused across sessions
4. **Socket Mounting** — Docker socket mounted for testcontainers support
5. **Entrypoint Magic** — Permissions and services configured at container start

---

## Development

### Prerequisites

- Go 1.22+
- Docker

Go is only required to build.

### Build from Source

```bash
git clone https://github.com/wfaler/rig.git
cd rig
make build
```

### Common Tasks

```bash
make build      # Build binary
make test       # Run tests
make test-v     # Verbose tests
make fmt        # Format code
make clean      # Clean artifacts
make install    # Install to $GOPATH/bin
```

### Project Structure

```
rig/
├── main.go                 # Entry point
├── cmd/                    # CLI commands (Cobra)
│   ├── root.go             # Root command (enters container)
│   ├── init.go             # rig init
│   ├── rebuild.go          # rig rebuild
│   └── session.go          # Container session logic
├── internal/
│   ├── config/             # YAML parsing & validation
│   ├── docker/             # Docker SDK wrapper
│   ├── dockerfile/         # Dockerfile generation
│   └── project/            # Project utilities
├── REQUIREMENTS.md         # Technical specification
└── Makefile
```

### Contributing

1. Add languages: `internal/config/config.go` + `internal/dockerfile/languages.go`
2. Add VS Code extensions: `internal/dockerfile/languages.go`
3. Modify container setup: `internal/dockerfile/template.go`

See [REQUIREMENTS.md](REQUIREMENTS.md) for the full technical specification.

## License

MIT
