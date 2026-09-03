package cmd

import (
	"context"
	"errors"
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
			&cli.BoolFlag{Name: "force", Aliases: []string{"f"}, Usage: "Skip confirmation and ignore missing entries"},
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
		if force && errors.Is(err, db.ErrNotFound) {
			fmt.Printf("Entry %s does not exist, skipping\n", id)
			return nil
		}
		return fmt.Errorf("entry %s not found: %w", id, err)
	}

	// Confirm unless --force
	if !force {
		fmt.Printf("Delete entry %s and all its articles? [y/N] ", id)
		var response string
		if _, err := fmt.Scanln(&response); err != nil && response == "" {
			// EOF / empty input counts as "no"
			response = "n"
		}
		if response != "y" && response != "Y" {
			fmt.Printf("Skipped entry %s\n", id)
			return nil
		}
	}

	// Delete all articles first (to clean up vectors)
	articles, err := database.GetArticles(id)
	if err != nil {
		return fmt.Errorf("failed to list articles for %s: %w", id, err)
	}
	for _, a := range articles {
		if err := database.DeleteVector(a.ID); err != nil {
			return fmt.Errorf("failed to delete vector for %s: %w", a.ID, err)
		}
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
	// Entry attachment rows cascade with the entry; remove their stored bytes.
	if err := assetstore.RemoveEntryAttachmentsTree(assetsPath, id); err != nil {
		return fmt.Errorf("entry %s deleted but failed to remove attachment store: %w", id, err)
	}

	fmt.Printf("Deleted entry %s\n", id)
	return nil
}
