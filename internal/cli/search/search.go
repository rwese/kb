package search

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

func NewSearchCommand() *cli.Command {
	return &cli.Command{
		Name:  "search",
		Usage: "Search knowledgebase articles with weighted retrieval",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "context", Aliases: []string{"C"}, Usage: "Context text (weighted lower than prompt)"},
			&cli.StringFlag{Name: "context-file", Aliases: []string{"F"}, Usage: "Read context from file"},
			&cli.StringFlag{Name: "prompt", Aliases: []string{"p"}, Usage: "Final prompt (weighted higher)"},
			&cli.IntFlag{Name: "top-k", Aliases: []string{"k"}, Usage: "Number of results"},
			&cli.StringFlag{Name: "format", Aliases: []string{"o"}, Usage: "Output format", DefaultText: "markdown"},
			&cli.StringFlag{
				Name:    "algorithm",
				Usage:   "Ranking algorithm: auto, bm25, or hybrid (default from config ranker.algorithm)",
				Sources: cli.EnvVars("KB_ALGORITHM"),
			},
			&cli.FloatFlag{
				Name:    "bm25-weight",
				Usage:   "Hybrid weight of the normalized BM25 score (default from config ranker.bm25_weight)",
				Sources: cli.EnvVars("KB_BM25_WEIGHT"),
			},
			&cli.FloatFlag{
				Name:    "semantic-weight",
				Usage:   "Hybrid weight of the query cosine similarity (default from config ranker.semantic_weight)",
				Sources: cli.EnvVars("KB_SEMANTIC_WEIGHT"),
			},
			&cli.BoolFlag{Name: "all", Usage: "Include deleted entries"},
			&cli.BoolFlag{Name: "bm25-only", Usage: "Use BM25-only search (skip semantic)"},
			&cli.BoolFlag{Name: "content", Usage: "Include a content excerpt for the best-matching article of each entry"},
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
			defer func() { _ = database.Close() }()

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

			algorithm, bm25Weight, semanticWeight, explicitRanking, err := resolveRanking(cmd, cfg)
			if err != nil {
				return err
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

			// Hybrid re-ranking when the resolved algorithm asks for it.
			// An explicit hybrid request is a hard error on embedder
			// trouble; the auto default falls back to BM25 like before.
			if algorithm == search.AlgorithmHybrid {
				e := embed.NewEmbedder(cfg)
				queryEmb, embedErr := e.Embed(ctx, query)
				switch {
				case embedErr != nil:
					if explicitRanking {
						return fmt.Errorf("embed query: %w", embedErr)
					}
					fmt.Fprintf(os.Stderr, "Warning: query embedding failed, falling back to BM25: %v\n", embedErr)
				case queryEmb == nil:
					if explicitRanking {
						return fmt.Errorf("embed query: empty embedding")
					}
					fmt.Fprintln(os.Stderr, "Warning: empty query embedding, falling back to BM25")
				default:
					ranker := search.NewRanker(bm25Weight, semanticWeight)
					results = ranker.HybridSearch(ctx, results, database, queryEmb)
				}
			}

			// Trim to requested count
			if len(results) > topK {
				results = results[:topK]
			}

			return formatSearchResults(results, cmd.String("format"), cmd.Bool("bm25-only"), cmd.Bool("content"), cmd.Bool("full-content"))
		},
	}
}

// resolveRanking applies flag/env (via Sources) over config discovery:
// --bm25-only forces bm25, otherwise --algorithm wins over
// ranker.algorithm, defaulting to auto. Weight flags override
// ranker weights, which Discover already defaulted.
func resolveRanking(cmd *cli.Command, cfg *config.Config) (algorithm string, bm25Weight, semanticWeight float64, explicitRanking bool, err error) {
	algorithm = cmd.String("algorithm")
	if algorithm == "" {
		algorithm = cfg.Ranker.Algorithm
	}
	if !search.ValidAlgorithm(algorithm) {
		return "", 0, 0, false, fmt.Errorf("unknown ranking algorithm %q (want auto, bm25, or hybrid)", algorithm)
	}

	bm25Weight = cmd.Float("bm25-weight")
	if bm25Weight == 0 {
		bm25Weight = cfg.Ranker.BM25Weight
	}
	semanticWeight = cmd.Float("semantic-weight")
	if semanticWeight == 0 {
		semanticWeight = cfg.Ranker.SemanticWeight
	}
	if bm25Weight < 0 || semanticWeight < 0 {
		return "", 0, 0, false, fmt.Errorf("ranking weights must be non-negative")
	}

	if cmd.Bool("bm25-only") {
		algorithm = search.AlgorithmBM25
	}
	embedderAvailable := cfg.Embedder == "ollama"
	resolved, err := search.ResolveAlgorithm(algorithm, embedderAvailable)
	if err != nil {
		return "", 0, 0, false, err
	}
	if resolved == search.AlgorithmHybrid && !embedderAvailable {
		return "", 0, 0, false, fmt.Errorf("algorithm %q requires an embedder (config embedder: %s)", resolved, cfg.Embedder)
	}
	explicitRanking = cmd.String("algorithm") != "" || cmd.Bool("bm25-only")
	return resolved, bm25Weight, semanticWeight, explicitRanking, nil
}

