package cmd

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rwese/kb/internal/db"
)

// runCmdErr runs a kb command and returns the error (no stdout capture needed).
func runCmdErr(c *Commands, args ...string) error {
	return c.Run(context.Background(), args)
}

func seedAttachmentTestEnv(t *testing.T) testEnv {
	t.Helper()
	env := setupTempKBTestEnv(t)

	database, err := db.Open(env.DBPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer func() { _ = database.Close() }()
	if err := database.Init(); err != nil {
		t.Fatalf("init db: %v", err)
	}
	if err := database.AddEntry("abc123", "Linux helper", "tools"); err != nil {
		t.Fatal(err)
	}
	if err := database.AddArticle("abc123-111111", "abc123", "Notes", "helper notes"); err != nil {
		t.Fatal(err)
	}
	return env
}

func writeSourceFile(t *testing.T, dir, name string, content []byte, perm os.FileMode) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, content, perm); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestAttachmentAddListGetUpdateDelete(t *testing.T) {
	env := seedAttachmentTestEnv(t)
	c := &Commands{}
	srcDir := t.TempDir()
	helper := writeSourceFile(t, srcDir, "helper", []byte("#!/bin/sh\necho hi\n"), 0755)

	out := runCmdCapture(t, c, "kb", "entry", "attachment", "add", "--title", "Helper script", "abc123", helper)
	if !strings.Contains(out, "Added attachment") {
		t.Fatalf("add output missing confirmation, got:\n%s", out)
	}

	// List
	out = runCmdCapture(t, c, "kb", "entry", "attachment", "list", "abc123")
	if !strings.Contains(out, "| ") || !strings.Contains(out, "Helper script") {
		t.Fatalf("list output missing attachment, got:\n%s", out)
	}

	// List JSON carries file metadata
	out = runCmdCapture(t, c, "kb", "entry", "attachment", "list", "--json", "abc123")
	if !strings.Contains(out, `"file_name": "helper"`) || !strings.Contains(out, `"title": "Helper script"`) {
		t.Fatalf("list json missing fields, got:\n%s", out)
	}
	if !strings.Contains(out, `"mode_perm": 493`) { // 0755
		t.Fatalf("list json missing mode_perm, got:\n%s", out)
	}

	// Inspect managed store path
	database, err := db.Open(env.DBPath)
	if err != nil {
		t.Fatal(err)
	}
	attachments, err := database.ListEntryAttachments("abc123")
	if err != nil {
		t.Fatal(err)
	}
	_ = database.Close()
	if len(attachments) != 1 {
		t.Fatalf("attachments = %d, want 1", len(attachments))
	}
	att := attachments[0]
	stored := filepath.Join(env.AssetsPath, "entries", "abc123", "attachments", att.ID, "helper")
	info, err := os.Stat(stored)
	if err != nil {
		t.Fatalf("stored file missing via managed path: %v", err)
	}
	if info.Mode().Perm() != 0755 {
		t.Fatalf("stored perm = %o, want 755", info.Mode().Perm())
	}

	// Get shows metadata and managed path
	out = runCmdCapture(t, c, "kb", "entry", "attachment", "get", "abc123", att.ID)
	if !strings.Contains(out, "Stored Path: "+stored) {
		t.Fatalf("get output missing managed path, got:\n%s", out)
	}
	if !strings.Contains(out, "SHA256:") || !strings.Contains(out, "Mode: 755") {
		t.Fatalf("get output missing fields, got:\n%s", out)
	}

	// Update title only
	runCmdCapture(t, c, "kb", "entry", "attachment", "update", "--title", "Renamed helper", "abc123", att.ID)
	getDB := mustOpenTestDB(t, env)
	defer func() { _ = getDB.Close() }()
	got, err := getDB.GetEntryAttachment("abc123", att.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != "Renamed helper" {
		t.Fatalf("title = %q, want Renamed helper", got.Title)
	}
	if got.FileName != "helper" {
		t.Fatalf("file name changed on title update: %q", got.FileName)
	}

	// Replace file with a different name: old bytes removed, new bytes stored
	newFile := writeSourceFile(t, srcDir, "helper-v2", []byte("#!/bin/sh\necho hi v2\n"), 0700)
	out = runCmdCapture(t, c, "kb", "entry", "attachment", "update", "--file", newFile, "abc123", att.ID)
	if !strings.Contains(out, "Updated attachment") {
		t.Fatalf("update output missing confirmation, got:\n%s", out)
	}
	got, err = getDB.GetEntryAttachment("abc123", att.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.FileName != "helper-v2" {
		t.Fatalf("file name = %q, want helper-v2", got.FileName)
	}
	if got.ModePerm != 0700 {
		t.Fatalf("mode_perm = %o, want 700", got.ModePerm)
	}
	stored2 := filepath.Join(env.AssetsPath, "entries", "abc123", "attachments", att.ID, "helper-v2")
	if _, err := os.Stat(stored2); err != nil {
		t.Fatalf("replacement bytes missing: %v", err)
	}
	if _, err := os.Stat(stored); err == nil {
		t.Fatal("previous stored bytes still exist after replacement")
	}

	// Delete
	out = runCmdCapture(t, c, "kb", "entry", "attachment", "delete", "--force", "abc123", att.ID)
	if !strings.Contains(out, "Deleted attachment") {
		t.Fatalf("delete output missing confirmation, got:\n%s", out)
	}
	if _, err := os.Stat(filepath.Join(env.AssetsPath, "entries", "abc123", "attachments", att.ID)); !os.IsNotExist(err) {
		t.Fatalf("attachment store tree still exists: %v", err)
	}
}

func TestAttachmentAddValidation(t *testing.T) {
	env := seedAttachmentTestEnv(t)
	c := &Commands{}

	// Empty title
	if err := runCmdErr(c, "kb", "entry", "attachment", "add", "-t", "   ", "abc123", "whatever"); err == nil {
		t.Fatal("expected empty title error")
	}

	// Missing file argument
	if err := runCmdErr(c, "kb", "entry", "attachment", "add", "-t", "Title", "abc123"); err == nil {
		t.Fatal("expected missing path error")
	}

	// Directory rejected
	dir := t.TempDir()
	if err := runCmdErr(c, "kb", "entry", "attachment", "add", "-t", "Title", "abc123", dir); err == nil {
		t.Fatal("expected directory rejection")
	}

	// Symlink rejected and leaves no staged bytes
	target := writeSourceFile(t, dir, "real.txt", []byte("x"), 0644)
	link := filepath.Join(dir, "link.txt")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	before := mustOpenTestDB(t, env)
	count, err := before.AttachmentCount()
	if err != nil {
		t.Fatal(err)
	}
	_ = before.Close()
	if err := runCmdErr(c, "kb", "entry", "attachment", "add", "-t", "Title", "abc123", link); err == nil {
		t.Fatal("expected symlink rejection")
	}
	after := mustOpenTestDB(t, env)
	defer func() { _ = after.Close() }()
	count2, err := after.AttachmentCount()
	if err != nil {
		t.Fatal(err)
	}
	if count2 != count {
		t.Fatalf("attachment count changed on failed add: %d -> %d", count, count2)
	}
	// No orphaned storage trees under entries/
	entriesDir := filepath.Join(env.AssetsPath, "entries")
	if entries, err := os.ReadDir(entriesDir); err == nil && len(entries) != 0 {
		for _, e := range entries {
			atts := filepath.Join(entriesDir, e.Name(), "attachments")
			if children, err := os.ReadDir(atts); err == nil && len(children) != 0 {
				t.Fatalf("leaked staged attachments under %s: %v", atts, children)
			}
		}
	}

	// Unknown entry
	if err := runCmdErr(c, "kb", "entry", "attachment", "add", "-t", "Title", "zzz999", target); err == nil {
		t.Fatal("expected unknown entry error")
	}
}

func TestAttachmentUpdateValidation(t *testing.T) {
	env := seedAttachmentTestEnv(t)
	c := &Commands{}
	helper := writeSourceFile(t, t.TempDir(), "helper", []byte("x"), 0644)
	runCmdCapture(t, c, "kb", "entry", "attachment", "add", "-t", "Helper", "abc123", helper)

	db2 := mustOpenTestDB(t, env)
	atts, err := db2.ListEntryAttachments("abc123")
	if err != nil {
		t.Fatal(err)
	}
	_ = db2.Close()
	if len(atts) != 1 {
		t.Fatalf("attachments = %d, want 1", len(atts))
	}
	attID := atts[0].ID

	// Neither flag
	if err := runCmdErr(c, "kb", "entry", "attachment", "update", "abc123", attID); err == nil {
		t.Fatal("expected update-without-flags error")
	}
	// Replacement file that is a directory
	if err := runCmdErr(c, "kb", "entry", "attachment", "update", "--file", t.TempDir(), "abc123", attID); err == nil {
		t.Fatal("expected directory rejection on update")
	}
	// Unknown attachment
	if err := runCmdErr(c, "kb", "entry", "attachment", "update", "--title", "X", "abc123", "ffffff"); err == nil {
		t.Fatal("expected unknown attachment error")
	}
}

func TestAttachmentDeletePromptsWithoutForce(t *testing.T) {
	env := seedAttachmentTestEnv(t)
	c := &Commands{}
	helper := writeSourceFile(t, t.TempDir(), "helper", []byte("x"), 0644)
	runCmdCapture(t, c, "kb", "entry", "attachment", "add", "-t", "Helper", "abc123", helper)

	db2 := mustOpenTestDB(t, env)
	atts, err := db2.ListEntryAttachments("abc123")
	if err != nil {
		t.Fatal(err)
	}
	_ = db2.Close()
	if len(atts) != 1 {
		t.Fatal("seed attachment missing")
	}
	attID := atts[0].ID

	// Delete without --force still needs confirmation; run returns nil on "n"
	// from EOF, so the row must survive.
	if err := runCmdErr(c, "kb", "entry", "attachment", "delete", "abc123", attID); err != nil {
		t.Fatalf("delete without force errored: %v", err)
	}
	db3 := mustOpenTestDB(t, env)
	defer func() { _ = db3.Close() }()
	got, err := db3.GetEntryAttachment("abc123", attID)
	if err != nil {
		t.Fatalf("attachment deleted without confirmation: %v", err)
	}
	if got.ID != attID {
		t.Fatalf("unexpected attachment: %+v", got)
	}
}

func TestSearchFindsAttachmentOnlyEntry(t *testing.T) {
	env := seedAttachmentTestEnv(t)
	c := &Commands{}

	// Seed a second attachment-only entry (no articles)
	database := mustOpenTestDB(t, env)
	if err := database.AddEntry("def456", "Deploy helper", "ops"); err != nil {
		t.Fatal(err)
	}
	if err := database.AddEntryAttachment(db.EntryAttachment{
		ID:           "att00a",
		EntryID:      "def456",
		Title:        "Deploy runner executable",
		FileName:     "deploy-runner",
		OriginalPath: "/tmp/deploy-runner",
		StoreRelPath: "entries/def456/attachments/att00a/deploy-runner",
		SHA256:       "abc",
		SizeBytes:    2048,
		ModePerm:     0755,
	}); err != nil {
		t.Fatal(err)
	}
	_ = database.Close()

	// Compact output groups the attachment under its entry
	out := runCmdCapture(t, c, "kb", "search", "executable")
	if !strings.Contains(out, "ID: def456, Title: Deploy helper, Tags: ops") {
		t.Fatalf("missing attachment-only entry headline, got:\n%s", out)
	}
	if !strings.Contains(out, "Entry-Attachment(s):") {
		t.Fatalf("missing attachment section, got:\n%s", out)
	}
	if !strings.Contains(out, "Attachment-ID: att00a, Title: Deploy runner executable, File: deploy-runner") {
		t.Fatalf("missing attachment hit details, got:\n%s", out)
	}
	// --content must not emit an entry-content section without article hits
	if strings.Contains(out, "Entry-Content:") {
		t.Fatalf("attachment-only entry must not show content excerpt:\n%s", out)
	}

	// JSON output carries typed attachment hits without binary content
	out = runCmdCapture(t, c, "kb", "search", "--format", "json", "runner")
	if !strings.Contains(out, `"type": "attachment"`) {
		t.Fatalf("json missing type field, got:\n%s", out)
	}
	if !strings.Contains(out, `"file_name": "deploy-runner"`) {
		t.Fatalf("json missing file_name, got:\n%s", out)
	}
	if !strings.Contains(out, `"size_bytes": 2048`) {
		t.Fatalf("json missing size_bytes, got:\n%s", out)
	}
	if strings.Contains(out, `"type": "article"`) && !strings.Contains(out, "def456") {
		t.Fatalf("article hits unexpected, got:\n%s", out)
	}
}

func TestEntryViewsExposeAttachments(t *testing.T) {
	seedAttachmentTestEnv(t)
	c := &Commands{}
	helper := writeSourceFile(t, t.TempDir(), "helper", []byte("x"), 0755)
	runCmdCapture(t, c, "kb", "entry", "attachment", "add", "-t", "Helper", "abc123", helper)

	// entry get --attachments prints the section
	out := runCmdCapture(t, c, "kb", "entry", "get", "--attachments", "abc123")
	if !strings.Contains(out, "## Attachments") || !strings.Contains(out, "Helper (helper, 1 B") {
		t.Fatalf("get --attachments missing section, got:\n%s", out)
	}

	// JSON combines articles and attachments
	out = runCmdCapture(t, c, "kb", "entry", "get", "--articles", "--attachments", "--json", "abc123")
	if !strings.Contains(out, `"articles": [`) || !strings.Contains(out, `"attachments": [`) {
		t.Fatalf("get json missing arrays, got:\n%s", out)
	}
	if !strings.Contains(out, `"file_name": "helper"`) {
		t.Fatalf("get json missing attachment fields, got:\n%s", out)
	}

	// entry list shows the attachment count column
	out = runCmdCapture(t, c, "kb", "entry", "list")
	if !strings.Contains(out, "| Attachments |") {
		t.Fatalf("list missing attachment column, got:\n%s", out)
	}
	if !strings.Contains(out, "| abc123 | Linux helper | tools | 1 | 1 |") {
		t.Fatalf("list missing attachment count, got:\n%s", out)
	}
}

func TestEntryDeletionRemovesAttachmentBytesButArticleDeletionKeepsThem(t *testing.T) {
	env := seedAttachmentTestEnv(t)
	c := &Commands{}
	helper := writeSourceFile(t, t.TempDir(), "helper", []byte("x"), 0755)
	runCmdCapture(t, c, "kb", "entry", "attachment", "add", "-t", "Helper", "abc123", helper)

	db2 := mustOpenTestDB(t, env)
	atts, err := db2.ListEntryAttachments("abc123")
	if err != nil {
		t.Fatal(err)
	}
	_ = db2.Close()
	if len(atts) != 1 {
		t.Fatal("seed attachment missing")
	}
	attTree := filepath.Join(env.AssetsPath, "entries", "abc123")

	// Deleting the article must leave attachment bytes intact
	runCmdCapture(t, c, "kb", "entry", "article", "delete", "--force", "abc123", "abc123-111111")
	if _, err := os.Stat(attTree); err != nil {
		t.Fatalf("article deletion removed attachment tree: %v", err)
	}
	db3 := mustOpenTestDB(t, env)
	remaining, err := db3.ListEntryAttachments("abc123")
	if err != nil {
		t.Fatal(err)
	}
	_ = db3.Close()
	if len(remaining) != 1 {
		t.Fatalf("article deletion removed attachment row: %d", len(remaining))
	}

	// Deleting the entry removes the whole attachment tree
	runCmdCapture(t, c, "kb", "entry", "delete", "--force", "abc123")
	if _, err := os.Stat(attTree); !os.IsNotExist(err) {
		t.Fatalf("entry deletion left attachment tree: %v", err)
	}
}

func mustOpenTestDB(t *testing.T, env testEnv) *db.DB {
	t.Helper()
	database, err := db.Open(env.DBPath)
	if err != nil {
		t.Fatal(err)
	}
	return database
}
