package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/jeprecated/juggle/internal/cli"
)

// version is set at build time via -ldflags
var version = "dev"

func main() {
	cli.SetVersion(version)
	if err := cli.Execute(); err != nil {
		if errors.Is(err, cli.ErrInterrupted) {
			os.Exit(130)
		}
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
