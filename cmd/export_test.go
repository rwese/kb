package cmd

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rwese/kb/internal/db"
)

func TestExportEntryWritesFolderMarkdownAndAssets(t *testing.T) {
	tmpDir := t.TempDir()
	assetsRoot := filepath.Join(tmpDir, "store")
	storedAssetPath := filepath.Join(assetsRoot, "entry1-art1", "asset01", "trace.har")
	if err := os.MkdirAll(filepath.Dir(storedAssetPath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(storedAssetPath, []byte("trace"), 0644); err != nil {
		t.Fatal(err)
	}

	entry := &db.Entry{
		ID:        "entry1",
		Title:     "HTTP Cache Bug",
		Tags:      "bug,cache",
		CreatedAt: "2026-05-01 10:00:00",
		UpdatedAt: "2026-05-02 12:00:00",
	}
	articles := []articleView{
		{
			Article: db.Article{
				ID:        "entry1-art1",
				EntryID:   "entry1",
				Title:     "Reproduction Notes",
				Content:   "Steps",
				CreatedAt: "2026-05-01 10:00:00",
			},
			Assets: []db.ArticleAsset{
				{
					ID:           "asset01",
					ArticleID:    "entry1-art1",
					LogicalPath:  "trace.har",
					StoreRelPath: "entry1-art1/asset01/trace.har",
				},
			},
		},
	}

	exportPath, err := ExportEntry(entry, articles, nil, filepath.Join(tmpDir, "out"), assetsRoot, false, false)
	if err != nil {
		t.Fatal(err)
	}

	mainFile := filepath.Join(exportPath, "http-cache-bug.md")
	content, err := os.ReadFile(mainFile)
	if err != nil {
		t.Fatal(err)
	}
	text := string(content)
	if !strings.Contains(text, "## Assets") {
		t.Fatalf("missing assets section: %s", text)
	}
	if !strings.Contains(text, "[trace.har](assets/entry1-art1/trace.har)") {
		t.Fatalf("missing relative asset link: %s", text)
	}

	exportedAsset := filepath.Join(exportPath, "assets", "entry1-art1", "trace.har")
	if _, err := os.Stat(exportedAsset); err != nil {
		t.Fatalf("exported asset missing: %v", err)
	}
}

func TestExportEntryWritesAttachmentsAndDryRun(t *testing.T) {
	tmpDir := t.TempDir()
	assetsRoot := filepath.Join(tmpDir, "store")

	// Stored attachment bytes (0755, executable)
	storedPath := filepath.Join(assetsRoot, "entries", "entry1", "attachments", "att01", "helper")
	if err := os.MkdirAll(filepath.Dir(storedPath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(storedPath, []byte("#!/bin/sh\n"), 0755); err != nil {
		t.Fatal(err)
	}

	entry := &db.Entry{
		ID:        "entry1",
		Title:     "Linux Helper",
		CreatedAt: "2026-05-01 10:00:00",
		UpdatedAt: "2026-05-02 12:00:00",
	}
	attachments := []db.EntryAttachment{
		{
			ID:           "att01",
			EntryID:      "entry1",
			Title:        "Linux helper executable",
			FileName:     "helper",
			StoreRelPath: "entries/entry1/attachments/att01/helper",
			SizeBytes:    11,
			ModePerm:     0755,
		},
	}

	// Dry run lists the attachment destination without writing
	dryOutput := filepath.Join(tmpDir, "dry")
	if _, err := ExportEntry(entry, nil, attachments, dryOutput, assetsRoot, true, true); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(dryOutput); !os.IsNotExist(err) {
		t.Fatal("dry run must not create output dir")
	}

	// Attachment-only entry (no articles)
	exportPath, err := ExportEntry(entry, nil, attachments, filepath.Join(tmpDir, "out"), assetsRoot, true, false)
	if err != nil {
		t.Fatal(err)
	}
	mainFile := filepath.Join(exportPath, "linux-helper.md")
	content, err := os.ReadFile(mainFile)
	if err != nil {
		t.Fatal(err)
	}
	text := string(content)
	if !strings.Contains(text, "## Attachments") {
		t.Fatalf("missing attachments section:\n%s", text)
	}
	if !strings.Contains(text, "[Linux helper executable](attachments/att01/helper) (`helper`, 11 B)") {
		t.Fatalf("missing relative attachment link:\n%s", text)
	}

	exported := filepath.Join(exportPath, "attachments", "att01", "helper")
	info, err := os.Stat(exported)
	if err != nil {
		t.Fatalf("exported attachment missing: %v", err)
	}
	if info.Mode().Perm() != 0755 {
		t.Fatalf("exported perm = %o, want 755", info.Mode().Perm())
	}
}

func TestExportEntrySkipsAttachmentsWithoutFlag(t *testing.T) {
	tmpDir := t.TempDir()
	assetsRoot := filepath.Join(tmpDir, "store")
	storedPath := filepath.Join(assetsRoot, "entries", "entry1", "attachments", "att01", "helper")
	if err := os.MkdirAll(filepath.Dir(storedPath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(storedPath, []byte("#!/bin/sh\n"), 0755); err != nil {
		t.Fatal(err)
	}

	entry := &db.Entry{ID: "entry1", Title: "Linux Helper", CreatedAt: "2026-05-01 10:00:00"}
	attachments := []db.EntryAttachment{
		{ID: "att01", EntryID: "entry1", Title: "Helper", FileName: "helper", StoreRelPath: "entries/entry1/attachments/att01/helper", SizeBytes: 11, ModePerm: 0755},
	}

	exportPath, err := ExportEntry(entry, nil, attachments, filepath.Join(tmpDir, "out"), assetsRoot, false, false)
	if err != nil {
		t.Fatal(err)
	}

	content, err := os.ReadFile(filepath.Join(exportPath, "linux-helper.md"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(content), "## Attachments") {
		t.Fatalf("attachments section must be omitted without the flag:\n%s", content)
	}
	if _, err := os.Stat(filepath.Join(exportPath, "attachments", "att01")); !os.IsNotExist(err) {
		t.Fatalf("attachments must not be copied without the flag (err = %v)", err)
	}
}

func TestExportEntryAttachmentWithArticleKeepsAssetSection(t *testing.T) {
	tmpDir := t.TempDir()
	assetsRoot := filepath.Join(tmpDir, "store")
	storedPath := filepath.Join(assetsRoot, "entries", "entry1", "attachments", "att01", "helper")
	if err := os.MkdirAll(filepath.Dir(storedPath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(storedPath, []byte("x"), 0755); err != nil {
		t.Fatal(err)
	}

	entry := &db.Entry{ID: "entry1", Title: "Linux Helper", CreatedAt: "2026-05-01 10:00:00"}
	articles := []articleView{
		{Article: db.Article{ID: "entry1-art1", EntryID: "entry1", Title: "Notes", Content: "body", CreatedAt: "2026-05-01 10:00:00"}},
	}
	attachments := []db.EntryAttachment{
		{ID: "att01", EntryID: "entry1", Title: "Helper", FileName: "helper", StoreRelPath: "entries/entry1/attachments/att01/helper", SizeBytes: 1, ModePerm: 0755},
	}

	exportPath, err := ExportEntry(entry, articles, attachments, filepath.Join(tmpDir, "out"), assetsRoot, true, false)
	if err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(filepath.Join(exportPath, "linux-helper.md"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(content)
	if !strings.Contains(text, "## Attachments") {
		t.Fatalf("missing attachments section in entry file:\n%s", text)
	}
	if !strings.Contains(text, "body") {
		t.Fatalf("article body missing from entry file:\n%s", text)
	}
}

func TestGenerateIndexProducesBrowsableObsidianIndex(t *testing.T) {
	outputDir := filepath.Join(t.TempDir(), "out")
	generatedAt := time.Date(2026, 5, 20, 0, 0, 0, 0, time.UTC)

	entries := []exportItem{
		{
			entry: &db.Entry{ID: "entry1", Title: "HTTP Cache Bug", Tags: "bug,cache", CreatedAt: "2026-05-01 10:00:00"},
			articles: []articleView{
				{Article: db.Article{ID: "entry1-art1", EntryID: "entry1", Title: "Reproduction Notes", Content: "**Steps** to reproduce with [link](https://example.com)\n\nSecond paragraph."}},
				{Article: db.Article{ID: "entry1-art2", EntryID: "entry1", Title: "Fix Details", Content: "Throttle with requestAnimationFrame."}},
				{Article: db.Article{ID: "entry1-art3", EntryID: "entry1", Title: "Notes", Content: "Scratch notes."}},
			},
		},
		{
			entry: &db.Entry{ID: "entry2", Title: "Linux Helper", Tags: "", CreatedAt: "2026-05-01 10:00:00"},
		},
		{
			entry: &db.Entry{ID: "entry3", Title: "Second Bug", Tags: "bug", CreatedAt: "2026-05-01 10:00:00"},
			articles: []articleView{
				{Article: db.Article{ID: "entry3-art1", EntryID: "entry3", Title: "Repro", Content: "How to reproduce."}},
				{Article: db.Article{ID: "entry3-art2", EntryID: "entry3", Title: "Notes", Content: "Other notes."}},
			},
		},
	}

	content, err := generateIndex(entries, outputDir, generatedAt)
	if err != nil {
		t.Fatal(err)
	}

	// Front matter and heading.
	if !strings.Contains(content, "title: Knowledge Base Index") {
		t.Fatalf("missing index front matter title:\n%s", content)
	}
	if !strings.Contains(content, "# Knowledge Base Index") {
		t.Fatalf("missing index heading:\n%s", content)
	}

	// Section heading per entry.
	if !strings.Contains(content, "## HTTP Cache Bug") {
		t.Fatalf("missing entry section heading:\n%s", content)
	}

	// Primary file: basename wikilink with heading alias when unique.
	if !strings.Contains(content, "[[http-cache-bug|HTTP Cache Bug]]") {
		t.Fatalf("missing primary wikilink:\n%s", content)
	}
	// Unique article basename resolves to the basename link.
	if !strings.Contains(content, "[[fix-details|Fix Details]]") {
		t.Fatalf("missing unique article wikilink:\n%s", content)
	}
	// Duplicate basenames (Notes) fall back to vault-relative paths.
	if !strings.Contains(content, "[[http-cache-bug/notes|Notes]]") {
		t.Fatalf("missing path-disambiguated wikilink:\n%s", content)
	}
	if !strings.Contains(content, "[[second-bug/notes|Notes]]") {
		t.Fatalf("missing second path-disambiguated wikilink:\n%s", content)
	}
	if strings.Contains(content, "- [[notes|") {
		t.Fatalf("ambiguous basename must not be used bare:\n%s", content)
	}

	// Short description: bold/link markdown stripped, one paragraph only.
	if !strings.Contains(content, "Steps to reproduce with link") {
		t.Fatalf("markdown not stripped from description:\n%s", content)
	}
	if strings.Contains(content, "Second paragraph") {
		t.Fatalf("description must stop at first paragraph:\n%s", content)
	}

	// Tags in Obsidian syntax, inherited from the entry.
	if !strings.Contains(content, "#bug #cache") {
		t.Fatalf("missing obsidian tags:\n%s", content)
	}

	// Attachment-only entry without articles.
	if !strings.Contains(content, "[[linux-helper|Linux Helper]] — *No content*") {
		t.Fatalf("missing attachment-only entry line:\n%s", content)
	}
}

func TestShortDescriptionTruncates(t *testing.T) {
	long := strings.Repeat("word ", 100)
	desc := shortDescription(long + "\n\n# Heading")
	if len(desc) > 165 {
		t.Fatalf("description not truncated: %d chars", len(desc))
	}
	if !strings.HasSuffix(desc, "…") {
		t.Fatalf("truncated description should end with ellipsis: %q", desc)
	}
}

func TestExportWithAttachmentsFlagGatesAttachmentCopy(t *testing.T) {
	env := setupTempKBTestEnv(t)

	database, err := db.Open(env.DBPath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = database.Close() }()
	if err := database.Init(); err != nil {
		t.Fatal(err)
	}
	if err := database.AddEntry("entry01", "Linux Helper", "shell"); err != nil {
		t.Fatal(err)
	}
	// Seed a stored attachment like attachment add would.
	stored := filepath.Join(env.AssetsPath, "entries", "entry01", "attachments", "att01", "helper")
	if err := os.MkdirAll(filepath.Dir(stored), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(stored, []byte("#!/bin/sh\n"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := database.AddEntryAttachment(db.EntryAttachment{
		ID: "att01", EntryID: "entry01", Title: "Helper script", FileName: "helper",
		OriginalPath: stored, StoreRelPath: "entries/entry01/attachments/att01/helper",
		SHA256: "x", SizeBytes: 11, ModePerm: 0755,
	}); err != nil {
		t.Fatal(err)
	}

	// Without the flag: no attachments dir and no links.
	outPlain := filepath.Join(t.TempDir(), "plain")
	commands := &Commands{}
	if err := commands.Run(context.Background(), []string{"kb", "export", "--all", "--force", "-o", outPlain}); err != nil {
		t.Fatalf("export without flag failed: %v", err)
	}
	plainContent, err := os.ReadFile(filepath.Join(outPlain, "linux-helper", "linux-helper.md"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(plainContent), "## Attachments") {
		t.Fatalf("attachments section present without flag:\n%s", plainContent)
	}
	if _, err := os.Stat(filepath.Join(outPlain, "linux-helper", "attachments")); !os.IsNotExist(err) {
		t.Fatalf("attachments copied without flag (err = %v)", err)
	}

	// With the flag: attachments land under <entry>/attachments/ and are linked.
	outWith := filepath.Join(t.TempDir(), "with")
	if err := commands.Run(context.Background(), []string{"kb", "export", "--all", "--with-attachments", "--force", "-o", outWith}); err != nil {
		t.Fatalf("export with flag failed: %v", err)
	}
	withContent, err := os.ReadFile(filepath.Join(outWith, "linux-helper", "linux-helper.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(withContent), "[Helper script](attachments/att01/helper)") {
		t.Fatalf("missing attachment link:\n%s", withContent)
	}
	exported := filepath.Join(outWith, "linux-helper", "attachments", "att01", "helper")
	info, err := os.Stat(exported)
	if err != nil {
		t.Fatalf("exported attachment missing: %v", err)
	}
	if info.Mode().Perm() != 0755 {
		t.Fatalf("exported perm = %o, want 755", info.Mode().Perm())
	}
}

func TestExportAllWritesIndexFile(t *testing.T) {
	env := setupTempKBTestEnv(t)

	database, err := db.Open(env.DBPath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = database.Close() }()
	if err := database.Init(); err != nil {
		t.Fatal(err)
	}
	if err := database.AddEntry("entry01", "Alpha Entry", "alpha,test"); err != nil {
		t.Fatal(err)
	}
	if err := database.AddArticle("entry01-art01", "entry01", "First Article", "Alpha body text."); err != nil {
		t.Fatal(err)
	}
	if err := database.AddArticle("entry01-art02", "entry01", "Second Article", "More alpha details."); err != nil {
		t.Fatal(err)
	}

	outputDir := filepath.Join(t.TempDir(), "vault")
	commands := &Commands{}
	if err := commands.Run(context.Background(), []string{"kb", "export", "--all", "--force", "-o", outputDir}); err != nil {
		t.Fatalf("export command failed: %v", err)
	}

	indexPath := filepath.Join(outputDir, "INDEX.md")
	content, err := os.ReadFile(indexPath)
	if err != nil {
		t.Fatalf("INDEX.md missing: %v", err)
	}
	text := string(content)
	if !strings.Contains(text, "## Alpha Entry") {
		t.Fatalf("index missing entry section:\n%s", text)
	}
	if !strings.Contains(text, "[[alpha-entry|Alpha Entry]]") {
		t.Fatalf("index missing entry wikilink:\n%s", text)
	}
	if !strings.Contains(text, "[[second-article|Second Article]]") {
		t.Fatalf("index missing article wikilink:\n%s", text)
	}
	if !strings.Contains(text, "#alpha #test") {
		t.Fatalf("index missing tags:\n%s", text)
	}
	if !strings.Contains(text, "Alpha body text") {
		t.Fatalf("index missing description:\n%s", text)
	}
}
