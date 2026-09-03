---
name: knowledgebase
description: "knowledgebase search, lookup and storing of learnings, findings and helpers."
---

# Knowledgebase

Use `kb` as durable local memory.

- **Entry**: one topic; holds the title and tags.
- **Article**: one searchable note within an entry.
- **Asset**: a KB-owned file attached to an article.
- **Attachment**: one titled, KB-owned regular file attached directly to an entry (parallel to an article), e.g. a script or archive.

Keep each article focused on one reusable fact, procedure, or decision. Add an article when the topic stays the same; create an entry when the topic changes.

## Inspect

```bash
kb status
kb config
kb init       # Run only when the database is not initialized
```

Use `kb --help` and `kb <command> --help` as the source of truth for uncommon operations.

## Search

Search before writing to find prior solutions and avoid duplicate topics.

```bash
kb search --top-k 5 "wrapper quoting"
kb search --format json --top-k 5 "wrapper quoting"
kb search --bm25-only "wrapper quoting"
```

Use short FTS5-safe keywords. If a query fails with `fts5: syntax error near "?"`, remove punctuation or simplify it. Put the complete retrieval query in either the positional argument or `--prompt`; when both are present, `--prompt` wins.

Search omits content by default. Pass `--content` for a per-entry excerpt or `--full-content` for uncut bodies; use `kb entry get <entry-id>` to read full text of a hit.

Use `--format json` when parsing results. Use `--bm25-only` to diagnose semantic-ranking problems.

## Store

Write multiline content to a file and pass `-f`. Reserve `-c` for short text.

```bash
# Create a topic with its first article.
kb entry create -t "Wrapper script quoting" \
  --tags "bash,quoting" \
  -f note.md

# Add another searchable concern to the same topic.
kb entry article add -t "Smoke-test procedure" \
  -f smoke-test.md \
  <entry-id>
```

Use titles and opening lines that contain terms likely to be searched. Capture the entry and article IDs printed by each command.

## Read and update

```bash
kb entry list --json --articles
kb entry get --articles <entry-id>
kb entry article list --json <entry-id>
kb entry article get <entry-id> <article-id>
kb entry update -t "New topic title" <entry-id>
kb entry article update -t "New article title" -f revised.md \
  <entry-id> <article-id>
```

Article updates replace the complete body. Read the current article first when any existing content must survive.

## Attach assets

```bash
kb entry article asset add --json \
  <entry-id> <article-id> path/to/file path/to/directory
kb entry article asset list --json <entry-id> <article-id>
kb entry article asset get --json <entry-id> <article-id> <asset-id>
kb entry article asset delete --json <entry-id> <article-id> <asset-id>
```

Use `--overwrite` on `asset add` only when replacing a colliding logical path is intended.

## Attach entry attachments

Keep a binary or executable file with the entry it belongs to (not inside an article):

```bash
kb entry attachment add --title "Linux helper executable" \
  <entry-id> path/to/file
kb entry attachment list --json <entry-id>
kb entry attachment get --json <entry-id> <attachment-id>
kb entry attachment update --title "New title" --file path/to/replacement \
  <entry-id> <attachment-id>
kb entry attachment delete --force <entry-id> <attachment-id>
```

`attachment add` accepts exactly one regular file; directories, symlinks, and special files are rejected, so add attachments for single files only. Attachment IDs are six characters and entry-scoped - always pass the entry ID first.

## Export and delete

```bash
kb export --all --dry-run -o export/
kb export --all -o export/
kb export --entry <entry-id> -o export/

kb entry article delete <entry-id> <article-id>
kb entry delete <entry-id>
```

Run export with `--dry-run` before writing. Pass `--with-attachments` to also
export entry attachments into an `attachments/` subdirectory of each entry
(markdown-only exports omit them entirely). Every export run writes an Obsidian
`INDEX.md` at the output root with one section per entry and, per exported file,
a wikilink, heading, short description, and the entry tags - open it to browse
the vault. Entry and article deletion is permanent; an entry deletion also removes its articles, vectors, article assets, and attachments. Keep the confirmation prompt unless non-interactive deletion was explicitly authorized; with `--force`, missing entry IDs are skipped and the remaining entries are still deleted.

## Completion

After every mutation:

1. Verify the result with `entry get`, `article get`, `asset list`, or `search`.
2. Report the affected IDs and any output path.
3. Quote command errors exactly.
