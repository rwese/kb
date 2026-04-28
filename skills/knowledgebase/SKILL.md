---
name: knowledgebase
description: "Manage and query a personal knowledgebase with weighted retrieval. Use when: (1) adding notes, documentation, or session outputs to memory, (2) searching for solutions to problems, (3) asking questions against stored knowledge with final prompt weighted higher than context. Triggers: 'add to knowledgebase', 'remember', 'lookup', 'search knowledge', 'what do we know about', 'how do I', 'fix for', 'problem with'."
---

> ⚠️ **Important**: Split large topics into multiple articles. Each article is individually searchable. A single monolithic entry hurts retrieval quality.

# Knowledgebase

Store and retrieve notes, documentation, and session outputs using the [`kb` CLI](https://github.com/rwese/kb).

## Prerequisites

- [`kb`](https://github.com/rwese/kb) installed (see [installation](https://github.com/rwese/kb#installation))
- Run `kb setup` to initialize the database

## Data Model

- **Entry**: Container for a topic (title + tags)
- **Article**: Individual piece of content within an entry (searchable unit)

Split large content into multiple articles. Example:

```bash
# ❌ Bad: One massive entry
kb add -t "Kubernetes Debugging" -c "<5000 lines of logs>"

# ✅ Good: Split by concern
kb add -t "Kubernetes Debugging" -c "Overview and common patterns"
kb append --entry-title "Kubernetes Debugging" -c "Pod scheduling failures"
kb append --entry-title "Kubernetes Debugging" -c "Network policies"
kb append --entry-title "Kubernetes Debugging" -c "Resource limits and OOMKills"
```

## Commands

```bash
kb list                          # List all entries
kb add -t "Title" -c "Content"   # Create entry (first article)
kb append --entry <id> -c "..." # Add article to entry
kb append --entry-title "Title" -c "..."  # Append by title (creates entry if missing)
kb get <entry-id>                # Get entry with all articles
kb get --articles               # Include articles in list output
kb search "query"               # Search articles (BM25 ranked)
kb stats                         # Show entry/article counts
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
