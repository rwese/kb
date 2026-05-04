package cmd

import (
	"context"
	"fmt"

	assetstore "github.com/rwese/kb/internal/assets"
	"github.com/rwese/kb/internal/config"
	"github.com/rwese/kb/internal/db"
	"github.com/urfave/cli/v3"
)

func (c *Commands) entryDelete() *cli.Command {
	return &cli.Command{
		Name:      "delete",
		Usage:     "Delete an entry and all its articles",
		ArgsUsage: "<id> [id...]",
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

			ids := cmd.Args().Slice()
			if len(ids) == 0 {
				return fmt.Errorf("at least one ID required")
			}

			for _, id := range ids {
				if err := deleteEntry(database, cfg.AssetsPath, id, cmd.Bool("force")); err != nil {
					return err
				}
			}

			return nil
		},
	}
}

func deleteEntry(database *db.DB, assetsPath, id string, force bool) error {
	// Verify entry exists
	_, err := database.GetEntry(id)
	if err != nil {
		return fmt.Errorf("entry %s not found: %w", id, err)
	}

	// Confirm unless --force
	if !force {
		fmt.Printf("Delete entry %s and all its articles? [y/N] ", id)
		var response string
		fmt.Scanln(&response)
		if response != "y" && response != "Y" {
			fmt.Printf("Skipped entry %s\n", id)
			return nil
		}
	}

	// Delete all articles first (to clean up vectors)
	articles, _ := database.GetArticles(id)
	for _, a := range articles {
		database.DeleteVector(a.ID)
	}

	// Delete entry (articles cascade)
	if err := database.DeleteEntry(id); err != nil {
		return err
	}
	for _, article := range articles {
		if err := assetstore.RemoveArticleTree(assetsPath, article.ID); err != nil {
			return fmt.Errorf("entry %s deleted but failed to remove asset store for article %s: %w", id, article.ID, err)
		}
	}

	fmt.Printf("Deleted entry %s\n", id)
	return nil
}
