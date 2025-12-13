# Devbox - AI Agent Development Sandbox

A dockerized development sandbox for AI agents (Claude, Gemini, Codex, GitHub CLI).

## Technology

- **Language**: Golang
- **Base Docker Image**: `debian:bookworm-slim` (lightweight, glibc-based, good compatibility)

## CLI Commands

- `devbox` - Enter the container with bash (creates/starts container if needed)
- `devbox init` - Initialize a new workspace with an empty `.assistant.yml` file
- `devbox rebuild` - Force a clean rebuild (removes container and image, rebuilds from scratch)

## Container Behavior

- **Persistence**: Containers are persistent between sessions (not ephemeral)
- **Image naming**: `devbox-<project-directory-name>:<config-hash>`
  - Tag is a truncated SHA256 hash of `.assistant.yml` content
  - Automatically rebuilds when config changes
- **Build**: Uses existing image if available, builds if it doesn't exist or config changed
- **Mounting**: Current directory is mounted; container cannot escape via `cd ../`
- **Docker-in-Docker**: Supported for running testcontainers tests
- **Networking**: Full external internet access

## Agent Installation

AI agents (Claude, Gemini, Codex, GitHub CLI) are installed via npm during Docker build.

## `.assistant.yml` Configuration

### Supported Languages

One version per language (no multi-version support):

| Language | Build Systems |
|----------|---------------|
| Go | (built-in) |
| Node | npm, yarn, pnpm |
| Rust | cargo |
| Java/JVM | Gradle, Maven, Ant, SBT |
| Python | pip, poetry, pipenv |
| Ruby | bundler, gem |

### Version Specification

- Specific version: `"20.10.0"`
- LTS: `"lts"`
- Latest: `"latest"` (default if not specified)

### Build System Versions

Optional. If not specified, uses latest or version defined in project files.

### Port Configuration

Uses Docker's `host:container` format:

```yaml
ports:
  - "8080:8080"    # explicit host:container mapping
  - "3000"         # shorthand: same port on both host and container
```

### Environment Variables

Custom environment variables can be defined for API keys, etc.

### Agent Configuration

Out of scope - agent-specific config files should be placed in the project directory (mounted into container).

### Code Server (VS Code in Browser)

Optional. Set `code_server: true` to install code-server with language-specific extensions.

```yaml
code_server: true       # Install VS Code in browser
code_server_port: 8080  # Optional, defaults to 8080
```

When enabled:
- Installs code-server (VS Code accessible via browser)
- Configures **no password authentication** (suitable for local development)
- **Starts automatically** when the container runs
- Port is automatically exposed (no need to add to `ports` manually)
- Installs language extensions based on configured languages

**Usage:** Just run any devbox command and open the browser:
```bash
devbox bash
# code-server is already running at http://localhost:8080
```

**Language extensions installed:**
- Go: `golang.go`
- Node: `dbaeumer.vscode-eslint`, `esbenp.prettier-vscode`, `ms-vscode.vscode-typescript-next`
- Python: `ms-python.python`, `ms-python.vscode-pylance`, `ms-python.debugpy`
- Java: `redhat.java`, `vscjava.vscode-java-debug`, `vscjava.vscode-java-dependency`, `vscjava.vscode-maven`, `vscjava.vscode-gradle`
- Rust: `rust-lang.rust-analyzer`
- Ruby: `shopify.ruby-lsp`

## Example `.assistant.yml`

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

code_server: true        # Optional: VS Code in browser (auto-starts, port auto-exposed)
# code_server_port: 9000 # Optional: custom port (default: 8080)
```

## Implementation Notes

- Use Go templates for generating Dockerfile (no intermediate files written to disk if possible)
- Executable runs Docker directly with correct parameters (no batch/shell scripts)
- Basic development tooling (git, bash tools, curl, etc.) pre-installed in base image

## Language/Tool Installation

Languages and build tools are installed using modern version managers for flexibility:

| Tool | Languages/Tools | Why |
|------|-----------------|-----|
| [Mise](https://mise.jdx.dev/) | Go, Node, Python, Ruby, Rust | Polyglot version manager, consistent `mise use --global <lang>@<version>` syntax |
| [SDKMAN](https://sdkman.io/) | Java, Gradle, Maven, SBT, Ant | Best-in-class JVM toolchain manager, handles distribution variants (e.g., Temurin) |

- Java versions default to Temurin (Eclipse Adoptium) distribution
- Node.js is always installed (required for AI agent CLIs)
- Mise and SDKMAN are auto-configured in `.bashrc` for interactive sessions
