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
			&cli.StringFlag{Name: "format", Aliases: []string{"o"}, Usage: "Output format", DefaultText: "markdown"},
			&cli.BoolFlag{Name: "all", Usage: "Include deleted entries"},
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

			results, err := database.SearchWithDeleted(query, topK, cmd.Bool("all"))
			if err != nil {
				return err
			}

			if len(results) == 0 {
				fmt.Println("No results found")
				return nil
			}

			return formatSearchResults(results, cmd.String("format"))
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
			fmt.Printf("[%.2f] %s (entry %s)\n%s\n\n", r.Score, r.EntryTitle, r.EntryID, r.Content)
		}
	default: // markdown
		fmt.Printf("## Search Results (%d found)\n\n", len(results))
		for i, r := range results {
			fmt.Printf("### Result #%d\n\n", i+1)
			fmt.Printf("| Property | Value |\n|---------|-------|\n")
			fmt.Printf("| Entry | [%s](%s) |\n", r.EntryTitle, r.EntryID)
			fmt.Printf("| Entry ID | %s |\n", r.EntryID)
			fmt.Printf("| Score | %.2f |\n", r.Score)
			if r.Title != "" {
				fmt.Printf("| Article Title | %s |\n", r.Title)
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
