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

## IDs

String-based IDs (6 hex chars, collision-resistant):
- Entries: `2f018d`
- Articles: `2f018d-273b00` (entry hash + article hash)

Auto-detected: IDs with `-` are articles, without are entries.

## Commands

| Command | Usage | Description |
|---------|-------|-------------|
| `init` | | Initialize database |
| `add` | `-t <title> -c <content>` | Create entry with article |
| `append` | `<entry> [content]` | Add article to entry |
| `list` | | List entries |
| `get` | `<id>` | Get entry or article |
| `search` | `<query>` | Full-text search |
| `delete` | `<id>` | Delete entry or article |
| `stats` | | Show database statistics |
| `config` | | Show config |
| `check` | | Validate installation |

**Flags:**
- `--all` — Include deleted entries (get, list, search)
- `--articles` — Include articles when getting entry (get)
- `--json` — JSON output (get, list)

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
