package docker

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/davidroman0O/turingpi/pkg/tpi/platform"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
)

func TestNew(t *testing.T) {
	// Create test directories
	sourceDir, err := os.MkdirTemp("", "turingpi-test-source-*")
	if err != nil {
		t.Fatal("Failed to create temp directory:", err)
	}
	defer os.RemoveAll(sourceDir)

	tempDir, err := os.MkdirTemp("", "turingpi-test-temp-*")
	if err != nil {
		t.Fatal("Failed to create temp directory:", err)
	}
	defer os.RemoveAll(tempDir)

	outputDir, err := os.MkdirTemp("", "turingpi-test-output-*")
	if err != nil {
		t.Fatal("Failed to create temp directory:", err)
	}
	defer os.RemoveAll(outputDir)

	// Create a config with a lightweight image for testing
	config := platform.NewDefaultDockerConfig(sourceDir, tempDir, outputDir)
	config.DockerImage = "alpine:latest"
	config.ContainerName = "turingpi-test-container-" + time.Now().Format("20060102150405")

	// Create a new container
	container, err := New(config)
	if err != nil {
		t.Fatal("Failed to create Docker container:", err)
	}
	if container == nil {
		t.Fatal("Container is nil")
	}

	// Clean up after test
	defer container.Cleanup()

	// Validate container properties
	if container.ContainerID == "" {
		t.Error("Expected container ID to be non-empty")
	}
	if container.Config.ContainerName != config.ContainerName {
		t.Errorf("Expected container name to be '%s', got '%s'", config.ContainerName, container.Config.ContainerName)
	}
}

func TestDockerContextDetails(t *testing.T) {
	// Test getting Docker context details
	contextInfo, err := getDockerContextDetails()
	if err != nil {
		t.Fatal("Failed to get Docker context details:", err)
	}

	// Verify context info
	t.Logf("Docker context name: %s", contextInfo.Name)
	t.Logf("Docker context host: %s", contextInfo.Host)

	if contextInfo.Host == "" {
		t.Error("Expected Docker host to be non-empty")
	}
}

func TestEnsureDockerImage(t *testing.T) {
	// First create a proper container with the right Docker client initialization
	config := &platform.DockerExecutionConfig{
		DockerImage:   "alpine:latest", // Use a small, commonly available image
		ContainerName: "turingpi-test-image-container",
	}

	// Create a complete container first to get a properly initialized client
	fullContainer, err := New(config)
	if err != nil {
		t.Fatal("Failed to create container:", err)
	}
	defer fullContainer.Cleanup()

	// Now create a test container with only the configuration but using
	// the initialized client from the full container
	testContainer := &Container{
		Config: config,
		ctx:    fullContainer.ctx,
		cli:    fullContainer.cli,
	}

	// Clear any existing image first to test the pull functionality
	// This is a bit hacky but needed for a proper test
	testContainer.cli.ImageRemove(testContainer.ctx, "alpine:latest", image.RemoveOptions{Force: true})

	// This should pull the image if not present
	err = testContainer.ensureDockerImage()
	if err != nil {
		t.Fatal("Failed to ensure Docker image:", err)
	}

	// Verify the image was pulled
	_, _, err = testContainer.cli.ImageInspectWithRaw(testContainer.ctx, "alpine:latest")
	if err != nil {
		t.Error("Expected image to be available after ensureDockerImage:", err)
	}
}

func TestCreateContainer(t *testing.T) {
	// Create unique container name to avoid conflicts
	containerName := "turingpi-test-create-" + time.Now().Format("20060102150405")

	// Create a config for container creation
	config := &platform.DockerExecutionConfig{
		DockerImage:   "alpine:latest",
		ContainerName: containerName,
		SourceDir:     "", // Not needed for this test
		TempDir:       "", // Not needed for this test
		OutputDir:     "", // Not needed for this test
	}

	// Create a new container
	container, err := New(config)
	if err != nil {
		t.Fatal("Failed to create Docker container:", err)
	}
	defer container.Cleanup()

	// Verify container was created
	if container.ContainerID == "" {
		t.Error("Expected container ID to be non-empty")
	}
	if container.GetContainerName() != containerName {
		t.Errorf("Expected container name to be '%s', got '%s'", containerName, container.GetContainerName())
	}

	// Recreate a container with the same name (should reuse or replace)
	container2, err := New(config)
	if err != nil {
		t.Fatal("Failed to create second Docker container:", err)
	}
	defer container2.Cleanup()

	// Should either get the same container or a new one with the same name
	if container2.GetContainerName() != containerName {
		t.Errorf("Expected container name to be '%s', got '%s'", containerName, container2.GetContainerName())
	}
}

