# Entry Attachment Specification

Status: Implemented
Task: `2026-09-03-EWOOSIDF-NOOIK`

## Goal

Allow an entry to contain either searchable text articles, titled file attachments, or both. An attachment is one regular file copied into kb-owned storage. It is a first-class child of an entry, parallel to an article.

## Terminology

- **Article**: searchable text stored in SQLite and optionally embedded.
- **Attachment**: one titled file attached directly to an entry.
- **Article asset**: an existing file or directory attached to an article. This feature does not rename or migrate article assets.

## User stories

- As a user, I can keep an executable, script, archive, document, or other file with the entry it belongs to.
- As a user, I can find an attachment-only entry by the attachment title or filename.
- As a user, I can inspect the attachment's managed path and export a byte-identical copy later.
- As an existing user, I can continue using article assets without migration or command changes.

## User experience

Primary command:

```bash
kb entry attachment add \
  --title "Linux helper executable" \
  <entry-id> ./bin/helper
```

The attachment command group should provide:

```text
kb entry attachment
├── add       # create a titled attachment from one file
├── list      # list attachments for an entry
├── get       # show attachment metadata and managed path
├── update    # change the title and/or replace the stored file
└── delete    # permanently delete metadata and stored bytes
```

All commands accept `--json`. `add` requires one non-empty `--title`, one entry ID, and exactly one file path. `delete` prompts unless `--force` is supplied. `update` requires at least one of `--title` or `--file`.

Creating an attachment-only entry remains a two-step operation:

```bash
kb entry create --no-article --title "Linux helper"
kb entry attachment add --title "Executable" <entry-id> ./helper
```

Adding attachment flags to `entry create` is outside the first release.

## Data model

Add an additive table; existing databases are initialized without rewriting current data:

```sql
CREATE TABLE IF NOT EXISTS entry_attachments (
    id TEXT NOT NULL,
    entry_id TEXT NOT NULL,
    title TEXT NOT NULL,
    file_name TEXT NOT NULL,
    original_path TEXT NOT NULL,
    store_rel_path TEXT NOT NULL UNIQUE,
    sha256 TEXT NOT NULL,
    size_bytes INTEGER NOT NULL,
    mode_perm INTEGER NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (entry_id, id),
    FOREIGN KEY (entry_id) REFERENCES entries(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_entry_attachments_entry
    ON entry_attachments(entry_id);
```

Attachment IDs are six hexadecimal characters generated with the existing short-ID generator. They are entry-scoped in the CLI: every get, update, and delete command requires both the entry ID and attachment ID. This avoids conflict with the current `entry get` rule that treats hyphenated IDs as article IDs.

Duplicate titles and source filenames are allowed. Each attachment has an isolated storage directory, so replacement is explicit through `attachment update --file` rather than path collision or `--overwrite` behavior.

## Managed file storage

Files are stored under a namespace separate from article assets:

```text
<assets_path>/entries/<entry-id>/attachments/<attachment-id>/<file-name>
```

Requirements:

- Copy bytes into kb-owned storage; do not retain an external-path-only reference.
- Record the absolute original path for provenance.
- Trim the title and reject an empty result.
- Accept exactly one regular file per attachment.
- Reject directories, symbolic links, sockets, devices, and named pipes.
- Stream the copy and record SHA-256 and byte size.
- Preserve regular permission bits, including executable bits; strip special bits.
- Never inspect, source, or execute the attachment.
- Stage additions and replacements before committing metadata.
- Remove staged files after any database failure.
- Return an explicit partial-failure error when metadata changed but old stored bytes could not be removed.

Entry deletion removes both the database rows through the foreign-key cascade and the entry attachment storage tree. Article deletion does not affect entry attachments.

## Entry views

Add attachments as a parallel collection to articles.

- `kb entry get --attachments <entry-id>` prints an `Attachments` section.
- `kb entry get --articles --attachments --json <entry-id>` returns both arrays.
- `kb entry list` adds an attachment count beside the article count.
- JSON fields use `attachments`; existing `articles` and nested article `assets` remain unchanged.
- Attachment mutations update `entries.updated_at`.

## Search

Attachment binary contents are never indexed. Attachment title and filename are indexed with FTS5 so an attachment-only entry can be retrieved.

Add an attachment FTS table and insert, update, and delete triggers. Search returns typed hits:

