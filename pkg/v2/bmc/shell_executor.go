package bmc

import (
	"os/exec"
	"strings"
)

// ShellExecutor implements CommandExecutor by executing commands in the local shell
type ShellExecutor struct{}

// ExecuteCommand implements CommandExecutor interface
func (s *ShellExecutor) ExecuteCommand(command string) (stdout string, stderr string, err error) {
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
