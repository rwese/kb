package main

import (
	"context"
	"fmt"
	"os"

	"github.com/rwese/kb/internal/cli"
)

var version = "dev"

func main() {
	c := &cli.Commands{Version: version}

	if err := c.Run(context.Background(), os.Args); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}
