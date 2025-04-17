package docker

import (
	"fmt"
	"os/exec"

	"github.com/davidroman0O/turingpi/pkg/tpi/platform"
)

// isDockerAvailable checks if Docker is actually available using Docker CLI
// Used for error reporting only, not for skipping tests
func isDockerAvailable() bool {
	// Primary check - try docker version command
	cmd := exec.Command("docker", "version")
	if err := cmd.Run(); err != nil {
		fmt.Println("Docker not available:", err)
		return false
	}
	return true
}

// getDockerContextInfo tries to get Docker host info from current context
// This mimics the approach in the container.go implementation
func getDockerContextInfo() (string, error) {
	cmd := exec.Command("docker", "context", "inspect", "--format", "{{.Endpoints.docker.Host}}")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to get Docker context: %w", err)
	}
	return string(output), nil
}

// createTestConfig creates a test Docker config for testing
func createTestConfig() *platform.DockerExecutionConfig {
	return &platform.DockerExecutionConfig{
		DockerImage:      "alpine:latest",
		ContainerName:    "turingpi-test-container",
		AdditionalMounts: map[string]string{},
	}
}

// MockContainer can be used for testing without actual Docker
type MockContainer struct {
	ID         string
	Name       string
	IsRunning  bool
	cmdOutputs map[string]string
	cmdErrors  map[string]error
}

// NewMockContainer creates a mock container for testing
func NewMockContainer() *MockContainer {
	return &MockContainer{
		ID:         "mock-container-id",
		Name:       "mock-container",
		IsRunning:  true,
		cmdOutputs: make(map[string]string),
		cmdErrors:  make(map[string]error),
	}
}

// SetCommandOutput sets the expected output for a command
func (m *MockContainer) SetCommandOutput(cmd string, output string) {
	m.cmdOutputs[cmd] = output
}

// SetCommandError sets the expected error for a command
func (m *MockContainer) SetCommandError(cmd string, err error) {
	m.cmdErrors[err.Error()] = err
}

// ExecuteCommand simulates running a command in the container
func (m *MockContainer) ExecuteCommand(cmd []string) (string, error) {
	if !m.IsRunning {
		return "", &exec.ExitError{}
	}

	cmdStr := ""
	if len(cmd) > 0 {
		cmdStr = cmd[0]
		if len(cmd) > 1 {
			cmdStr = cmd[0] + " " + cmd[1]
		}
	}

	if output, ok := m.cmdOutputs[cmdStr]; ok {
		return output, nil
	}

	if err, ok := m.cmdErrors[cmdStr]; ok {
		return "", err
	}

	// Default to echo command success for echo commands
	if len(cmd) >= 2 && cmd[0] == "echo" {
		return cmd[1], nil
	}

	return "mock output", nil
}

// Cleanup simulates cleaning up the container
func (m *MockContainer) Cleanup() error {
	m.IsRunning = false
	return nil
}

// GetContainerID returns the mock container ID
func (m *MockContainer) GetContainerID() string {
	return m.ID
}

// GetContainerName returns the mock container name
func (m *MockContainer) GetContainerName() string {
	return m.Name
}
