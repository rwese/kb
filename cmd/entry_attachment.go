package cmd

import "github.com/urfave/cli/v3"

// entryAttachmentCmd creates the attachment subcommand group under entry.
func (c *Commands) entryAttachmentCmd() *cli.Command {
	return &cli.Command{
		Name:  "attachment",
		Usage: "Manage entry attachments",
		Commands: []*cli.Command{
			c.entryAttachmentAdd(),
			c.entryAttachmentList(),
			c.entryAttachmentGet(),
			c.entryAttachmentUpdate(),
			c.entryAttachmentDelete(),
		},
	}
}
