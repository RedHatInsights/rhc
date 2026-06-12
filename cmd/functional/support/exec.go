package support

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"

	"github.com/google/shlex"
)

// CommandResult holds the captured output of a command execution.
type CommandResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

// RunCommand executes a shell command string and returns its result.
// The command runs under the clean environment established by ResetEnvironment.
//
// Examples:
//
//	RunCommand(`rhc configure features status`)
//	RunCommand(`rhc configure features set --path "/var/lib/rhc/some file.json"`)
func RunCommand(command string) (*CommandResult, error) {
	return RunCommandInDir(command, "")
}

// RunCommandInDir executes a shell command string in the given working directory
// and returns its result.  When dir is empty the process inherits its working
// directory.  The command runs under the clean environment established by
// ResetEnvironment.
//
// Examples:
//
//	RunCommandInDir(`/usr/libexec/rhc/collector/com.redhat.minimal`, "/tmp/rhc-transparency-test")
func RunCommandInDir(command, dir string) (*CommandResult, error) {
	parts, err := shlex.Split(command)
	if err != nil {
		return nil, fmt.Errorf("failed to parse command %q: %w", command, err)
	}
	if len(parts) == 0 {
		return nil, fmt.Errorf("empty command")
	}

	cmd := exec.Command(parts[0], parts[1:]...)
	if dir != "" {
		cmd.Dir = dir
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err = cmd.Run()

	result := &CommandResult{
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		ExitCode: 0,
	}

	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			result.ExitCode = exitErr.ExitCode()
		}
	}

	return result, nil
}
