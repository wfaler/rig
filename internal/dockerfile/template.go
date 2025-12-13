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
    openssh-client \
    gnupg \
    lsb-release \
    sudo \
    gosu \
    vim \
    less \
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
{{ if eq .Shell "zsh" }}    zsh \
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
    '# Start code-server in background if installed' \
    'if command -v code-server > /dev/null 2>&1; then' \
    '  code-server --bind-addr 0.0.0.0:${CODE_SERVER_PORT:-8080} --auth none > /tmp/code-server.log 2>&1 &' \
    '  echo "code-server started on http://localhost:${CODE_SERVER_PORT:-8080}"' \
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

# Install AI agents via npm
RUN eval "$(~/.local/bin/mise activate bash)" && npm install -g @anthropic-ai/claude-code @google/gemini-cli openai

{{ .BuildSystemInstalls }}

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
