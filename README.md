# kb - Knowledgebase CLI

Lightweight knowledgebase with SQLite+FTS5 and weighted retrieval.

## Install

```bash
# Clone and install
git clone https://github.com/rwese/kb ~/Repos/github.com/rwese/kb
cd ~/Repos/github.com/rwese/kb
./build.sh install

# Or from source
go install -tags sqlite_fts5 .
```

## Config

Create `~/.config/kb/config.yaml`:

```yaml
db_path: ~/.local/share/kb/knowledgebase.db
embedder: ollama  # or "openai", "none" (FTS only)
ollama:
  model: nomic-embed-text
  base_url: http://localhost:11434
top_k: 5
```

## Commands

### init
Initialize the database:
```bash
kb init
```

### add
Add entries to knowledgebase:
```bash
# Inline content
kb add --title "List flickering bug" --content "When scrolling fast..." --tags "ui,bug"

# From file
kb add --file /path/to/notes.md

# From stdin
cat session.log | kb add --title "Debug session"
```

### search
Query with weighted retrieval (prompt weighted higher than context):
```bash
kb search "list flickering"
kb search --context-file debug.log --prompt "what causes flickering"
kb search -C "session output" -p "fix the bug"
```

### config
Show current configuration:
```bash
kb config
```

## Weighted Retrieval

Query/prompt has 3x weight over context:
```
score = bm25(query) + 3×sim(prompt, chunk) - 0.5×sim(context, chunk)
```

This ensures the specific question dominates while context helps filter relevant results.

## Build

Requires SQLite with FTS5 extension:

```bash
# macOS
brew install sqlite3

# Build
./build.sh

# Or manually
CGO_CFLAGS="-DSQLITE_ENABLE_FTS5" go build -tags sqlite_fts5 .
```
