package docker

import (
	"fmt"
	"os/exec"

	"github.com/davidroman0O/turingpi/pkg/tpi/platform"
)

// DockerAdapter provides an interface for imageops to interact with Docker
type DockerAdapter struct {
	// The Docker container instance
	Container *Container
}

// NewAdapter creates a new Docker adapter
func NewAdapter(sourceDir, tempDir, outputDir string) (*DockerAdapter, error) {
	// Create a Docker configuration first
	config := platform.NewDefaultDockerConfig(sourceDir, tempDir, outputDir)

	return NewAdapterWithConfig(config)
}

// NewAdapterWithConfig creates a new Docker adapter with a specific configuration
func NewAdapterWithConfig(config *platform.DockerExecutionConfig) (*DockerAdapter, error) {
	// Pass the config to the New function
	container, err := New(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create Docker container: %w", err)
	}

	return &DockerAdapter{
		Container: container,
	}, nil
}

// ExecuteCommand runs a command in the Docker container
func (a *DockerAdapter) ExecuteCommand(cmd string) (string, error) {
	// Use bash -c to execute the command as a string to preserve quoting, pipes, etc.
	return a.Container.ExecuteCommand([]string{"bash", "-c", cmd})
}

// CopyFileToContainer copies a file from the host to the container
func (a *DockerAdapter) CopyFileToContainer(localPath, containerPath string) error {
	// Use the docker cp command
	copyToDockerCmd := fmt.Sprintf("docker cp %s %s:%s",
		localPath,
		a.Container.Config.ContainerName,
		containerPath)

	copyCmd := exec.Command("bash", "-c", copyToDockerCmd)
	output, err := copyCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to copy file to Docker: %w\nOutput: %s", err, string(output))
	}
	return nil
}

// Cleanup cleans up Docker resources
func (a *DockerAdapter) Cleanup() {
	if a.Container != nil {
		a.Container.Cleanup()
	}
}

// GetContainerID returns the ID of the current Docker container
func (a *DockerAdapter) GetContainerID() string {
	if a.Container == nil {
		return ""
	}
	return a.Container.ContainerID
}

// GetContainerName returns the name of the current Docker container
func (a *DockerAdapter) GetContainerName() string {
	if a.Container == nil || a.Container.Config == nil {
		return ""
	}
	return a.Container.Config.ContainerName
}
