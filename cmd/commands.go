package cmd

import (
	"context"

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

			c.status(),
			c.init(),
			c.config(),
			c.search(),
			c.stats(),
			c.export(),
			c.deleteCmd(),
			c.entryCmd(),
		},
	}
	return cmd.Run(ctx, args)
}
