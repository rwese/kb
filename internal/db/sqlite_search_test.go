package db

import (
	"path/filepath"
	"testing"
)

// Search ranking direction: SQLite FTS5 bm25() is negated (best hit = most
// negative raw score), so after merge scoring the best hit must rank first
// with Score closer to 1.0 and the worst hit closer to 0.0.
func TestSearchRankingDirection(t *testing.T) {
	database, err := Open(filepath.Join(t.TempDir(), "kb.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = database.Close() }()

	if err := database.Init(); err != nil {
		t.Fatal(err)
	}
	if err := database.AddEntry("e1", "Entry", ""); err != nil {
		t.Fatal(err)
	}
	// Strong match: rare term saturated in title and content.
	if err := database.AddArticle(
		"strong", "e1", "quuxly",
		"quuxly quuxly quuxly quuxly quuxly quuxly surrounding words",
	); err != nil {
		t.Fatal(err)
	}
	// Weak match: rare term once among unrelated text.
	if err := database.AddArticle(
		"weak", "e1", "unrelated subject entirely",
		"quuxly appears once here amid an unrelated passage about completely different subjects overall",
	); err != nil {
		t.Fatal(err)
	}

	results, err := database.Search("quuxly", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Fatalf("got %d hits, want 2: %+v", len(results), results)
	}
	if results[0].ID != "strong" {
		t.Fatalf("best hit must rank first, got %s: %+v", results[0].ID, results)
	}
	if results[0].Score <= results[1].Score {
		t.Fatalf("scores not descending: %f, %f", results[0].Score, results[1].Score)
	}
	if results[0].Score <= 0.5 || results[0].Score <= results[1].Score+0.1 {
		t.Fatalf("strong hit must score close to 1.0: %+v", results)
	}
	if results[1].Score >= 0.5 || results[1].Score < 0 {
		t.Fatalf("weak hit must score close to 0.0: %+v", results)
	}
}
