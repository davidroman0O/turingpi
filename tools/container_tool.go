package tools

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/davidroman0O/turingpi/container"
)

// ContainerToolImpl is the implementation of the ContainerTool interface
type ContainerToolImpl struct {
	registry       container.Registry
	trackedIDs     map[string]bool
	trackedNamesMu sync.RWMutex
}

// NewContainerTool creates a new ContainerTool
func NewContainerTool(registry container.Registry) ContainerTool {
	tool := &ContainerToolImpl{
		registry:   registry,
		trackedIDs: make(map[string]bool),
	}

	// Initially discover and register all relevant containers in background
	go tool.discoverAndRegisterContainers()

	return tool
}

// discoverAndRegisterContainers discovers existing containers and registers them with the registry
func (t *ContainerToolImpl) discoverAndRegisterContainers() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Only look for turingpi containers
	cmd := exec.Command("docker", "ps", "-a", "--format", "{{.ID}}|{{.Names}}|{{.Image}}")
	output, err := cmd.Output()
	if err != nil {
		return // Silently fail if docker isn't available
	}

	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}

		parts := strings.Split(line, "|")
		if len(parts) < 3 {
			continue
		}

		id := parts[0]
		name := parts[1]
		image := parts[2]

		// Only register turingpi-related containers
		if strings.HasPrefix(name, "turingpi-") {
			t.registerExistingContainer(ctx, id, container.ContainerConfig{
				Name:  name,
				Image: image,
			})
		}
	}
}

// registerExistingContainer registers an existing container with the registry (private helper)
func (t *ContainerToolImpl) registerExistingContainer(ctx context.Context, containerID string, config container.ContainerConfig) (container.Container, error) {
	// Skip if we've already tracked this container
	t.trackedNamesMu.RLock()
	if t.trackedIDs[containerID] {
		t.trackedNamesMu.RUnlock()
		return t.registry.Get(ctx, containerID)
	}
	t.trackedNamesMu.RUnlock()

	// Register the container
	c, err := t.registry.RegisterExistingContainer(ctx, containerID, config)
	if err != nil {
		return nil, err
	}

	// Mark as tracked
	t.trackedNamesMu.Lock()
	t.trackedIDs[containerID] = true
	t.trackedNamesMu.Unlock()

	return c, nil
}

// CreateContainer creates a new container
func (t *ContainerToolImpl) CreateContainer(ctx context.Context, config container.ContainerConfig) (container.Container, error) {
	c, err := t.registry.Create(ctx, config)
	if err != nil {
		return nil, err
	}

	// Track the new container ID
	t.trackedNamesMu.Lock()
	t.trackedIDs[c.ID()] = true
	t.trackedNamesMu.Unlock()

	return c, nil
}

// GetContainer retrieves a container by ID
func (t *ContainerToolImpl) GetContainer(ctx context.Context, containerID string) (container.Container, error) {
	// First try to get from registry
	c, err := t.registry.Get(ctx, containerID)
	if err == nil {
		return c, nil
	}

	// If that fails, try to register it first (maybe it's an external container)
	// Get container info from Docker
	cmd := exec.Command("docker", "inspect", "--format", "{{.Name}}|{{.Config.Image}}", containerID)
	output, cmdErr := cmd.Output()
	if cmdErr != nil {
		// If docker inspect fails, return the original error
		return nil, err
	}

	parts := strings.Split(strings.TrimSpace(string(output)), "|")
	if len(parts) != 2 {
		return nil, err
	}

	// Clean container name (remove leading slash)
	name := parts[0]
	if strings.HasPrefix(name, "/") {
		name = name[1:]
	}

	// Try to register and return the container
	return t.registerExistingContainer(ctx, containerID, container.ContainerConfig{
		Name:  name,
		Image: parts[1],
	})
}

// ListContainers returns all managed containers
func (t *ContainerToolImpl) ListContainers(ctx context.Context) ([]container.Container, error) {
	// First make sure we've discovered all containers
	t.discoverAndRegisterContainers()

	// Then return the list from the registry
	return t.registry.List(ctx)
}

// StartContainer starts a container
func (t *ContainerToolImpl) StartContainer(ctx context.Context, containerID string) error {
	c, err := t.GetContainer(ctx, containerID)
	if err != nil {
		return err
	}
	return c.Start(ctx)
}

// StopContainer stops a container
func (t *ContainerToolImpl) StopContainer(ctx context.Context, containerID string) error {
	c, err := t.GetContainer(ctx, containerID)
	if err != nil {
		return err
	}
	return c.Stop(ctx)
}

