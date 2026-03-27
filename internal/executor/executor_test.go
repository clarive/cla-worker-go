package executor

import (
	"context"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- ParseCommand tests ---

func TestParseCommand_String(t *testing.T) {
	spec, err := ParseCommand("echo hello", "/tmp")
	require.NoError(t, err)
	assert.True(t, spec.Shell)
	assert.Equal(t, "echo hello", spec.Raw)
	assert.Equal(t, "/tmp", spec.Dir)
	assert.Empty(t, spec.Args)
}

func TestParseCommand_SingleElementArray(t *testing.T) {
	// claude: single-element array is treated as shell, same as bare string
	spec, err := ParseCommand([]interface{}{"echo hello"}, "")
	require.NoError(t, err)
	assert.True(t, spec.Shell)
	assert.Equal(t, "echo hello", spec.Raw)
}

func TestParseCommand_MultiElementArray(t *testing.T) {
	// claude: multi-element array is direct exec, no shell
	spec, err := ParseCommand([]interface{}{"python", "script.py", "--flag"}, "")
	require.NoError(t, err)
	assert.False(t, spec.Shell)
	assert.Equal(t, []string{"python", "script.py", "--flag"}, spec.Args)
}

func TestParseCommand_EmptyArray(t *testing.T) {
	_, err := ParseCommand([]interface{}{}, "")
	require.Error(t, err)
}

func TestParseCommand_UnsupportedType(t *testing.T) {
	_, err := ParseCommand(42, "")
	require.Error(t, err)
}

// --- Shell execution tests ---

func TestExecute_ShellString(t *testing.T) {
	e := NewOsExecutor()
	output, rc, err := e.Execute(context.Background(), "echo hello", "")
	require.NoError(t, err)
	assert.Equal(t, 0, rc)
	assert.Contains(t, output, "hello")
}

func TestExecute_ShellSingleArray(t *testing.T) {
	e := NewOsExecutor()
	output, rc, err := e.Execute(context.Background(), []interface{}{"echo hello"}, "")
	require.NoError(t, err)
	assert.Equal(t, 0, rc)
	assert.Contains(t, output, "hello")
}

func TestExecute_ShellExpansion(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix shell expansion test")
	}
	e := NewOsExecutor()
	output, rc, err := e.Execute(context.Background(), "echo $((1+2))", "")
	require.NoError(t, err)
	assert.Equal(t, 0, rc)
	assert.Contains(t, output, "3")
}

// --- Direct execution tests ---

func TestExecute_DirectExec(t *testing.T) {
	e := NewOsExecutor()
	output, rc, err := e.Execute(context.Background(), []interface{}{"echo", "world"}, "")
	require.NoError(t, err)
	assert.Equal(t, 0, rc)
	assert.Contains(t, output, "world")
}

func TestExecute_DirectExecNotFound(t *testing.T) {
	e := NewOsExecutor()
	// claude: multi-element array bypasses shell; missing executable returns error
	_, _, err := e.Execute(context.Background(), []interface{}{"nonexistent_command_xyz", "arg"}, "")
	require.Error(t, err)
}

// --- Exit code tests ---

func TestExecute_NonZeroExit(t *testing.T) {
	e := NewOsExecutor()
	_, rc, err := e.Execute(context.Background(), "exit 42", "")
	require.NoError(t, err)
	assert.Equal(t, 42, rc)
}

func TestExecute_CommandNotFoundShell(t *testing.T) {
	e := NewOsExecutor()
	_, rc, err := e.Execute(context.Background(), "nonexistent_command_xyz", "")
	require.NoError(t, err)
	assert.NotEqual(t, 0, rc)
}

// --- Stderr capture ---

func TestExecute_Stderr(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix stderr redirect test")
	}
	e := NewOsExecutor()
	output, rc, err := e.Execute(context.Background(), "echo err >&2", "")
	require.NoError(t, err)
	assert.Equal(t, 0, rc)
	assert.Contains(t, output, "err")
}

// --- Working directory ---

func TestExecute_Chdir(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix chdir test")
	}
	e := NewOsExecutor()
	output, rc, err := e.Execute(context.Background(), "pwd", "/tmp")
	require.NoError(t, err)
	assert.Equal(t, 0, rc)
	assert.Contains(t, output, "/tmp")
}

// --- Context cancellation ---

