---
title: "Rig Development Environments"
type: prd
status: Approved
feature: "rig-core"
created: 2025-01-01
updated: 2026-03-27
adrs: ["0001"]
---

# Rig — Product Requirements Document

## Vision

Rig is a CLI tool that gives developers instant, reproducible development environments purpose-built for AI-assisted coding. One YAML file, one command, and you're in a fully-equipped container with your language runtimes, build tools, and AI coding assistants ready to go.

## Problem Statement

Setting up a development environment that works with AI coding agents is painful. Developers must install language runtimes, configure build tools, set up Docker-in-Docker for testcontainers, install and authenticate AI assistants, and then do it all again on a different machine. Dev container solutions exist but are IDE-locked, config-heavy, and not designed for AI-first workflows.

The result: developers spend hours on environment setup instead of building software, and AI agents operate in unpredictable environments that cause tool failures, missing dependencies, and wasted tokens.

## Target Users

- **AI-assisted developers** using Claude Code, Gemini CLI, or similar agents who want their agent to have a consistent, well-equipped environment
- **Terminal-first developers** who prefer shell workflows over IDE-heavy setups
- **Teams** wanting reproducible environments without the complexity of Nix, Devbox, or devcontainer.json feature matrices
- **Developers working across machines** (desktop, laptop, remote servers, CI) who need the same environment everywhere

## Design Principles

1. **Opinionated over configurable.** Make good default choices (Debian Bookworm, Mise, Zsh + Oh My Zsh) so users don't have to. Expose configuration only where preferences genuinely vary (language versions, shell choice, ports).

2. **AI-first.** Every container ships with AI coding assistants pre-installed. The environment is designed to make AI agents productive — correct PATH setup, global tool access, documentation searchable via MCP.

3. **Zero to productive in one command.** `rig init && rig up` should get any developer from nothing to a working environment. No JSON authoring, no feature selection, no debugging build failures.

4. **Persistent, not ephemeral.** Containers survive between sessions. Switch branches, close your terminal, come back tomorrow — your environment is warm and ready. Rebuilds only happen when your config actually changes.

5. **Terminal-native, IDE-optional.** Rig works from any terminal on any OS. VS Code in the browser (code-server) is available but never required.

6. **Secure by default.** No privileged containers. Docker-in-Docker via socket mounting, not elevated kernel capabilities.

## Core Capabilities

### Container Lifecycle

Rig manages the full lifecycle of development containers:

- **Create**: Generate a Dockerfile from `.rig.yml`, build an image, create a container with workspace and Docker socket mounts
- **Enter**: Attach a TTY with the configured shell; start background services (code-server, markdown server)
- **Persist**: Containers are named `rig-<project>` and survive between sessions. Internal state (installed packages, caches, tool configs) is preserved
- **Rebuild**: When `.rig.yml` changes (detected via content hash), the image rebuilds automatically on next `rig up`
- **Destroy**: Clean removal of container and all associated images

### Language & Build System Support

Rig supports polyglot development with managed runtimes:

| Language | Version Manager | Build Systems |
|----------|----------------|---------------|
| Node.js | Mise | npm, yarn, pnpm |
| Python | Mise | pip, poetry, pipenv, uv |
| Go | Mise | built-in |
| Java/Kotlin/Scala | SDKMAN | gradle, maven, sbt, ant |
| Rust | Mise | cargo |
| Ruby | Mise | bundler, gem |

Node.js LTS is always installed (required by AI agent CLIs) even if not explicitly configured.

### AI Agent Integration

Every container includes:

- **Claude Code** (native binary)
- **Gemini CLI** (npm)
- **OpenAI CLI** (npm)
- **GitHub CLI** (apt)

Agents run inside the container with full access to the workspace, Docker socket, and all installed tools. The environment is pre-configured so agents can run tests, build projects, manage containers (testcontainers), and access host services via `host.docker.internal`.

### Documentation Server

A built-in markdown server renders `.md` files from the workspace as navigable HTML:

- Live-reload via Server-Sent Events — edit in your editor, see changes instantly in the browser
- Directory-tree navigation sidebar with collapsible folders
- Light/dark theme with persistent preference
- Mermaid diagram rendering (loaded from CDN on demand)
- Breadcrumb navigation showing the file's repo-relative path

Enabled by default on port 3030. No configuration required.

### Code Server (VS Code in Browser)

Optional browser-based VS Code via code-server:

- Configurable theme and extensions
- No authentication (local development context)
- Access from any device on the network

### Docker-in-Docker

Testcontainers and Docker workflows work out of the box:

- Host Docker socket mounted into the container (not privileged mode)
- `TESTCONTAINERS_*` environment variables pre-configured
- Containers created inside rig are siblings on the host Docker daemon
- Safer than traditional privileged DinD while supporting the same workflows

### Configuration

A single `.rig.yml` file defines the entire environment:

```yaml
languages:
  node:
    version: "lts"
    build_systems:
      yarn: true
  python:
    version: "3.12"
    build_systems:
      poetry: "1.8.0"

ports:
  - "3000"
  - "5432:5432"

env:
  API_KEY: "${API_KEY}"    # expanded from host environment

shell: zsh                 # zsh (default), bash, or fish

code_server:
  enabled: true
  extensions:
    - github.copilot

markdown_server:
  port: 3030               # enabled by default
```

## Planned Capabilities

### Workspace Search & MCP Server (ADR-0001)

Full-text and semantic search across workspace files, exposed both as a browser UI and as MCP tools for AI agents. See [ADR-0001](../adr/0001-search-and-mcp-server.md) for architecture.

**Goals:**
- Let users search documentation from the browser without leaving the markdown server
- Let AI agents discover and search workspace documentation programmatically, reducing token waste from broad grep queries
- Support semantic queries ("how does authentication work?") that grep cannot answer

### MCP Auto-Configuration

`rig init` should generate `.mcp.json` and agent guidance files (`CLAUDE.md`, etc.) so AI agents can discover workspace tools without manual setup. The goal is that an AI agent dropped into a rig container can immediately use the search MCP server without the user configuring anything.

## Competitive Positioning

### vs. Dev Containers

| | Rig | Dev Containers |
|---|---|---|
| Config complexity | Single `.rig.yml` (~10 lines) | `devcontainer.json` + Dockerfile + features |
| IDE lock-in | Any editor, terminal, or browser | VS Code (or compatible) |
| AI assistants | Pre-installed and ready | Manual setup |
| Setup time | `rig init && rig up` | Configure JSON, choose features, debug builds |
| Testcontainers | Works out of the box | Requires manual DinD setup |
| Branch switching | Same container, instant | Often rebuilds per branch |

### vs. Nix / Devbox

Rig trades Nix's reproducibility guarantees for dramatically simpler setup. Nix requires learning a functional language and managing flakes; Rig requires a 10-line YAML file. For teams where "good enough reproducibility" (Debian + pinned versions via Mise) is acceptable, Rig is the faster path.

### vs. Docker Compose

Docker Compose manages multi-service architectures; Rig manages single-developer environments. They're complementary — a developer might use Rig for their dev environment and Docker Compose for the services their application depends on (accessed via `host.docker.internal`).

## Success Criteria

- A developer with Docker installed can go from zero to a working AI-assisted environment in under 3 minutes (including first image build)
- Switching branches never triggers a rebuild
- AI agents (Claude Code, Gemini) can run tests, build projects, and use testcontainers without any manual environment configuration
- The markdown server renders any standard GitHub-flavored markdown correctly, including mermaid diagrams
- Configuration changes are the only thing that triggers rebuilds — everything else persists
