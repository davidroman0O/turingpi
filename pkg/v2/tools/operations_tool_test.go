package tools

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/davidroman0O/turingpi/pkg/v2/container"
)

// TestOperationsToolWithContainer tests the operations tool using a real container
// This is an integration test that requires Docker
func TestOperationsToolWithContainer(t *testing.T) {
	// Skip if not running integration tests (can be controlled with env var)
	/*if os.Getenv("RUN_INTEGRATION_TESTS") != "1" {
		t.Skip("Skipping integration test; set RUN_INTEGRATION_TESTS=1 to run")
	}*/

	ctx := context.Background()

	// Create a container registry
	registry, err := container.NewDockerRegistry()
	if err != nil {
		t.Fatalf("Failed to create Docker registry: %v", err)
	}
	defer registry.Close()

	// Create a container tool
	containerTool := NewContainerTool(registry)
	if containerTool == nil {
		t.Fatal("Failed to create container tool")
	}

	// Create operations tool
	opsTool, err := NewOperationsTool(containerTool)
	if err != nil {
		t.Fatalf("Failed to create operations tool: %v", err)
	}

	// Create a temporary directory for test files
	tempDir, err := os.MkdirTemp("", "operations_tool_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a test file
	testContent := []byte("Hello, Operations Tool!")
	testFile := filepath.Join(tempDir, "test.txt")
	if err := os.WriteFile(testFile, testContent, 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Test filesystem operations
	// Since we don't have a real disk image, we'll just test basic file operations
	mountDir := tempDir
	testOutputPath := "output.txt"

	// Test WriteFile
	err = opsTool.WriteFile(ctx, mountDir, testOutputPath, testContent, 0644)
	if err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	// Verify the file was created
	outputFile := filepath.Join(mountDir, testOutputPath)
	if _, err := os.Stat(outputFile); os.IsNotExist(err) {
		t.Fatalf("Expected file %s to exist", outputFile)
	}

	// Test ReadFile
	content, err := opsTool.ReadFile(ctx, mountDir, testOutputPath)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}

	// Verify content
	if string(content) != string(testContent) {
		t.Fatalf("Expected content %q, got %q", testContent, content)
	}

	// Test CopyFile
	copyDestPath := "copy_output.txt"
	err = opsTool.CopyFile(ctx, mountDir, outputFile, copyDestPath)
	if err != nil {
		t.Fatalf("CopyFile failed: %v", err)
	}

	// Verify the copy was created
	copyFile := filepath.Join(mountDir, copyDestPath)
	if _, err := os.Stat(copyFile); os.IsNotExist(err) {
		t.Fatalf("Expected file %s to exist", copyFile)
	}

	// Read the copied file
	copyContent, err := os.ReadFile(copyFile)
	if err != nil {
		t.Fatalf("Failed to read copied file: %v", err)
	}

	// Verify copied content
	if string(copyContent) != string(testContent) {
		t.Fatalf("Expected content %q, got %q", testContent, copyContent)
	}
}

// TestNewOperationsTool tests creating an operations tool
func TestNewOperationsTool(t *testing.T) {
	// Skip if not running integration tests (can be controlled with env var)
	/*if os.Getenv("RUN_INTEGRATION_TESTS") != "1" {
		t.Skip("Skipping integration test; set RUN_INTEGRATION_TESTS=1 to run")
	}*/

	// Create a container registry
	registry, err := container.NewDockerRegistry()
	if err != nil {
		t.Fatalf("Failed to create Docker registry: %v", err)
	}
	defer registry.Close()

	// Create a container tool
	containerTool := NewContainerTool(registry)
	if containerTool == nil {
		t.Fatal("Failed to create container tool")
	}

	// Create operations tool
	opsTool, err := NewOperationsTool(containerTool)
	if err != nil {
		t.Fatalf("Failed to create operations tool: %v", err)
	}

	// Verify it's not nil
	if opsTool == nil {
		t.Fatal("Expected non-nil operations tool")
	}

	// Runtime check for interface implementation
	_, ok := opsTool.(OperationsTool)
	if !ok {
		t.Fatal("opsTool does not implement OperationsTool interface")
	}
}
