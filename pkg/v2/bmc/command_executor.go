package bmc

import (
	"os/exec"
	"strings"
)

// TpiCommandExecutor is a command executor that prefixes all commands with a specific binary name
type TpiCommandExecutor struct {
	binaryName string
}

// ExecuteCommand implements CommandExecutor interface by prefixing commands with the binary name
func (t *TpiCommandExecutor) ExecuteCommand(command string) (stdout string, stderr string, err error) {
	// Add the binary name prefix if the command doesn't already start with it
	if !strings.HasPrefix(command, t.binaryName+" ") {
		command = t.binaryName + " " + command
	}

	// Execute the command using shell
	cmd := exec.Command("sh", "-c", command)

	// Get stdout and stderr
	stdoutBytes, err := cmd.Output()
	stdout = string(stdoutBytes)

	// If there was an error, try to extract stderr from it
	var stderrBytes []byte
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			stderrBytes = exitErr.Stderr
		}
	}
	stderr = string(stderrBytes)

	// Trim any trailing newlines for consistent behavior
	stdout = strings.TrimSuffix(stdout, "\n")
	stderr = strings.TrimSuffix(stderr, "\n")

	return stdout, stderr, err
}

// NewCommandExecutor creates a new CommandExecutor that prefixes all commands with the specified binary name
func NewCommandExecutor(binaryName string) CommandExecutor {
	return &TpiCommandExecutor{
		binaryName: binaryName,
	}
}