- `type: "article"` for current article results.
- `type: "attachment"` for attachment metadata results.

Article hits keep the current BM25 and optional semantic ranking. Attachment hits use metadata BM25 only. Normalize each candidate set before merging so raw scores from separate FTS tables are not compared directly. Group both result types under their entry in compact output. `--content` applies only to article bodies; attachment hits show title, filename, and attachment ID.

Existing article-only searches and JSON fields remain compatible apart from the additive `type` and attachment-specific fields.

## Export

Export entry attachments with the entry, including entries that have no articles.

```text
<entry-export-dir>/
├── <entry-slug>.md
└── assets/
    └── attachments/
        └── <attachment-id>/
            └── <file-name>
```

The main entry Markdown file contains:

```markdown
## Attachments

- [Linux helper executable](assets/attachments/a1b2c3/helper) (`helper`, 1.2 MB)
```

Export preserves regular permission bits. `--dry-run` lists every attachment destination. Existing article asset links and paths remain unchanged.

## Statistics and status

- Keep `article_assets` and entry attachments as separate concepts and counts.
- Add `TotalAttachments` to database statistics.
- Rename user-facing `Assets` statistics to `Article assets` for clarity.
- `kb status` checks and reports both tables.

## Compatibility and migration

- The schema change is additive through `DB.Init()`.
- Existing entries, articles, vectors, article assets, commands, IDs, and storage paths are unchanged.
- No existing file is moved.
- FTS5 remains mandatory for builds and tests.
- Soft-delete behavior is not added by this feature.

## Security

- Treat all files as untrusted opaque bytes.
- Do not infer commands from executable or script files.
- Do not follow symlinks during validation or copying.
- Use path-safe basenames and fixed managed roots.
- Prevent traversal outside `assets_path` during store, export, update, and delete.
- Do not expose attachment contents in search output or embeddings.

## Acceptance criteria

1. A user can add, list, inspect, rename, replace, and delete one titled entry attachment through the CLI and JSON output.
2. Stored bytes are identical to the source, SHA-256 and size are correct, and executable permission bits survive store and export.
3. Invalid titles, directories, symlinks, special files, and ownership mismatches fail without committed metadata or leaked staged files.
4. Existing databases gain the attachment schema without data migration or changes to article assets.
5. `entry get`, `entry list`, `stats`, and `status` expose attachment information without removing existing fields.
6. Search finds attachment-only entries by attachment title or filename and does not index file contents.
7. Export copies entry attachments, emits relative links, handles entries without articles, supports dry-run, and preserves permissions.
8. Deleting an attachment or entry removes its managed bytes; deleting an article does not.
9. Existing article, article asset, search, export, and delete tests remain green.
10. The FTS5 build, full test suite, and project quality gate pass.

## Implementation plan

- `2026-09-03-EWRPBIYE-ZACMT` — Add the attachment schema, model, CRUD, FTS metadata index, statistics, and database tests in `internal/db/`.
- `2026-09-03-EWRPBJTG-ZACMT` — Generalize managed-file staging/copy/export helpers while preserving article asset behavior; add permission and file-type tests in `internal/assets/`.
- `2026-09-03-EWRPBJTM-ZACMT` — Add `kb entry attachment add|list|get|update|delete` and hermetic CLI tests in `cmd/`.
- `2026-09-03-EWRPBJTP-ZACMT` — Integrate attachments into entry views, timestamps, counts, status, and entry deletion cleanup.
- `2026-09-03-EWRPBJTR-ZACMT` — Add typed attachment metadata hits to search and preserve article ranking compatibility.
- `2026-09-03-EWRPBJTT-ZACMT` — Extend export, dry-run, permission preservation, and `docs/prds/export/README.md`.
- `2026-09-03-EWRPBJTV-ZACMT` — Update `README.md`, `DEVELOPMENT.md`, `AGENTS.md`, and `skills/knowledgebase/SKILL.md`; run all quality gates.

## Quality gates

```bash
CGO_CFLAGS="-DSQLITE_ENABLE_FTS5" go test ./...
CGO_CFLAGS="-DSQLITE_ENABLE_FTS5" go build -tags sqlite_fts5 -o bin/kb .
bin/kb status

export CGO_CFLAGS="-DSQLITE_ENABLE_FTS5"
just check
```
