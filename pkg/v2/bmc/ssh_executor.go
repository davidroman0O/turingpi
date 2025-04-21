package bmc

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

// SSHConfig holds the configuration for SSH connections
type SSHConfig struct {
	Host      string `json:"host"`
	Port      int    `json:"port"`
	User      string `json:"user"`
	Password  string `json:"password"`
	RemoteDir string `json:"remote_dir"`
}

// SSHExecutor implements CommandExecutor by executing commands over SSH on a remote Turing Pi cluster
type SSHExecutor struct {
	config SSHConfig
}

// NewSSHExecutorFromConfig creates a new SSHExecutor from a config file
func NewSSHExecutorFromConfig(configPath string) (CommandExecutor, error) {
	// Read and parse the SSH config file
	configFile, err := os.Open(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open SSH config file: %w", err)
	}
	defer configFile.Close()

	configData, err := io.ReadAll(configFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read SSH config file: %w", err)
	}

	var config SSHConfig
	if err := json.Unmarshal(configData, &config); err != nil {
		return nil, fmt.Errorf("failed to parse SSH config: %w", err)
	}

	return &SSHExecutor{
		config: config,
	}, nil
}

// ExecuteCommand implements CommandExecutor interface by running commands over SSH
func (s *SSHExecutor) ExecuteCommand(command string) (stdout string, stderr string, err error) {
	// Build the SSH command
	// Example: ssh user@host -p port "command"
	sshCmd := fmt.Sprintf("ssh %s@%s -p %d",
		s.config.User,
		s.config.Host,
		s.config.Port)

	// If password auth is used, we would need to use sshpass or similar tools
	// This is a simplified version - in production, consider using SSH keys or a Go SSH library
	if s.config.Password != "" {
		sshCmd = fmt.Sprintf("sshpass -p '%s' %s", s.config.Password, sshCmd)
	}

	// Add the actual command to execute remotely
	fullCmd := fmt.Sprintf("%s \"%s\"", sshCmd, command)

	// Execute the SSH command
	cmd := exec.Command("sh", "-c", fullCmd)
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

	// Trim trailing newlines for consistent behavior
	stdout = strings.TrimSuffix(stdout, "\n")
	stderr = strings.TrimSuffix(stderr, "\n")

	return stdout, stderr, err
}
