package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

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

	exportPath, err := ExportEntry(entry, articles, nil, filepath.Join(tmpDir, "out"), assetsRoot, false)
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
	if _, err := ExportEntry(entry, nil, attachments, dryOutput, assetsRoot, true); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(dryOutput); !os.IsNotExist(err) {
		t.Fatal("dry run must not create output dir")
	}

	// Attachment-only entry (no articles)
	exportPath, err := ExportEntry(entry, nil, attachments, filepath.Join(tmpDir, "out"), assetsRoot, false)
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
	if !strings.Contains(text, "[Linux helper executable](assets/attachments/att01/helper) (`helper`, 11 B)") {
		t.Fatalf("missing relative attachment link:\n%s", text)
	}

	exported := filepath.Join(exportPath, "assets", "attachments", "att01", "helper")
	info, err := os.Stat(exported)
	if err != nil {
		t.Fatalf("exported attachment missing: %v", err)
	}
	if info.Mode().Perm() != 0755 {
		t.Fatalf("exported perm = %o, want 755", info.Mode().Perm())
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

	exportPath, err := ExportEntry(entry, articles, attachments, filepath.Join(tmpDir, "out"), assetsRoot, false)
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
