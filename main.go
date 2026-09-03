package main

import (
	"context"
	"fmt"
	"os"

	"github.com/rwese/kb/cmd"
)

var version = "dev"

func main() {
	c := &cmd.Commands{Version: version}

	if err := c.Run(context.Background(), os.Args); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}
