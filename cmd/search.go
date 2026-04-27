package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

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

			// Try hybrid search if embeddings are available
			if !cmd.Bool("bm25-only") && (cfg.Embedder == "local" || cfg.Embedder == "ollama") {
				e := embed.NewEmbedder(cfg)

				// Check if local embedder is ready
				if cfg.Embedder == "local" {
					le, ok := e.(*embed.LocalEmbedder)
					if ok && !le.IsAvailable() {
						// Local embedder not ready, skip semantic search
						return formatSearchResults(results[:min(len(results), topK)], cmd.String("format"), cmd.Bool("bm25-only"))
					}
				}

				// Compute query embedding
				queryEmb, err := e.Embed(ctx, query)
				if err == nil && queryEmb != nil {
					// Apply hybrid ranking
					bm25Weight := cfg.Local.BM25Weight
					semanticWeight := cfg.Local.SemanticWeight

					// Ensure weights sum to 1
					total := bm25Weight + semanticWeight
					if total > 0 {
						bm25Weight /= total
						semanticWeight /= total
					}

					ranker := search.NewRanker(bm25Weight, semanticWeight)
					results = ranker.HybridSearch(ctx, results, database, queryEmb)
				}
			}

			// Trim to requested count
			if len(results) > topK {
				results = results[:topK]
			}

			return formatSearchResults(results, cmd.String("format"), cmd.Bool("bm25-only"))
		},
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func formatSearchResults(results []db.SearchResult, format string, bm25Only bool) error {
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
	return nil
}

func formatJSON(v interface{}) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}
