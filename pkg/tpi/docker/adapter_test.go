package docker

import (
	"os"
	"strings"
	"testing"

	"github.com/davidroman0O/turingpi/pkg/tpi/platform"
)

func TestNewAdapter(t *testing.T) {
	// Test directories - using temporary dirs for safety
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

	// Test creating a new adapter
	adapter, err := NewAdapter(sourceDir, tempDir, outputDir)
	if err != nil {
		t.Fatal("Failed to create Docker adapter:", err)
	}
	if adapter == nil {
		t.Fatal("Adapter is nil")
	}
	if adapter.Container == nil {
		t.Fatal("Container is nil")
	}

	// Clean up after test
	defer adapter.Cleanup()

	// Validate container properties
	if adapter.GetContainerID() == "" {
		t.Error("Expected container ID to be non-empty")
	}
	if adapter.GetContainerName() == "" {
		t.Error("Expected container name to be non-empty")
	}
}

func TestNewAdapterWithConfig(t *testing.T) {
	// Skip if Docker is not available
	if !platform.DockerAvailable() {
		t.Skip("Docker not available, skipping test")
	}

	// Create temporary directories for test
	sourceDir, err := os.MkdirTemp("", "turingpi-test-source-*")
	if err != nil {
		t.Fatalf("Failed to create temp source dir: %v", err)
	}
	defer os.RemoveAll(sourceDir)

	tempDir, err := os.MkdirTemp("", "turingpi-test-temp-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	outputDir, err := os.MkdirTemp("", "turingpi-test-output-*")
	if err != nil {
		t.Fatalf("Failed to create temp output dir: %v", err)
	}
	defer os.RemoveAll(outputDir)

	// Create a custom config
	config := &platform.DockerExecutionConfig{
		DockerImage:            "alpine:latest",
		SourceDir:              sourceDir,
		TempDir:                tempDir,
		OutputDir:              outputDir,
		AdditionalMounts:       map[string]string{},
		ContainerName:          "turingpi-test-container",
		UseUniqueContainerName: true,
	}

	// Create an adapter with the custom config
	adapter, err := NewAdapterWithConfig(config)
	if err != nil {
		t.Fatalf("Failed to create adapter with custom config: %v", err)
	}
	defer adapter.Cleanup()

	// The container name should be different from the original name due to uniqueness
	containerName := adapter.GetContainerName()
	if !strings.Contains(containerName, "turingpi-test-container-") {
		t.Errorf("Expected container name to contain 'turingpi-test-container-', got '%s'", containerName)
	}

	// Check that the container exists
	if adapter.GetContainerID() == "" {
		t.Error("Expected container ID to be non-empty")
	}
}

func TestAdapterExecuteCommand(t *testing.T) {
	// Create test directories
	tempDir, err := os.MkdirTemp("", "turingpi-test-*")
	if err != nil {
		t.Fatal("Failed to create temp directory:", err)
	}
	defer os.RemoveAll(tempDir)

	// Use Ubuntu for better command compatibility
	config := platform.NewDefaultDockerConfig("", tempDir, "")
	config.DockerImage = "ubuntu:22.04" // Use Ubuntu instead of Alpine
	config.ContainerName = "turingpi-test-exec-container"

	adapter, err := NewAdapterWithConfig(config)
	if err != nil {
		t.Fatal("Failed to create Docker adapter:", err)
	}
	defer adapter.Cleanup()

	// Test simple command with bash
	output, err := adapter.ExecuteCommand("echo 'Hello Docker'")
	if err != nil {
		t.Fatal("Failed to execute command:", err)
	}

	// Trim output
	output = strings.TrimSpace(output)
	if !strings.Contains(output, "Hello Docker") {
		t.Errorf("Expected output to contain 'Hello Docker', got: '%s'", output)
	}

	// Test command with environment variables
	output, err = adapter.ExecuteCommand("export TEST_VAR='test value' && echo $TEST_VAR")
	if err != nil {
		t.Fatal("Failed to execute command with env var:", err)
	}

	output = strings.TrimSpace(output)
	if !strings.Contains(output, "test value") {
		t.Errorf("Expected output to contain 'test value', got: '%s'", output)
	}
}

func TestCopyFileToContainer(t *testing.T) {
	// Create a test file
	tempDir, err := os.MkdirTemp("", "turingpi-test-*")
	if err != nil {
		t.Fatal("Failed to create temp directory:", err)
	}
	defer os.RemoveAll(tempDir)

	testFilePath := tempDir + "/test.txt"
	testContent := "test content for Docker copy"
	err = os.WriteFile(testFilePath, []byte(testContent), 0644)
	if err != nil {
		t.Fatal("Failed to write test file:", err)
	}

	// Create Docker adapter with Ubuntu instead of Alpine for better command support
	config := platform.NewDefaultDockerConfig("", tempDir, "")
	config.DockerImage = "ubuntu:22.04" // Use Ubuntu instead of Alpine
	config.ContainerName = "turingpi-test-copy-container"

	adapter, err := NewAdapterWithConfig(config)
	if err != nil {
		t.Fatal("Failed to create Docker adapter:", err)
	}
	defer adapter.Cleanup()

	// Copy the file to the container
	containerPath := "/tmp/test.txt"
	err = adapter.CopyFileToContainer(testFilePath, containerPath)
	if err != nil {
		t.Fatal("Failed to copy file to container:", err)
	}

	// Verify the file exists and has correct content using bash
	output, err := adapter.ExecuteCommand("bash -c 'cat /tmp/test.txt'")
	if err != nil {
		t.Fatal("Failed to read file from container:", err)
	}

	// Trim any whitespace and newlines from the output
	output = strings.TrimSpace(output)
	if output != testContent {
		t.Errorf("Expected file content to be '%s', got '%s'", testContent, output)
	}
}

func TestDockerAdapter_Lifecycle(t *testing.T) {
	// Skip if Docker is not available
	if !platform.DockerAvailable() {
		t.Skip("Docker not available, skipping test")
	}

	// Create a temporary directory for the test
	tempDir, err := os.MkdirTemp("", "turingpi-docker-adapter-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a basic adapter
	adapter, err := NewAdapter(tempDir, tempDir, tempDir)
	if err != nil {
		t.Fatalf("Failed to create Docker adapter: %v", err)
	}

	// Verify it was created correctly
	if adapter.Container == nil {
		t.Fatal("Adapter Container field is nil")
	}
	if adapter.GetContainerName() == "" {
		t.Fatal("Container name is empty")
	}
	if adapter.GetContainerID() == "" {
		t.Fatal("Container ID is empty")
	}

	// Test basic command execution
	output, err := adapter.ExecuteCommand("echo 'Hello from adapter test'")
	if err != nil {
		t.Errorf("Failed to execute command: %v", err)
	} else {
		t.Logf("Command output: %s", output)
	}

	// Test proper cleanup
	err = adapter.Close()
	if err != nil {
		t.Errorf("Failed to close adapter: %v", err)
	}

	// Verify container was properly nullified
	if adapter.Container != nil {
		t.Error("Container was not set to nil after Close()")
	}

	// Verify operations after close fail gracefully
	_, err = adapter.ExecuteCommand("echo 'This should fail'")
	if err == nil {
		t.Error("Execute after Close() did not return an error")
	} else {
		t.Logf("Expected error after close: %v", err)
	}

	// Test Cleanup convenience method
	adapter, err = NewAdapter(tempDir, tempDir, tempDir)
	if err != nil {
		t.Fatalf("Failed to create Docker adapter for second test: %v", err)
	}

	// Store container ID for verification
	containerID := adapter.GetContainerID()
	if containerID == "" {
		t.Fatal("Container ID is empty in second test")
	}

	// Call Cleanup method
	adapter.Cleanup()

	// Verify the container is gone
	if adapter.Container != nil {
		t.Error("Container was not set to nil after Cleanup()")
	}
}

func TestDockerAdapter_WithConfig(t *testing.T) {
	// Skip if Docker is not available
	if !platform.DockerAvailable() {
		t.Skip("Docker not available, skipping test")
	}

	// Create a configuration with unique container naming
	config := platform.NewDefaultDockerConfig("", "", "")
	config.UseUniqueContainerName = true

	// Create adapter with custom config
	adapter, err := NewAdapterWithConfig(config)
	if err != nil {
		t.Fatalf("Failed to create adapter with config: %v", err)
	}
	defer adapter.Cleanup() // Ensure cleanup happens

	// Verify the container exists
	if adapter.Container == nil {
		t.Fatal("Container is nil despite successful creation")
	}

	// Verify it received a unique name (original + suffix)
	containerName := adapter.GetContainerName()
	if !strings.Contains(containerName, "-") {
		t.Errorf("Expected container name to have a unique suffix with hyphen, got: %s", containerName)
	}
}
