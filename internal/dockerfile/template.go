package dockerfile

// MarkdownServerScript is the Node.js script that serves markdown files as HTML
const MarkdownServerScript = `const http = require('http');
const fs = require('fs');
const path = require('path');
const { marked } = require('marked');

const PORT = parseInt(process.env.RIG_MD_PORT || '3030', 10);
const ROOT = process.env.RIG_MD_ROOT || '/workspace';

// Track SSE clients for hot-reload
const clients = new Set();

// Watch for file changes recursively
function watchDir(dir) {
  try {
    fs.watch(dir, { recursive: true }, (eventType, filename) => {
      if (filename && filename.endsWith('.md')) {
        if (typeof fileListDirty !== 'undefined') fileListDirty = true;
        const payload = JSON.stringify({ file: filename, event: eventType });
        for (const res of clients) {
          res.write('data: ' + payload + '\n\n');
        }
      }
    });
  } catch (e) {
    console.error('Watch error:', e.message);
  }
}

// GitHub-inspired CSS for rendered markdown with light/dark theme support
const CSS = ` + "`" + `
:root {
  --color-fg: #1f2328; --color-fg-muted: #656d76; --color-bg: #ffffff;
  --color-border: #d0d7de; --color-border-muted: #d8dee4;
  --color-bg-subtle: #f6f8fa; --color-link: #0969da;
  --color-heading: #1f2328; --color-badge-bg: #1a7f37; --color-badge-err: #cf222e;
  --nav-width: 260px;
}
[data-theme="dark"] {
  --color-fg: #c9d1d9; --color-fg-muted: #8b949e; --color-bg: #0d1117;
  --color-border: #30363d; --color-border-muted: #21262d;
  --color-bg-subtle: #161b22; --color-link: #58a6ff;
  --color-heading: #e6edf3; --color-badge-bg: #238636; --color-badge-err: #da3633;
}
* { box-sizing: border-box; }
body {
  font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Helvetica, Arial, sans-serif;
  line-height: 1.6; color: var(--color-fg); background: var(--color-bg);
  margin: 0; padding: 0;
}
.layout { display: flex; min-height: 100vh; }
.content { flex: 1; max-width: 900px; margin: 0 auto; padding: 2rem; min-width: 0; }
/* Left nav sidebar */
.nav-sidebar {
  width: var(--nav-width); min-width: var(--nav-width); border-right: 1px solid var(--color-border-muted); order: -1;
  background: var(--color-bg-subtle); padding: 1rem; overflow-y: auto; position: sticky;
  top: 0; height: 100vh; font-size: 0.85em; transition: min-width 0.2s, width 0.2s, padding 0.2s;
}
.nav-sidebar.collapsed { width: 0; min-width: 0; padding: 0; overflow: hidden; border-right: none; }
.nav-sidebar h3 { margin: 0 0 0.4em 0; font-size: 0.95em; color: var(--color-heading); border: none; padding: 0; }
.nav-sidebar ul { list-style: none; padding: 0; margin: 0; }
.nav-sidebar li { padding: 0.2em 0; }
.nav-sidebar li.dir > span { font-weight: 600; color: var(--color-fg-muted); font-size: 0.9em; display: block; padding: 0.3em 0 0.1em; cursor: pointer; user-select: none; }
.nav-sidebar li.dir > span:hover { color: var(--color-fg); }
.nav-sidebar li.dir > span::before { content: '\u25BE '; font-size: 0.8em; }
.nav-sidebar li.dir.collapsed > span::before { content: '\u25B8 '; }
.nav-sidebar li.dir.collapsed > ul { display: none; }
.nav-sidebar li.dir > ul { padding-left: 0.8em; }
.nav-sidebar a { color: var(--color-link); text-decoration: none; display: block; padding: 0.15em 0.4em; border-radius: 4px; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.nav-sidebar a:hover { background: var(--color-border-muted); text-decoration: none; }
.nav-sidebar a.active { background: var(--color-link); color: #fff; }
.nav-toggle {
  position: fixed; left: 1rem; bottom: 1rem; background: var(--color-bg-subtle);
  border: 1px solid var(--color-border); color: var(--color-fg); width: 2em; height: 2em;
  border-radius: 50%; cursor: pointer; font-size: 0.9em; display: flex; align-items: center;
  justify-content: center; z-index: 10; line-height: 1;
}
.nav-toggle:hover { background: var(--color-border-muted); }
a { color: var(--color-link); text-decoration: none; }
a:hover { text-decoration: underline; }
h1, h2, h3, h4, h5, h6 { color: var(--color-heading); margin-top: 1.5em; margin-bottom: 0.5em; border-bottom: 1px solid var(--color-border-muted); padding-bottom: 0.3em; }
h1 { font-size: 2em; } h2 { font-size: 1.5em; } h3 { font-size: 1.25em; }
code { background: var(--color-bg-subtle); padding: 0.2em 0.4em; border-radius: 6px; font-size: 85%; font-family: ui-monospace, "SFMono-Regular", "SF Mono", Menlo, monospace; }
pre { background: var(--color-bg-subtle); padding: 1em; border-radius: 6px; overflow-x: auto; border: 1px solid var(--color-border); }
pre code { background: none; padding: 0; font-size: 90%; }
blockquote { border-left: 4px solid var(--color-border); padding: 0 1em; color: var(--color-fg-muted); margin: 0; }
table { border-collapse: collapse; width: 100%; margin: 1em 0; }
th, td { border: 1px solid var(--color-border); padding: 0.5em 1em; text-align: left; }
th { background: var(--color-bg-subtle); }
tr:nth-child(even) { background: var(--color-bg-subtle); }
img { max-width: 100%; }
hr { border: none; border-top: 1px solid var(--color-border-muted); margin: 1.5em 0; }
ul, ol { padding-left: 2em; }
.file-list { list-style: none; padding: 0; }
.file-list li { padding: 0.4em 0; border-bottom: 1px solid var(--color-border-muted); }
.file-list li:last-child { border-bottom: none; }
.file-list a { font-size: 1.1em; }
.breadcrumb { color: var(--color-fg-muted); margin-bottom: 1em; font-size: 0.9em; }
.breadcrumb a { color: var(--color-link); }
.reload-badge { position: fixed; top: 1rem; right: 1rem; background: var(--color-badge-bg); color: #fff; padding: 0.3em 0.8em; border-radius: 20px; font-size: 0.75em; opacity: 0.8; z-index: 10; }
.reload-badge.disconnected { background: var(--color-badge-err); }
.theme-toggle { position: fixed; top: 1rem; right: 4rem; background: var(--color-bg-subtle); border: 1px solid var(--color-border); color: var(--color-fg); padding: 0.3em 0.7em; border-radius: 20px; font-size: 0.85em; cursor: pointer; z-index: 10; }
.theme-toggle:hover { background: var(--color-border-muted); }
` + "`" + `;

// Hot-reload + theme toggle + mermaid client script
const CLIENT_JS = ` + "`" + `
(function() {
  // Theme toggle
  var saved = localStorage.getItem('rig-theme') || 'light';
  document.documentElement.setAttribute('data-theme', saved);
  var btn = document.createElement('button');
  btn.className = 'theme-toggle';
  btn.textContent = saved === 'dark' ? 'Light' : 'Dark';
  btn.onclick = function() {
    var cur = document.documentElement.getAttribute('data-theme');
    var next = cur === 'dark' ? 'light' : 'dark';
    document.documentElement.setAttribute('data-theme', next);
    localStorage.setItem('rig-theme', next);
    btn.textContent = next === 'dark' ? 'Light' : 'Dark';
    // Re-render mermaid with matching theme (requires page reload for full re-render)
    if (window.mermaid) { window.location.reload(); }
  };
  document.body.appendChild(btn);

  // Nav sidebar toggle
  var nav = document.getElementById('nav-sidebar');
  if (nav) {
    var navHidden = localStorage.getItem('rig-nav-hidden') === 'true';
    if (navHidden) nav.classList.add('collapsed');
    var navBtn = document.createElement('button');
    navBtn.className = 'nav-toggle';
    navBtn.innerHTML = navHidden ? '&#9776;' : '&#10005;';
    navBtn.title = 'Toggle file nav';
    navBtn.onclick = function() {
      nav.classList.toggle('collapsed');
      var hidden = nav.classList.contains('collapsed');
      localStorage.setItem('rig-nav-hidden', hidden);
      navBtn.innerHTML = hidden ? '&#9776;' : '&#10005;';
    };
    document.body.appendChild(navBtn);
    // Highlight current page and expand its parent dirs
    var cur = decodeURIComponent(window.location.pathname);
    var links = nav.querySelectorAll('a');
    for (var i = 0; i < links.length; i++) {
      if (links[i].getAttribute('href') === cur) {
        links[i].classList.add('active');
        // Expand parent directories
        var p = links[i].parentElement;
        while (p && p !== nav) { if (p.classList && p.classList.contains('dir')) p.classList.remove('collapsed'); p = p.parentElement; }
      }
    }
    // Directory collapse/expand
    var dirs = nav.querySelectorAll('li.dir > span');
    for (var d = 0; d < dirs.length; d++) {
      dirs[d].addEventListener('click', (function(el) { return function() { el.parentElement.classList.toggle('collapsed'); }; })(dirs[d]));
    }
    // Start directories collapsed (except those with active page)
    var allDirs = nav.querySelectorAll('li.dir');
    for (var dd = 0; dd < allDirs.length; dd++) {
      if (!allDirs[dd].querySelector('a.active')) allDirs[dd].classList.add('collapsed');
    }
  }

  // Live-reload badge
  var badge = document.createElement('div');
  badge.className = 'reload-badge';
  badge.textContent = 'live';
  document.body.appendChild(badge);
  function connect() {
    var es = new EventSource('/_rig/events');
    es.onopen = function() { badge.className = 'reload-badge'; badge.textContent = 'live'; };
    es.onmessage = function(e) {
      var data = JSON.parse(e.data);
      var current = decodeURIComponent(window.location.pathname.slice(1));
      if (data.file === current || window.location.pathname === '/') {
        window.location.reload();
      }
    };
    es.onerror = function() { badge.className = 'reload-badge disconnected'; badge.textContent = 'reconnecting'; es.close(); setTimeout(connect, 2000); };
  }
  connect();

  // Mermaid diagram rendering
  var mermaidBlocks = document.querySelectorAll('pre code.language-mermaid');
  if (mermaidBlocks.length > 0) {
    // Replace code blocks with mermaid divs before loading the library
    for (var mi = 0; mi < mermaidBlocks.length; mi++) {
      var pre = mermaidBlocks[mi].parentElement;
      var div = document.createElement('div');
      div.className = 'mermaid';
      div.textContent = mermaidBlocks[mi].textContent;
      pre.parentNode.replaceChild(div, pre);
    }
    // Load mermaid and let it auto-render all .mermaid divs
    var theme = (localStorage.getItem('rig-theme') || 'light') === 'dark' ? 'dark' : 'default';
    var s = document.createElement('script');
    s.src = 'https://cdn.jsdelivr.net/npm/mermaid@11/dist/mermaid.min.js';
    s.onload = function() { mermaid.initialize({ startOnLoad: false, theme: theme }); mermaid.run(); };
    document.head.appendChild(s);
  }
})();
` + "`" + `;

function findMarkdownFiles(dir, base) {
  base = base || '';
  var results = [];
  try {
    var entries = fs.readdirSync(dir, { withFileTypes: true });
    for (var i = 0; i < entries.length; i++) {
      var entry = entries[i];
      var rel = base ? base + '/' + entry.name : entry.name;
      if (entry.name.startsWith('.') || entry.name === 'node_modules') continue;
      if (entry.isDirectory()) {
        results = results.concat(findMarkdownFiles(path.join(dir, entry.name), rel));
      } else if (entry.name.endsWith('.md')) {
        results.push(rel);
      }
    }
  } catch (e) {}
  return results;
}

function buildNavTree(files) {
  // Group files by directory
  var tree = { _files: [] };
  for (var i = 0; i < files.length; i++) {
    var parts = files[i].split('/');
    var node = tree;
    for (var j = 0; j < parts.length - 1; j++) {
      if (!node[parts[j]]) node[parts[j]] = { _files: [] };
      node = node[parts[j]];
    }
    node._files.push(files[i]);
  }
  return tree;
}

function renderNavNode(node, name) {
  var html = '';
  // Render files at this level
  for (var i = 0; i < node._files.length; i++) {
    var f = node._files[i];
    var label = f.split('/').pop();
    html += '<li><a href="/' + encodeURIComponent(f) + '">' + label + '</a></li>';
  }
  // Render subdirectories
  var keys = Object.keys(node).filter(function(k) { return k !== '_files'; }).sort();
  for (var j = 0; j < keys.length; j++) {
    html += '<li class="dir"><span>' + keys[j] + '/</span><ul>' + renderNavNode(node[keys[j]], keys[j]) + '</ul></li>';
  }
  return html;
}

function renderNav(files) {
  if (files.length === 0) return '';
  var tree = buildNavTree(files);
  return '<nav class="nav-sidebar" id="nav-sidebar"><h3>Files</h3>' +
    '<ul>' + renderNavNode(tree, '') + '</ul></nav>';
}

function renderPage(title, body, files) {
  var nav = renderNav(files || []);
  return '<!DOCTYPE html><html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>' +
    title + ' - Rig Docs</title><style>' + CSS + '</style></head><body><div class="layout"><div class="content">' + body +
    '</div>' + nav + '</div><script>' + CLIENT_JS + '<\/script></body></html>';
}

// Cached file list — refreshed on .md file changes
var cachedFiles = findMarkdownFiles(ROOT);
cachedFiles.sort();
var fileListDirty = false;

function getFiles() {
  if (fileListDirty) {
    cachedFiles = findMarkdownFiles(ROOT);
    cachedFiles.sort();
    fileListDirty = false;
  }
  return cachedFiles;
}

var server = http.createServer(function(req, res) {
  var url = decodeURIComponent(req.url.split('?')[0]);

  // SSE endpoint for hot-reload
  if (url === '/_rig/events') {
    res.writeHead(200, { 'Content-Type': 'text/event-stream', 'Cache-Control': 'no-cache', 'Connection': 'keep-alive', 'Access-Control-Allow-Origin': '*' });
    res.write('data: {"status":"connected"}\n\n');
    clients.add(res);
    req.on('close', function() { clients.delete(res); });
    return;
  }

  var files = getFiles();

  // Index page: serve README.md if it exists, with file listing below
  if (url === '/' || url === '') {
    var indexHtml = '';
    // Try to serve README.md as the landing page
    var readmePath = path.join(ROOT, 'README.md');
    try {
      var readmeContent = fs.readFileSync(readmePath, 'utf8');
      indexHtml += marked.parse(readmeContent);
      indexHtml += '<hr>';
    } catch (e) {}
    // Always show file listing for navigation
    indexHtml += '<h2>All Markdown Files</h2>';
    if (files.length === 0) {
      indexHtml += '<p>No markdown files found in workspace.</p>';
    } else {
      indexHtml += '<ul class="file-list">';
      for (var i = 0; i < files.length; i++) {
        indexHtml += '<li><a href="/' + encodeURIComponent(files[i]) + '">' + files[i] + '</a></li>';
      }
      indexHtml += '</ul>';
    }
    res.writeHead(200, { 'Content-Type': 'text/html; charset=utf-8' });
    res.end(renderPage('Index', indexHtml, files));
    return;
  }

  // Serve markdown file
  var filePath = path.join(ROOT, url.replace(/^\//, ''));
  if (!filePath.endsWith('.md')) filePath += '.md';

  // Security: ensure we stay within ROOT
  if (!filePath.startsWith(ROOT)) {
    res.writeHead(403); res.end('Forbidden'); return;
  }

  fs.readFile(filePath, 'utf8', function(err, content) {
    if (err) {
      res.writeHead(404, { 'Content-Type': 'text/html; charset=utf-8' });
      res.end(renderPage('Not Found', '<h1>404</h1><p>File not found: ' + url + '</p><p><a href="/">Back to index</a></p>', files));
      return;
    }
    var html = marked.parse(content);
    // Rewrite relative .md links to work as server routes
    html = html.replace(/href="([^"]*?\.md)"/g, function(match, href) {
      if (href.startsWith('http://') || href.startsWith('https://')) return match;
      // Resolve relative to current file directory
      var dir = path.dirname(url.replace(/^\//, ''));
      var resolved = dir && dir !== '.' ? dir + '/' + href : href;
      return 'href="/' + resolved + '"';
    });
    var relPath = url.replace(/^\//, '');
    var parts = relPath.split('/');
    var breadcrumb = '<div class="breadcrumb">';
    for (var bi = 0; bi < parts.length - 1; bi++) {
      breadcrumb += parts[bi] + ' / ';
    }
    breadcrumb += parts[parts.length - 1] + '</div>';
    res.writeHead(200, { 'Content-Type': 'text/html; charset=utf-8' });
    res.end(renderPage(relPath, breadcrumb + html, files));
  });
});

watchDir(ROOT);
server.listen(PORT, '0.0.0.0', function() {
  console.log('Rig markdown server listening on http://0.0.0.0:' + PORT);
  console.log('Serving markdown from: ' + ROOT);
});
`

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
