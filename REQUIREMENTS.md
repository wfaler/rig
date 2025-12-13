# Devbox Technical Specification

This document serves as the technical reference for devbox internals and configuration options.

## Architecture

### Technology Stack

- **Language**: Go 1.22+
- **CLI Framework**: Cobra
- **Container Runtime**: Docker (via Docker SDK for Go)
- **Base Image**: `debian:bookworm-slim`

### Version Managers

| Tool | Languages/Tools |
|------|-----------------|
| [Mise](https://mise.jdx.dev/) | Go, Node, Python, Ruby, Rust |
| [SDKMAN](https://sdkman.io/) | Java, Gradle, Maven, SBT, Ant |

Java defaults to Temurin (Eclipse Adoptium) distribution.

---

## CLI Commands

| Command | Description |
|---------|-------------|
| `devbox` | Enter container with bash (creates/starts if needed) |
| `devbox init` | Create `.assistant.yml` template in current directory |
| `devbox rebuild` | Force clean rebuild (removes container + image) |

---

## Container Behavior

### Lifecycle

- **Persistent**: Containers are reused across sessions (not ephemeral)
- **Auto-rebuild**: Image rebuilds when `.assistant.yml` content changes
- **Named**: Container named `devbox-<project-directory>`

### Image Tagging

```
devbox-<project>:<hash>
```

- `<project>`: Current directory name
- `<hash>`: First 12 characters of SHA256 hash of `.assistant.yml`

### Mounts

| Host | Container | Purpose |
|------|-----------|---------|
| Current directory | `/workspace` | Project files |
| `/var/run/docker.sock` | `/var/run/docker.sock` | Docker-in-Docker |

### Networking

- Full external internet access
- `host.docker.internal` resolves to host machine
- Configured ports exposed to host

### Entrypoint

The container entrypoint:
1. Fixes Docker socket permissions (`chmod 666`)
2. Starts code-server if installed
3. Executes the requested command

---

## Configuration Reference

### `.assistant.yml` Schema

```yaml
# Language runtimes
languages:
  <language>:
    version: "<version>"                    # "lts", "latest", or specific
    build_system: "<system>"                # optional
    build_system_version: "<version>"       # optional

# Port mappings
ports:
  - "<host>:<container>"   # explicit mapping
  - "<port>"               # same on both

# Environment variables (supports ${VAR} expansion)
env:
  KEY: "value"
  SECRET: "${HOST_SECRET}"

# Default shell
shell: zsh                       # zsh with oh-my-zsh (default), bash, fish

# VS Code in browser
code_server:
  enabled: true|false
  port: 8080                    # default: 8080, auto-exposed
  theme: "Default Dark Modern"  # VS Code theme name
  extensions:                   # additional extensions
    - extension.id
```

### Supported Shells

| Shell | Value | Notes |
|-------|-------|-------|
| Zsh | `zsh` | Default, includes Oh My Zsh |
| Bash | `bash` | Standard bash |
| Fish | `fish` | Fish shell |

### Supported Languages

| Language | Key | Versions | Build Systems |
|----------|-----|----------|---------------|
| Node.js | `node` | `lts`, `latest`, `20`, `20.10.0` | `npm`, `yarn`, `pnpm` |
| Python | `python` | `latest`, `3.12`, `3.12.1` | `pip`, `poetry`, `pipenv` |
| Go | `go` | `latest`, `1.22`, `1.22.1` | (built-in) |
| Java | `java` | `latest`, `21`, `17` | `gradle`, `maven`, `sbt`, `ant` |
| Rust | `rust` | `latest`, `1.75`, `1.75.0` | `cargo` |
| Ruby | `ruby` | `latest`, `3.3`, `3.3.0` | `bundler`, `gem` |

### Recommended Code Server Extensions

Extensions are configured explicitly in the `extensions` list. Recommended extensions by language:

| Language | Extensions |
|----------|------------|
| Go | `golang.go` |
| Node | `dbaeumer.vscode-eslint`, `esbenp.prettier-vscode`, `ms-vscode.vscode-typescript-next` |
| Python | `ms-python.python`, `ms-python.vscode-pylance`, `ms-python.debugpy` |
| Java | `redhat.java`, `vscjava.vscode-java-debug`, `vscjava.vscode-java-dependency`, `vscjava.vscode-maven`, `vscjava.vscode-gradle` |
| Rust | `rust-lang.rust-analyzer` |
| Ruby | `shopify.ruby-lsp` |
| General | `github.copilot`, `eamodio.gitlens` |

---

## Pre-installed Tools

### System Packages

```
ca-certificates curl wget git build-essential openssh-client
gnupg lsb-release sudo gosu vim less jq unzip zip procps
libssl-dev zlib1g-dev libbz2-dev libreadline-dev libsqlite3-dev libffi-dev
```

### Docker & GitHub CLI

- `docker-ce-cli` (Docker CLI for DinD)
- `gh` (GitHub CLI)

### AI Agent CLIs

Installed via npm:
- `@anthropic-ai/claude-code` (Claude Code)
- `@google/gemini-cli` (Gemini CLI)
- `openai` (OpenAI CLI)

---

## Example Configuration

```yaml
languages:
  node:
    version: "lts"
    build_system: npm
  python:
    version: "3.12"
    build_system: poetry
    build_system_version: "1.7.0"
  java:
    version: "21"
    build_system: gradle

ports:
  - "3000"
  - "5432:5432"

env:
  OPENAI_API_KEY: "${OPENAI_API_KEY}"
  DATABASE_URL: "postgres://localhost:5432/dev"

shell: zsh

code_server:
  enabled: true
  theme: "Default Dark Modern"
  extensions:
    - github.copilot
    - eamodio.gitlens
```

---

## Environment Variables

### Set in Container

| Variable | Value | Purpose |
|----------|-------|---------|
| `DOCKER_HOST` | `unix:///var/run/docker.sock` | Docker daemon location |
| `TESTCONTAINERS_DOCKER_SOCKET_OVERRIDE` | `/var/run/docker.sock` | Testcontainers config |
| `TESTCONTAINERS_HOST_OVERRIDE` | `host.docker.internal` | Testcontainers host resolution |
| `TESTCONTAINERS_RYUK_DISABLED` | `true` | Disable Ryuk cleanup container |
| `CODE_SERVER_PORT` | (configured port) | Code-server port (if enabled) |

---

## Project Structure

```
devbox/
├── main.go                      # Entry point
├── cmd/
│   ├── root.go                  # Root command, enters container
│   ├── init.go                  # devbox init
│   ├── rebuild.go               # devbox rebuild
│   └── session.go               # Container session orchestration
├── internal/
│   ├── config/
│   │   ├── config.go            # Config struct, parsing, validation
│   │   └── config_test.go
│   ├── docker/
│   │   ├── client.go            # Docker SDK client wrapper
│   │   ├── image.go             # Image build/check/remove
│   │   ├── container.go         # Container lifecycle
│   │   ├── attach.go            # TTY attachment
│   │   └── interfaces.go        # DockerClient interface
│   ├── dockerfile/
│   │   ├── generator.go         # Template execution
│   │   ├── generator_test.go
│   │   ├── template.go          # Embedded Dockerfile template
│   │   ├── languages.go         # Language/build system installers
│   │   └── languages_test.go
│   └── project/
│       ├── project.go           # Project naming, hash computation
│       └── project_test.go
├── REQUIREMENTS.md              # This file
├── README.md                    # User documentation
├── Makefile
└── go.mod
```

---

## Adding New Languages

1. Add to `SupportedLanguages` map in `internal/config/config.go`
2. Add build systems to `BuildSystemsForLanguage` in `internal/config/config.go`
3. Add installer function in `internal/dockerfile/languages.go`
4. Add VS Code extensions to `VSCodeExtensionsForLanguage` in `internal/dockerfile/languages.go`
5. Add tests

## Adding New AI Agents

1. Add npm package to AI agents install line in `internal/dockerfile/template.go`
