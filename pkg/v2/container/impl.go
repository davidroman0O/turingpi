package container

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/davidroman0O/turingpi/pkg/tpi/platform"
)

// DockerAdapter provides compatibility with the old Docker container API
type DockerAdapter struct {
	registry  Registry
	container Container
	config    *platform.DockerExecutionConfig
}

// NewDockerAdapter creates a new adapter with the given configuration
func NewDockerAdapter(config *platform.DockerExecutionConfig) (*DockerAdapter, error) {
	registry, err := NewDockerRegistry()
	if err != nil {
		return nil, fmt.Errorf("failed to create Docker registry: %w", err)
	}

	// Convert config to new format
	containerConfig := ContainerConfig{
		Image:      config.DockerImage,
		Name:       config.ContainerName,
		Command:    []string{"sleep", "infinity"},
		WorkDir:    "/workspace",
		Privileged: true,
		Capabilities: []string{
			"SYS_ADMIN",
			"MKNOD",
		},
		Mounts: map[string]string{
			config.SourceDir: "/source:ro",
			config.TempDir:   "/tmp",
			config.OutputDir: "/output",
		},
	}

	// Add additional mounts
	for hostPath, containerPath := range config.AdditionalMounts {
		containerConfig.Mounts[hostPath] = containerPath
	}

	// Create container
	ctx := context.Background()
	container, err := registry.Create(ctx, containerConfig)
	if err != nil {
		registry.Close()
		return nil, fmt.Errorf("failed to create container: %w", err)
	}

	// Start container
	if err := container.Start(ctx); err != nil {
		container.Cleanup(ctx)
		registry.Close()
		return nil, fmt.Errorf("failed to start container: %w", err)
	}

	return &DockerAdapter{
		registry:  registry,
		container: container,
		config:    config,
	}, nil
}

// ExecuteCommand executes a command in the container and returns the output
func (a *DockerAdapter) ExecuteCommand(cmd []string) (string, error) {
	ctx := context.Background()
	return a.container.Exec(ctx, cmd)
}

// CopyFileToContainer copies a file from the host to the container
func (a *DockerAdapter) CopyFileToContainer(srcPath, destPath string) error {
	ctx := context.Background()
	return a.container.CopyTo(ctx, srcPath, destPath)
}

// Cleanup removes the container and releases resources
func (a *DockerAdapter) Cleanup() error {
	// Get container ID for old API compatibility
	containerID := a.container.ID()

	ctx := context.Background()
	if err := a.container.Cleanup(ctx); err != nil {
		return fmt.Errorf("failed to cleanup container: %w", err)
	}

	if err := a.registry.Close(); err != nil {
		return fmt.Errorf("failed to close registry: %w", err)
	}

	fmt.Printf("Container %s has been removed\n", containerID)
	return nil
}

// GetContainerID returns the container ID
func (a *DockerAdapter) GetContainerID() string {
	return a.container.ID()
}

// GetContainerName returns the container name
func (a *DockerAdapter) GetContainerName() string {
	return a.config.ContainerName
}

// ExecuteDetached executes a command in the container without waiting for output
func (a *DockerAdapter) ExecuteDetached(cmd []string) error {
	ctx := context.Background()
	return a.container.ExecDetached(ctx, cmd)
}

// CopyDirectoryToContainer copies a directory from the host to the container
func (a *DockerAdapter) CopyDirectoryToContainer(srcDir, destDir string) error {
	ctx := context.Background()

	// First make sure the destination directory exists in the container
	if err := a.container.ExecDetached(ctx, []string{"mkdir", "-p", destDir}); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	// Walk through the directory and copy each file
	// This is a simplified implementation
	return filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories, they'll be created when copying files
		if info.IsDir() {
			return nil
		}

		// Calculate relative path
		relPath, err := filepath.Rel(srcDir, path)
		if err != nil {
			return fmt.Errorf("failed to get relative path: %w", err)
		}

		// Create destination path
		destPath := filepath.Join(destDir, relPath)

		// Make sure parent directory exists
		parentDir := filepath.Dir(destPath)
		if err := a.container.ExecDetached(ctx, []string{"mkdir", "-p", parentDir}); err != nil {
			return fmt.Errorf("failed to create parent directory %s: %w", parentDir, err)
		}

		// Copy file
		return a.container.CopyTo(ctx, path, destPath)
	})
}
