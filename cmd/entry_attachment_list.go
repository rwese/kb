package cmd

import (
	"context"
	"fmt"

	assetstore "github.com/rwese/kb/internal/assets"
	"github.com/urfave/cli/v3"
)

func (c *Commands) entryAttachmentList() *cli.Command {
	return &cli.Command{
		Name:      "list",
		Usage:     "List attachments for an entry",
		ArgsUsage: "<entry-id>",
		Flags: []cli.Flag{
			&cli.BoolFlag{Name: "json", Usage: "Output as JSON"},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			_, database, err := openDBFromConfig()
			if err != nil {
				return err
			}
			defer func() { _ = database.Close() }()

			entryID := cmd.Args().First()
			if entryID == "" {
				return fmt.Errorf("entry ID required")
			}
			if _, err := database.GetEntry(entryID); err != nil {
				return fmt.Errorf("entry not found: %w", err)
			}

			attachments, err := database.ListEntryAttachments(entryID)
			if err != nil {
				return err
			}
			if cmd.Bool("json") {
				return formatJSON(attachments)
			}

			if len(attachments) == 0 {
				fmt.Printf("No attachments found for entry %s\n", entryID)
				return nil
			}

			fmt.Printf("Attachments in entry %s:\n\n", entryID)
			fmt.Println("| ID | Title | File | Size | Created |")
			fmt.Println("|----|-------|------|------|---------|")
			for _, att := range attachments {
				fmt.Printf("| %s | %s | %s | %s | %s |\n", att.ID, att.Title, att.FileName, assetstore.FormatSize(att.SizeBytes), att.CreatedAt)
			}
			return nil
		},
	}
}
