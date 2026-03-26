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

### Split server into multiple files

The current server is ~400 lines in a single Go string constant. Adding search, vector indexing, and MCP would push this past 1500 lines. Instead, split into multiple JS files, each as its own Go constant in a dedicated file:

| Go File | Go Constant | Installed As | Purpose |
|---|---|---|---|
| `template.go` (existing) | `MarkdownServerScript` | `rig-md-server.js` | HTTP server, rendering, SSE, search routes |
| `search_scripts.go` (new) | `SearchIndexScript` | `rig-search-index.js` | Text search index (filename + full-text) |
| `search_scripts.go` | `VectorSearchScript` | `rig-vector-search.js` | Semantic search with local embeddings |
| `search_scripts.go` | `McpServerScript` | `rig-mcp-server.js` | MCP server (JSON-RPC over stdio) |

All files are added to the Docker build context via the existing `BuildContext.ExtraFiles` map and `COPY`'d into the image.

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

### MCP auto-configuration

When `rig init` creates a new project, it should also generate MCP configuration files so AI assistants discover the search server automatically:

- **Claude**: write `.mcp.json` (project-level MCP config) with the rig-docs server entry
- **Other agents**: follow their respective MCP config conventions as they emerge

This means the MCP server is usable out of the box — no manual config step for the user. The generated config points to `node /usr/local/bin/rig-mcp-server.js` which is available inside the container where the AI agents run.

## Implementation Phases

### Phase 1: Text search
1. Create `search_scripts.go` with `SearchIndexScript` (using MiniSearch)
2. Add `minisearch` to npm install in Dockerfile template
3. Add search API routes to main server
4. Add search UI (CSS + JS)
5. Wire into `generator.go` ExtraFiles and Dockerfile template
6. Config + tests

### Phase 2: MCP server + auto-config
1. Add `McpServerScript` to `search_scripts.go`
2. Include in ExtraFiles and Dockerfile
3. Update `rig init` to generate `.mcp.json` with rig-docs server entry

### Phase 3: Vector search (optional, config-gated)
1. Add `VectorSearchScript` to `search_scripts.go`
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
| Multiple Go string constants | Each file self-contained, testable | More constants to manage | Separate `.go` file; clear naming |
| Raw JSON-RPC (not MCP SDK) | No dependency | Must maintain protocol compliance | MCP protocol is simple; ~200 lines |
| Vanilla JS (not TypeScript) | No build step | No type safety | Scripts are small and self-contained |

## Consequences

- Text search (Phase 1) adds `minisearch` (~10KB) — proper full-text search with TF-IDF ranking, fuzzy matching, and prefix search
- MCP server (Phase 2) makes workspace docs searchable by AI assistants out of the box; `.mcp.json` is auto-generated by `rig init`
- When semantic search is enabled, text and vector results are merged via RRF into a single ranked list — one search, best of both worlds
- Vector search (Phase 3) is opt-in; users who enable it accept the ~100MB image size trade-off
- The multi-file split keeps each script manageable and independently testable