// KillContainer forcefully stops a container
func (t *ContainerToolImpl) KillContainer(ctx context.Context, containerID string) error {
	c, err := t.GetContainer(ctx, containerID)
	if err != nil {
		return err
	}
	return c.Kill(ctx)
}

// PauseContainer pauses a container
func (t *ContainerToolImpl) PauseContainer(ctx context.Context, containerID string) error {
	c, err := t.GetContainer(ctx, containerID)
	if err != nil {
		return err
	}
	return c.Pause(ctx)
}

// UnpauseContainer unpauses a container
func (t *ContainerToolImpl) UnpauseContainer(ctx context.Context, containerID string) error {
	c, err := t.GetContainer(ctx, containerID)
	if err != nil {
		return err
	}
	return c.Unpause(ctx)
}

// RunCommand executes a command in a container
func (t *ContainerToolImpl) RunCommand(ctx context.Context, containerID string, cmd []string) (string, error) {
	c, err := t.GetContainer(ctx, containerID)
	if err != nil {
		return "", err
	}

	stdout, err := c.Exec(ctx, cmd)
	if err != nil {
		return "", err
	}

	return stdout, nil
}

// RunDetachedCommand executes a command in a container without waiting for output
func (t *ContainerToolImpl) RunDetachedCommand(ctx context.Context, containerID string, cmd []string) error {
	c, err := t.GetContainer(ctx, containerID)
	if err != nil {
		return err
	}
	return c.ExecDetached(ctx, cmd)
}

// CopyToContainer copies a file or directory to a container
func (t *ContainerToolImpl) CopyToContainer(ctx context.Context, containerID, hostPath, containerPath string) error {
	c, err := t.GetContainer(ctx, containerID)
	if err != nil {
		return err
	}

	return c.CopyTo(ctx, hostPath, containerPath)
}

// CopyFromContainer copies a file or directory from a container
func (t *ContainerToolImpl) CopyFromContainer(ctx context.Context, containerID, containerPath, hostPath string) error {
	c, err := t.GetContainer(ctx, containerID)
	if err != nil {
		return err
	}

	return c.CopyFrom(ctx, containerPath, hostPath)
}

// GetContainerLogs returns container logs
func (t *ContainerToolImpl) GetContainerLogs(ctx context.Context, containerID string) (io.ReadCloser, error) {
	c, err := t.GetContainer(ctx, containerID)
	if err != nil {
		return nil, err
	}
	return c.Logs(ctx)
}

// WaitForContainer waits for the container to exit
func (t *ContainerToolImpl) WaitForContainer(ctx context.Context, containerID string) (int, error) {
	c, err := t.GetContainer(ctx, containerID)
	if err != nil {
		return -1, err
	}
	return c.Wait(ctx)
}

// RemoveContainer removes a container
func (t *ContainerToolImpl) RemoveContainer(ctx context.Context, containerID string) error {
	// Try to get the container - if it's not found, it may not be registered
	_, err := t.GetContainer(ctx, containerID)
	if err != nil {
		// If the container isn't registered, try to remove it directly with Docker
		rmCmd := exec.Command("docker", "rm", "-f", containerID)
		if rmErr := rmCmd.Run(); rmErr == nil {
			// Removed successfully via Docker CLI
			t.trackedNamesMu.Lock()
			delete(t.trackedIDs, containerID)
			t.trackedNamesMu.Unlock()
			return nil
		}
		return err
	}

	// Remove from registry
	err = t.registry.Remove(ctx, containerID)
	if err == nil {
		// If successful, remove from tracked IDs
		t.trackedNamesMu.Lock()
		delete(t.trackedIDs, containerID)
		t.trackedNamesMu.Unlock()
	}
	return err
}

// RemoveAllContainers removes all managed containers
func (t *ContainerToolImpl) RemoveAllContainers(ctx context.Context) error {
	return t.registry.RemoveAll(ctx)
}

// GetContainerStats returns container statistics
func (t *ContainerToolImpl) GetContainerStats(ctx context.Context, containerID string) (*container.ContainerState, error) {
	return t.registry.Stats(ctx, containerID)
}

// CloseRegistry releases all resources and removes all containers
func (t *ContainerToolImpl) CloseRegistry() error {
	return t.registry.Close()
}

// EmergencyCleanup performs an immediate forceful cleanup of all test containers
// using direct Docker CLI commands for maximum reliability
func (t *ContainerToolImpl) EmergencyCleanup() error {
	// First try to use the registry's RemoveAll
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Try to clean up containers via registry
	_ = t.registry.RemoveAll(ctx)

	// Then perform a brute-force cleanup via Docker CLI
	err := t.CleanupTestContainers()

	// Reset the tracked IDs
	t.trackedNamesMu.Lock()
	t.trackedIDs = make(map[string]bool)
	t.trackedNamesMu.Unlock()

	return err
}

