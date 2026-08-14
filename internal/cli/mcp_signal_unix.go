//go:build unix

package cli

import (
	"os"
	"syscall"
)

func mcpShutdownSignals() []os.Signal {
	return []os.Signal{os.Interrupt, syscall.SIGTERM}
}

func mcpSignalExitCode(sig os.Signal) int {
	if value, ok := sig.(syscall.Signal); ok {
		return 128 + int(value)
	}
	return 1
}
