// Package main at the module root exists so that `go install
// github.com/irootkernel/gaori@<version>` with no subpath resolves a main
// package; the documented network install depends on it. `make build` and `make
// install` use ./cmd/gaori instead. Keep both entrypoints identical, including
// the version default, because `go install` supplies no linker values.
package main

import (
	"os"

	"github.com/irootkernel/gaori/internal/cli"
)

var (
	version   = "0.1.14"
	commit    = "unknown"
	buildDate = "unknown"
)

func main() {
	info := cli.NewBuildInfo("gaori", version, commit, buildDate)
	os.Exit(cli.Run(os.Args[1:], os.Stdout, os.Stderr, info))
}
