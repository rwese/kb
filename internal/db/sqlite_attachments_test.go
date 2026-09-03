package db

import (
	"path/filepath"
	"testing"
	"time"
)

func TestEntryAttachmentCRUDAndStats(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "kb.db")
	database, err := Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = database.Close() }()

	if err := database.Init(); err != nil {
		t.Fatal(err)
	}
	if err := database.AddEntry("entry1", "Entry", ""); err != nil {
		t.Fatal(err)
	}

	att := EntryAttachment{
		ID:           "att001",
		EntryID:      "entry1",
		Title:        "Linux helper executable",
		FileName:     "helper",
		OriginalPath: "/tmp/helper",
		StoreRelPath: "entries/entry1/attachments/att001/helper",
		SHA256:       "abc123",
		SizeBytes:    42,
		ModePerm:     0755,
	}
	if err := database.AddEntryAttachment(att); err != nil {
		t.Fatal(err)
	}

	got, err := database.GetEntryAttachment("entry1", "att001")
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != att.Title || got.StoreRelPath != att.StoreRelPath {
		t.Fatalf("unexpected attachment: %+v", got)
	}
	if got.ModePerm != 0755 {
		t.Fatalf("mode_perm = %o, want 755", got.ModePerm)
	}

	list, err := database.ListEntryAttachments("entry1")
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 {
		t.Fatalf("attachment count = %d, want 1", len(list))
	}

	// Entry updated_at must change on attachment mutation (1s timestamp resolution)
	before, err := database.GetEntry("entry1")
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(1100 * time.Millisecond)
	beforeTitle, _ := database.GetEntryAttachment("entry1", "att001")
	beforeTitle.Title = "Renamed helper"
	if err := database.UpdateEntryAttachment(*beforeTitle); err != nil {
		t.Fatal(err)
	}
	after, err := database.GetEntry("entry1")
	if err != nil {
		t.Fatal(err)
	}
	if before.UpdatedAt == after.UpdatedAt {
		t.Fatalf("entry updated_at unchanged after attachment title update")
	}

	replaced := EntryAttachment{
		ID:           "att001",
		EntryID:      "entry1",
		Title:        "Renamed helper",
		FileName:     "helper2",
		OriginalPath: "/tmp/helper2",
		StoreRelPath: "entries/entry1/attachments/att001/helper2",
		SHA256:       "def456",
		SizeBytes:    99,
		ModePerm:     0750,
	}
	if err := database.UpdateEntryAttachment(replaced); err != nil {
		t.Fatal(err)
	}
	got, err = database.GetEntryAttachment("entry1", "att001")
	if err != nil {
		t.Fatal(err)
	}
	if got.FileName != "helper2" || got.SHA256 != "def456" || got.ModePerm != 0750 {
		t.Fatalf("file replacement not applied: %+v", got)
	}

	stats, err := database.Stats()
	if err != nil {
		t.Fatal(err)
	}
	if stats.TotalAttachments != 1 {
		t.Fatalf("stats.TotalAttachments = %d, want 1", stats.TotalAttachments)
	}

	if err := database.DeleteEntryAttachment("entry1", "att001"); err != nil {
		t.Fatal(err)
	}
	list, err = database.ListEntryAttachments("entry1")
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 0 {
		t.Fatalf("attachment count after delete = %d, want 0", len(list))
	}
	if _, err := database.GetEntryAttachment("entry1", "att001"); err != ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestEntryAttachmentEntryDeletionCascades(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "kb.db")
	database, err := Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = database.Close() }()

	if err := database.Init(); err != nil {
		t.Fatal(err)
	}
	if err := database.AddEntry("entry1", "Entry", ""); err != nil {
		t.Fatal(err)
	}
	att := EntryAttachment{
		ID:           "att001",
		EntryID:      "entry1",
		Title:        "t",
		FileName:     "f",
		OriginalPath: "/tmp/f",
		StoreRelPath: "entries/entry1/attachments/att001/f",
		SHA256:       "abc",
		SizeBytes:    1,
	}
	if err := database.AddEntryAttachment(att); err != nil {
		t.Fatal(err)
	}

	if err := database.DeleteEntry("entry1"); err != nil {
		t.Fatal(err)
	}
	list, err := database.ListEntryAttachments("entry1")
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 0 {
		t.Fatalf("attachments survived entry deletion: %d", len(list))
	}
	count, err := database.AttachmentCount()
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("attachment count = %d, want 0", count)
	}
}

func TestSearchFindsAttachmentsByTitleOrFileName(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "kb.db")
	database, err := Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = database.Close() }()

	if err := database.Init(); err != nil {
		t.Fatal(err)
	}
	if err := database.AddEntry("entry1", "Linux helper", "tools"); err != nil {
		t.Fatal(err)
	}
	if err := database.AddArticle("entry1-art1", "entry1", "Notes", "helper details"); err != nil {
		t.Fatal(err)
	}
	if err := database.AddEntryAttachment(EntryAttachment{
		ID:           "att001",
		EntryID:      "entry1",
		Title:        "Linux helper executable",
		FileName:     "helper",
		OriginalPath: "/tmp/helper",
		StoreRelPath: "entries/entry1/attachments/att001/helper",
		SHA256:       "abc",
		SizeBytes:    1,
		ModePerm:     0755,
	}); err != nil {
		t.Fatal(err)
	}

	// Match on attachment title
	results, err := database.Search("executable", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("attachment title search returned %d results, want 1: %+v", len(results), results)
	}
	if results[0].Type != "attachment" {
		t.Fatalf("type = %q, want attachment", results[0].Type)
	}
	if results[0].ID != "att001" || results[0].FileName != "helper" {
		t.Fatalf("unexpected attachment hit: %+v", results[0])
	}

	// Match on filename
	results, err = database.Search("helper", 10)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, r := range results {
		if r.Type == "attachment" && r.ID == "att001" {
			found = true
		}
	}
	if !found {
		t.Fatalf("filename search missed attachment hit: %+v", results)
	}

	// Deletion removes the FTS entry
	if err := database.DeleteEntryAttachment("entry1", "att001"); err != nil {
		t.Fatal(err)
	}
	results, err = database.Search("executable", 10)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range results {
		if r.Type == "attachment" {
			t.Fatalf("attachment hit after delete: %+v", r)
		}
	}
}
