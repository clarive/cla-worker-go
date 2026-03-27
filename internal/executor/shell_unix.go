//go:build !windows

package executor

import (
	"context"
	"os/exec"
)

// claude: buildShellCommand wraps a command string in /bin/sh -c on Unix.
func buildShellCommand(ctx context.Context, cmdString string) *exec.Cmd {
	return exec.CommandContext(ctx, "/bin/sh", "-c", cmdString)
}
