# ADR-0001: Search and MCP Server for Rig Markdown Server

## Status
Proposed

## Context

The rig markdown server renders `.md` files from `/workspace` in the browser. Users want to search across their documentation — both by filename and content. AI assistants (Claude, Gemini, etc.) running inside the container would also benefit from being able to search and read workspace docs programmatically via MCP (Model Context Protocol).

### Requirements
- **Text search**: filename matching + full-text content search with snippets
- **Semantic/vector search**: natural language queries (e.g., "how does authentication work?")
- **MCP server**: expose search as tools for AI assistants over stdio
- **Code file search**: optionally index non-markdown files
- **Scale**: handle thousands of files without degrading startup or request latency

### Constraints
- The Node.js server runs inside a Docker container (debian:bookworm-slim, no GPU)
- The server script is embedded as a Go string constant
- No database or external services available by default
- Container already has Node.js (via mise) and `marked` (npm global)

## Decision

### Split server into multiple files, embedded via `go:embed`

The current server is ~400 lines in a single Go string constant. Adding search, vector indexing, and MCP would push this past 1500 lines. Instead, all JS/CSS lives as standalone files under `internal/dockerfile/scripts/`, embedded into the Go binary at compile time via `//go:embed`:

| Source File | Installed As | Purpose |
|---|---|---|
| `scripts/rig-md-server.js` | `/usr/local/bin/rig-md-server.js` | HTTP server, rendering, SSE, search routes |
| `scripts/rig-md-client.js` | Inlined in HTML `<script>` | Browser JS (theme, nav, mermaid, live-reload, search UI) |
| `scripts/rig-md-styles.css` | Inlined in HTML `<style>` | All CSS (themes, layout, nav, search) |
| `scripts/rig-search-index.js` | `/usr/local/bin/rig-search-index.js` | Text search index (filename + full-text) |
| `scripts/rig-vector-search.js` | `/usr/local/bin/rig-vector-search.js` | Semantic search with local embeddings |
| `scripts/rig-mcp-server.js` | `/usr/local/bin/rig-mcp-server.js` | MCP server (JSON-RPC over stdio) |

Files installed to `/usr/local/bin/` are added to the Docker build context via `BuildContext.ExtraFiles` and `COPY`'d into the image. CSS and client JS are injected into the HTML by the server at render time.

This eliminates the multi-layer escaping issues (Go raw string → JS template literal → CSS/regex) that have caused repeated bugs, and gives proper editor support (syntax highlighting, linting) for all embedded code.

### Text search: MiniSearch (proper full-text search)

