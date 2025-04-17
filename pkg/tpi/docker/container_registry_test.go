package docker

import (
	"context"
	"fmt"
	"os/exec"
	"testing"
	"time"

	"github.com/docker/docker/client"

	"github.com/davidroman0O/turingpi/pkg/tpi/platform"
)

// verifyContainerDeleted checks if a container has been deleted with retries
// It will retry the check a few times with a delay between attempts
func verifyContainerDeleted(t *testing.T, containerID string, maxRetries int) bool {
	t.Logf("Verifying deletion of container %s with up to %d retries", containerID, maxRetries)

	for i := 0; i < maxRetries; i++ {
		// Check using docker ps command
		cmd := exec.Command("docker", "ps", "-a", "--filter", "id="+containerID, "--format", "{{.ID}}")
		output, err := cmd.CombinedOutput()

		if err != nil {
			t.Logf("Error running docker ps for container %s: %v", containerID, err)
			// Don't consider command error as container deleted, continue retrying
		} else if len(output) == 0 {
			// Container not found, which means it was deleted
			t.Logf("Container %s confirmed deleted on attempt %d", containerID, i+1)
			return true
		}

		// If we reach here, the container still exists or there was an error
		// If not the last attempt, wait before trying again
		if i < maxRetries-1 {
			t.Logf("Container %s still exists or check failed, waiting before retry %d", containerID, i+1)
			time.Sleep(1 * time.Second)
		}
	}

	// If we exhausted all retries, check one last time and return the result
	cmd := exec.Command("docker", "ps", "-a", "--filter", "id="+containerID, "--format", "{{.ID}}")
	output, _ := cmd.CombinedOutput()
	deleted := len(output) == 0

	if !deleted {
		t.Logf("Container %s still exists after %d verification attempts", containerID, maxRetries)
		// Try to forcefully remove it to avoid leaving test containers around
		t.Logf("Attempting force removal of container %s", containerID)
		cleanupCmd := exec.Command("docker", "rm", "-f", containerID)
		if err := cleanupCmd.Run(); err != nil {
			t.Logf("Error during force removal of container %s: %v", containerID, err)
		}
	}

	return deleted
}

func TestContainerRegistry(t *testing.T) {
	// Get registry instance
	registry := GetRegistry()

	// Make sure it's not nil
	if registry == nil {
		t.Fatal("Registry should not be nil")
	}

	// Make sure it has a Docker client
	if registry.dockerClient == nil {
		t.Skip("Docker client not available, skipping test")
	}

	// Test container registration
	initialCount := registry.GetContainerCount()
	fakeContainerID := "fake-container-id-for-testing"
	registry.RegisterContainer(fakeContainerID)

	// Verify container was registered
	if registry.GetContainerCount() != initialCount+1 {
		t.Errorf("Expected container count to increase after registration")
	}

	// Test container unregistration
	registry.UnregisterContainer(fakeContainerID)

	// Verify container was unregistered
	if registry.GetContainerCount() != initialCount {
		t.Errorf("Expected container count to decrease after unregistration")
	}
}

func TestContainerRegistrationOnCreation(t *testing.T) {
	// Skip if Docker is not available
	if !platform.DockerAvailable() {
		t.Skip("Docker not available, skipping test")
	}

	// Get initial container count
	registry := GetRegistry()
	initialCount := registry.GetContainerCount()

	// Create a container with a unique name
	containerName := "turingpi-registry-test-" + time.Now().Format("20060102150405")
	config := &platform.DockerExecutionConfig{
		DockerImage:            "alpine:latest",
		ContainerName:          containerName,
		UseUniqueContainerName: true,
	}

	// Create the container
	container, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create container: %v", err)
	}

	// Verify container actually exists in Docker
	cmd := exec.Command("docker", "ps", "-a", "--filter", "id="+container.ContainerID, "--format", "{{.ID}}")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Errorf("Failed to verify container %s exists: %v", container.ContainerID, err)
	}
	if len(output) == 0 {
		t.Errorf("Container %s was not found in docker ps output", container.ContainerID)
	}

	// Verify container was registered in the registry
	if registry.GetContainerCount() != initialCount+1 {
		t.Errorf("Expected container count to increase after container creation")
	}

	// Clean up the container
	defer container.Cleanup()

	// Test cleanup removes the container
	container.Cleanup()

	// Verify container was unregistered from the registry
	if registry.GetContainerCount() != initialCount {
		t.Errorf("Expected container count to decrease after cleanup")
	}

	// Verify container was actually removed from Docker (using SDK)
	cli, _ := client.NewClientWithOpts(client.FromEnv)
	_, err = cli.ContainerInspect(context.Background(), container.ContainerID)
	if err == nil {
		t.Errorf("Container should have been removed from Docker according to SDK")
	}

	// Triple check with the verifyContainerDeleted function (includes retries)
	if !verifyContainerDeleted(t, container.ContainerID, 3) {
		t.Errorf("Container %s still exists after cleanup and verification with retries", container.ContainerID)
	}
}

func TestCleanupAll(t *testing.T) {
	// Skip if Docker is not available
	if !platform.DockerAvailable() {
		t.Skip("Docker not available, skipping test")
	}

	// Get initial container count
	registry := GetRegistry()
	initialCount := registry.GetContainerCount()

	// Create multiple containers
	numContainers := 3
	containers := make([]*Container, numContainers)
	containerIDs := make([]string, numContainers)

	for i := 0; i < numContainers; i++ {
		containerName := "turingpi-cleanup-test-" + time.Now().Format("20060102150405") + "-" + fmt.Sprintf("%c", 'a'+i)
		config := &platform.DockerExecutionConfig{
			DockerImage:            "alpine:latest",
			ContainerName:          containerName,
			UseUniqueContainerName: true,
		}

		container, err := New(config)
		if err != nil {
			t.Fatalf("Failed to create container %d: %v", i, err)
		}
		containers[i] = container
		containerIDs[i] = container.ContainerID

		// Verify container actually exists in Docker
		cmd := exec.Command("docker", "ps", "-a", "--filter", "id="+container.ContainerID, "--format", "{{.ID}}")
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Errorf("Failed to verify container %s exists: %v", container.ContainerID, err)
		}
		if len(output) == 0 {
			t.Errorf("Container %s was not found in docker ps output", container.ContainerID)
		}
	}

	// Verify all containers were registered
	if registry.GetContainerCount() != initialCount+numContainers {
		t.Errorf("Expected %d containers to be registered", numContainers)
	}

	// Test CleanupAll
	registry.CleanupAll()

	// Add a small delay to give Docker time to process the cleanup
	time.Sleep(1 * time.Second)

	// Verify all containers were unregistered
	if registry.GetContainerCount() != initialCount {
		t.Errorf("Expected all containers to be unregistered after CleanupAll")
	}

	// Verify all containers were actually removed from Docker using SDK
	cli, _ := client.NewClientWithOpts(client.FromEnv)
	for i, container := range containers {
		_, err := cli.ContainerInspect(context.Background(), container.ContainerID)
		if err == nil {
			t.Errorf("Container %d should have been removed from Docker according to SDK", i)
		}
	}

	// Triple check with the verifyContainerDeleted function (includes retries)
	for i, containerID := range containerIDs {
		if !verifyContainerDeleted(t, containerID, 3) {
			t.Errorf("Container %d (%s) still exists after cleanup and verification with retries", i, containerID)
		}
	}
}