func TestContainerExecuteCommand(t *testing.T) {
	// Create a config for command execution testing
	config := &platform.DockerExecutionConfig{
		DockerImage:   "alpine:latest",
		ContainerName: "turingpi-test-exec-" + time.Now().Format("20060102150405"),
	}

	// Create a new container
	container, err := New(config)
	if err != nil {
		t.Fatal("Failed to create Docker container:", err)
	}
	defer container.Cleanup()

	// Test basic command
	output, err := container.ExecuteCommand([]string{"echo", "Hello from Docker"})
	if err != nil {
		t.Fatal("Failed to execute command:", err)
	}
	if !strings.Contains(output, "Hello from Docker") {
		t.Errorf("Expected output to contain 'Hello from Docker', got: '%s'", output)
	}

	// Test command with environment variable
	output, err = container.ExecuteCommand([]string{"sh", "-c", "echo $HOME"})
	if err != nil {
		t.Fatal("Failed to execute command with env var:", err)
	}
	if !strings.Contains(output, "/root") && !strings.Contains(output, "/home") {
		t.Errorf("Expected output to contain home directory path, got: '%s'", output)
	}

	// Test failing command
	_, err = container.ExecuteCommand([]string{"ls", "/nonexistent"})
	if err == nil {
		t.Error("Expected error for nonexistent directory")
	}
}

func TestContainerCleanup(t *testing.T) {
	// Create a unique container name
	containerName := "turingpi-test-cleanup-" + time.Now().Format("20060102150405")

	// Create a container to test cleanup
	config := &platform.DockerExecutionConfig{
		DockerImage:   "alpine:latest",
		ContainerName: containerName,
	}

	container, err := New(config)
	if err != nil {
		t.Fatal("Failed to create Docker container:", err)
	}
	if container == nil {
		t.Fatal("Container is nil")
	}

	// Record the container ID before cleanup
	containerID := container.ContainerID
	if containerID == "" {
		t.Error("Expected container ID to be non-empty")
	}

	// Perform cleanup
	err = container.Cleanup()
	if err != nil {
		t.Fatal("Failed to cleanup container:", err)
	}

	// Try to execute a command after cleanup - should fail
	_, err = container.ExecuteCommand([]string{"echo", "test"})
	if err == nil {
		t.Error("Container should be stopped after cleanup")
	}
}

func TestBasicPrepareImage(t *testing.T) {
	// Create test directories
	tempDir, err := os.MkdirTemp("", "turingpi-test-temp-*")
	if err != nil {
		t.Fatal("Failed to create temp directory:", err)
	}
	defer os.RemoveAll(tempDir)

	// Use a simpler approach just to test if the image builds
	config := &platform.DockerExecutionConfig{
		DockerImage:   "ubuntu:22.04", // Use Ubuntu for simplicity
		ContainerName: "turingpi-test-basic-" + time.Now().Format("20060102150405"),
		TempDir:       tempDir,
	}

	container, err := New(config)
	if err != nil {
		t.Fatal("Failed to create test container:", err)
	}
	defer container.Cleanup()

	// Test a basic command to ensure container is running
	output, err := container.ExecuteCommand([]string{"echo", "test"})
	if err != nil {
		t.Fatal("Failed to execute command:", err)
	}

	if !strings.Contains(output, "test") {
		t.Errorf("Expected output to contain 'test', got: %s", output)
	}
}

