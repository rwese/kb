package cmd

import "github.com/urfave/cli/v3"

// deleteCmd creates a top-level delete command group.
func (c *Commands) deleteCmd() *cli.Command {
	return &cli.Command{
		Name:  "delete",
		Usage: "Delete entries or articles",
		Commands: []*cli.Command{
			{
				Name:      "entry",
				Usage:     "Delete one or more entries and all their articles",
				ArgsUsage: "<id> [id...]",
				Flags: []cli.Flag{
					&cli.BoolFlag{Name: "force", Aliases: []string{"f"}, Usage: "Skip confirmation"},
				},
				Action: c.entryDelete().Action,
			},
			{
				Name:      "article",
				Usage:     "Delete an article from an entry",
				ArgsUsage: "<entry-id> <article-id>",
				Flags: []cli.Flag{
					&cli.BoolFlag{Name: "force", Aliases: []string{"f"}, Usage: "Skip confirmation"},
				},
				Action: c.entryArticleDelete().Action,
			},
		},
	}
}
