package docker

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestNewAdapter(t *testing.T) {
	// Create temporary directories for testing
	sourceDir := t.TempDir()
	tempDir := t.TempDir()
	outputDir := t.TempDir()

	// Test successful creation
	ctx := context.Background()
	adapter, err := NewAdapter(ctx, sourceDir, tempDir, outputDir)
	if err != nil {
		t.Fatalf("Failed to create adapter: %v", err)
	}
	defer adapter.Close()

	if adapter.Container == nil {
		t.Error("Container is nil")
	}
	if adapter.client == nil {
		t.Error("Docker client is nil")
	}
}

func TestAdapterExecuteCommand(t *testing.T) {
	// Create temporary directories for testing
	tempDir := t.TempDir()
	ctx := context.Background()

	// Create adapter
	adapter, err := NewAdapter(ctx, tempDir, tempDir, tempDir)
	if err != nil {
		t.Fatalf("Failed to create adapter: %v", err)
	}
	defer adapter.Close()

	// Test simple command execution
	output, err := adapter.ExecuteCommand(ctx, "echo 'Hello Docker'")
	if err != nil {
		t.Fatalf("Failed to execute command: %v", err)
	}
	if output != "Hello Docker\n" {
		t.Errorf("Unexpected output: %s", output)
	}

	// Test command with environment variable
	output, err = adapter.ExecuteCommand(ctx, "export TEST_VAR='test value' && echo $TEST_VAR")
	if err != nil {
		t.Fatalf("Failed to execute command with env var: %v", err)
	}
	if output != "test value\n" {
		t.Errorf("Unexpected output: %s", output)
	}
}

func TestCopyFileToContainer(t *testing.T) {
	// Create temporary directories for testing
	tempDir := t.TempDir()
	ctx := context.Background()

	// Create a test file
	testContent := "test content"
	testFilePath := filepath.Join(tempDir, "test.txt")
	err := os.WriteFile(testFilePath, []byte(testContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Create adapter
	adapter, err := NewAdapter(ctx, tempDir, tempDir, tempDir)
	if err != nil {
		t.Fatalf("Failed to create adapter: %v", err)
	}
	defer adapter.Close()

	// Test copying file to container
	containerPath := "/tmp/test.txt"
	err = adapter.CopyFileToContainer(ctx, testFilePath, containerPath)
	if err != nil {
		t.Fatalf("Failed to copy file to container: %v", err)
	}

	// Verify file content in container
	output, err := adapter.ExecuteCommand(ctx, "bash -c 'cat /tmp/test.txt'")
	if err != nil {
		t.Fatalf("Failed to read file in container: %v", err)
	}
	if output != testContent {
		t.Errorf("Unexpected file content in container: %s", output)
	}
}

func TestCleanup(t *testing.T) {
	// Create temporary directories for testing
	tempDir := t.TempDir()
	ctx := context.Background()

	// Create adapter
	adapter, err := NewAdapter(ctx, tempDir, tempDir, tempDir)
	if err != nil {
		t.Fatalf("Failed to create adapter: %v", err)
	}

	// Test container is working
	output, err := adapter.ExecuteCommand(ctx, "echo 'Hello from adapter test'")
	if err != nil {
		t.Fatalf("Failed to execute command: %v", err)
	}
	if output != "Hello from adapter test\n" {
		t.Errorf("Unexpected output: %s", output)
	}

	// Test cleanup
	if err := adapter.Cleanup(ctx); err != nil {
		t.Fatalf("Failed to cleanup: %v", err)
	}

	// Verify container is stopped
	_, err = adapter.ExecuteCommand(ctx, "echo 'This should fail'")
	if err == nil {
		t.Error("Expected error after cleanup, got none")
	}

	// Test creating new adapter after cleanup
	adapter, err = NewAdapter(ctx, tempDir, tempDir, tempDir)
	if err != nil {
		t.Fatalf("Failed to create new adapter after cleanup: %v", err)
	}
	defer adapter.Close()
}
