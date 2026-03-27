//go:build !windows

package executor

import (
	"context"
	"os/exec"
)

// buildShellCommand wraps a raw command string in /bin/sh -c.
// On Unix, Go's argument handling is straightforward — no escaping
// issues — so we pass the string directly.
func buildShellCommand(ctx context.Context, raw string) (*exec.Cmd, func(), error) {
	return exec.CommandContext(ctx, "/bin/sh", "-c", raw), nil, nil
}