// EnsureNoTestContainers verifies that no test containers are left running
func (t *ContainerToolImpl) EnsureNoTestContainers() error {
	// Test container name patterns to look for
	testPrefixes := []string{
		"turingpi-test-",
		"test-registry-",
		"registry-test-",
		"test-docker-",
	}

	for _, prefix := range testPrefixes {
		// Find all containers with this prefix
		cleanCmd := exec.Command("docker", "ps", "-a", "-q", "--filter", fmt.Sprintf("name=%s", prefix))
		output, err := cleanCmd.Output()
		if err == nil && len(output) > 0 {
			containerList := strings.Split(strings.TrimSpace(string(output)), "\n")
			if len(containerList) > 0 && containerList[0] != "" {
				return fmt.Errorf("found %d test containers with prefix '%s' that were not cleaned up",
					len(containerList), prefix)
			}
		}
	}

	return nil
}

// CleanupTestContainers removes any containers matching known test patterns
func (t *ContainerToolImpl) CleanupTestContainers() error {
	// Test container name patterns to look for
	testPrefixes := []string{
		"turingpi-test-",
		"test-registry-",
		"registry-test-",
		"test-docker-",
	}

	var lastErr error
	for _, prefix := range testPrefixes {
		// Find all containers with this prefix
		cleanCmd := exec.Command("docker", "ps", "-a", "-q", "--filter", fmt.Sprintf("name=%s", prefix))
		output, err := cleanCmd.Output()
		if err != nil {
			lastErr = err
			continue
		}

		if len(output) > 0 {
			containerList := strings.Split(strings.TrimSpace(string(output)), "\n")
			for _, id := range containerList {
				if id != "" {
					rmCmd := exec.Command("docker", "rm", "-f", id)
					if err := rmCmd.Run(); err != nil {
						lastErr = err
					} else {
						// If removed successfully, remove from tracked IDs
						t.trackedNamesMu.Lock()
						delete(t.trackedIDs, id)
						t.trackedNamesMu.Unlock()
					}
				}
			}
		}
	}

	return lastErr
}

// ContainerToolAdapter adapts between the ContainerTool interface and the container.Registry interface
type ContainerToolAdapter struct {
	tool ContainerTool
}

// NewContainerToolAdapter creates a new adapter that converts a ContainerTool to a container.Registry
func NewContainerToolAdapter(tool ContainerTool) container.Registry {
	return &ContainerToolAdapter{
		tool: tool,
	}
}

// Create creates a new container
func (a *ContainerToolAdapter) Create(ctx context.Context, config container.ContainerConfig) (container.Container, error) {
	// Use the CreateContainer method from the ContainerTool interface
	container, err := a.tool.CreateContainer(ctx, config)
	if err != nil {
		return nil, err
	}

	// Start the container (since Create is expected to create and start)
	err = a.tool.StartContainer(ctx, container.ID())
	if err != nil {
		// Clean up if start fails
		a.tool.RemoveContainer(ctx, container.ID())
		return nil, err
	}

	return container, nil
}

// Get returns a container by ID
func (a *ContainerToolAdapter) Get(ctx context.Context, id string) (container.Container, error) {
	return a.tool.GetContainer(ctx, id)
}

// List returns all managed containers
func (a *ContainerToolAdapter) List(ctx context.Context) ([]container.Container, error) {
	return a.tool.ListContainers(ctx)
}

// Remove removes a container
func (a *ContainerToolAdapter) Remove(ctx context.Context, id string) error {
	return a.tool.RemoveContainer(ctx, id)
}

// RemoveAll removes all managed containers
func (a *ContainerToolAdapter) RemoveAll(ctx context.Context) error {
	return a.tool.RemoveAllContainers(ctx)
}

// Stats returns container statistics
func (a *ContainerToolAdapter) Stats(ctx context.Context, id string) (*container.ContainerState, error) {
	return a.tool.GetContainerStats(ctx, id)
}

// RegisterExistingContainer registers an existing container with the registry
func (a *ContainerToolAdapter) RegisterExistingContainer(ctx context.Context, id string, config container.ContainerConfig) (container.Container, error) {
	// There's no direct analog in ContainerTool, so we'll try to get the container
	return a.tool.GetContainer(ctx, id)
}

// Close releases all resources and removes all containers
func (a *ContainerToolAdapter) Close() error {
	return a.tool.CloseRegistry()
}
