package dockerfile

// BaseTemplate is the Dockerfile template used for generating container images
const BaseTemplate = `FROM debian:bookworm-slim

# Prevent interactive prompts during package installation
ENV DEBIAN_FRONTEND=noninteractive

# Docker-in-Docker support for testcontainers
ENV DOCKER_HOST=unix:///var/run/docker.sock
ENV TESTCONTAINERS_DOCKER_SOCKET_OVERRIDE=/var/run/docker.sock
ENV TESTCONTAINERS_HOST_OVERRIDE=host.docker.internal
ENV TESTCONTAINERS_RYUK_DISABLED=true

# Base system packages
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    curl \
    wget \
    git \
    build-essential \
    clang \
    openssh-client \
    gnupg \
    lsb-release \
    sudo \
    gosu \
    vim \
    less \
    tmux \
    jq \
    unzip \
    zip \
    procps \
    libssl-dev \
    zlib1g-dev \
    libbz2-dev \
    libreadline-dev \
    libsqlite3-dev \
    libffi-dev \
{{ if .Herdr }}    socat \
{{ end }}{{ if eq .Shell "zsh" }}    zsh \
{{ else if eq .Shell "fish" }}    fish \
{{ end }}    && rm -rf /var/lib/apt/lists/*

# Docker CLI for DinD support (testcontainers)
RUN curl -fsSL https://download.docker.com/linux/debian/gpg | gpg --dearmor -o /usr/share/keyrings/docker-archive-keyring.gpg \
    && echo "deb [arch=$(dpkg --print-architecture) signed-by=/usr/share/keyrings/docker-archive-keyring.gpg] https://download.docker.com/linux/debian $(lsb_release -cs) stable" > /etc/apt/sources.list.d/docker.list \
    && apt-get update && apt-get install -y --no-install-recommends docker-ce-cli \
    && rm -rf /var/lib/apt/lists/*

# GitHub CLI
RUN mkdir -p /etc/apt/keyrings \
    && curl -fsSL https://cli.github.com/packages/githubcli-archive-keyring.gpg -o /etc/apt/keyrings/githubcli-archive-keyring.gpg \
    && chmod go+r /etc/apt/keyrings/githubcli-archive-keyring.gpg \
    && echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/githubcli-archive-keyring.gpg] https://cli.github.com/packages stable main" > /etc/apt/sources.list.d/github-cli.list \
    && apt-get update && apt-get install -y gh \
    && rm -rf /var/lib/apt/lists/*

{{ if .CodeServer }}
# Install code-server (VS Code in browser)
RUN curl -fsSL https://code-server.dev/install.sh | sh
{{ end }}

# Create non-root user for development
RUN useradd -m -s /bin/{{ .Shell }} developer \
    && echo "developer ALL=(ALL) NOPASSWD:ALL" >> /etc/sudoers

# Add developer to docker group for socket access
RUN groupadd -f docker && usermod -aG docker developer

{{ if eq .Shell "zsh" }}
# Install Oh My Zsh for developer user
USER developer
RUN sh -c "$(curl -fsSL https://raw.githubusercontent.com/ohmyzsh/ohmyzsh/master/tools/install.sh)" "" --unattended
USER root
{{ end }}

# Create entrypoint script to fix Docker socket permissions and start services
RUN printf '%s\n' '#!/bin/bash' \
    '# Fix Docker socket permissions' \
    'if [ -S /var/run/docker.sock ]; then' \
    '  sudo chmod 666 /var/run/docker.sock' \
    'fi' \
{{ if .Herdr }}    '# Fix herdr socket permissions if mounted (Linux bind-mount path)' \
    'if [ -S {{ .HerdrSocketPath }} ]; then' \
    '  sudo chmod 666 {{ .HerdrSocketPath }} 2>/dev/null || true' \
    'fi' \
    '# Bridge the herdr socket over TCP when rig provides a proxy port (macOS' \
    '# path: unix sockets cannot cross the Docker VM boundary, so rig proxies' \
    '# the host socket on a loopback port and socat re-exposes it here)' \
    'if [ -n "${RIG_HERDR_PROXY_PORT}" ] && [ ! -S {{ .HerdrSocketPath }} ]; then' \
    '  sudo mkdir -p /run/herdr' \
    '  sudo chown developer /run/herdr' \
    '  (socat UNIX-LISTEN:{{ .HerdrSocketPath }},fork,unlink-early,mode=666 TCP:host.docker.internal:${RIG_HERDR_PROXY_PORT} > /tmp/herdr-proxy.log 2>&1 &)' \
    'fi' \
{{ end }}
    '# Start code-server in background if installed' \
    'if command -v code-server > /dev/null 2>&1; then' \
    '  code-server --bind-addr 0.0.0.0:${CODE_SERVER_PORT:-8080} --auth none > /tmp/code-server.log 2>&1 &' \
    '  echo "code-server started on http://localhost:${CODE_SERVER_PORT:-8080}"' \
    'fi' \
    '# Start markdown server in background if installed' \
    'if [ -f /usr/local/bin/rig-md-server.js ]; then' \
    '  NODE_PATH=$(~/.local/bin/mise exec -- npm root -g) ~/.local/bin/mise exec -- node /usr/local/bin/rig-md-server.js > /tmp/rig-md-server.log 2>&1 &' \
    '  echo "Markdown server started on http://localhost:${RIG_MD_PORT:-3030}"' \
    'fi' \
    'exec "$@"' > /usr/local/bin/docker-entrypoint.sh \
    && chmod +x /usr/local/bin/docker-entrypoint.sh

ENTRYPOINT ["/usr/local/bin/docker-entrypoint.sh"]

# Switch to developer user for tool installation
USER developer
WORKDIR /home/developer

# Use bash for all subsequent RUN commands (Mise requires bash-specific syntax)
SHELL ["/bin/bash", "-c"]

# Install Mise (polyglot version manager) for Go, Node, Python, Ruby, Rust
RUN curl https://mise.run | sh
ENV PATH="/home/developer/.local/bin:${PATH}"

# Avoid freethreaded Python builds which are missing the lib directory
ENV MISE_PYTHON_PRECOMPILED_FLAVOR=install_only_stripped
ENV MISE_PYTHON_FREETHREADED=0

{{ if .HasJava }}
# Install SDKMAN for Java and JVM tools
RUN curl -s "https://get.sdkman.io?rcupdate=false" | bash
{{ end }}

# Configure shell to load Mise and SDKMAN
{{ if eq .Shell "bash" }}RUN echo 'eval "$(~/.local/bin/mise activate bash)"' >> ~/.bashrc {{ if .HasJava }}&& echo 'source ~/.sdkman/bin/sdkman-init.sh' >> ~/.bashrc{{ end }}
{{ else if eq .Shell "zsh" }}RUN echo 'eval "$(~/.local/bin/mise activate zsh)"' >> ~/.zshrc {{ if .HasJava }}&& echo 'source ~/.sdkman/bin/sdkman-init.sh' >> ~/.zshrc{{ end }}
{{ else if eq .Shell "fish" }}RUN mkdir -p ~/.config/fish && echo 'mise activate fish | source' >> ~/.config/fish/config.fish {{ if .HasJava }}&& echo 'source ~/.sdkman/bin/sdkman-init.sh' >> ~/.config/fish/config.fish{{ end }}
{{ end }}

{{ .LanguageInstalls }}

{{ if not .HasNode }}
# Install Node.js LTS for AI agents (required even if not explicitly configured)
RUN mise use --global node@lts
{{ end }}

# Install Claude Code (native binary - recommended method)
RUN curl -fsSL https://claude.ai/install.sh | bash

# Install other AI agents via npm
RUN eval "$(~/.local/bin/mise activate bash)" && npm install -g @google/gemini-cli openai

{{ if .Herdr }}
# Install the herdr CLI (static binary) so a containerized agent can drive the
# herdr control socket API. uname -m yields x86_64/aarch64, matching the release
# asset names.
USER root
RUN HERDR_ARCH="$(uname -m)" \
    && curl -fsSL "https://github.com/herdrdev/herdr/releases/latest/download/herdr-linux-${HERDR_ARCH}" -o /usr/local/bin/herdr \
    && chmod 755 /usr/local/bin/herdr
USER developer

# Install the herdr agent skill so Claude Code auto-discovers it
RUN mkdir -p /home/developer/.claude/skills/herdr \
    && curl -fsSL https://raw.githubusercontent.com/herdrdev/herdr/master/skills/herdr/SKILL.md \
       -o /home/developer/.claude/skills/herdr/SKILL.md

# Install the rig sandbox skill so in-container agents spawn new herdr panes
# through rig (sandboxed) instead of as bare host shells
USER root
COPY rig-sandbox-skill.md /home/developer/.claude/skills/rig-sandbox/SKILL.md
RUN chown -R developer:developer /home/developer/.claude/skills/rig-sandbox
USER developer
{{ end }}

{{ .BuildSystemInstalls }}

{{ if .MarkdownServer }}
# Install marked (markdown parser) for markdown server
RUN eval "$(~/.local/bin/mise activate bash)" && npm install -g marked@latest

# Configure markdown server port
ENV RIG_MD_PORT={{ .MarkdownServerPort }}

# Install markdown server script (switch to root for /usr/local/bin access)
USER root
COPY rig-md-server.js /usr/local/bin/rig-md-server.js
RUN chmod 755 /usr/local/bin/rig-md-server.js
USER developer
{{ end }}

{{ if .CodeServer }}
# Configure code-server port
ENV CODE_SERVER_PORT={{ .CodeServerPort }}

# Configure code-server: no authentication, bind to all interfaces
RUN mkdir -p /home/developer/.config/code-server \
    && echo 'bind-addr: 0.0.0.0:{{ .CodeServerPort }}' > /home/developer/.config/code-server/config.yaml \
    && echo 'auth: none' >> /home/developer/.config/code-server/config.yaml \
    && echo 'cert: false' >> /home/developer/.config/code-server/config.yaml

# Configure VS Code settings (theme)
RUN mkdir -p /home/developer/.local/share/code-server/User \
    && echo '{"workbench.colorTheme": "{{ .CodeServerTheme }}"}' > /home/developer/.local/share/code-server/User/settings.json

{{ if .CodeServerExtensions }}
# Install VS Code extensions for configured languages
RUN {{ range $i, $ext := .CodeServerExtensions }}{{ if $i }} && {{ end }}code-server --install-extension {{ $ext }}{{ end }}
{{ end }}
{{ end }}

WORKDIR /workspace

{{ range $key, $value := .Env }}
ENV {{ $key }}="{{ $value }}"
{{ end }}

CMD ["/bin/{{ .Shell }}"]
`
