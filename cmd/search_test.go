package cmd

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"

	"github.com/rwese/kb/internal/config"
	"github.com/rwese/kb/internal/db"
)

// runCmdCapture runs a kb command and returns everything written to stdout.
func runCmdCapture(t *testing.T, c *Commands, args ...string) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w

	runErr := c.Run(context.Background(), args)

	w.Close()
	os.Stdout = old

	if runErr != nil {
		t.Fatalf("Run(%v): %v", args, runErr)
	}
	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	_ = r.Close()
	return buf.String()
}

// seedSearchTestEnv creates two entries with articles via the db directly,
// so ids are deterministic.
func seedSearchTestEnv(t *testing.T) {
	t.Helper()

	cfg, err := config.Discover()
	if err != nil {
		t.Fatalf("config: %v", err)
	}
	database, err := db.Open(cfg.DBPath)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if err := database.Init(); err != nil {
		t.Fatal(err)
	}

	longContent := strings.Join([]string{
		"l1 items flickering when scrolling",
		"l2", "l3", "l4", "l5", "l6", "l7", "l8", "l9", "l10", "l11", "l12 beyond",
	}, "\n")

	if err := database.AddEntry("abc123", "Bug: flickering list", "ui,bug"); err != nil {
		t.Fatal(err)
	}
	if err := database.AddArticle("abc123-111111", "abc123", "", longContent); err != nil {
		t.Fatal(err)
	}
	if err := database.AddArticle("abc123-222222", "abc123", "follow-up", "flickering verified fix"); err != nil {
		t.Fatal(err)
	}

	if err := database.AddEntry("def456", "Hydration bug", "react"); err != nil {
		t.Fatal(err)
	}
	if err := database.AddArticle("def456-111111", "def456", "", "flashing hydration mismatch\nsecond line\nthird line"); err != nil {
		t.Fatal(err)
	}
}

func TestSearchCompactFormat(t *testing.T) {
	env := setupTempKBTestEnv(t)
	_ = env
	seedSearchTestEnv(t)

	c := &Commands{}
	out := runCmdCapture(t, c, "kb", "search", "flickering")

	// Entry headline with tags
	if !strings.Contains(out, "ID: abc123, Title: Bug: flickering list, Tags: ui,bug") {
		t.Fatalf("missing entry headline, got:\n%s", out)
	}
	// Both matching articles listed; no Tags on article lines
	if !strings.Contains(out, "Article-ID: abc123-111111, Title:") {
		t.Fatalf("missing first article, got:\n%s", out)
	}
	if !strings.Contains(out, "Article-ID: abc123-222222, Title: follow-up") {
		t.Fatalf("missing second article, got:\n%s", out)
	}
	if strings.Contains(out, "Title: follow-up, Tags:") {
		t.Fatalf("article lines must not carry Tags:\n%s", out)
	}
	// No verbose artifacts
	if strings.Contains(out, "### Result #") {
		t.Fatalf("compact format must not use verbose headers:\n%s", out)
	}

	// Truncation: query a token only the 12-line article contains
	out2 := runCmdCapture(t, c, "kb", "search", "beyond")
	if !strings.Contains(out2, "Article-ID: abc123-111111, Title:") {
		t.Fatalf("long article must be the only match, got:\n%s", out2)
	}
	if !strings.Contains(out2, "l10") {
		t.Fatalf("missing 10th line, got:\n%s", out2)
	}
	if strings.Contains(out2, "l11") {
		t.Fatalf("11th line must be truncated:\n%s", out2)
	}
	if !strings.Contains(out2, "... output was truncated use `kb entry get abc123` for full content.") {
		t.Fatalf("missing truncation hint, got:\n%s", out2)
	}

	// Short content is not truncated
	out3 := runCmdCapture(t, c, "kb", "search", "hydration")
	if !strings.Contains(out3, "ID: def456, Title: Hydration bug, Tags: react") {
		t.Fatalf("missing second entry headline, got:\n%s", out3)
	}
	if strings.Contains(out3, "output was truncated") {
		t.Fatalf("short content must not be truncated:\n%s", out3)
	}
}

func TestSearchFullContentFlag(t *testing.T) {
	env := setupTempKBTestEnv(t)
	_ = env
	seedSearchTestEnv(t)

	c := &Commands{}
	out := runCmdCapture(t, c, "kb", "search", "--full-content", "flickering")

	if !strings.Contains(out, "### Result #1") {
		t.Fatalf("full-content must keep verbose format, got:\n%s", out)
	}
	if !strings.Contains(out, "- Entry: [Bug: flickering list](abc123)") {
		t.Fatalf("missing entry link, got:\n%s", out)
	}
	if !strings.Contains(out, "l12 beyond") {
		t.Fatalf("full-content must show uncut body, got:\n%s", out)
	}
	if strings.Contains(out, "Entry-Article(s):") {
		t.Fatalf("full-content must not use compact sections:\n%s", out)
	}
}
