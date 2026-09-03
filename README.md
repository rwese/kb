# kb — Knowledgebase CLI

A lightweight knowledgebase for the terminal. Keep notes on topics, grow them over time, attach files, and search everything in one place.

## Features

- **Entries & articles** — a topic with a growing history of notes
- **File attachments** — keep screenshots, traces, or documents with the notes they belong to
- **Full-text search** — fast keyword search over everything you stored
- **Semantic search** *(optional)* — find notes by meaning, not just keywords, via Ollama or a bundled local model
- **Obsidian export** — export everything as markdown at any time

## Install

### Prebuilt (recommended)

Download the latest binary for your platform from [GitHub Releases](https://github.com/rwese/kb/releases). The binary is self-contained — SQLite with FTS5 is built in, so no Go or extra system libraries are needed.

### Build from source

**Requirements**: Go 1.22+ and SQLite with the FTS5 extension (pre-installed on macOS; on Linux install `libsqlite3-dev`).

```bash
git clone https://github.com/rwese/kb
cd kb
./build.sh install
```

Go users can also install directly:

```bash
CGO_CFLAGS="-DSQLITE_ENABLE_FTS5" go install -tags sqlite_fts5 github.com/rwese/kb@latest
```

## Quick Start

```bash
# Create a knowledgebase
kb init

# Start a topic
kb entry create -t "List flickering bug" -c "Items flicker when scrolling" --tags "ui,bug"

# Append follow-up notes
kb entry article add 2f018d "Fix: throttle with requestAnimationFrame"

# Attach evidence
kb entry article asset add 2f018d 2f018d-273b00 ./trace.har ./screenshots

# Search everything
kb search "flickering"
```

## Where data lives

Everything is stored in one SQLite database, defaulting to `~/.local/share/kb/knowledgebase.db`; attached files go to `~/.local/share/kb/assets`.

To change the defaults, create `~/.config/kb/config.yaml` (if no user config exists, kb also looks for a `.kb.yaml` in the current directory):

```yaml
db_path: ~/.local/share/kb/knowledgebase.db
assets_path: ~/.local/share/kb/assets
top_k: 5   # default number of search results
```

## Search

`kb search <query>` finds articles by keyword, ranked by relevance. To get results more relevant to what you are currently working on, pass your task or prompt text:

```bash
kb search --prompt "I am fixing the flickering list"
```

### Semantic search (optional)

Keyword search matches what you write, not what you mean. For meaning-based search, set an embedder in the config:

- **Ollama**: `embedder: ollama` with a model served locally (e.g. `nomic-embed-text`)
- **Bundled local model**: run `kb download` once, then set `embedder: local` — no server needed

With an embedder configured, search blends keyword and semantic ranking.

## Export

```bash
kb export -o out -e 2f018d    # one entry
kb export --all               # everything
```

Produces Obsidian-compatible markdown with assets copied alongside:

```
out/http-cache-bug/http-cache-bug.md
out/http-cache-bug/assets/2f018d-273b00/trace.har
```

## Command reference

| Command | What it does |
|---------|--------------|
| `kb init` | Create the database |
| `kb entry create -t <title>` | New entry (optionally with a first article via `-c` or `-f`) |
| `kb entry list` | List entries |
| `kb entry get <id> --articles` | Show an entry with its articles and assets |
| `kb entry article add <entry> "note"` | Append a note to an entry |
| `kb entry article asset add <entry> <article> <path>...` | Attach files or directories |
| `kb search <query>` | Search your notes |
| `kb status` | Validate installation and database |
| `kb stats` | Database statistics |
| `kb export` | Export to markdown |

Use `kb <command> --help` for full options on any command.

## How entries work

```
Entry (a topic)                kb entry create
└── Article 1                  initial notes
└── Article 2                  kb entry article add
└── Article 3
        └── Assets             kb entry article asset add
```

Entries get short ids (`2f018d`), articles get `entry-article` ids (`2f018d-273b00`). Use these ids with any command, or look them up with `kb entry list`.

## Deleting

```bash
kb entry delete 2f018d a1b2c3    # removes entries together with their articles
```

## Building from source

See [DEVELOPMENT.md](DEVELOPMENT.md) for build, test, and contribution details.