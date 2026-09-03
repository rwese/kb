# Export Specification

## Goal

Export kb knowledgebase entries to Obsidian-compatible Markdown files with YAML front matter in a flat directory structure. Each entry has a primary file and, when needed, a folder containing additional article files.

## Export Spec (v1)

### Directory Structure

```
output/
├── my-knowledge-note.md           # Entry file (first article)
├── my-knowledge-note/             # Folder for additional articles
│   ├── article-two-title.md
│   └── another-article-title.md
└── another-note.md                # Single article entry
```

### Entry attachments

Entry attachments are exported only with `--with-attachments` (markdown-only
exports leave them out, including entries that have no articles). They live
under an `attachments/` subdirectory of the entry export directory:

```
<entry-export-dir>/
├── <entry-slug>.md
├── attachments/
│   └── <attachment-id>/
│       └── <file-name>
└── assets/
    └── <article-id>/            # article assets (unchanged)
        └── <logical-path>
```

The main entry Markdown file contains an `## Attachments` section with relative
links when the flag is set:

```markdown
## Attachments

- [Linux helper executable](attachments/a1b2c3/helper) (`helper`, 1.2 MB)
```

Without `--with-attachments` neither the files nor the `## Attachments` section
are written, so no links dangle. Existing article asset links and paths remain
unchanged. Export preserves regular permission bits of both article assets and
attachments. `--dry-run` lists every attachment destination alongside article
assets and markdown files when the flag is set.

### Front Matter Template

Entry files:
```yaml
---
title: "Entry Title"
kb_id: "2f018d"
aliases:
  - "Entry Title"
tags:
  - tag1
  - tag2
created: 2024-01-15
updated: 2024-01-16
kb_source: kb
---

# Entry Title

Article content...
```

Article files (inherit entry tags):
```yaml
---
title: "Article 2 Title"
kb_id: "2f018d-a1b2c3"
parent_id: "2f018d"
tags:
  - tag1
  - tag2
created: 2024-01-15
kb_source: kb
---

# Article 2 Title

Article content...
```

### Folder Slug Collision

When two entries share the same title, append entry ID:
```
my-note/
my-note-2f018d/   # collision resolved
```

### Index file

Every export run writes `INDEX.md` at the output directory root, listing every
file written in that run so the vault can be browsed from a single entry point.
The index uses Obsidian syntax: wikilinks, entry section headings, and `#tag`
tags.

```markdown
---
title: Knowledge Base Index
aliases:
  - Knowledge Base Index
created: 2026-05-20
kb_source: kb
---

# Knowledge Base Index

Exported 3 entries from kb on 2026-05-20.

## HTTP Cache Bug

- [[http-cache-bug|HTTP Cache Bug]] - Steps to reproduce... #bug #cache
- [[fix-details|Fix Details]] - Throttle with requestAnimationFrame. #bug #cache

## Linux Helper

- [[linux-helper|Linux Helper]] - *No content*
```

Per file the index lists:

- **Link** - an Obsidian wikilink. The file basename is used when it is unique
  in the vault (matching Obsidian's own resolution); otherwise the
  vault-relative path disambiguates, e.g. `[[my-note-2f018d/my-note|My Note]]`.
- **Heading** - the `#` heading of the exported file: the entry title for the
  primary file, the article title for additional article files.
- **Short description** - the article's first paragraph with markdown stripped
  (links become their text), truncated to 160 characters. Entries without
  articles show `*No content*`.
- **Tags** - the entry's tags in Obsidian tag syntax (`#bug #cache`), inherited
  by every file of the entry.

Entries are sorted by title. The index has no `kb_id`, so re-import into kb
ignores it. If `INDEX.md` already exists in the output directory, the same
confirmation prompt as for entry conflicts applies (`--force` skips it, and a
previously chosen `[A]ll` also covers the index); the index references only the
entries actually written in the current run, so a skipped conflict entry is not
listed. `--dry-run` prints the index destination alongside entry files.

### Conflict Detection Flow

1. Scan `--output` directory recursively for `*.md` files
2. Parse front matter from each file, extract `kb_id`
3. Collect `kb_id` → file path mapping
4. For each entry to export:
   - If `kb_id` exists in mapping and `--force` not set → prompt user
   - Show: `Found existing: kb_id "2f018d" → my-note/my-note.md`
   - Options: [Y]es, [N]o, [A]ll, [Q]uit

### Flags

| Flag | Description |
|------|-------------|
| `--output, -o` | Output directory (required) |
| `--entry` | Export single entry by ID |
| `--all` | Export all entries |
| `--force` | Skip overwrite confirmation prompt (entries and `INDEX.md`) |
| `--with-attachments` | Also export entry attachments into `<entry>/attachments/` |
| `--dry-run` | Preview without writing |