Use [MiniSearch](https://github.com/lucaong/minisearch) — a lightweight (~10KB), zero-dependency, in-memory full-text search library for JavaScript. It provides Lucene-style capabilities:

- **TF-IDF scoring** with BM25-like ranking
- **Tokenization and stemming** (configurable)
- **Prefix search** and **fuzzy matching** (typo tolerance)
- **Field boosting** (weight filename matches higher than body matches)
- **Boolean queries** and phrase matching
- **Sub-millisecond search** on tens of thousands of documents

Each document is indexed with fields: `id` (path), `title` (filename), `body` (content). Filename matches are boosted above content matches.

- **Incremental updates**: on `fs.watch` events, remove and re-add only the changed document
- **Performance**: indexing 10,000 files takes <2s; searches return in <1ms
- **npm dependency**: `minisearch` (installed globally alongside `marked`)

Why MiniSearch over alternatives?
- **vs lunr.js**: MiniSearch supports incremental index updates (add/remove documents); lunr requires full rebuild. MiniSearch is also actively maintained and smaller.
- **vs flexsearch**: MiniSearch has better relevance ranking (TF-IDF) and more Lucene-like query features.
- **vs Elasticsearch/SQLite FTS**: No external services or native modules required. Pure JS, works everywhere.

### Vector search: local embeddings via @huggingface/transformers

Use `@huggingface/transformers` (formerly `@xenova/transformers`) with the `all-MiniLM-L6-v2` model (~23MB).

- Runs on CPU, ~5ms per embedding
- Chunk files into ~512 token paragraphs, embed each chunk
- Brute-force cosine similarity for search (for 10,000 chunks, <10ms)
- Model downloaded at image build time to avoid first-run latency
- **Config-gated and off by default** — adds ~100MB to image size

Why not an API? The container may not have API keys. A local model avoids latency, cost, and connectivity requirements for a local dev tool.

### Combined ranking: text + vector results

When semantic search is enabled, both MiniSearch and vector search run in parallel for every query. Results are merged using Reciprocal Rank Fusion (RRF):

```
RRF_score(doc) = 1/(k + rank_text) + 1/(k + rank_semantic)
```

Where `k = 60` (standard constant). This produces a single ranked result list that benefits from both exact keyword matches and semantic similarity, without needing to normalize scores across different ranking systems.

When semantic search is disabled, only MiniSearch results are returned — still a proper full-text search experience.

### MCP server: separate process, stdio transport

MCP uses stdio (stdin/stdout JSON-RPC), which is incompatible with an HTTP server sharing stdout for logs. The MCP server runs as a separate process that calls the main server's HTTP API internally.

**Tools exposed:**

| Tool | Description |
|---|---|
| `search` | Combined text + semantic search (uses RRF when semantic is enabled) |
| `read_doc` | Read a specific file's content |
| `list_docs` | List all indexed files |

AI assistants configure it as:
```json
{
  "mcpServers": {
    "rig-docs": {
      "command": "node",
      "args": ["/usr/local/bin/rig-mcp-server.js"]
    }
  }
}
```

The MCP server implements JSON-RPC directly (~200 lines) to avoid adding `@modelcontextprotocol/sdk` as a dependency.

### Why MCP search matters (and why grep isn't enough)

AI agents like Claude Code have excellent built-in tools (grep, glob, read) and are deeply trained to use them. An MCP search tool will **not** automatically replace grep — Claude will default to what it knows. The MCP search adds value in two specific ways:

**1. Semantic search can't be done with grep.** A query like "how does the build pipeline handle caching?" requires understanding concepts, not matching strings. This is the primary reason to expose search via MCP — it gives AI agents a capability they simply don't have with their built-in tools.

**2. Token efficiency for broad queries.** When an AI agent greps for a common term across a large codebase, it gets back raw file contents from every match — potentially thousands of lines dumped into context. The MCP search returns ranked results with snippets (top 10, ~50 tokens each vs. potentially thousands of raw lines). For discovery-oriented queries, this is dramatically more token-efficient.

**Making agents actually use it:** `.mcp.json` makes the tool *available*, but agents need guidance to *prefer* it. `rig init` should generate a `CLAUDE.md` (and equivalent for other agents) with instructions like:

```markdown
## Search

This project has a rig-docs MCP search server available. Use the `search` tool
for finding relevant documentation before grepping — it supports semantic search
and returns ranked results with snippets, using fewer tokens than grep for
broad queries. Use grep for exact string/symbol lookups.
```

This ensures agents reach for MCP search when it's the right tool (discovery, broad queries, "how does X work?") while still using grep for precise lookups ("find all calls to `processPayment`").

### Search API

All endpoints under `/_rig/` (consistent with existing `/_rig/events`):

| Endpoint | Method | Description |
|---|---|---|
| `/_rig/search?q=<query>` | GET | Combined search (text + semantic if enabled, merged via RRF) |
| `/_rig/search/status` | GET | Index stats |
| `/_rig/search/reindex` | POST | Force full reindex |

Response format:
```json
{
  "query": "authentication",
  "results": [
    {"file": "docs/auth.md", "snippet": "...configure <mark>authentication</mark>...", "score": 0.92, "sources": ["text", "semantic"]},
    {"file": "docs/login.md", "snippet": "The login flow uses OAuth2...", "score": 0.78, "sources": ["semantic"]},
    {"file": "docs/setup.md", "snippet": "...set up <mark>authentication</mark> middleware...", "score": 0.65, "sources": ["text"]}
  ]
}
```

### Search UI

Enhance the existing page with a search bar at the top of the content area (above the left nav sidebar's file filter):
- Debounced requests to `/_rig/search` as the user types
- Dropdown overlay showing combined results with highlighted snippets
- Results that matched via both text and semantic search are visually indicated
- Clicking a result navigates to the file
- Vanilla JS, no framework — consistent with existing client code

### Non-markdown file support

Controlled by config. When enabled, the scanner indexes common text file extensions (`.js`, `.ts`, `.py`, `.go`, `.rs`, `.java`, `.rb`, `.yaml`, `.json`, `.toml`, `.sh`, etc.). Filename search always covers all files; content search is toggled separately.

### Configuration

Extend `MarkdownServerConfig`:

```go
type SearchConfig struct {
    Semantic    bool     `yaml:"semantic"`      // default: false
    IncludeCode bool     `yaml:"include_code"`  // default: false
    Extensions  []string `yaml:"extensions"`    // additional extensions to index
}
```

Example `.rig.yml`:
```yaml
markdown_server:
  enabled: true
  port: 3030
  search:
    semantic: true
    include_code: true
```

### Dockerfile template changes

The markdown server section conditionally installs extra packages and copies extra files:

```dockerfile
# Always: text search (minisearch) + MCP
RUN npm install -g minisearch
COPY rig-search-index.js /usr/local/bin/rig-search-index.js
COPY rig-mcp-server.js /usr/local/bin/rig-mcp-server.js

# If semantic search enabled:
RUN npm install -g @huggingface/transformers
RUN node -e "..." # pre-download model
COPY rig-vector-search.js /usr/local/bin/rig-vector-search.js
```

### MCP auto-configuration and agent guidance

When `rig init` creates a new project, it generates two things:

**1. `.mcp.json`** — MCP tool discovery for AI assistants:
```json
{
  "mcpServers": {
    "rig-docs": {
      "command": "node",
      "args": ["/usr/local/bin/rig-mcp-server.js"]
    }
  }
}
```

**2. `CLAUDE.md`** (and equivalents for other agents) — behavioural guidance so agents actually use the search tool when appropriate:
```markdown
## Search

This project has a rig-docs MCP search server available. Use the `search` tool
for finding relevant documentation before grepping — it supports semantic search
and returns ranked results with snippets, using fewer tokens than grep for
broad queries. Use grep for exact string/symbol lookups.
```

Both files are generated into the project directory. `.mcp.json` makes the tool available; `CLAUDE.md` makes the agent prefer it for the right queries. Without the guidance file, agents will ignore the MCP tool in favour of their built-in grep/read tools.

If these files already exist, `rig init` should merge the MCP server entry into the existing `.mcp.json` and append the search section to `CLAUDE.md` rather than overwriting.

## Implementation Phases

### Phase 0: Externalise embedded JS from Go strings

The current approach of embedding JS/CSS as Go string constants (backtick-delimited raw strings) has caused repeated issues:
- Multi-layer escaping bugs (Go raw string → JS template literal → CSS/regex)
- No syntax highlighting, linting, or editor support for the embedded code
- Difficult to read and maintain at scale

**Solution: Use Go's built-in `embed` package** (available since Go 1.16, no external dependency).

Move all JS/CSS into standalone files under `internal/dockerfile/scripts/`:

```
internal/dockerfile/scripts/
├── rig-md-server.js       # Main HTTP server, rendering, SSE
├── rig-md-client.js       # Browser-side JS (theme toggle, nav, mermaid, live-reload)
├── rig-md-styles.css      # All CSS (light/dark themes, layout, nav)
```

Embed them in Go:

```go
package dockerfile

import "embed"

//go:embed scripts/rig-md-server.js
var MarkdownServerScript string

//go:embed scripts/rig-md-client.js
var MarkdownClientJS string

//go:embed scripts/rig-md-styles.css
var MarkdownCSS string
```

The server script then references the CSS and client JS as separate embedded strings rather than concatenating them inline. The `renderPage` function injects them via `<style>` and `<script>` tags as before, but the source of truth is now proper `.js` and `.css` files.

**Benefits:**
- Proper syntax highlighting and linting in editors
- No escaping issues — the files are exactly what gets embedded
- Testable independently (can run the JS through Node, validate CSS)
- Natural place for search scripts to live in later phases

**Migration steps:**
1. Create `internal/dockerfile/scripts/` directory
2. Extract `MarkdownServerScript` into `scripts/rig-md-server.js`
3. Extract `CSS` and `CLIENT_JS` into `scripts/rig-md-styles.css` and `scripts/rig-md-client.js`
4. Update `template.go` to use `//go:embed` and reference the embedded vars
5. Update `generator.go` to pass CSS/JS strings to the server script (via template substitution or as separate ExtraFiles)
6. Verify all tests pass — behaviour should be identical

### Phase 1: Text search
1. Add `scripts/rig-search-index.js` (using MiniSearch)
2. Add `minisearch` to npm install in Dockerfile template
3. Add search API routes to main server
4. Add search UI (CSS + JS)
5. Wire into `generator.go` ExtraFiles and Dockerfile template
6. Config + tests

### Phase 2: MCP server + auto-config + agent guidance
1. Add `scripts/rig-mcp-server.js`
2. Include in ExtraFiles and Dockerfile
3. Update `rig init` to generate `.mcp.json` with rig-docs server entry
4. Update `rig init` to generate `CLAUDE.md` with search guidance (append if exists)
5. Consider equivalent guidance files for other agents (Gemini, Codex) as their conventions stabilize

### Phase 3: Vector search (optional, config-gated)
1. Add `scripts/rig-vector-search.js`
2. Conditional npm install + model download in Dockerfile template
3. Add `SearchConfig.Semantic` to config
4. Wire into combined search API (RRF merging) and MCP server

### Phase 4: Non-markdown file support
1. Extend scanner for configurable extensions
2. Add `SearchConfig.IncludeCode` to config

## Trade-offs

| Decision | Pro | Con | Mitigation |
|---|---|---|---|
| MiniSearch (not SQLite FTS / Elasticsearch) | Proper TF-IDF ranking, fuzzy matching, prefix search, pure JS, ~10KB | RAM scales with corpus | <50MB for thousands of files; can switch to `better-sqlite3` + FTS5 later if needed |
| Local embedding model (not API) | Works offline, no API keys, no cost | ~100MB image size increase | Off by default, config-gated |
| Separate MCP process | Clean stdio transport, no stdout conflicts | Extra process | Lightweight; communicates via localhost HTTP |
| `go:embed` with standalone files | Proper editor support, no escaping bugs, lintable | Slightly more files in the repo | Files are self-contained and independently testable |
| Raw JSON-RPC (not MCP SDK) | No dependency | Must maintain protocol compliance | MCP protocol is simple; ~200 lines |
| Vanilla JS (not TypeScript) | No build step | No type safety | Scripts are small and self-contained |

## Consequences

- Text search (Phase 1) adds `minisearch` (~10KB) — proper full-text search with TF-IDF ranking, fuzzy matching, and prefix search
- MCP server (Phase 2) makes workspace docs searchable by AI assistants out of the box; `.mcp.json` is auto-generated by `rig init`
- `CLAUDE.md` guidance (Phase 2) is critical — without it, AI agents will ignore the MCP tool and default to grep. The guidance steers agents toward MCP search for discovery/broad queries while preserving grep for exact lookups
- When semantic search is enabled, text and vector results are merged via RRF into a single ranked list — one search, best of both worlds
- Vector search (Phase 3) is opt-in; users who enable it accept the ~100MB image size trade-off
- The multi-file split keeps each script manageable and independently testable
