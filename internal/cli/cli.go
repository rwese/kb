package cli

import (
	"context"

	"github.com/rwese/kb/internal/cli/article"
	"github.com/rwese/kb/internal/cli/asset"
	"github.com/rwese/kb/internal/cli/attachment"
	"github.com/rwese/kb/internal/cli/entry"
	"github.com/rwese/kb/internal/cli/export"
	"github.com/rwese/kb/internal/cli/search"
	"github.com/rwese/kb/internal/cli/system"
	"github.com/urfave/cli/v3"
)

type Commands struct {
	Version string
}

func (c *Commands) Run(ctx context.Context, args []string) error {
	cmd := &cli.Command{
		Name:    "kb",
		Usage:   "Knowledgebase CLI with weighted retrieval",
		Version: c.Version,
		Commands: []*cli.Command{
			system.NewStatusCommand(),
			system.NewInitCommand(),
			system.NewConfigCommand(),
			search.NewSearchCommand(),
			system.NewStatsCommand(),
			export.NewExportCommand(),
			deleteCommand(),
			entryCommand(),
		},
	}
	return cmd.Run(ctx, args)
}

func deleteCommand() *cli.Command {
	return &cli.Command{
		Name:  "delete",
		Usage: "Delete entries or articles",
		Commands: []*cli.Command{
			{
				Name:      "entry",
				Usage:     "Delete one or more entries and all their articles",
				ArgsUsage: "<id> [id...]",
				Flags: []cli.Flag{
					&cli.BoolFlag{Name: "force", Aliases: []string{"f"}, Usage: "Skip confirmation and ignore missing entries"},
				},
				Action: entry.NewDeleteCommand().Action,
			},
			{
				Name:      "article",
				Usage:     "Delete an article from an entry",
				ArgsUsage: "<entry-id> <article-id>",
				Flags: []cli.Flag{
					&cli.BoolFlag{Name: "force", Aliases: []string{"f"}, Usage: "Skip confirmation"},
				},
				Action: article.NewDeleteCommand().Action,
			},
		},
	}
}

func entryCommand() *cli.Command {
	return &cli.Command{
		Name:  "entry",
		Usage: "Manage entries",
		Commands: []*cli.Command{
			entry.NewListCommand(),
			entry.NewCreateCommand(),
			entry.NewGetCommand(),
			entry.NewUpdateCommand(),
			entry.NewDeleteCommand(),
			articleCommand(),
			attachmentCommand(),
		},
	}
}

func articleCommand() *cli.Command {
	return &cli.Command{
		Name:  "article",
		Usage: "Manage articles within an entry",
		Commands: []*cli.Command{
			article.NewListCommand(),
			article.NewAddCommand(),
			article.NewGetCommand(),
			article.NewUpdateCommand(),
			article.NewDeleteCommand(),
			assetCommand(),
		},
	}
}

func assetCommand() *cli.Command {
	return &cli.Command{
		Name:  "asset",
		Usage: "Manage article assets",
		Commands: []*cli.Command{
			asset.NewAddCommand(),
			asset.NewListCommand(),
			asset.NewGetCommand(),
			asset.NewDeleteCommand(),
		},
	}
}

func attachmentCommand() *cli.Command {
	return &cli.Command{
		Name:  "attachment",
		Usage: "Manage entry attachments",
		Commands: []*cli.Command{
			attachment.NewAddCommand(),
			attachment.NewListCommand(),
			attachment.NewGetCommand(),
			attachment.NewUpdateCommand(),
			attachment.NewDeleteCommand(),
		},
	}
}
