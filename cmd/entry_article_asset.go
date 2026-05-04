package cmd

import "github.com/urfave/cli/v3"

func (c *Commands) entryArticleAssetCmd() *cli.Command {
	return &cli.Command{
		Name:  "asset",
		Usage: "Manage article assets",
		Commands: []*cli.Command{
			c.entryArticleAssetAdd(),
			c.entryArticleAssetList(),
			c.entryArticleAssetGet(),
			c.entryArticleAssetDelete(),
		},
	}
}