func TestContainer_UniqueNames(t *testing.T) {
	// Skip if Docker is not available
	if _, err := client.NewClientWithOpts(client.FromEnv); err != nil {
		t.Skip("Docker not available, skipping test")
	}

	// Create a temporary directory for the test
	tempDir, err := os.MkdirTemp("", "turingpi-docker-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create two containers with the same base name but unique names enabled
	config1 := &platform.DockerExecutionConfig{
		DockerImage:            "ubuntu:22.04",
		TempDir:                tempDir,
		ContainerName:          "turingpi-test-container",
		UseUniqueContainerName: true,
	}

	config2 := &platform.DockerExecutionConfig{
		DockerImage:            "ubuntu:22.04",
		TempDir:                tempDir,
		ContainerName:          "turingpi-test-container",
		UseUniqueContainerName: true,
	}

	// Create first container
	t.Log("Creating first container...")
	container1, err := New(config1)
	if err != nil {
		t.Fatalf("Failed to create first container: %v", err)
	}
	defer container1.Cleanup()

	// Create second container
	t.Log("Creating second container...")
	container2, err := New(config2)
	if err != nil {
		t.Fatalf("Failed to create second container: %v", err)
	}
	defer container2.Cleanup()

	// Verify the containers have different names
	if container1.Config.ContainerName == container2.Config.ContainerName {
		t.Errorf("Expected unique container names, but both are: %s", container1.Config.ContainerName)
	}

	// Verify both names start with the base name
	if !strings.HasPrefix(container1.Config.ContainerName, "turingpi-test-container-") {
		t.Errorf("Expected container name to start with base name, got: %s", container1.Config.ContainerName)
	}
	if !strings.HasPrefix(container2.Config.ContainerName, "turingpi-test-container-") {
		t.Errorf("Expected container name to start with base name, got: %s", container2.Config.ContainerName)
	}

	// Test that both containers can successfully execute a command
	t.Log("Testing command execution on first container...")
	output1, err := container1.ExecuteCommand([]string{"echo", "hello from container 1"})
	if err != nil {
		t.Errorf("Failed to execute command on first container: %v", err)
	} else {
		t.Logf("Container 1 output: %s", output1)
	}

	t.Log("Testing command execution on second container...")
	output2, err := container2.ExecuteCommand([]string{"echo", "hello from container 2"})
	if err != nil {
		t.Errorf("Failed to execute command on second container: %v", err)
	} else {
		t.Logf("Container 2 output: %s", output2)
	}

	// Test cleanup works properly
	t.Log("Testing cleanup of first container...")
	if err := container1.Cleanup(); err != nil {
		t.Errorf("Failed to clean up first container: %v", err)
	}

	// Verify container was removed
	time.Sleep(1 * time.Second) // Give Docker some time to process the removal
	dockerClient, _ := client.NewClientWithOpts(client.FromEnv)
	_, err = dockerClient.ContainerInspect(context.Background(), container1.ContainerID)
	if err == nil {
		t.Errorf("Container 1 still exists after cleanup")
	}
}

func TestContainer_Cleanup(t *testing.T) {
	// Skip if Docker is not available
	if _, err := client.NewClientWithOpts(client.FromEnv); err != nil {
		t.Skip("Docker not available, skipping test")
	}

	// Create a container with a unique name
	config := &platform.DockerExecutionConfig{
		DockerImage:            "ubuntu:22.04",
		ContainerName:          "turingpi-cleanup-test",
		UseUniqueContainerName: true,
	}

	// Create container
	t.Log("Creating container for cleanup test...")
	container, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create container: %v", err)
	}

	// Store the container ID for later verification
	containerID := container.ContainerID

	// Test cleanup
	t.Log("Testing cleanup...")
	if err := container.Cleanup(); err != nil {
		t.Errorf("Failed to clean up container: %v", err)
	}

	// Verify container was removed
	time.Sleep(1 * time.Second) // Give Docker some time to process the removal
	dockerClient, _ := client.NewClientWithOpts(client.FromEnv)
	_, err = dockerClient.ContainerInspect(context.Background(), containerID)
	if err == nil {
		t.Errorf("Container still exists after cleanup")
	}

	// Test cleanup with nil ContainerID (shouldn't crash)
	container.ContainerID = ""
	if err := container.Cleanup(); err != nil {
		t.Errorf("Cleanup with empty container ID failed: %v", err)
	}
}
