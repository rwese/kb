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
kb entry create -t "List flickering bug" -c "Initial report: items flicker when scrolling" --tags "ui,bug"

# Append more info
kb entry article add 2f018d "Fix: add requestAnimationFrame throttling"

# Attach a file or directory to an article
kb entry article asset add 2f018d 2f018d-273b00 ./trace.har ./screenshots

# Search
kb search "flickering"
```

## Config

Create `~/.config/kb/config.yaml`:

```yaml
db_path: ~/.local/share/kb/knowledgebase.db
assets_path: ~/.local/share/kb/assets
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
| `kb entry create` | Create entry with optional initial article |
| `kb entry article add` | Add article to existing entry |
| `kb entry article asset add` | Attach files or directories to an article |
| `kb entry list` | List all entries |
| `kb entry get --articles` | Get an entry with articles and assets |
| `kb delete entry` | Delete one or more entries |
| `kb search` | Search articles |
| `kb export` | Export markdown plus KB-owned asset copies |
| `kb config` | Show config |

## Data Model

```
Entry (topic)          → kb entry create
└── Article 1          → initial content
└── Article 2          → kb entry article add
└── Article 3          → kb entry article add
    └── Asset files    → kb entry article asset add
```

## Examples

### Append by Entry ID
```bash
kb entry article add 2f018d "More notes..."
```

### Create and Inspect
```bash
kb entry create -t "Bug: List Flickering" -c "Found workaround..."
kb entry get 2f018d --articles
```

### Attach Assets
```bash
kb entry article asset add 2f018d 2f018d-273b00 ./report.pdf
kb entry article asset add 2f018d 2f018d-273b00 ./docs --overwrite
kb entry article asset list 2f018d 2f018d-273b00
```

### Export
```bash
kb export -o out -e 2f018d

# Produces:
# out/http-cache-bug/http-cache-bug.md
# out/http-cache-bug/assets/2f018d-273b00/trace.har
```

### Delete Entries
```bash
kb delete entry 2f018d a1b2c3 --force
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

# Test
CGO_CFLAGS="-DSQLITE_ENABLE_FTS5" go test ./...
```
