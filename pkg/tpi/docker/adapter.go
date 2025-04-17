package docker

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"

	"github.com/davidroman0O/turingpi/pkg/tpi/platform"
)

/// can use `docker context list`

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

	// Register finalizer to help clean up in case of unexpected termination
	adapter := &DockerAdapter{
		Container: container,
	}

	// Use runtime finalizer as a safety net (not primary cleanup mechanism)
	runtime.SetFinalizer(adapter, func(a *DockerAdapter) {
		if a.Container != nil {
			fmt.Printf("Warning: Finalizer cleaning up Docker container %s that wasn't properly closed\n",
				a.Container.ContainerID)
			a.Container.Cleanup()
		}
	})

	return adapter, nil
}

// ExecuteCommand runs a command in the Docker container
func (a *DockerAdapter) ExecuteCommand(cmd string) (string, error) {
	if a.Container == nil {
		return "", fmt.Errorf("container is nil, adapter may have been closed")
	}

	// Use bash -c to execute the command as a string to preserve quoting, pipes, etc.
	return a.Container.ExecuteCommand([]string{"bash", "-c", cmd})
}

// CopyFileToContainer copies a file from the host to the container
func (a *DockerAdapter) CopyFileToContainer(localPath, containerPath string) error {
	if a.Container == nil {
		return fmt.Errorf("container is nil, adapter may have been closed")
	}

	// Make sure both paths are specified
	if localPath == "" || containerPath == "" {
		return fmt.Errorf("both localPath and containerPath must be specified")
	}

	// Ensure local file exists
	if _, err := os.Stat(localPath); os.IsNotExist(err) {
		return fmt.Errorf("local file does not exist: %s", localPath)
	}

	// Use the docker cp command with proper escaping
	copyToDockerCmd := fmt.Sprintf("docker cp %s %s:%s",
		localPath,
		a.Container.Config.ContainerName,
		containerPath)

	// Use bash -c to execute the command
	copyCmd := exec.Command("bash", "-c", copyToDockerCmd)
	output, err := copyCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to copy file to Docker: %w\nOutput: %s", err, string(output))
	}

	return nil
}

// Cleanup cleans up Docker resources (convenience method, doesn't return error)
func (a *DockerAdapter) Cleanup() {
	if err := a.Close(); err != nil {
		fmt.Printf("Warning: Error during Docker cleanup: %v\n", err)
	}
}

// Close properly cleans up Docker resources and returns any error
// This is the preferred method to call explicitly when done with the adapter
func (a *DockerAdapter) Close() error {
	if a.Container == nil {
		return nil // Already closed or never initialized
	}

	err := a.Container.Cleanup()
	// Set Container to nil to prevent double cleanup and mark as closed
	a.Container = nil

	return err
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
