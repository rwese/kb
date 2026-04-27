---
name: knowledgebase
description: "Manage and query a personal knowledgebase with weighted retrieval. Use when: (1) adding notes, documentation, or session outputs to memory, (2) searching for solutions to problems, (3) asking questions against stored knowledge with final prompt weighted higher than context. Triggers: 'add to knowledgebase', 'remember', 'lookup', 'search knowledge', 'what do we know about', 'how do I', 'fix for', 'problem with'."
---

# Knowledgebase

Store and retrieve notes, documentation, and session outputs using the [`kb` CLI](https://github.com/rwese/kb).

## Prerequisites

- [`kb`](https://github.com/rwese/kb) installed (see [installation](https://github.com/rwese/kb#installation))
- Run `kb setup` to initialize the database

## Commands

```bash
kb list                          # List all entries
kb add -t "Title" -c "Content"   # Create entry
kb append --entry N -c "More"   # Append to entry
kb append --entry-title "Title" -c "Creates if not exists"
kb get --entry N                # Get entry with articles
kb search "query"               # Search articles (BM25 ranked)
```

## Usage

```bash
# Add session output
cat session.log | kb add -t "Debug session $(date +%Y-%m-%d)"

# Append to existing entry
kb append --entry 1 -c "Follow-up notes..."

# Search for solutions
kb search "flickering in scroll handler"
```

## Troubleshooting

**"Error: no such module: fts5"** — Run `just kb-rebuild` to rebuild `kb` with FTS5 support.
