//go:build !unix

package cli

import "os"

func mcpShutdownSignals() []os.Signal {
	return []os.Signal{os.Interrupt}
}

func mcpSignalExitCode(sig os.Signal) int {
	if sig == os.Interrupt {
		return 130
	}
	return 1
}
