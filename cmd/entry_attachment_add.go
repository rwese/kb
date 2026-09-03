package cmd

import (
	"context"
	"fmt"
	"strings"

	assetstore "github.com/rwese/kb/internal/assets"
	"github.com/rwese/kb/internal/id"
	"github.com/urfave/cli/v3"
)

func (c *Commands) entryAttachmentAdd() *cli.Command {
	return &cli.Command{
		Name:      "add",
		Usage:     "Attach one regular file to an entry as a titled attachment",
		ArgsUsage: "<entry-id> <file>",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "title", Aliases: []string{"t"}, Usage: "Attachment title (required)"},
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
				return fmt.Errorf("entry ID and exactly one file path are required")
			}
			if args.Len() > 2 {
				return fmt.Errorf("exactly one file path is required, got %d", args.Len()-1)
			}

			title := strings.TrimSpace(cmd.String("title"))
			if title == "" {
				return fmt.Errorf("a non-empty --title is required")
			}

			entryID := args.Get(0)
			if _, err := database.GetEntry(entryID); err != nil {
				return fmt.Errorf("entry not found: %w", err)
			}

			sourcePath := args.Get(1)
			if _, err := assetstore.ValidateAttachmentSource(sourcePath); err != nil {
				return err
			}

			attachmentID := id.Entry()
			staged, err := assetstore.StageAttachment(cfg.AssetsPath, entryID, attachmentID, sourcePath)
			if err != nil {
				return err
			}
			defer func() {
				if staged != nil {
					assetstore.CleanupStagedAttachment(cfg.AssetsPath, entryID, attachmentID)
				}
			}()

			staged.Title = title
			if err := database.AddEntryAttachment(*staged); err != nil {
				return err
			}

			committed := *staged
			staged = nil
			if cmd.Bool("json") {
				return formatJSON(committed)
			}

			fmt.Printf("Added attachment %s to entry %s at %s\n", attachmentID, entryID, committed.StoreRelPath)
			return nil
		},
	}
}
