package cmd

import (
	"context"
	"fmt"

	"github.com/rwese/kb/internal/config"
	"github.com/rwese/kb/internal/db"
	"github.com/urfave/cli/v3"
)

func (c *Commands) entryDelete() *cli.Command {
	return &cli.Command{
		Name:      "delete",
		Usage:     "Delete an entry and all its articles",
		ArgsUsage: "<id>",
		Flags: []cli.Flag{
			&cli.BoolFlag{Name: "force", Aliases: []string{"f"}, Usage: "Skip confirmation"},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			cfg, err := config.Discover()
			if err != nil {
				return err
			}

			database, err := db.Open(cfg.DBPath)
			if err != nil {
				return err
			}
			defer database.Close()

			id := cmd.Args().First()
			if id == "" {
				return fmt.Errorf("ID required")
			}

			// Verify entry exists
			_, err = database.GetEntry(id)
			if err != nil {
				return fmt.Errorf("entry not found: %w", err)
			}

			// Confirm unless --force
			if !cmd.Bool("force") {
				fmt.Printf("Delete entry %s and all its articles? [y/N] ", id)
				var response string
				fmt.Scanln(&response)
				if response != "y" && response != "Y" {
					fmt.Println("Aborted")
					return nil
				}
			}

			// Delete all articles first (to clean up vectors)
			articles, _ := database.GetArticles(id)
			for _, a := range articles {
				database.DeleteVector(a.ID)
			}

			// Delete entry (articles cascade)
			if err := database.DeleteEntry(id); err != nil {
				return err
			}

			fmt.Printf("Deleted entry %s\n", id)
			return nil
		},
	}
}
