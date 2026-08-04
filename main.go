package main

import (
	"os"

	"github.com/irootkernel/gaori/internal/cli"
)

var (
	version   = "0.1.8"
	commit    = "unknown"
	buildDate = "unknown"
)

func main() {
	info := cli.NewBuildInfo("gaori", version, commit, buildDate)
	os.Exit(cli.Run(os.Args[1:], os.Stdout, os.Stderr, info))
}
