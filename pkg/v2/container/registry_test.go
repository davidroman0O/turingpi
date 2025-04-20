package container

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestDockerRegistry_Create(t *testing.T) {
	// Create a new registry
	registry, err := NewDockerRegistry()
	if err != nil {
		t.Fatalf("Failed to create Docker registry: %v", err)
	}
	defer registry.Close()

	// Create a unique container name
	containerName := "test-registry-container-" + time.Now().Format("20060102150405")

	// Create a container config
	config := ContainerConfig{
		Image:      "alpine:latest",
		Name:       containerName,
		Command:    []string{"sleep", "infinity"},
		Privileged: false,
	}

	// Create a container
	ctx := context.Background()
	container, err := registry.Create(ctx, config)
	if err != nil {
		t.Fatalf("Failed to create container: %v", err)
	}

	// Make sure to clean up
	defer registry.Remove(ctx, container.ID())

	// Verify container was created with the correct name
	if container.ID() == "" {
		t.Errorf("Expected container ID to be non-empty")
	}

	// Start the container
	if err := container.Start(ctx); err != nil {
		t.Errorf("Failed to start container: %v", err)
	}

	// Get container by ID
	retrieved, err := registry.Get(ctx, container.ID())
	if err != nil {
		t.Errorf("Failed to get container by ID: %v", err)
	}
	if retrieved == nil || retrieved.ID() != container.ID() {
		t.Errorf("Retrieved container does not match created container")
	}
}

func TestDockerRegistry_List(t *testing.T) {
	// Create a new registry
	registry, err := NewDockerRegistry()
	if err != nil {
		t.Fatalf("Failed to create Docker registry: %v", err)
	}
	defer registry.Close()

	// Create a unique container name prefix
	prefix := "test-registry-list-" + time.Now().Format("20060102150405")

	// Create context
	ctx := context.Background()

	// Create multiple containers
	var createdContainers []Container
	for i := 0; i < 3; i++ {
		config := ContainerConfig{
			Image:      "alpine:latest",
			Name:       prefix + "-" + time.Now().Format("150405.000"),
			Command:    []string{"sleep", "infinity"},
			Privileged: false,
		}

		container, err := registry.Create(ctx, config)
		if err != nil {
			t.Fatalf("Failed to create container: %v", err)
		}
		createdContainers = append(createdContainers, container)
	}

	// Clean up containers when done
	defer func() {
		for _, c := range createdContainers {
			registry.Remove(ctx, c.ID())
		}
	}()

	// List containers
	containers, err := registry.List(ctx)
	if err != nil {
		t.Errorf("Failed to list containers: %v", err)
	}

	// Verify all created containers are in the list
	containerMap := make(map[string]bool)
	for _, c := range containers {
		containerMap[c.ID()] = true
	}

	for _, c := range createdContainers {
		if !containerMap[c.ID()] {
			t.Errorf("Container %s not found in list", c.ID())
		}
	}
}

func TestDockerRegistry_Stats(t *testing.T) {
	// Create a new registry
	registry, err := NewDockerRegistry()
	if err != nil {
		t.Fatalf("Failed to create Docker registry: %v", err)
	}
	defer registry.Close()

	// Create a unique container name
	containerName := "test-registry-stats-" + time.Now().Format("20060102150405")

	// Create a container config
	config := ContainerConfig{
		Image:      "alpine:latest",
		Name:       containerName,
		Command:    []string{"sleep", "infinity"},
		Privileged: false,
	}

	// Create a container
	ctx := context.Background()
	container, err := registry.Create(ctx, config)
	if err != nil {
		t.Fatalf("Failed to create container: %v", err)
	}
	defer registry.Remove(ctx, container.ID())

	// Start the container
	if err := container.Start(ctx); err != nil {
		t.Errorf("Failed to start container: %v", err)
	}

	// Get container stats
	stats, err := registry.Stats(ctx, container.ID())
	if err != nil {
		t.Errorf("Failed to get container stats: %v", err)
	}

	// Verify stats
	if stats.ID != container.ID() {
		t.Errorf("Expected stats ID to be %s, got %s", container.ID(), stats.ID)
	}

	// The image field might be either "alpine:latest" or a SHA256 digest
	validImage := strings.Contains(stats.Image, "alpine") || strings.HasPrefix(stats.Image, "sha256:")
	if !validImage {
		t.Errorf("Expected stats Image to be 'alpine:latest' or a SHA256 digest, got %s", stats.Image)
	}

	if !stats.Running {
		t.Errorf("Expected container to be running")
	}
}

