// Package executor provides command execution for the Clarive Worker.
//
// Commands arrive from the server in two forms:
//
//  1. A raw string: "dir c:\" or "ls -la /tmp"
//     → requires shell interpretation (cmd.exe or /bin/sh)
//
//  2. An array of strings: ["python", "script.py", "--flag"]
//     → direct process execution, no shell involved
//
// On Windows, shell execution uses a temporary .cmd file to avoid
// Go's argument escaping (syscall.EscapeArg) which corrupts trailing
// backslashes in paths. See shell_windows.go for details.
package executor

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
)

// CommandSpec is the internal structured representation of a command.
// Call sites should use ParseCommand to build one from wire data.
type CommandSpec struct {
	// Shell indicates the command requires shell interpretation.
	// When true, Raw is executed via the platform shell.
	// When false, Args is executed directly.
	Shell bool

	// Raw is the verbatim command string for shell execution.
	// Only used when Shell is true.
	Raw string

	// Args is [executable, arg1, arg2, ...] for direct execution.
	// Only used when Shell is false.
	Args []string

	// Dir is the working directory. Empty means inherit.
	Dir string
}

// ParseCommand converts the wire-format cmd (string or []interface{})
// into a CommandSpec.
//
// Wire format rules:
//   - string           → shell execution (Raw)
//   - [single_string]  → shell execution (Raw), same as bare string
//   - [exe, arg, ...]  → direct execution (Args), no shell
func ParseCommand(cmd interface{}, chdir string) (CommandSpec, error) {
	switch c := cmd.(type) {
	case string:
		return CommandSpec{Shell: true, Raw: c, Dir: chdir}, nil
	case []interface{}:
		if len(c) == 0 {
			return CommandSpec{}, fmt.Errorf("empty command array")
		}
		args := make([]string, len(c))
		for i, a := range c {
			args[i] = fmt.Sprintf("%v", a)
		}
		if len(args) == 1 {
			return CommandSpec{Shell: true, Raw: args[0], Dir: chdir}, nil
		}
		return CommandSpec{Shell: false, Args: args, Dir: chdir}, nil
	default:
		return CommandSpec{}, fmt.Errorf("unsupported command type: %T", cmd)
	}
}

// CommandExecutor is the interface consumed by the dispatcher.
type CommandExecutor interface {
	Execute(ctx context.Context, cmd interface{}, chdir string) (output string, rc int, err error)
}

// OsExecutor implements CommandExecutor using real OS processes.
type OsExecutor struct{}

func NewOsExecutor() *OsExecutor {
	return &OsExecutor{}
}

// Execute parses the wire-format command and runs it.
func (e *OsExecutor) Execute(ctx context.Context, cmd interface{}, chdir string) (string, int, error) {
	spec, err := ParseCommand(cmd, chdir)
	if err != nil {
		return "", 1, err
	}
	return e.run(ctx, spec)
}

// run executes a parsed CommandSpec.
func (e *OsExecutor) run(ctx context.Context, spec CommandSpec) (string, int, error) {
	var command *exec.Cmd
	var cleanup func()

	if spec.Shell {
		var err error
		command, cleanup, err = buildShellCommand(ctx, spec.Raw)
		if err != nil {
			return "", 1, fmt.Errorf("preparing shell command: %w", err)
		}
		if cleanup != nil {
			defer cleanup()
		}
	} else {
		command = exec.CommandContext(ctx, spec.Args[0], spec.Args[1:]...)
	}

	if spec.Dir != "" {
		command.Dir = spec.Dir
	}

	var output bytes.Buffer
	command.Stdout = &output
	command.Stderr = &output

	err := command.Run()
	rc := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			rc = exitErr.ExitCode()
		} else {
			return output.String(), 1, err
		}
	}

	return output.String(), rc, nil
}
