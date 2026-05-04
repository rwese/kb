package cmd

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/rwese/kb/internal/db"
)

func TestDeleteEntryUsesIsolatedTempDatabase(t *testing.T) {
	env := setupTempKBTestEnv(t)

	database, err := db.Open(env.DBPath)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	if err := database.Init(); err != nil {
		t.Fatal(err)
	}

	entryIDs := []string{"entry01", "entry02"}
	articleIDs := []string{"entry01-art01", "entry02-art01"}
	for i, entryID := range entryIDs {
		if err := database.AddEntry(entryID, "title "+entryID, ""); err != nil {
			t.Fatal(err)
		}
		if err := database.AddArticle(articleIDs[i], entryID, "", "content"); err != nil {
			t.Fatal(err)
		}
		assetDir := filepath.Join(env.AssetsPath, articleIDs[i], "asset01")
		if err := os.MkdirAll(assetDir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(assetDir, "note.txt"), []byte("asset"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	commands := &Commands{}
	if err := commands.Run(context.Background(), []string{"kb", "delete", "entry", "entry01", "entry02", "--force"}); err != nil {
		t.Fatalf("delete command failed: %v", err)
	}

	entryCount, err := database.Count()
	if err != nil {
		t.Fatal(err)
	}
	if entryCount != 0 {
		t.Fatalf("entry count = %d, want 0", entryCount)
	}

	articleCount, err := database.ArticleCount()
	if err != nil {
		t.Fatal(err)
	}
	if articleCount != 0 {
		t.Fatalf("article count = %d, want 0", articleCount)
	}

	for _, articleID := range articleIDs {
		if _, err := os.Stat(filepath.Join(env.AssetsPath, articleID)); !os.IsNotExist(err) {
			t.Fatalf("asset tree for %s still exists or errored: %v", articleID, err)
		}
	}
}
