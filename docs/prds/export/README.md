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

Entry attachments are exported with the entry, including entries that have no
articles. They live under an `assets/attachments/` tree parallel to article
assets in the entry export directory:

```
<entry-export-dir>/
├── <entry-slug>.md
└── assets/
    ├── <article-id>/            # article assets (unchanged)
    │   └── <logical-path>
    └── attachments/
        └── <attachment-id>/
            └── <file-name>
```

The main entry Markdown file contains an `## Attachments` section with relative
links:

```markdown
## Attachments

- [Linux helper executable](assets/attachments/a1b2c3/helper) (`helper`, 1.2 MB)
```

Existing article asset links and paths remain unchanged. Export preserves regular
permission bits of both article assets and attachments. `--dry-run` lists every
attachment destination alongside article assets and markdown files.

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
| `--force` | Skip overwrite confirmation prompt |
| `--dry-run` | Preview without writing |
