package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/rwese/kb/internal/config"
	"github.com/rwese/kb/internal/db"
	"github.com/rwese/kb/internal/embed"
	"github.com/rwese/kb/internal/search"
	"github.com/urfave/cli/v3"
)

func (c *Commands) search() *cli.Command {
	return &cli.Command{
		Name:  "search",
		Usage: "Search knowledgebase articles with weighted retrieval",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "context", Aliases: []string{"C"}, Usage: "Context text (weighted lower than prompt)"},
			&cli.StringFlag{Name: "context-file", Aliases: []string{"F"}, Usage: "Read context from file"},
			&cli.StringFlag{Name: "prompt", Aliases: []string{"p"}, Usage: "Final prompt (weighted higher)"},
			&cli.IntFlag{Name: "top-k", Aliases: []string{"k"}, Usage: "Number of results"},
			&cli.StringFlag{Name: "format", Aliases: []string{"o"}, Usage: "Output format", DefaultText: "markdown"},
			&cli.BoolFlag{Name: "all", Usage: "Include deleted entries"},
			&cli.BoolFlag{Name: "bm25-only", Usage: "Use BM25-only search (skip semantic)"},
			&cli.BoolFlag{Name: "full-content", Usage: "Show full result details (previous verbose format)"},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			cfg, err := config.Discover()
			if err != nil {
				return err
			}

			database, err := db.Open(cfg.DBPath)
			if err != nil {
				return err
			}
			defer database.Close()

			args := cmd.Args()
			var query string

			if p := cmd.String("prompt"); p != "" {
				query = p
			} else if args.Len() > 0 {
				query = args.First()
			} else {
				return fmt.Errorf("query required")
			}

			topK := cmd.Int("top-k")
			if topK == 0 {
				topK = cfg.TopK
			}

			// Get BM25 results
			results, err := database.SearchWithDeleted(query, topK*2, cmd.Bool("all"))
			if err != nil {
				return err
			}

			if len(results) == 0 {
				fmt.Println("No results found")
				return nil
			}

			// Try hybrid search if ollama embeddings are available
			if !cmd.Bool("bm25-only") && cfg.Embedder == "ollama" {
				e := embed.NewEmbedder(cfg)

				// Compute query embedding
				queryEmb, err := e.Embed(ctx, query)
				if err == nil && queryEmb != nil {
					// Apply hybrid ranking
					ranker := search.DefaultRanker()
					results = ranker.HybridSearch(ctx, results, database, queryEmb)
				}
			}

			// Trim to requested count
			if len(results) > topK {
				results = results[:topK]
			}

			return formatSearchResults(results, cmd.String("format"), cmd.Bool("bm25-only"), cmd.Bool("full-content"))
		},
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func formatSearchResults(results []db.SearchResult, format string, bm25Only, fullContent bool) error {
	switch format {
	case "json":
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(results)
	case "simple":
		for _, r := range results {
			fmt.Printf("[%.2f] %s (entry %s)\n%s\n\n", r.Score, r.EntryTitle, r.EntryID, r.Content)
		}
	default: // markdown
		if fullContent {
			formatSearchResultsVerbose(results, bm25Only)
		} else {
			formatSearchResultsCompact(results)
		}
	}
	return nil
}

// formatSearchResultsCompact groups matches by entry: entry headline, the
// matching articles, and a truncated content excerpt from the best match.
func formatSearchResultsCompact(results []db.SearchResult) {
	type entryGroup struct {
		entryID    string
		entryTitle string
		entryTags  string
		articles   []db.SearchResult
	}

	seen := make(map[string]int)
	var groups []entryGroup
	for _, r := range results {
		if i, ok := seen[r.EntryID]; ok {
			groups[i].articles = append(groups[i].articles, r)
			continue
		}
		seen[r.EntryID] = len(groups)
		groups = append(groups, entryGroup{
			entryID:    r.EntryID,
			entryTitle: r.EntryTitle,
			entryTags:  r.EntryTags,
			articles:   []db.SearchResult{r},
		})
	}

	for _, g := range groups {
		fmt.Printf("ID: %s, Title: %s", g.entryID, g.entryTitle)
		if g.entryTags != "" {
			fmt.Printf(", Tags: %s", g.entryTags)
		}
		fmt.Printf("\n\n")
		fmt.Println("Entry-Article(s):")
		fmt.Printf("\n")
		for _, a := range g.articles {
			fmt.Printf("Article-ID: %s, Title: %s\n", a.ID, a.Title)
		}
		fmt.Printf("\nEntry-Content:\n\n")
		excerpt, truncated := truncateContent(g.articles[0].Content, 10)
		fmt.Println(excerpt)
		if truncated {
			fmt.Printf("... output was truncated use `kb entry get %s` for full content.\n", g.entryID)
		}
		fmt.Println()
	}
}

// formatSearchResultsVerbose is the previous markdown output: full result
// details including scores and complete article content.
func formatSearchResultsVerbose(results []db.SearchResult, bm25Only bool) {
	fmt.Printf("## Search Results (%d found)\n\n", len(results))
	if !bm25Only {
		fmt.Println("*Using hybrid BM25 + semantic search*")
	}
	for i, r := range results {
		fmt.Printf("### Result #%d\n\n", i+1)
		fmt.Printf("- Entry: [%s](%s)\n", r.EntryTitle, r.EntryID)
		fmt.Printf("- Entry ID: %s\n", r.EntryID)
		fmt.Printf("- Score: %.3f\n", r.Score)
		if r.BM25Score > 0 || r.SemanticScore > 0 {
			fmt.Printf("  (BM25: %.2f + Semantic: %.2f)\n", r.BM25Score, r.SemanticScore)
		}
		if r.Title != "" {
			fmt.Printf("- Article: %s\n", r.Title)
		}
		fmt.Printf("\n---\n\n%s\n\n", r.Content)
	}
}

// truncateContent returns the first maxLines non-empty-trailing lines and
// whether content was cut off.
func truncateContent(content string, maxLines int) (string, bool) {
	lines := strings.Split(content, "\n")
	for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
		lines = lines[:len(lines)-1]
	}
	if len(lines) <= maxLines {
		return strings.Join(lines, "\n"), false
	}
	return strings.Join(lines[:maxLines], "\n"), true
}

func formatJSON(v interface{}) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}