func TestDockerRegistry_RemoveAll(t *testing.T) {
	// Create a new registry
	registry, err := NewDockerRegistry()
	if err != nil {
		t.Fatalf("Failed to create Docker registry: %v", err)
	}
	defer registry.Close()

	// Create a unique container name prefix
	prefix := "test-registry-removeall-" + time.Now().Format("20060102150405")

	// Create context
	ctx := context.Background()

	// Create multiple containers
	for i := 0; i < 3; i++ {
		config := ContainerConfig{
			Image:      "alpine:latest",
			Name:       prefix + "-" + time.Now().Format("150405.000"),
			Command:    []string{"sleep", "infinity"},
			Privileged: false,
		}

		_, err := registry.Create(ctx, config)
		if err != nil {
			t.Fatalf("Failed to create container: %v", err)
		}
	}

	// List containers to verify they were created
	containers, err := registry.List(ctx)
	if err != nil {
		t.Errorf("Failed to list containers: %v", err)
	}
	initialCount := len(containers)
	if initialCount == 0 {
		t.Errorf("Expected to have created containers, but found none")
	}

	// Remove all containers
	if err := registry.RemoveAll(ctx); err != nil {
		t.Errorf("Failed to remove all containers: %v", err)
	}

	// List containers again to verify they were removed
	containersAfter, err := registry.List(ctx)
	if err != nil {
		t.Errorf("Failed to list containers after removal: %v", err)
	}
	if len(containersAfter) > 0 {
		t.Errorf("Expected all containers to be removed, but found %d", len(containersAfter))
	}
}

// TestDockerRegistry_Integration performs a basic integration test of the Docker registry.
func TestDockerRegistry_Integration(t *testing.T) {
	// Create a new registry
	registry, err := NewDockerRegistry()
	if err != nil {
		t.Fatalf("Failed to create Docker registry: %v", err)
	}
	defer func() {
		if err := registry.Close(); err != nil {
			t.Logf("Failed to close registry: %v", err)
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Create a test container
	config := ContainerConfig{
		Image:   "alpine:latest",
		Name:    "registry-test-container",
		Command: []string{"sleep", "60"},
	}

	container, err := registry.Create(ctx, config)
	if err != nil {
		t.Fatalf("Failed to create container: %v", err)
	}

	// Start the container
	if err := container.Start(ctx); err != nil {
		t.Fatalf("Failed to start container: %v", err)
	}

	// Execute a command in the container
	output, err := container.Exec(ctx, []string{"echo", "hello world"})
	if err != nil {
		t.Fatalf("Failed to exec command: %v", err)
	}

	if !strings.Contains(output, "hello world") {
		t.Errorf("Expected output to contain 'hello world', got '%s'", output)
	}

	// List containers
	containers, err := registry.List(ctx)
	if err != nil {
		t.Fatalf("Failed to list containers: %v", err)
	}

	if len(containers) < 1 {
		t.Errorf("Expected at least one container, got %d", len(containers))
	}

	// Get container stats
	state, err := registry.Stats(ctx, container.ID())
	if err != nil {
		t.Fatalf("Failed to get container stats: %v", err)
	}

	if !state.Running {
		t.Errorf("Expected container to be running")
	}

	// Stop the container
	if err := container.Stop(ctx); err != nil {
		t.Fatalf("Failed to stop container: %v", err)
	}

	// Remove the container
	if err := registry.Remove(ctx, container.ID()); err != nil {
		t.Fatalf("Failed to remove container: %v", err)
	}
}
