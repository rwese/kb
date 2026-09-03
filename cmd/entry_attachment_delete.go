package cmd

import (
	"context"
	"fmt"
	"path/filepath"

	assetstore "github.com/rwese/kb/internal/assets"
	"github.com/urfave/cli/v3"
)

func (c *Commands) entryAttachmentDelete() *cli.Command {
	return &cli.Command{
		Name:      "delete",
		Usage:     "Permanently delete an attachment (metadata and stored bytes)",
		ArgsUsage: "<entry-id> <attachment-id>",
		Flags: []cli.Flag{
			&cli.BoolFlag{Name: "force", Aliases: []string{"f"}, Usage: "Skip confirmation"},
			&cli.BoolFlag{Name: "json", Usage: "Output as JSON"},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			cfg, database, err := openDBFromConfig()
			if err != nil {
				return err
			}
			defer func() { _ = database.Close() }()

			args := cmd.Args()
			if args.Len() < 2 {
				return fmt.Errorf("entry ID and attachment ID required")
			}

			entryID := args.Get(0)
			attachmentID := args.Get(1)
			if _, err := database.GetEntry(entryID); err != nil {
				return fmt.Errorf("entry not found: %w", err)
			}

			att, err := database.GetEntryAttachment(entryID, attachmentID)
			if err != nil {
				return fmt.Errorf("attachment not found: %w", err)
			}

			if !cmd.Bool("force") {
				fmt.Printf("Delete attachment %s from entry %s? [y/N] ", attachmentID, entryID)
				var response string
				if _, err := fmt.Scanln(&response); err != nil && response == "" {
					response = "n"
				}
				if response != "y" && response != "Y" {
					fmt.Printf("Skipped attachment %s\n", attachmentID)
					return nil
				}
			}

			if err := database.DeleteEntryAttachment(entryID, attachmentID); err != nil {
				return err
			}

			if err := assetstore.RemoveAttachmentTree(cfg.AssetsPath, entryID, attachmentID); err != nil {
				return fmt.Errorf("attachment metadata deleted but failed to remove stored bytes %s: %w", filepath.Join(cfg.AssetsPath, "entries", entryID, "attachments", attachmentID), err)
			}

			if cmd.Bool("json") {
				return formatJSON(*att)
			}

			fmt.Printf("Deleted attachment %s from entry %s\n", attachmentID, entryID)
			return nil
		},
	}
}
