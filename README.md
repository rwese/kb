# kb - Knowledgebase CLI

Lightweight knowledgebase with SQLite+FTS5 and weighted retrieval.

**Features:**
- Multi-article entries (append information to topics)
- Full-text search with BM25 ranking
- Weighted retrieval (prompt weighted 3x over context)
- Optional semantic search via Ollama embeddings

## Install

```bash
go install github.com/rwese/kb@latest
```

Or build from source:

```bash
git clone https://github.com/rwese/kb
cd kb
./build.sh install
```

**Requirements:**
- Go 1.22+
- SQLite with FTS5 extension (pre-installed on macOS, install `libsqlite3-dev` on Linux)

## Quick Start

```bash
# Initialize database
kb init

# Create entry with article
kb add -t "List flickering bug" -c "Initial report: items flicker when scrolling" --tags "ui,bug"

# Append more info
kb append --entry 1 -c "Fix: add requestAnimationFrame throttling"

# Search
kb search "flickering"
```

## Config

Create `~/.config/kb/config.yaml`:

```yaml
db_path: ~/.local/share/kb/knowledgebase.db
embedder: none  # or "ollama" for semantic search
ollama:
  model: nomic-embed-text
  base_url: http://localhost:11434
top_k: 5
```

## Commands

| Command | Description |
|---------|-------------|
| `kb init` | Initialize database |
| `kb add` | Create entry with initial article |
| `kb append` | Add article to existing entry |
| `kb list` | List all entries |
| `kb get` | Get entry with articles |
| `kb search` | Search articles |
| `kb delete` | Delete entry or article |
| `kb config` | Show config |

## Data Model

```
Entry (topic)          → kb add
└── Article 1          → initial content
└── Article 2          → kb append
└── Article 3          → kb append
```

## Examples

### Append by Entry ID
```bash
kb append --entry 1 -c "More notes..."
```

### Append by Entry Title (creates if not exists)
```bash
kb append --entry-title "Bug: List Flickering" -c "Found workaround..."
```

### From File or Stdin
```bash
cat session.log | kb append --entry 1
kb add -f notes.md --tags "docs"
```

### Search with Context
```bash
kb search --context-file debug.log -p "what causes the flickering"
```

## Weighted Retrieval

Final prompt weighted 3x over context:
```
score = bm25(query) + 3×sim(prompt, chunk) - 0.5×sim(context, chunk)
```

## Build

```bash
# macOS
brew install sqlite3

# Build
./build.sh

# Or manually
CGO_CFLAGS="-DSQLITE_ENABLE_FTS5" go build -tags sqlite_fts5 -o kb .
```
