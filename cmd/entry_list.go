package cmd

import (
	"context"
	"fmt"

	"github.com/rwese/kb/internal/config"
	"github.com/rwese/kb/internal/db"
	"github.com/urfave/cli/v3"
)

func (c *Commands) entryList() *cli.Command {
	return &cli.Command{
		Name:  "list",
		Usage: "List all entries",
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
			if err := database.Init(); err != nil {
				return err
			}

			entries, err := database.ListEntriesWithDeleted(cmd.Bool("all"))
			if err != nil {
				return err
			}

			if len(entries) == 0 {
				fmt.Println("No entries found")
				return nil
			}

			if cmd.Bool("json") {
				var result []entryWithArticleViews
				for _, e := range entries {
					articles, _ := database.GetArticles(e.ID)
					views, err := loadArticleViews(database, articles)
					if err != nil {
						return err
					}
					result = append(result, entryWithArticleViews{Entry: e, Articles: views})
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
