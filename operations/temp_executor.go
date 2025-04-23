package operations

import (
	"context"
	"fmt"
	"os"
	"runtime"

	"github.com/davidroman0O/turingpi/container"
)

// TemporaryContainerExecutor creates a new temporary container for each operation
type TemporaryContainerExecutor struct {
	containerRegistry container.Registry
}

// NewTemporaryContainerExecutor creates a new TemporaryContainerExecutor
func NewTemporaryContainerExecutor(containerRegistry container.Registry) CommandExecutor {
	// If we're on Linux, use the native executor
	if runtime.GOOS == "linux" {
		return &NativeExecutor{}
	}

	// If we're on a different OS, use the temporary container executor
	return &TemporaryContainerExecutor{
		containerRegistry: containerRegistry,
	}
}

// createTemporaryContainer creates a temporary container for an operation
func (e *TemporaryContainerExecutor) createTemporaryContainer(ctx context.Context) (container.Container, error) {
	// Get current working directory to mount
	pwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("failed to get current working directory: %w", err)
	}

	// Create a unique name for the container
	containerName := fmt.Sprintf("turingpi-op-%d", os.Getpid())

	// Set up container config
	containerConfig := container.ContainerConfig{
		Name:       containerName,
		Image:      "ubuntu:latest",
		Command:    []string{"sleep", "10"}, // Short timeout in case cleanup fails
		Privileged: true,                    // Enable privileged mode for operations that require it
		Capabilities: []string{
			"SYS_ADMIN", // Required for mount operations
		},
		// Mount volumes with correct binding modes
		Mounts: map[string]string{
			pwd: pwd, // Mount current working directory
		},
		// Set working directory to match host
		WorkDir: pwd,
	}

	// Create the container
	containerInstance, err := e.containerRegistry.Create(ctx, containerConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create temporary container: %w", err)
	}

	// Start the container - this is handled automatically by Create in most registry implementations
	if err := containerInstance.Start(ctx); err != nil {
		// Clean up if start fails
		e.containerRegistry.Remove(ctx, containerInstance.ID())
		return nil, fmt.Errorf("failed to start temporary container: %w", err)
	}

	return containerInstance, nil
}

// Execute implements CommandExecutor.Execute for temporary container execution
func (e *TemporaryContainerExecutor) Execute(ctx context.Context, name string, args ...string) ([]byte, error) {
	// Create a temporary container
	containerInstance, err := e.createTemporaryContainer(ctx)
	if err != nil {
		return nil, err
	}

	// Ensure cleanup
	defer e.containerRegistry.Remove(ctx, containerInstance.ID())

	// Create a container executor for this container
	executor := NewContainerExecutor(containerInstance)

	// Execute the command using the container executor
	return executor.Execute(ctx, name, args...)
}

// ExecuteWithInput implements CommandExecutor.ExecuteWithInput for temporary container execution
func (e *TemporaryContainerExecutor) ExecuteWithInput(ctx context.Context, input string, name string, args ...string) ([]byte, error) {
	// Create a temporary container
	containerInstance, err := e.createTemporaryContainer(ctx)
	if err != nil {
		return nil, err
	}

	// Ensure cleanup
	defer e.containerRegistry.Remove(ctx, containerInstance.ID())

	// Create a container executor for this container
	executor := NewContainerExecutor(containerInstance)

	// Execute the command using the container executor
	return executor.ExecuteWithInput(ctx, input, name, args...)
}

// ExecuteInPath implements CommandExecutor.ExecuteInPath for temporary container execution
func (e *TemporaryContainerExecutor) ExecuteInPath(ctx context.Context, dir string, name string, args ...string) ([]byte, error) {
	// Create a temporary container
	containerInstance, err := e.createTemporaryContainer(ctx)
	if err != nil {
		return nil, err
	}

	// Ensure cleanup
	defer e.containerRegistry.Remove(ctx, containerInstance.ID())

	// Create a container executor for this container
	executor := NewContainerExecutor(containerInstance)

	// Execute the command using the container executor
	return executor.ExecuteInPath(ctx, dir, name, args...)
}
