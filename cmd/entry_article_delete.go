package cmd

import (
	"context"
	"fmt"

	assetstore "github.com/rwese/kb/internal/assets"
	"github.com/rwese/kb/internal/config"
	"github.com/rwese/kb/internal/db"
	"github.com/urfave/cli/v3"
)

func (c *Commands) entryArticleDelete() *cli.Command {
	return &cli.Command{
		Name:      "delete",
		Usage:     "Delete an article from entry",
		ArgsUsage: "<entry-id> <article-id>",
		Flags: []cli.Flag{
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

			if _, err := requireArticleOwnership(database, entryID, articleID); err != nil {
				return err
			}

			// Confirm unless --force
			if !cmd.Bool("force") {
				fmt.Printf("Delete article %s from entry %s? [y/N] ", articleID, entryID)
				var response string
				fmt.Scanln(&response)
				if response != "y" && response != "Y" {
					fmt.Println("Aborted")
					return nil
				}
			}

			// Delete vector
			database.DeleteVector(articleID)

			// Delete article
			if err := database.DeleteArticle(articleID); err != nil {
				return err
			}
			if err := assetstore.RemoveArticleTree(cfg.AssetsPath, articleID); err != nil {
				return fmt.Errorf("article deleted but failed to remove asset store for %s: %w", articleID, err)
			}

			fmt.Printf("Deleted article %s from entry %s\n", articleID, entryID)
			return nil
		},
	}
}
