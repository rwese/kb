## Goal

Export kb knowledgebase entries to Obsidian-compatible markdown files with YAML front matter in a flat directory structure, where each entry is a folder containing separate article files.

## Export — TODO

### In Progress

- [ ] ...

### Ready

- [ ] **Core Export** — Single entry → folder with article .md files + front matter
- [ ] **Front Matter Generation** — YAML with title, tags, aliases, dates, kb_id
- [ ] **Flat Directory Layout** — `output/{entry-title}/{article-1}.md`
- [ ] **Filename Sanitization** — Slugify titles for folder/file names
- [ ] **Conflict Detection** — Scan output dir, parse front matter, collect existing kb_ids
- [ ] **Interactive Overwrite** — Prompt user for each conflicting entry
- [ ] **Batch Export** — Export all entries with `--all` flag

### Blocked

- [ ] ...

### Done

- [x] Clarified requirements (flat dir, separate article files, title-based naming)
- [x] Front matter `id` → `kb_id`; articles use article's kb_id
- [x] Article filenames: slugified article title
- [x] Articles inherit entry tags in front matter
- [x] Folder collision: append entry ID suffix
- [x] Core Export — Single entry → folder with article .md files + front matter
- [x] Front Matter Generation — YAML with title, tags, aliases, dates, kb_id
- [x] CLI Command — `kb export [--entry <id>] --output <dir>`
- [x] Conflict Detection — Scan output dir, parse front matter, collect existing kb_ids
- [x] Interactive Overwrite — Prompt user for each conflicting entry
- [x] Batch Export — Export all entries with `--all` flag
- [x] Filename Sanitization — Slugify titles for folder/file names
- [x] Flat Directory Layout — `output/{entry-title}/{article-1}.md`

---

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
