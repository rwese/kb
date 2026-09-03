package assets

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"testing"

	"github.com/rwese/kb/internal/db"
)

func TestValidateAttachmentSourceRejectsNonRegularFiles(t *testing.T) {
	tmpDir := t.TempDir()

	// Directory
	if _, err := ValidateAttachmentSource(tmpDir); err == nil {
		t.Fatal("expected directory rejection")
	}

	// Symlink
	target := filepath.Join(tmpDir, "target")
	if err := os.WriteFile(target, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(tmpDir, "link")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	if _, err := ValidateAttachmentSource(link); err == nil {
		t.Fatal("expected symlink rejection")
	}

	// Named pipe (best-effort; mkfifo may be unavailable in CI sandboxes)
	if runtime.GOOS == "linux" || runtime.GOOS == "darwin" {
		fifo := filepath.Join(tmpDir, "fifo")
		if err := syscall.Mkfifo(fifo, 0644); err == nil {
			defer func() { _ = os.Remove(fifo) }()
			if _, err := ValidateAttachmentSource(fifo); err == nil {
				t.Fatal("expected fifo rejection")
			}
		}
	}
}

func TestValidateAttachmentSourceAcceptsRegularFile(t *testing.T) {
	tmpDir := t.TempDir()
	file := filepath.Join(tmpDir, "helper")
	if err := os.WriteFile(file, []byte("#!/bin/sh\n"), 0755); err != nil {
		t.Fatal(err)
	}
	info, err := ValidateAttachmentSource(file)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0755 {
		t.Fatalf("perm = %o, want 755", info.Mode().Perm())
	}
}

func TestStageAttachmentPreservesModeAndBytes(t *testing.T) {
	tmpDir := t.TempDir()
	source := filepath.Join(tmpDir, "helper")
	content := []byte("#!/bin/sh\necho hello\n")
	if err := os.WriteFile(source, content, 0755); err != nil {
		t.Fatal(err)
	}

	assetsRoot := filepath.Join(tmpDir, "assets")
	staged, err := StageAttachment(assetsRoot, "entry01", "att001", source)
	if err != nil {
		t.Fatal(err)
	}
	if staged.FileName != "helper" {
		t.Fatalf("file name = %q, want helper", staged.FileName)
	}
	if staged.StoreRelPath != "entries/entry01/attachments/att001/helper" {
		t.Fatalf("store rel path = %q", staged.StoreRelPath)
	}
	if staged.ModePerm != 0755 {
		t.Fatalf("mode_perm = %o, want 755", staged.ModePerm)
	}
	if staged.SizeBytes != int64(len(content)) {
		t.Fatalf("size = %d, want %d", staged.SizeBytes, len(content))
	}

	sum := sha256.Sum256(content)
	if staged.SHA256 != hex.EncodeToString(sum[:]) {
		t.Fatalf("sha mismatch: %s", staged.SHA256)
	}

	stored := filepath.Join(assetsRoot, "entries", "entry01", "attachments", "att001", "helper")
	info, err := os.Stat(stored)
	if err != nil {
		t.Fatalf("stored file missing: %v", err)
	}
	if info.Mode().Perm() != 0755 {
		t.Fatalf("stored perm = %o, want 755", info.Mode().Perm())
	}
}

func TestStageAttachmentRejectsTitlesAndCleanup(t *testing.T) {
	tmpDir := t.TempDir()
	assetsRoot := filepath.Join(tmpDir, "assets")

	// Staging then cleanup leaves no tree behind
	file := filepath.Join(tmpDir, "f.txt")
	if err := os.WriteFile(file, []byte("data"), 0644); err != nil {
		t.Fatal(err)
	}

	// symlink rejection before any bytes are copied
	target := filepath.Join(tmpDir, "target")
	if err := os.WriteFile(target, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(tmpDir, "link")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	if _, err := StageAttachment(assetsRoot, "entry01", "att001", link); err == nil {
		t.Fatal("expected symlink staging rejection")
	}

	staged, err := StageAttachment(assetsRoot, "entry01", "att001", file)
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(assetsRoot, "entries", "entry01", "attachments", "att001")
	if _, err := os.Stat(dir); err != nil {
		t.Fatalf("staged dir missing: %v", err)
	}
	CleanupStagedAttachment(assetsRoot, "entry01", "att001")
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("staged dir still exists after cleanup: %v", err)
	}
	_ = staged
}

func TestExportAttachmentFilePreservesMode(t *testing.T) {
	tmpDir := t.TempDir()
	att := db.EntryAttachment{
		ID:           "att001",
		EntryID:      "entry01",
		FileName:     "helper",
		StoreRelPath: "entries/entry01/attachments/att001/helper",
		ModePerm:     0755,
	}
	stored := filepath.Join(tmpDir, "store", "entries", "entry01", "attachments", "att001", "helper")
	if err := os.MkdirAll(filepath.Dir(stored), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(stored, []byte("#!/bin/sh\n"), 0755); err != nil {
		t.Fatal(err)
	}

	outputRoot := filepath.Join(tmpDir, "out")
	if err := ExportAttachmentFile(filepath.Join(tmpDir, "store"), outputRoot, att); err != nil {
		t.Fatal(err)
	}
	exported := filepath.Join(outputRoot, "attachments", "att001", "helper")
	info, err := os.Stat(exported)
	if err != nil {
		t.Fatalf("exported file missing: %v", err)
	}
	if info.Mode().Perm() != 0755 {
		t.Fatalf("exported perm = %o, want 755", info.Mode().Perm())
	}
}

func TestRemoveEntryAttachmentsTree(t *testing.T) {
	tmpDir := t.TempDir()
	tree := filepath.Join(tmpDir, "entries", "entry01")
	if err := os.MkdirAll(filepath.Join(tree, "attachments", "att001"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tree, "attachments", "att001", "f"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := RemoveEntryAttachmentsTree(tmpDir, "entry01"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(tree); !os.IsNotExist(err) {
		t.Fatalf("entry attachment tree still exists: %v", err)
	}
}
