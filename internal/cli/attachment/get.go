package attachment

import (
	"context"
	"fmt"

	"github.com/rwese/kb/internal/cli/shared"
	"github.com/urfave/cli/v3"
)

func NewGetCommand() *cli.Command {
	return &cli.Command{
		Name:      "get",
		Usage:     "Show attachment metadata and managed path",
		ArgsUsage: "<entry-id> <attachment-id>",
		Flags: []cli.Flag{
			&cli.BoolFlag{Name: "json", Usage: "Output as JSON"},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			cfg, database, err := shared.OpenDBFromConfig()
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

			if cmd.Bool("json") {
				return shared.FormatJSON(*att)
			}

			fmt.Printf("# Attachment %s\n\n", att.ID)
			fmt.Printf("- Title: %s\n", att.Title)
			fmt.Printf("- Entry ID: %s\n", att.EntryID)
			fmt.Printf("- File: %s\n", att.FileName)
			fmt.Printf("- Original Path: %s\n", att.OriginalPath)
			fmt.Printf("- Stored Path: %s\n", shared.ManagedAttachmentPath(cfg, *att))
			fmt.Printf("- SHA256: %s\n", att.SHA256)
			fmt.Printf("- Size: %d bytes\n", att.SizeBytes)
			fmt.Printf("- Mode: %o\n", att.ModePerm)
			fmt.Printf("- Created: %s\n", att.CreatedAt)
			fmt.Printf("- Updated: %s\n", att.UpdatedAt)
			return nil
		},
	}
}
