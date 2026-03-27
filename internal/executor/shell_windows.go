//go:build windows

package executor

import (
	"context"
	"os/exec"
	"syscall"
)

// claude: buildShellCommand uses SysProcAttr.CmdLine to pass the command
// string directly to CreateProcess, bypassing Go's syscall.EscapeArg.
// Go's EscapeArg doubles trailing backslashes before the closing quote it
// adds, which corrupts Windows paths like "dir c:\" into "dir c:\\".
// By setting CmdLine directly we pass the raw string to cmd.exe /C.
func buildShellCommand(ctx context.Context, cmdString string) *exec.Cmd {
	command := exec.CommandContext(ctx, "cmd")
	command.SysProcAttr = &syscall.SysProcAttr{
		CmdLine: `cmd /C ` + cmdString,
	}
	return command
}
