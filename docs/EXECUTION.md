# Command Execution Design

## Overview

The `internal/executor` package handles all command execution for the
Clarive Worker. Commands arrive from the server via pubsub and are
executed on the worker's host OS.

## Execution modes

### 1. Shell execution (string commands)

When the server sends a raw command string like `"dir c:\"` or
`"ls -la /tmp"`, the executor runs it through the platform shell:

- **Unix**: `/bin/sh -c "<command>"`
- **Windows**: writes command to a temp `.cmd` file, runs `cmd /C tempfile.cmd`

Shell execution is used when:
- The command is a bare string
- The command is a single-element array `["echo hello"]`
- The command uses shell builtins (dir, copy, del, cd, etc.)
- The command uses shell syntax (pipes, redirects, &&, ||, etc.)

### 2. Direct execution (array commands)

When the server sends a multi-element array like `["python", "script.py", "--flag"]`,
the executor runs the first element as the executable with the remaining
elements as arguments. No shell is involved.

Direct execution is used when:
- The command is a multi-element array `["exe", "arg1", "arg2"]`
- The target is a real executable (not a shell builtin)
- No shell syntax is needed

## Why Windows uses temp files

Go's `os/exec` package passes each command-line argument through
`syscall.EscapeArg` before calling `CreateProcess`. EscapeArg quotes
arguments containing spaces or backslashes, and doubles trailing
backslashes before the closing quote:

```
Input string:  dir c:\
EscapeArg:     "dir c:\\"
```

`cmd.exe` receives `"dir c:\\"`, strips quotes, and gets `dir c:\\`
(two backslashes). This corrupts every command ending with a Windows
path separator.

The temp-file approach eliminates this entirely. The command text is
written to a `.cmd` file and never appears in a process argument:

```
Temp file contents:
    @echo off
    dir c:\

Execution:
    cmd /C C:\Users\...\Temp\cla-exec-12345.cmd
```

The temp file path has no special characters, so EscapeArg has nothing
to corrupt. The command inside the file is executed verbatim by cmd.exe.

### Batch file note

In `.cmd` files, `for`-loop variables use `%%i` instead of `%i`. This
is standard Windows batch syntax. If you need literal `%i` syntax,
use the direct-exec array form:

```json
["cmd", "/C", "for %i in (*.txt) do echo %i"]
```

## Adding new execution call sites

All command execution must go through the `executor.OsExecutor`. Do not
call `os/exec` directly elsewhere in the codebase. The correct pattern:

```go
exec := executor.NewOsExecutor()

// Shell command (string):
output, rc, err := exec.Execute(ctx, "dir c:\", "")

// Direct exec (array):
output, rc, err := exec.Execute(ctx, []interface{}{"python", "script.py"}, "")
```

## Wire format

Commands arrive from the Clarive server as JSON. The `cmd` field is
either a string or an array:

```json
{"event": "worker.exec", "cmd": "dir c:\\", "chdir": ""}
{"event": "worker.exec", "cmd": ["python", "script.py"], "chdir": "/app"}
```

The executor's `ParseCommand` function converts the wire format into
a typed `CommandSpec` struct internally.
