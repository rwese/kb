package cmd

import (
	"context"
	"fmt"

	"github.com/rwese/kb/internal/config"
	"github.com/rwese/kb/internal/db"
	"github.com/urfave/cli/v3"
)

func (c *Commands) list() *cli.Command {
	return &cli.Command{
		Name:  "list",
		Usage:   "List all entries and their articles",
		Flags: []cli.Flag{
			&cli.BoolFlag{Name: "json", Usage: "Output as JSON"},
			&cli.BoolFlag{Name: "articles", Usage: "Show article count per entry"},
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

			entries, err := database.ListEntriesWithDeleted(cmd.Bool("all"))
			if err != nil {
				return err
			}

			if len(entries) == 0 {
				fmt.Println("No entries found")
				return nil
			}

			if cmd.Bool("json") {
				type EntryJSON struct {
					db.Entry
					Articles []db.Article `json:"articles"`
				}
				var result []EntryJSON
				for _, e := range entries {
					articles, _ := database.GetArticles(e.ID)
					result = append(result, EntryJSON{Entry: e, Articles: articles})
				}
				return formatJSON(result)
			}

			// Markdown table as default
			fmt.Println("| ID | Title | Tags | Articles | Updated |")
			fmt.Println("|----|-------|------|----------|---------|")
			for _, e := range entries {
				articles, _ := database.GetArticles(e.ID)
				fmt.Printf("| %s | %s | %s | %d | %s |\n",
					e.ID, e.Title, e.Tags, len(articles), e.UpdatedAt)
			}

			return nil
		},
	}
}
