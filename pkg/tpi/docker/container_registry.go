package docker

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"runtime"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
)

// ContainerRegistry keeps track of all containers created by this application
// and provides a mechanism to clean them up, even in case of panic or interruption
type ContainerRegistry struct {
	mu             sync.Mutex
	containers     map[string]bool    // Map of container IDs to track
	dockerClient   *client.Client     // Docker client for cleanup
	cleanupHandler func()             // Handler to run on cleanup
	signalSetup    bool               // Whether signal handlers have been set up
	ctx            context.Context    // Context for cleanup operations
	cancel         context.CancelFunc // Cancel function for the context
}

var (
	// Global registry instance
	globalRegistry *ContainerRegistry
	registryOnce   sync.Once
)

// GetRegistry returns the global container registry instance
func GetRegistry() *ContainerRegistry {
	registryOnce.Do(func() {
		ctx, cancel := context.WithCancel(context.Background())
		globalRegistry = &ContainerRegistry{
			containers:  make(map[string]bool),
			ctx:         ctx,
			cancel:      cancel,
			signalSetup: false,
		}

		// Initialize Docker client for cleanup
		var cli *client.Client
		var err error

		// Get the Docker context details first
		contextInfo, contextErr := getDockerContextDetails()
		if contextErr == nil && contextInfo.Host != "" {
			fmt.Printf("Registry: Using Docker host from context: %s\n", contextInfo.Host)

			// Try to create client with explicit context host
			cli, err = client.NewClientWithOpts(
				client.FromEnv,
				client.WithAPIVersionNegotiation(),
				client.WithHost(contextInfo.Host),
			)

			if err != nil {
				fmt.Printf("Registry: Failed to connect with context host, falling back: %v\n", err)
			}
		}

		// If context approach didn't work, try default options
		if cli == nil {
			// Check if DOCKER_HOST is explicitly set
			dockerHost := os.Getenv("DOCKER_HOST")
			if dockerHost != "" {
				fmt.Printf("Registry: Using DOCKER_HOST from environment: %s\n", dockerHost)
			}

			// Try with default environment settings
			cli, err = client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
		}

		if err != nil {
			fmt.Printf("Warning: Failed to initialize Docker client for registry: %v\n", err)
		} else {
			globalRegistry.dockerClient = cli
		}

		// Set up signal handlers
		globalRegistry.setupSignalHandlers()

		// Set up finalizer to ensure client is closed
		runtime.SetFinalizer(globalRegistry, func(r *ContainerRegistry) {
			if r.dockerClient != nil {
				r.dockerClient.Close()
			}
			if r.cancel != nil {
				r.cancel()
			}
		})
	})
	return globalRegistry
}

// RegisterContainer adds a container ID to the registry
func (r *ContainerRegistry) RegisterContainer(containerID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.containers[containerID] = true
	fmt.Printf("Registry: Registered container %s (total: %d)\n", containerID, len(r.containers))
}

// UnregisterContainer removes a container ID from the registry
func (r *ContainerRegistry) UnregisterContainer(containerID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.containers, containerID)
	fmt.Printf("Registry: Unregistered container %s (remaining: %d)\n", containerID, len(r.containers))
}

// SetCleanupHandler sets a function to be called during cleanup
func (r *ContainerRegistry) SetCleanupHandler(handler func()) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.cleanupHandler = handler
}

// setupSignalHandlers registers OS signal handlers for graceful cleanup
func (r *ContainerRegistry) setupSignalHandlers() {
	if r.signalSetup {
		return
	}

	r.signalSetup = true
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)

	go func() {
		defer func() {
			if r := recover(); r != nil {
				fmt.Printf("Registry: Recovered from panic in signal handler: %v\n", r)
			}
		}()

		sig := <-c
		fmt.Printf("Registry: Received signal %v, cleaning up containers...\n", sig)
		r.CleanupAll()

		// Re-send the signal to allow normal termination after cleanup
		signal.Stop(c)
		syscall.Kill(os.Getpid(), sig.(syscall.Signal))
	}()
}

