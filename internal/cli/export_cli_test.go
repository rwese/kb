package cli

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rwese/kb/internal/db"
)

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
