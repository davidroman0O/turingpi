package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/davidroman0O/turingpi/pkg/v2/container"
	"github.com/davidroman0O/turingpi/pkg/v2/operations"
)

// TestOperationsToolWithContainer tests the operations tool using a real container
// This is an integration test that requires Docker
func TestOperationsToolWithContainer(t *testing.T) {
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

	// Create a temporary directory in the current working directory
	pwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current working directory: %v", err)
	}

	tempDirName := fmt.Sprintf("test_ops_dir_%d", time.Now().UnixNano())
	tempDir := filepath.Join(pwd, tempDirName)
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)
	t.Logf("Created test directory: %s", tempDir)

	// Create a test file
	testContent := []byte("Hello, Operations Tool!")
	testFile := filepath.Join(tempDir, "test.txt")
	if err := os.WriteFile(testFile, testContent, 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// The container directory where the temp dir will be mounted
	containerMountPoint := "/testdir"

	// Create a proper container configuration with volume mounts
	containerConfig := container.ContainerConfig{
		Image:        "ubuntu:latest",
		Name:         "test-operations-container",
		Command:      []string{"sleep", "infinity"}, // Need a command to keep container running
		Privileged:   true,
		Capabilities: []string{"SYS_ADMIN"},
		WorkDir:      containerMountPoint,
		Mounts:       map[string]string{tempDir: containerMountPoint},
	}

	// Create operations tool with specific options
	options := OperationsToolOptions{
		ContainerTool:          containerTool,
		ExecutionMode:          operations.ExecuteContainer,
		ContainerConfig:        containerConfig,
		UsePersistentContainer: true,
	}

	opsTool, err := NewOperationsToolWithOptions(options)
	if err != nil {
		t.Fatalf("Failed to create operations tool: %v", err)
	}
	defer opsTool.Close()

	// Since we're using a container with a mount, use the container mount point as base
	mountDir := containerMountPoint
	testOutputPath := "output.txt"

	// Test WriteFile
	err = opsTool.WriteFile(ctx, mountDir, testOutputPath, testContent, 0644)
	if err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	// Verify the file was created in the host (due to mount)
	outputFile := filepath.Join(tempDir, testOutputPath)
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

	// Test CopyFile - use an absolute path for the source
	sourceFilePath := filepath.Join(mountDir, "test.txt")
	copyDestPath := "copy_output.txt"

	err = opsTool.CopyFile(ctx, mountDir, sourceFilePath, copyDestPath)
	if err != nil {
		t.Fatalf("CopyFile failed: %v", err)
	}

	// Verify the copy was created
	copyFile := filepath.Join(tempDir, copyDestPath)
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

// TestNewOperationsToolWithOptions tests creating an operations tool with different options
func TestNewOperationsToolWithOptions(t *testing.T) {
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

	// Test with native mode
	t.Run("NativeMode", func(t *testing.T) {
		options := OperationsToolOptions{
			ContainerTool:          containerTool,
			ExecutionMode:          operations.ExecuteNative,
			UsePersistentContainer: false,
		}

		opsTool, err := NewOperationsToolWithOptions(options)
		if err != nil {
			t.Fatalf("Failed to create operations tool: %v", err)
		}

		// Verify it's not nil
		if opsTool == nil {
			t.Fatal("Expected non-nil operations tool")
		}

		// Clean up
		opsTool.Close()
	})

	// Test with container mode
	t.Run("ContainerMode", func(t *testing.T) {
		options := OperationsToolOptions{
			ContainerTool:          containerTool,
			ExecutionMode:          operations.ExecuteContainer,
			UsePersistentContainer: true,
			ContainerConfig: container.ContainerConfig{
				Image:        "ubuntu:latest",
				Name:         "test-operations-container",
				Command:      []string{"sleep", "infinity"},
				Privileged:   true,
				Capabilities: []string{"SYS_ADMIN"},
			},
		}

		opsTool, err := NewOperationsToolWithOptions(options)
		if err != nil {
			t.Fatalf("Failed to create operations tool: %v", err)
		}

		// Verify it's not nil
		if opsTool == nil {
			t.Fatal("Expected non-nil operations tool")
		}

		// Clean up
		opsTool.Close()
	})

	// Test with auto mode
	t.Run("AutoMode", func(t *testing.T) {
		options := OperationsToolOptions{
			ContainerTool:          containerTool,
			ExecutionMode:          operations.ExecuteAuto,
			UsePersistentContainer: false,
		}

		opsTool, err := NewOperationsToolWithOptions(options)
		if err != nil {
			t.Fatalf("Failed to create operations tool: %v", err)
		}

		// Verify it's not nil
		if opsTool == nil {
			t.Fatal("Expected non-nil operations tool")
		}

		// Clean up
		opsTool.Close()
	})
}
