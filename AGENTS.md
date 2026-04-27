# KB Agent Configuration

## Project Overview

Knowledgebase CLI with SQLite+FTS5 and weighted retrieval for AI agents.

## Build

```bash
# Requires CGO for SQLite FTS5
CGO_CFLAGS="-DSQLITE_ENABLE_FTS5" go build -tags sqlite_fts5 -o bin/kb .
```

## Test

```bash
go test ./...
```

## Commands

| Command | Description |
|---------|-------------|
| `init` | Initialize database |
| `add` | Create entry with article |
| `append` | Add article to entry |
| `list` | List entries |
| `get` | Get entry details |
| `search` | Full-text search |
| `delete` | Remove entry/article |
| `config` | Show config |
| `check` | Validate installation |

## Config

`~/.config/kb/config.yaml`:
```yaml
db_path: ~/.local/share/kb/knowledgebase.db
embedder: none  # or "ollama"
ollama:
  model: nomic-embed-text
  base_url: http://localhost:11434
top_k: 5
```

## Tech Stack

- Go 1.22+
- SQLite + FTS5
- urfave/cli/v3
- Ollama (optional)
