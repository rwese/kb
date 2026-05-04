package assets

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/rwese/kb/internal/db"
)

func TestExpandPathsAndStageImportsPreserveLogicalPaths(t *testing.T) {
	tmpDir := t.TempDir()
	srcDir := filepath.Join(tmpDir, "src")
	if err := os.MkdirAll(filepath.Join(srcDir, "nested"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "root.txt"), []byte("root"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "nested", "child.txt"), []byte("child"), 0644); err != nil {
		t.Fatal(err)
	}

	files, err := ExpandPaths([]string{srcDir})
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 2 {
		t.Fatalf("len(files) = %d, want 2", len(files))
	}
	if files[0].LogicalPath != "nested/child.txt" || files[1].LogicalPath != "root.txt" {
		t.Fatalf("unexpected logical paths: %+v", files)
	}

	assetsRoot := filepath.Join(tmpDir, "assets")
	staged, err := StageImports(assetsRoot, "entry-art", files)
	if err != nil {
		t.Fatal(err)
	}
	if len(staged) != 2 {
		t.Fatalf("len(staged) = %d, want 2", len(staged))
	}
	for _, asset := range staged {
		if _, err := os.Stat(filepath.Join(assetsRoot, filepath.FromSlash(asset.StoreRelPath))); err != nil {
			t.Fatalf("stored asset missing for %s: %v", asset.LogicalPath, err)
		}
		if asset.SizeBytes == 0 {
			t.Fatalf("size not recorded for %s", asset.LogicalPath)
		}
	}
}

func TestExpandPathsRejectsDuplicateLogicalPaths(t *testing.T) {
	tmpDir := t.TempDir()
	first := filepath.Join(tmpDir, "a.txt")
	secondDir := filepath.Join(tmpDir, "dir")
	if err := os.WriteFile(first, []byte("a"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(secondDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(secondDir, "a.txt"), []byte("b"), 0644); err != nil {
		t.Fatal(err)
	}

	if _, err := ExpandPaths([]string{first, secondDir}); err == nil {
		t.Fatal("expected duplicate logical path error")
	}
}

func TestRemoveAssetTree(t *testing.T) {
	tmpDir := t.TempDir()
	asset := db.ArticleAsset{
		ID:        "asset01",
		ArticleID: "entry-art",
	}
	assetDir := filepath.Join(tmpDir, asset.ArticleID, asset.ID)
	if err := os.MkdirAll(filepath.Join(assetDir, "nested"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(assetDir, "nested", "file.txt"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := RemoveAssetTree(tmpDir, asset); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(assetDir); !os.IsNotExist(err) {
		t.Fatalf("asset dir still exists: %v", err)
	}
}
