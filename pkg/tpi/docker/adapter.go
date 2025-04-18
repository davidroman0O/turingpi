package docker

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"sync"

	"github.com/davidroman0O/turingpi/pkg/tpi/platform"
	"github.com/docker/docker/client"
)

/// can use `docker context list`

// DockerAdapter provides an interface for imageops to interact with Docker
type DockerAdapter struct {
	// The Docker container instance
	Container *Container
	// Docker client for management operations
	client     *client.Client
	mu         sync.Mutex
	containers map[string]*Container
}

// NewAdapter creates a new Docker adapter with optional source, temp, and output directories
func NewAdapter(ctx context.Context, sourceDir, tempDir, outputDir string) (*DockerAdapter, error) {
	// Create Docker client
	cli, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		return nil, fmt.Errorf("failed to create docker client: %v", err)
	}

	// Create a Docker configuration
	config := platform.NewDefaultDockerConfig(sourceDir, tempDir, outputDir)

	// Create container
	container, err := New(config)
	if err != nil {
		cli.Close()
		return nil, fmt.Errorf("failed to create Docker container: %w", err)
	}

	// Create adapter
	adapter := &DockerAdapter{
		Container:  container,
		client:     cli,
		containers: make(map[string]*Container),
	}

	// Register the main container
	adapter.RegisterContainer(container)

	// Use runtime finalizer as a safety net (not primary cleanup mechanism)
	runtime.SetFinalizer(adapter, func(a *DockerAdapter) {
		if a.Container != nil {
			fmt.Printf("Warning: Finalizer cleaning up Docker container %s that wasn't properly closed\n",
				a.Container.ContainerID)
			a.Container.Cleanup()
		}
		if a.client != nil {
			a.client.Close()
		}
	})

	return adapter, nil
}

// ExecuteCommand runs a command in the Docker container
func (a *DockerAdapter) ExecuteCommand(ctx context.Context, cmd string) (string, error) {
	if a.Container == nil {
		return "", fmt.Errorf("container is nil, adapter may have been closed")
	}

	// Use bash -c to execute the command as a string to preserve quoting, pipes, etc.
	return a.Container.ExecuteCommand([]string{"bash", "-c", cmd})
}

// CopyFileToContainer copies a file from the host to the container
func (a *DockerAdapter) CopyFileToContainer(ctx context.Context, localPath, containerPath string) error {
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
	copyCmd := exec.CommandContext(ctx, "bash", "-c", copyToDockerCmd)
	output, err := copyCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to copy file to Docker: %w\nOutput: %s", err, string(output))
	}

	return nil
}

// Cleanup stops and removes all managed containers
func (a *DockerAdapter) Cleanup(ctx context.Context) error {
	if a.client == nil {
		return nil
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	var lastErr error
	for id, container := range a.containers {
		if err := container.Cleanup(); err != nil {
			lastErr = fmt.Errorf("failed to cleanup container %s: %v", id, err)
		}
		delete(a.containers, id)
	}
	return lastErr
}

// Close properly cleans up Docker resources and returns any error
func (a *DockerAdapter) Close() error {
	// First cleanup all managed containers
	if err := a.Cleanup(context.Background()); err != nil {
		return err
	}

	// Then cleanup the main container if it exists
	if a.Container != nil {
		if err := a.Container.Cleanup(); err != nil {
			return err
		}
		a.Container = nil
	}

	// Finally close the Docker client
	if a.client != nil {
		if err := a.client.Close(); err != nil {
			return err
		}
		a.client = nil
	}

	return nil
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

// RegisterContainer adds a container to the adapter's management
func (a *DockerAdapter) RegisterContainer(container *Container) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.containers[container.ContainerID] = container
}

// UnregisterContainer removes a container from the adapter's management
func (a *DockerAdapter) UnregisterContainer(containerID string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	delete(a.containers, containerID)
}
