package db

import (
	"path/filepath"
	"testing"
)

func TestArticleAssetCRUDAndStats(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "kb.db")
	database, err := Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	if err := database.Init(); err != nil {
		t.Fatal(err)
	}
	if err := database.AddEntry("entry1", "Entry", ""); err != nil {
		t.Fatal(err)
	}
	if err := database.AddArticle("entry1-art1", "entry1", "Article", "content"); err != nil {
		t.Fatal(err)
	}

	asset := ArticleAsset{
		ID:           "asset01",
		ArticleID:    "entry1-art1",
		LogicalPath:  "docs/spec.md",
		OriginalPath: "/tmp/spec.md",
		StoreRelPath: "entry1-art1/asset01/docs/spec.md",
		SHA256:       "abc123",
		SizeBytes:    42,
	}
	if err := database.AddArticleAsset(asset); err != nil {
		t.Fatal(err)
	}

	got, err := database.GetArticleAsset("entry1-art1", "asset01")
	if err != nil {
		t.Fatal(err)
	}
	if got.LogicalPath != asset.LogicalPath || got.StoreRelPath != asset.StoreRelPath {
		t.Fatalf("unexpected asset: %+v", got)
	}

	list, err := database.ListArticleAssets("entry1-art1")
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 {
		t.Fatalf("asset count = %d, want 1", len(list))
	}

	stats, err := database.Stats()
	if err != nil {
		t.Fatal(err)
	}
	if stats.TotalAssets != 1 {
		t.Fatalf("stats.TotalAssets = %d, want 1", stats.TotalAssets)
	}

	if err := database.DeleteArticleAsset("entry1-art1", "asset01"); err != nil {
		t.Fatal(err)
	}
	list, err = database.ListArticleAssets("entry1-art1")
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 0 {
		t.Fatalf("asset count after delete = %d, want 0", len(list))
	}
}