// cleanupContainer handles the cleanup of a single container with its own timeout context
func (r *ContainerRegistry) cleanupContainer(containerID string) error {
	// Create a timeout context specifically for this container
	ctx, cancel := context.WithTimeout(r.ctx, 10*time.Second)
	defer cancel()

	fmt.Printf("Registry: Stopping container %s...\n", containerID)

	// Try SDK method first
	if r.dockerClient != nil {
		// Try to stop container with timeout
		stopTimeout := 5 // seconds
		err := r.dockerClient.ContainerStop(ctx, containerID, container.StopOptions{Timeout: &stopTimeout})
		if err != nil {
			fmt.Printf("Registry: Error stopping container %s via SDK: %v (will try force remove)\n", containerID, err)
			// Continue to removal attempt even if stop fails
		}

		// Force remove the container using SDK
		fmt.Printf("Registry: Removing container %s via SDK...\n", containerID)
		err = r.dockerClient.ContainerRemove(ctx, containerID, container.RemoveOptions{Force: true})
		if err != nil {
			fmt.Printf("Registry: Error removing container %s via SDK: %v (will try CLI fallback)\n", containerID, err)
		} else {
			fmt.Printf("Registry: Successfully removed container %s via SDK\n", containerID)
			// Verify container is actually gone
			if r.verifyContainerRemoved(containerID) {
				return nil // Container confirmed removed
			}
			fmt.Printf("Registry: Container %s still exists after SDK removal, trying CLI fallback\n", containerID)
		}
	}

	// Fallback to CLI if SDK failed or we don't have a client
	fmt.Printf("Registry: Attempting to remove container %s via CLI...\n", containerID)
	cmd := exec.CommandContext(ctx, "docker", "rm", "-f", containerID)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to remove container via CLI: %v, output: %s", err, string(output))
	}

	fmt.Printf("Registry: Successfully removed container %s via CLI\n", containerID)
	return nil
}

// CleanupAll removes all tracked containers
func (r *ContainerRegistry) CleanupAll() {
	r.mu.Lock()
	defer r.mu.Unlock()
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("Registry: Recovered from panic during cleanup: %v\n", r)
		}
		// Always run cleanup handler, even if there's a panic
		if r.cleanupHandler != nil {
			r.cleanupHandler()
		}
	}()

	if len(r.containers) == 0 {
		return
	}

	fmt.Printf("Registry: Cleaning up %d containers...\n", len(r.containers))

	// Make a copy of the container IDs
	containerIDs := make([]string, 0, len(r.containers))
	for containerID := range r.containers {
		containerIDs = append(containerIDs, containerID)
	}

	// Clean up each container
	for _, containerID := range containerIDs {
		err := r.cleanupContainer(containerID)
		if err != nil {
			fmt.Printf("Registry: Warning: Failed to clean up container %s: %v\n", containerID, err)
		}
		delete(r.containers, containerID)
	}
}

// verifyContainerRemoved checks if a container has been properly removed
func (r *ContainerRegistry) verifyContainerRemoved(containerID string) bool {
	ctx, cancel := context.WithTimeout(r.ctx, 5*time.Second)
	defer cancel()

	// First try using the SDK if available
	if r.dockerClient != nil {
		_, err := r.dockerClient.ContainerInspect(ctx, containerID)
		if client.IsErrNotFound(err) {
			// Container confirmed not found
			return true
		}
	}

	// Fallback to CLI
	cmd := exec.CommandContext(ctx, "docker", "ps", "-a", "--filter", "id="+containerID, "--format", "{{.ID}}")
	output, err := cmd.CombinedOutput()
	if err != nil {
		// If cli command fails, we can't verify - assume not removed to be safe
		fmt.Printf("Registry: Failed to verify container %s removal: %v\n", containerID, err)
		return false
	}

	// If output is empty, the container doesn't exist
	return len(output) == 0
}

// GetContainerCount returns the number of tracked containers
func (r *ContainerRegistry) GetContainerCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.containers)
}

// Close cleans up the registry resources
func (r *ContainerRegistry) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Clean up all containers first
	r.CleanupAll()

	// Close the Docker client
	if r.dockerClient != nil {
		if err := r.dockerClient.Close(); err != nil {
			return fmt.Errorf("failed to close Docker client: %w", err)
		}
		r.dockerClient = nil
	}

	// Cancel the context
	if r.cancel != nil {
		r.cancel()
		r.cancel = nil
	}

	return nil
}
