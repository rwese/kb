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
			c.init(),
			c.add(),
			c.append(),
			c.list(),
			c.get(),
			c.search(),
			c.delete(),
			c.config(),
		},
	}
	return cmd.Run(ctx, args)
}
