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

	exportPath, err := ExportEntry(entry, articles, filepath.Join(tmpDir, "out"), assetsRoot, false)
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
