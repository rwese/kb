package cmd

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	assetstore "github.com/rwese/kb/internal/assets"
	"github.com/rwese/kb/internal/db"
	"github.com/urfave/cli/v3"
)

func (c *Commands) entryAttachmentUpdate() *cli.Command {
	return &cli.Command{
		Name:      "update",
		Usage:     "Change the title and/or replace the stored file of an attachment",
		ArgsUsage: "<entry-id> <attachment-id>",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "title", Aliases: []string{"t"}, Usage: "New attachment title"},
			&cli.StringFlag{Name: "file", Aliases: []string{"f"}, Usage: "Replace the stored file with this one"},
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

			existing, err := database.GetEntryAttachment(entryID, attachmentID)
			if err != nil {
				return fmt.Errorf("attachment not found: %w", err)
			}

			title := strings.TrimSpace(cmd.String("title"))
			filePath := cmd.String("file")
			if title == "" && filePath == "" {
				return fmt.Errorf("at least one of --title or --file is required")
			}
			if title == "" {
				title = existing.Title
			}
			if title == "" {
				return fmt.Errorf("attachment title cannot be empty")
			}

			// Validate and stage the replacement file before touching metadata.
			var staged *db.EntryAttachment
			if filePath != "" {
				if _, err := assetstore.ValidateAttachmentSource(filePath); err != nil {
					return err
				}
				staged, err = assetstore.StageAttachment(cfg.AssetsPath, entryID, attachmentID, filePath)
				if err != nil {
					return err
				}
				defer func() {
					if staged != nil {
						assetstore.CleanupStagedAttachment(cfg.AssetsPath, entryID, attachmentID)
					}
				}()
			}

			updated := *existing
			updated.Title = title
			if staged != nil {
				updated.FileName = staged.FileName
				updated.OriginalPath = staged.OriginalPath
				updated.StoreRelPath = staged.StoreRelPath
				updated.SHA256 = staged.SHA256
				updated.SizeBytes = staged.SizeBytes
				updated.ModePerm = staged.ModePerm
			}

			if err := database.UpdateEntryAttachment(updated); err != nil {
				return err
			}

			// The staged replacement is now owned by the committed metadata. The
			// previous stored bytes only need removal when the stored file name
			// changed; otherwise the stage already overwrote them in place.
			replacedFile := staged != nil
			if replacedFile {
				staged = nil
			}
			if replacedFile && updated.StoreRelPath != existing.StoreRelPath {
				oldFile := filepath.Join(cfg.AssetsPath, "entries", entryID, "attachments", attachmentID, existing.FileName)
				if err := assetstore.RemoveAttachmentFile(cfg.AssetsPath, entryID, attachmentID, existing.FileName); err != nil {
					return fmt.Errorf("attachment metadata updated but failed to remove previous stored bytes %s: %w", oldFile, err)
				}
			}
			staged = nil

			if cmd.Bool("json") {
				return formatJSON(updated)
			}

			fmt.Printf("Updated attachment %s in entry %s\n", attachmentID, entryID)
			return nil
		},
	}
}
