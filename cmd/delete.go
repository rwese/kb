package cmd

import (
	"context"
	"fmt"

	"github.com/rwese/kb/internal/config"
	"github.com/rwese/kb/internal/db"
	"github.com/urfave/cli/v3"
)

func (c *Commands) delete() *cli.Command {
	return &cli.Command{
		Name:  "delete",
		Aliases: []string{"rm", "del"},
		Usage: "Delete entry or article",
		Flags: []cli.Flag{
			&cli.Int64Flag{Name: "entry", Aliases: []string{"e"}, Usage: "Entry ID to delete"},
			&cli.Int64Flag{Name: "article", Aliases: []string{"a"}, Usage: "Article ID to delete"},
			&cli.BoolFlag{Name: "force", Aliases: []string{"f"}, Usage: "Skip confirmation"},
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

			if articleID := cmd.Int64("article"); articleID > 0 {
				return database.DeleteArticle(articleID)
			}

			if entryID := cmd.Int64("entry"); entryID > 0 {
				return database.DeleteEntry(entryID)
			}

			return fmt.Errorf("either --entry or --article required")
		},
	}
}
