package container

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/davidroman0O/turingpi/pkg/tpi/platform"
)

func TestNewDockerAdapter(t *testing.T) {
	// Create temporary directories for testing
	sourceDir := t.TempDir()
	tempDir := t.TempDir()
	outputDir := t.TempDir()

	// Create config
	config := platform.NewDefaultDockerConfig(sourceDir, tempDir, outputDir)
	config.DockerImage = "alpine:latest" // Use a small image for testing

	// Create adapter
	adapter, err := NewDockerAdapter(config)
	if err != nil {
		t.Fatalf("Failed to create Docker adapter: %v", err)
	}
	defer adapter.Cleanup()

	// Test adapter has valid container
	if adapter.container == nil {
		t.Error("Adapter has nil container")
	}

	// Verify container ID is not empty
	containerID := adapter.GetContainerID()
	if containerID == "" {
		t.Error("Container ID is empty")
	}

	// Verify container name is set
	containerName := adapter.GetContainerName()
	if containerName == "" {
		t.Error("Container name is empty")
	}
}

func TestDockerAdapter_ExecuteCommand(t *testing.T) {
	// Create temporary directories for testing
	sourceDir := t.TempDir()
	tempDir := t.TempDir()
	outputDir := t.TempDir()

	// Create config
	config := platform.NewDefaultDockerConfig(sourceDir, tempDir, outputDir)
	config.DockerImage = "alpine:latest" // Use a small image for testing

	// Create adapter
	adapter, err := NewDockerAdapter(config)
	if err != nil {
		t.Fatalf("Failed to create Docker adapter: %v", err)
	}
	defer adapter.Cleanup()

	// Test command execution
	output, err := adapter.ExecuteCommand([]string{"echo", "Hello from adapter test"})
	if err != nil {
		t.Fatalf("Failed to execute command: %v", err)
	}

	if !strings.Contains(output, "Hello from adapter test") {
		t.Errorf("Expected output to contain greeting, got: %s", output)
	}

	// Test detached command execution
	err = adapter.ExecuteDetached([]string{"touch", "/tmp/test-file"})
	if err != nil {
		t.Fatalf("Failed to execute detached command: %v", err)
	}

	// Verify the detached command was executed
	output, err = adapter.ExecuteCommand([]string{"ls", "-la", "/tmp/test-file"})
	if err != nil {
		t.Fatalf("Failed to verify detached command: %v", err)
	}
	if !strings.Contains(output, "test-file") {
		t.Errorf("Detached command failed to create file, output: %s", output)
	}
}

func TestDockerAdapter_CopyFileToContainer(t *testing.T) {
	// Create temporary directories for testing
	sourceDir := t.TempDir()
	tempDir := t.TempDir()
	outputDir := t.TempDir()

	// Create a test file
	testFilePath := filepath.Join(tempDir, "test.txt")
	testContent := "Test content for adapter"
	if err := os.WriteFile(testFilePath, []byte(testContent), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Create config
	config := platform.NewDefaultDockerConfig(sourceDir, tempDir, outputDir)
	config.DockerImage = "alpine:latest" // Use a small image for testing

	// Create adapter
	adapter, err := NewDockerAdapter(config)
	if err != nil {
		t.Fatalf("Failed to create Docker adapter: %v", err)
	}
	defer adapter.Cleanup()

	// Test copying file to container
	err = adapter.CopyFileToContainer(testFilePath, "/tmp/copied-test.txt")
	if err != nil {
		t.Fatalf("Failed to copy file to container: %v", err)
	}

	// Verify the file was copied
	output, err := adapter.ExecuteCommand([]string{"cat", "/tmp/copied-test.txt"})
	if err != nil {
		t.Fatalf("Failed to verify copied file: %v", err)
	}
	if !strings.Contains(output, testContent) {
		t.Errorf("Expected copied file to contain test content, got: %s", output)
	}
}

func TestDockerAdapter_CopyDirectoryToContainer(t *testing.T) {
	// Create temporary directories for testing
	sourceDir := t.TempDir()
	tempDir := t.TempDir()
	outputDir := t.TempDir()
	testDir := filepath.Join(tempDir, "test-dir")

	// Create test directory with files
	if err := os.MkdirAll(testDir, 0755); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	// Create multiple test files
	testFiles := []struct {
		name    string
		content string
	}{
		{"file1.txt", "Content of file 1"},
		{"file2.txt", "Content of file 2"},
		{"subdir/file3.txt", "Content of file 3 in subdirectory"},
	}

	for _, tf := range testFiles {
		filePath := filepath.Join(testDir, tf.name)
		dir := filepath.Dir(filePath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("Failed to create directory %s: %v", dir, err)
		}
		if err := os.WriteFile(filePath, []byte(tf.content), 0644); err != nil {
			t.Fatalf("Failed to create test file %s: %v", tf.name, err)
		}
	}

	// Create config
	config := platform.NewDefaultDockerConfig(sourceDir, tempDir, outputDir)
	config.DockerImage = "alpine:latest" // Use a small image for testing

	// Create adapter
	adapter, err := NewDockerAdapter(config)
	if err != nil {
		t.Fatalf("Failed to create Docker adapter: %v", err)
	}
	defer adapter.Cleanup()

	// Test copying directory to container
	err = adapter.CopyDirectoryToContainer(testDir, "/tmp/test-copied-dir")
	if err != nil {
		t.Fatalf("Failed to copy directory to container: %v", err)
	}

	// Verify the files were copied
	for _, tf := range testFiles {
		containerPath := filepath.Join("/tmp/test-copied-dir", tf.name)
		output, err := adapter.ExecuteCommand([]string{"cat", containerPath})
		if err != nil {
			t.Fatalf("Failed to verify copied file %s: %v", containerPath, err)
		}
		if !strings.Contains(output, tf.content) {
			t.Errorf("Expected copied file %s to contain '%s', got: %s",
				containerPath, tf.content, output)
		}
	}
}
