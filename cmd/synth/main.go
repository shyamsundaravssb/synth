package main

import (
	"os"

	"github.com/shyamsundaravssb/synth/internal/cli"
)

// version is injected at build time via ldflags.
var version = "dev"

func main() {
	cmd := cli.NewRootCmd(version)
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
