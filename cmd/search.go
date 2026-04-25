package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/rwese/kb/internal/config"
	"github.com/rwese/kb/internal/db"
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
			&cli.StringFlag{Name: "format", Aliases: []string{"o"}, Usage: "Output format", DefaultText: "text"},
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
			var topK int

			// Determine query
			if p := cmd.String("prompt"); p != "" {
				query = p
			} else if args.Len() > 0 {
				query = args.First()
			} else {
				return fmt.Errorf("query required")
			}

			// Get top-k
			if k := cmd.Int("top-k"); k > 0 {
				topK = k
			} else {
				topK = cfg.TopK
			}

			// Perform search
			results, err := database.Search(query, topK)
			if err != nil {
				return err
			}

			if len(results) == 0 {
				fmt.Println("No results found")
				return nil
			}

			// Format output
			format := cmd.String("format")
			return formatSearchResults(results, format)
		},
	}
}

func formatSearchResults(results []db.SearchResult, format string) error {
	switch format {
	case "json":
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(results)
	case "simple":
		for _, r := range results {
			fmt.Printf("[%.2f] %s (entry #%d)\n%s\n\n", r.Score, r.EntryTitle, r.EntryID, r.Content)
		}
	default: // text
		for i, r := range results {
			fmt.Printf("--- Result #%d [score: %s] ---\n", i+1, formatScore(r.Score))
			fmt.Printf("Entry: %s (#%d)\n", r.EntryTitle, r.EntryID)
			if r.Title != "" {
				fmt.Printf("Article title: %s\n", r.Title)
			}
			fmt.Printf("\n%s\n\n", r.Content)
		}
	}
	return nil
}

func formatScore(s float64) string {
	return fmt.Sprintf("%.2f", s)
}

func formatJSON(v interface{}) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}
