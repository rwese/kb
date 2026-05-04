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
CGO_CFLAGS="-DSQLITE_ENABLE_FTS5" go test ./...
```

## IDs

String-based IDs (6 hex chars, collision-resistant):
- Entries: `2f018d`
- Articles: `2f018d-273b00` (entry hash + article hash)

Auto-detected: IDs with `-` are articles, without are entries.

## Commands

### Entry Management

| Command | Usage | Description |
|---------|-------|-------------|
| `entry create` | `-t <title> [-c <content>] [-f <file>] [--tags <tags>]` | Create entry (with optional initial article) |
| `entry list` | `[--json] [--articles] [--all]` | List all entries |
| `entry get` | `<id>` | Get entry by ID |
| `entry update` | `<id>` | Update an entry |
| `entry delete` | `<id> [id...] [--force]` | Delete one or more entries and all their articles |

### Article Management

| Command | Usage | Description |
|---------|-------|-------------|
| `entry article list` | `<entry-id>` | List articles in an entry |
| `entry article add` | `<entry-id> [content]` | Add article to entry |
| `entry article get` | `<entry-id> <article-id>` | Get article by ID |
| `entry article update` | `<article-id>` | Update an article |
| `entry article delete` | `<entry-id> <article-id>` | Delete article from entry |

### Article Asset Management

| Command | Usage | Description |
|---------|-------|-------------|
| `entry article asset add` | `<entry-id> <article-id> <path>... [--overwrite] [--json]` | Import files or directories as KB-owned article assets |
| `entry article asset list` | `<entry-id> <article-id> [--json]` | List assets attached to an article |
| `entry article asset get` | `<entry-id> <article-id> <asset-id> [--json]` | Show metadata for one asset |
| `entry article asset delete` | `<entry-id> <article-id> <asset-id> [--json]` | Delete one attached asset |

### Search & Utility

| Command | Usage | Description |
|---------|-------|-------------|
| `search` | `[--context <text>] [--context-file <file>] [--prompt <text>] [--top-k <n>] [--format <fmt>] [--all] [--bm25-only]` | Weighted full-text search |
| `status` | | Validate installation and database |
| `init` | | Initialize database |
| `config` | | Show current config |
| `stats` | | Show database statistics |
| `export` | `[-o <dir>] [-e <id>] [--all] [--force] [--dry-run]` | Export to Obsidian markdown |
| `download` | `[-f] [-v]` | Download embedding assets |

## Config

`~/.config/kb/config.yaml`:
```yaml
db_path: ~/.local/share/kb/knowledgebase.db
assets_path: ~/.local/share/kb/assets
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
