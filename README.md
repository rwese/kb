# kb - Knowledgebase CLI

A lightweight knowledgebase for the terminal. Keep notes on topics, grow them over time, attach files, and search everything in one place.

## Features

- **Entries & articles** - a topic with a growing history of notes
- **File attachments** - attach one titled file (script, archive, document) directly to an entry; keep screenshots, traces, or documents with the notes they belong to
- **Full-text search** - fast keyword search over articles and attachment titles/filenames
- **Semantic search** *(optional)* - find notes by meaning, not just keywords, via Ollama
- **Obsidian export** - export everything as markdown at any time

## Install

### Agentic install

Pass this prompt to your agent:

```markdown
Install this knowledgebase (kb) from github, use the latest release. Verify there is no `kb` installed and update it.

Repository: https://github.com/rwese/kb/

Install to the user local bin directory, commonly `~/.local/bin/, ask for user confirmation.

## Knowledgebase Agent Skill

Offer setup of the Knowledgebase skill for the user, found here:

https://github.com/rwese/kb/blob/main/skills/knowledgebase

User may choose to install it globally or in the currents project scope.
```

### Prebuilt

Download the latest binary for your platform from [GitHub Releases](https://github.com/rwese/kb/releases).

Linux/arm64 binaries are additionally published as Forgejo releases: pick the asset matching your platform / architecture (`kb-<os>-<arch>`, plus option-suffixed builds like `-onnx` once opt-in embedder variants ship; verify with the bundled `SHA256SUMS`).

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

# Attach a file directly to an entry
kb entry attachment add --title "Linux helper executable" 2f018d ./bin/helper

# Search everything
kb search "flickering"
```

## Where data lives

Everything is stored in one SQLite database, defaulting to `~/.local/share/kb/knowledgebase.db`; attached files go to `~/.local/share/kb/assets`.

To change the defaults, create `~/.config/kb/config.yaml` (kb also reads `$XDG_CONFIG_HOME/kb/config.yaml` when set, then `$XDG_CONFIG_DIRS/kb/config.yaml` for system-wide defaults, and finally a `.kb.yaml` in the current directory; `$KB_PATH` wins over all of them):

```yaml
db_path: ~/.local/share/kb/knowledgebase.db
assets_path: ~/.local/share/kb/assets
top_k: 5   # default number of search results
ranker:
  algorithm: auto   # auto | bm25 | hybrid
  bm25_weight: 0.3
  semantic_weight: 0.7
```

## Search

`kb search <query>` finds articles by keyword, ranked by relevance. Results are grouped by entry and stay compact - the entry's id, title and tags, plus the matching articles. Attachment-only entries are found by attachment **title** or **file name** (binary contents are never indexed). No content is included by default:

```
ID: 2f018d, Title: Bug: List Flickering, Tags: ui,bug

Entry-Article(s):

Article-ID: 2f018d-273b00, Title: Fix: throttle with requestAnimationFrame
```

Pass `--content` to include up to 10 lines of the best-matching article per entry (with a hint to `kb entry get <id>` for the rest). Pass `--full-content` to restore the previous verbose format (per-result scores and uncut content). To get results more relevant to what you are currently working on, pass your task or prompt text:

```bash
kb search --prompt "I am fixing the flickering list"
```

### Semantic search (optional)

Keyword search matches what you write, not what you mean. For meaning-based search, set an embedder in the config:

- **Ollama**: `embedder: ollama` with a model served locally (e.g. `nomic-embed-text`)

With an embedder configured, search blends keyword and semantic ranking (hybrid). The blend weights default to 0.3 keyword / 0.7 semantic; tune them with `ranker.bm25_weight` / `ranker.semantic_weight` in the config, the `KB_BM25_WEIGHT` / `KB_SEMANTIC_WEIGHT` env variables, or the `--algorithm`, `--bm25-weight`, and `--semantic-weight` flags. Use `kb search --bm25-only` (or `--algorithm bm25`) to compare against pure keyword ranking.

## Export

```bash
kb export -o out -e 2f018d    # one entry
kb export --all               # everything
```

Produces Obsidian-compatible markdown with assets copied alongside:

```
out/http-cache-bug/http-cache-bug.md
out/http-cache-bug/assets/2f018d-273b00/trace.har
out/http-cache-bug/attachments/a1b2c3/helper      # only with --with-attachments
out/INDEX.md
```

Entry attachments (titled files attached to an entry, not article assets) are
exported only with `--with-attachments`; without it neither the files nor the
`## Attachments` section are written. With the flag they land in an
`attachments/` subdirectory of the entry export and are linked relative:

```bash
kb export --all --with-attachments -o out
```

```markdown
## Attachments

- [Linux helper executable](attachments/a1b2c3/helper) (`helper`, 2.1 KB)
```

Every export also writes an Obsidian `INDEX.md` at the output root for browsing:
one section per entry, and per file a wikilink, the heading, a short
description of the article's first paragraph, and the entry tags:

```markdown
## HTTP Cache Bug

- [[http-cache-bug|HTTP Cache Bug]] - Steps to reproduce... #bug #cache
- [[fix-details|Fix Details]] - Throttle with requestAnimationFrame. #bug #cache
```

## Command reference

| Command | What it does |
|---------|--------------|
| `kb init` | Create the database |
| `kb entry create -t <title>` | New entry (optionally with a first article via `-c` or `-f`) |
| `kb entry list` | List entries |
| `kb entry get <id> --articles` | Show an entry with its articles and assets |
| `kb entry get <id> --attachments` | Show an entry with its attachments |
| `kb entry article add <entry> "note"` | Append a note to an entry |
| `kb entry article asset add <entry> <article> <path>...` | Attach files or directories to an article |
| `kb entry attachment add -t <title> <entry> <file>` | Attach one titled file to an entry |
| `kb entry attachment list|get|update|delete <entry> ...` | List, inspect, rename/replace, or delete attachments |
| `kb search <query>` | Search your notes |
| `kb status` | Validate installation and database |
| `kb stats` | Database statistics |
| `kb export` | Export to markdown |
| `kb --version` | Print the installed version |

Use `kb <command> --help` for full options on any command.

## How entries work

```
Entry (a topic)                kb entry create
├── Article 1                  initial notes
├── Article 2                  kb entry article add
│       └── Assets             kb entry article asset add
└── Attachment                 kb entry attachment add (one file, titled)
```

Entries get short ids (`2f018d`), articles get `entry-article` ids (`2f018d-273b00`), and attachments have six-character ids scoped to their entry - every attachment command takes the entry id first. Use these ids with any command, or look them up with `kb entry list`.

## Deleting

```bash
kb entry delete 2f018d a1b2c3    # removes entries together with their articles
```

`kb entry delete` asks for confirmation unless `--force` (`-f`) is given. With `--force`, IDs that do not exist are skipped and the remaining entries are still deleted.

## Building from source

See [DEVELOPMENT.md](DEVELOPMENT.md) for build, test, and contribution details.
