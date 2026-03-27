//go:build windows

package executor

import (
	"context"
	"fmt"
	"os"
	"os/exec"
)

// buildShellCommand executes a raw command string on Windows by writing
// it to a temporary .cmd file and running that file with cmd.exe.
//
// Why not pass the command directly to cmd.exe /C?
//
// Go's os/exec builds the Windows CreateProcess lpCommandLine by running
// each argument through syscall.EscapeArg, which quotes arguments
// containing spaces or backslashes. EscapeArg doubles every trailing
// backslash before the closing quote it appends:
//
//     input:  dir c:\
//     quoted: "dir c:\\"   (backslash doubled before closing quote)
//
// cmd.exe receives "dir c:\\" and after stripping quotes gets two
// backslashes — a corrupted path. This affects every command whose
// last character is a backslash, which is common in Windows paths.
//
// Using SysProcAttr.CmdLine to bypass EscapeArg is fragile and
// version-dependent. The temp-file approach eliminates the problem
// entirely: the command text lives in a file, not in a process
// argument, so no escaping is applied.
//
// The .cmd file contains:
//
//     @echo off
//     <raw command>
//
// @echo off prevents cmd.exe from echoing the command before running
// it, matching the behavior of cmd /C.
//
// Note: in .cmd files, for-loop variables use %%i instead of %i.
// This matches standard Windows batch semantics and is expected by
// anyone writing batch commands. For literal %i syntax, use the
// direct-exec path: ["cmd", "/C", "for %i in (*.txt) do echo %i"].
func buildShellCommand(ctx context.Context, raw string) (*exec.Cmd, func(), error) {
	f, err := os.CreateTemp("", "cla-exec-*.cmd")
	if err != nil {
		return nil, nil, fmt.Errorf("creating temp script: %w", err)
	}

	script := "@echo off\r\n" + raw + "\r\n"
	if _, err := f.WriteString(script); err != nil {
		f.Close()
		os.Remove(f.Name())
		return nil, nil, fmt.Errorf("writing temp script: %w", err)
	}
	f.Close()

	name := f.Name()
	command := exec.CommandContext(ctx, "cmd", "/C", name)
	cleanup := func() { os.Remove(name) }

	return command, cleanup, nil
}
