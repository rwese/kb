package cmd

import (
	"github.com/urfave/cli/v3"
)

// entryArticleCmd creates the article subcommand group under entry
func (c *Commands) entryArticleCmd() *cli.Command {
	return &cli.Command{
		Name:  "article",
		Usage: "Manage articles within an entry",
		Commands: []*cli.Command{
			c.entryArticleList(),
			c.entryArticleAdd(),
			c.entryArticleGet(),
			c.entryArticleUpdate(),
			c.entryArticleDelete(),
		},
	}
}
