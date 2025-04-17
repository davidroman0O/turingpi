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

	// Create a custom config with a specific image
	config := platform.NewDefaultDockerConfig(sourceDir, tempDir, outputDir)
	config.DockerImage = "alpine:latest" // Small image for testing
	config.ContainerName = "turingpi-test-container"

	// Create adapter with custom config
	adapter, err := NewAdapterWithConfig(config)
	if err != nil {
		t.Fatal("Failed to create Docker adapter with config:", err)
	}
	if adapter == nil {
		t.Fatal("Adapter is nil")
	}

	// Clean up after test
	defer adapter.Cleanup()

	// Validate adapter properties
	if adapter.GetContainerName() != "turingpi-test-container" {
		t.Errorf("Expected container name to be 'turingpi-test-container', got '%s'", adapter.GetContainerName())
	}
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
