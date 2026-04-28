package cmd

import (
	"context"
	"fmt"

	"github.com/rwese/kb/internal/config"
	"github.com/rwese/kb/internal/db"
	"github.com/urfave/cli/v3"
)

func (c *Commands) entryArticleGet() *cli.Command {
	return &cli.Command{
		Name:      "get",
		Usage:     "Get article from entry",
		ArgsUsage: "<entry-id> <article-id>",
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

			args := cmd.Args()
			if args.Len() < 2 {
				return fmt.Errorf("entry ID and article ID required")
			}

			entryID := args.Get(0)
			articleID := args.Get(1)

			// Verify entry exists
			_, err = database.GetEntry(entryID)
			if err != nil {
				return fmt.Errorf("entry not found: %w", err)
			}

			article, err := database.GetArticleWithDeleted(articleID, cmd.Bool("all"))
			if err != nil {
				return fmt.Errorf("article not found: %w", err)
			}

			// Verify article belongs to entry
			if article.EntryID != entryID {
				return fmt.Errorf("article %s does not belong to entry %s", articleID, entryID)
			}

			return printArticle(article, cmd.Bool("json"))
		},
	}
}
