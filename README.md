# Rig

**Instant, reproducible development environments for AI-assisted coding.**

Rig creates isolated Docker containers pre-configured with your language runtimes, build tools, AI coding assistants and optionally code-server (VS Code in the browser). One command to enter a fully-equipped sandbox—no manual setup, no "works on my machine" issues.

## Why Rig?

- **Zero Setup** — Define your stack in YAML, run `rig`, and you're coding
- **AI Agents Ready** — Claude Code, Gemini CLI, OpenAI Codex and GitHub CLI pre-installed
- **VS Code in Browser** — Optional code-server with language extensions, auto-configured
- **Testcontainers Support** — Docker-in-Docker works out of the box (**without** needing privileged mode)
- **Persistent Sessions** — Your container persists between sessions; instant startup after first build
- **Auto-Rebuild** — Change your config, and the image rebuilds automatically next time you enter.

## Why Rig and not Dev Containers?

VS Code Dev Containers are powerful, but they come with trade-offs that Rig avoids:

| | Rig | Dev Containers |
|---|---|---|
| **Config complexity** | Single `.rig.yml` (~10 lines) | `devcontainer.json` + Dockerfile + features |
| **IDE lock-in** | Any editor, terminal, or browser | VS Code (or compatible editors) |
| **AI assistants** | Claude Code, Gemini, OpenAI CLI pre-installed | Manual setup required |
| **Setup time** | `rig init && rig` | Configure JSON, choose features, debug builds |
| **Testcontainers** | Works out of the box | Requires manual Docker-in-Docker setup |
| **Branch switching** | Same container, instant | Often rebuilds per branch |

### One container, all branches

Dev Containers are often tied to your Git branch or workspace state. Switch branches and you might trigger a rebuild—or worse, lose your installed dependencies and cached builds.

Rig containers are **project-scoped, not branch-scoped**. The container persists based on your `.rig.yml` config hash, not which branch you're on. Switch from `main` to `feature-x` to `hotfix-123`—you're still in the same warm container with all your tools ready. Rebuilds only happen when your environment config actually changes.

### Rig is opinionated so you don't have to be

Dev Containers give you maximum flexibility—and maximum decisions. Rig makes sensible choices:

- **One version manager**: Mise for most languages, SDKMAN for JVM
- **One shell**: Zsh with Oh My Zsh (or bash/fish if you prefer)
- **One base image**: Debian Bookworm, battle-tested
- **Pre-wired for AI**: Every container is ready for AI-assisted development

### Terminal-native, IDE-optional

Rig is built for developers who live in the terminal. Run `rig` and you're in a shell with everything ready. Want VS Code? Enable `code_server` and open it in your browser—on any machine, any OS.

Dev Containers assume you're opening your project in VS Code. Rig assumes you might be SSHing from an iPad, pairing over tmux, or running Claude Code headless on a CI server.

### No JSON, no features matrix, no debugging

With Dev Containers, you're composing features, debugging `postCreateCommand` failures, and wondering why your Docker socket isn't mounted correctly.

With Rig:
```yaml
languages:
  node:
    version: "lts"
shell: zsh
```

That's a complete config. Testcontainers work. Docker works. AI assistants work.

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

### Security: No Privileged Mode

Rig does **not** run containers in privileged mode. Instead, it mounts the host's Docker socket (`/var/run/docker.sock`) into the container. This approach:

- **Avoids privileged mode** — No elevated kernel capabilities or direct root access to the host
- **Uses the host's Docker daemon** — Containers you create are siblings, not nested children
- **Works with Testcontainers** — Pre-configured environment variables ensure compatibility

This is safer than true Docker-in-Docker (which requires `--privileged`), while still enabling full Docker workflows inside your development environment.

It is still possible for malicious code to escape, but it is with extra steps: your rig environment would have to spin up _another_ docker image with privileged mode to escape, then proceed to use that to escape. It's possible, but with extra steps.
At some point, you have to ask yourself, how paranoid are you? Is this better than YOLO'ing Claude or Codex on your host machine without any barriers?

IF you actually are that paranoid, you could also run rig inside a VM quite easily: just create a VM, install docker on it, run rig.

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
