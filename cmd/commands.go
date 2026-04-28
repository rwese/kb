package cmd

import (
	"context"

	"github.com/urfave/cli/v3"
)

type Commands struct{}

func (c *Commands) Run(ctx context.Context, args []string) error {
	cmd := &cli.Command{
		Name:  "kb",
		Usage: "Knowledgebase CLI with weighted retrieval",
		Commands: []*cli.Command{
			c.setup(),
			c.status(),
			c.init(),
			c.config(),
			c.search(),
			c.stats(),
			c.download(),
			c.export(),
			c.entryCmd(),
		},
	}
	return cmd.Run(ctx, args)
}
