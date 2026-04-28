package cmd

import (
	"context"
	"fmt"

	"github.com/rwese/kb/internal/config"
	"github.com/rwese/kb/internal/db"
	"github.com/urfave/cli/v3"
)

func (c *Commands) entryArticleList() *cli.Command {
	return &cli.Command{
		Name:      "list",
		Usage:     "List articles in an entry",
		ArgsUsage: "<entry-id>",
		Flags: []cli.Flag{
			&cli.BoolFlag{Name: "json", Usage: "Output as JSON"},
			&cli.BoolFlag{Name: "all", Usage: "Include deleted articles"},
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

			entryID := cmd.Args().First()
			if entryID == "" {
				return fmt.Errorf("entry ID required")
			}

			// Verify entry exists
			_, err = database.GetEntry(entryID)
			if err != nil {
				return fmt.Errorf("entry not found: %w", err)
			}

			articles, err := database.GetArticlesWithDeleted(entryID, cmd.Bool("all"))
			if err != nil {
				return err
			}

			if len(articles) == 0 {
				fmt.Printf("No articles found in entry %s\n", entryID)
				return nil
			}

			if cmd.Bool("json") {
				return formatJSON(articles)
			}

			// Markdown table as default
			fmt.Printf("Articles in entry %s:\n\n", entryID)
			fmt.Println("| ID | Title | Created |")
			fmt.Println("|----|-------|---------|")
			for _, a := range articles {
				title := a.Title
				if title == "" {
					title = "(untitled)"
				}
				fmt.Printf("| %s | %s | %s |\n", a.ID, title, a.CreatedAt)
			}

			return nil
		},
	}
}
