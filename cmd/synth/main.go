package main

import (
	"os"

	"github.com/shyamsundaravssb/synth/internal/cli"
	"github.com/shyamsundaravssb/synth/internal/daemon"
)

// version is injected at build time via ldflags.
var version = "dev"

func main() {
	for _, arg := range os.Args[1:] {
		if arg == "--daemon-child" {
			runDaemonChild()
			return
		}
	}

	cmd := cli.NewRootCmd(version)
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func runDaemonChild() {
	d := daemon.New()
	if err := d.Run(); err != nil {
		os.Exit(1)
	}
}
