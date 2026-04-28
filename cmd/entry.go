package cmd

import (
	"github.com/urfave/cli/v3"
)

// entryCmd creates the entry command group
func (c *Commands) entryCmd() *cli.Command {
	return &cli.Command{
		Name:  "entry",
		Usage: "Manage entries",
		Commands: []*cli.Command{
			c.entryList(),
			c.entryCreate(),
			c.entryGet(),
			c.entryUpdate(),
			c.entryDelete(),
			c.entryArticleCmd(),
		},
	}
}