func formatSearchResults(results []db.SearchResult, format string, bm25Only, content, fullContent bool) error {
	switch format {
	case "json":
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(results)
	case "simple":
		for _, r := range results {
			if r.Type == "attachment" {
				fmt.Printf("[%.2f] attachment %s: %s (file %s, entry %s)\n\n", r.Score, r.Title, r.FileName, r.EntryID, r.ID)
				continue
			}
			fmt.Printf("[%.2f] %s (entry %s)\n%s\n\n", r.Score, r.EntryTitle, r.EntryID, r.Content)
		}
	default: // markdown
		if fullContent {
			formatSearchResultsVerbose(results, bm25Only)
		} else {
			formatSearchResultsCompact(results, content)
		}
	}
	return nil
}

// formatSearchResultsCompact groups matches by entry: entry headline, the
// matching articles, matching attachments, and a truncated content excerpt
// from the best matching article.
func formatSearchResultsCompact(results []db.SearchResult, includeContent bool) {
	type entryGroup struct {
		entryID     string
		entryTitle  string
		entryTags   string
		articles    []db.SearchResult
		attachments []db.SearchResult
	}

	seen := make(map[string]int)
	var groups []entryGroup
	for _, r := range results {
		if i, ok := seen[r.EntryID]; ok {
			if r.Type == "attachment" {
				groups[i].attachments = append(groups[i].attachments, r)
			} else {
				groups[i].articles = append(groups[i].articles, r)
			}
			continue
		}
		seen[r.EntryID] = len(groups)
		g := entryGroup{
			entryID:    r.EntryID,
			entryTitle: r.EntryTitle,
			entryTags:  r.EntryTags,
		}
		if r.Type == "attachment" {
			g.attachments = []db.SearchResult{r}
		} else {
			g.articles = []db.SearchResult{r}
		}
		groups = append(groups, g)
	}

	for _, g := range groups {
		fmt.Printf("ID: %s, Title: %s", g.entryID, g.entryTitle)
		if g.entryTags != "" {
			fmt.Printf(", Tags: %s", g.entryTags)
		}
		fmt.Printf("\n\n")
		if len(g.articles) > 0 {
			fmt.Println("Entry-Article(s):")
			fmt.Printf("\n")
			for _, a := range g.articles {
				fmt.Printf("Article-ID: %s, Title: %s\n", a.ID, a.Title)
			}
		}
		if len(g.attachments) > 0 {
			fmt.Println("Entry-Attachment(s):")
			fmt.Printf("\n")
			for _, a := range g.attachments {
				fmt.Printf("Attachment-ID: %s, Title: %s, File: %s\n", a.ID, a.Title, a.FileName)
			}
		}
		if includeContent && len(g.articles) > 0 {
			fmt.Printf("\nEntry-Content:\n\n")
			excerpt, truncated := truncateContent(g.articles[0].Content, 10)
			fmt.Println(excerpt)
			if truncated {
				fmt.Printf("... output was truncated use `kb entry get %s` for full content.\n", g.entryID)
			}
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
		switch r.Type {
		case "attachment":
			fmt.Printf("- Type: attachment\n")
			fmt.Printf("- Attachment ID: %s\n", r.ID)
			fmt.Printf("- Attachment: %s\n", r.Title)
			fmt.Printf("- File: %s\n", r.FileName)
			fmt.Printf("- Size: %d bytes\n", r.SizeBytes)
		default:
			fmt.Printf("- Type: article\n")
			if r.Title != "" {
				fmt.Printf("- Article: %s\n", r.Title)
			}
			fmt.Printf("\n---\n\n%s\n\n", r.Content)
		}
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