func TestExecute_ContextCancel(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	e := NewOsExecutor()
	_, rc, err := e.Execute(ctx, "sleep 10", "")
	// claude: context cancel kills the shell; may return error or non-zero rc
	if err == nil {
		assert.NotEqual(t, 0, rc)
	}
}

// --- Windows-specific path tests ---
// These tests verify that commands ending with backslash work correctly.
// On non-Windows they test the Unix shell path (which never had this bug).

func TestExecute_TrailingBackslash(t *testing.T) {
	// claude: this is the exact failure case — "dir c:\" failed before
	// because Go's EscapeArg doubled the trailing backslash.
	e := NewOsExecutor()
	if runtime.GOOS == "windows" {
		output, rc, err := e.Execute(context.Background(), `dir c:\`, "")
		require.NoError(t, err)
		assert.Equal(t, 0, rc)
		assert.NotEmpty(t, output)
	} else {
		output, rc, err := e.Execute(context.Background(), `echo 'c:\'`, "")
		require.NoError(t, err)
		assert.Equal(t, 0, rc)
		assert.Contains(t, output, `c:\`)
	}
}

func TestExecute_PathWithSpaces(t *testing.T) {
	e := NewOsExecutor()
	if runtime.GOOS == "windows" {
		// claude: paths with spaces must also work
		output, rc, err := e.Execute(context.Background(), `echo "C:\Program Files\"`, "")
		require.NoError(t, err)
		assert.Equal(t, 0, rc)
		assert.Contains(t, output, "Program Files")
	} else {
		output, rc, err := e.Execute(context.Background(), `echo "/path with spaces/"`, "")
		require.NoError(t, err)
		assert.Equal(t, 0, rc)
		assert.Contains(t, output, "path with spaces")
	}
}

func TestExecute_MultipleBackslashes(t *testing.T) {
	e := NewOsExecutor()
	if runtime.GOOS == "windows" {
		output, rc, err := e.Execute(context.Background(), `echo c:\users\`, "")
		require.NoError(t, err)
		assert.Equal(t, 0, rc)
		assert.Contains(t, output, `c:\users\`)
	} else {
		output, rc, err := e.Execute(context.Background(), `echo 'c:\users\'`, "")
		require.NoError(t, err)
		assert.Equal(t, 0, rc)
		assert.Contains(t, output, `c:\users\`)
	}
}

func TestExecute_ShellBuiltin(t *testing.T) {
	// claude: shell builtins (dir, copy, del on Windows; cd, pwd on Unix)
	// must work through the shell path
	e := NewOsExecutor()
	if runtime.GOOS == "windows" {
		_, rc, err := e.Execute(context.Background(), "dir", "")
		require.NoError(t, err)
		assert.Equal(t, 0, rc)
	} else {
		output, rc, err := e.Execute(context.Background(), "pwd", "")
		require.NoError(t, err)
		assert.Equal(t, 0, rc)
		assert.NotEmpty(t, output)
	}
}

func TestExecute_Pipe(t *testing.T) {
	// claude: pipes require shell interpretation
	e := NewOsExecutor()
	if runtime.GOOS == "windows" {
		output, rc, err := e.Execute(context.Background(), `echo hello | findstr hello`, "")
		require.NoError(t, err)
		assert.Equal(t, 0, rc)
		assert.Contains(t, output, "hello")
	} else {
		output, rc, err := e.Execute(context.Background(), "echo hello | grep hello", "")
		require.NoError(t, err)
		assert.Equal(t, 0, rc)
		assert.Contains(t, output, "hello")
	}
}

func TestExecute_Redirect(t *testing.T) {
	// claude: redirections require shell interpretation
	if runtime.GOOS == "windows" {
		t.Skip("redirect test for unix")
	}
	e := NewOsExecutor()
	output, rc, err := e.Execute(context.Background(), "echo redir 2>&1", "")
	require.NoError(t, err)
	assert.Equal(t, 0, rc)
	assert.Contains(t, output, "redir")
}

func TestExecute_EmptyArray(t *testing.T) {
	e := NewOsExecutor()
	_, _, err := e.Execute(context.Background(), []interface{}{}, "")
	require.Error(t, err)
}

func TestExecute_UnsupportedType(t *testing.T) {
	e := NewOsExecutor()
	_, _, err := e.Execute(context.Background(), 12345, "")
	require.Error(t, err)
}
