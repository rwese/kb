package cmd

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/rwese/kb/internal/config"
	"github.com/rwese/kb/internal/db"
	"github.com/urfave/cli/v3"
)

func (c *Commands) add() *cli.Command {
	return &cli.Command{
		Name:  "add",
		Usage: "Add entry to knowledgebase",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "title", Aliases: []string{"t"}, Usage: "Entry title"},
			&cli.StringFlag{Name: "content", Aliases: []string{"c"}, Usage: "Entry content"},
			&cli.StringFlag{Name: "file", Aliases: []string{"f"}, Usage: "Read content from file"},
			&cli.StringFlag{Name: "tags", Usage: "Comma-separated tags"},
			&cli.BoolFlag{Name: "stdin", Aliases: []string{"s"}, Usage: "Read content from stdin"},
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

			var title, content string

			// Get title
			if t := cmd.String("title"); t != "" {
				title = t
			}

			// Get content
			if f := cmd.String("file"); f != "" {
				data, err := os.ReadFile(f)
				if err != nil {
					return err
				}
				content = string(data)
				if title == "" {
					title = f
				}
			} else if cmd.Bool("stdin") {
				data, err := io.ReadAll(os.Stdin)
				if err != nil {
					return err
				}
				content = string(data)
				if title == "" {
					title = "stdin"
				}
			} else if c := cmd.String("content"); c != "" {
				content = c
			} else {
				// Interactive mode
				fmt.Print("Title: ")
				title = readLine()
				fmt.Print("Content (Ctrl+D to finish):\n")
				content = readMultiline()
			}

			tags := cmd.String("tags")

			id, err := database.Add(title, content, tags)
			if err != nil {
				return err
			}

			fmt.Printf("Added entry #%d\n", id)
			return nil
		},
	}
}

func readLine() string {
	s, _ := bufio.NewReader(os.Stdin).ReadString('\n')
	return strings.TrimSpace(s)
}

func readMultiline() string {
	var lines []string
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return strings.Join(lines, "\n")
}
