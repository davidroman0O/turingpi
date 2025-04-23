package container

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/davidroman0O/turingpi/platform"
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

// TestDockerAdapter_InitCommands tests initialization commands
func TestDockerAdapter_InitCommands(t *testing.T) {
	// Create temporary directories for testing
	sourceDir := t.TempDir()
	tempDir := t.TempDir()
	outputDir := t.TempDir()

	// Create config
	config := platform.NewDefaultDockerConfig(sourceDir, tempDir, outputDir)
	config.DockerImage = "alpine:latest" // Use a small image for testing

	// Add initialization commands
	config.InitCommands = [][]string{
		{"apk", "update"},
		{"apk", "add", "curl", "jq"},
		{"touch", "/tmp/init-complete"},
	}

	// Create adapter
	adapter, err := NewDockerAdapter(config)
	if err != nil {
		t.Fatalf("Failed to create Docker adapter: %v", err)
	}
	defer adapter.Cleanup()

	// Verify initialization commands were executed
	output, err := adapter.ExecuteCommand([]string{"ls", "-la", "/tmp/init-complete"})
	if err != nil {
		t.Fatalf("Failed to verify init file: %v", err)
	}
	if !strings.Contains(output, "init-complete") {
		t.Errorf("Initialization commands were not executed, marker file not found")
	}

	// Verify packages were installed
	output, err = adapter.ExecuteCommand([]string{"which", "curl"})
	if err != nil {
		t.Fatalf("Failed to verify curl installation: %v", err)
	}
	if !strings.Contains(output, "/usr/bin/curl") {
		t.Errorf("curl was not installed, expected it in /usr/bin/curl, got: %s", output)
	}

	output, err = adapter.ExecuteCommand([]string{"which", "jq"})
	if err != nil {
		t.Fatalf("Failed to verify jq installation: %v", err)
	}
	if !strings.Contains(output, "/usr/bin/jq") {
		t.Errorf("jq was not installed, expected it in /usr/bin/jq, got: %s", output)
	}
}

// TestDockerAdapter_ComplexInitCommands tests more complex initialization scenarios
func TestDockerAdapter_ComplexInitCommands(t *testing.T) {
	// Create temporary directories for testing
	sourceDir := t.TempDir()
	tempDir := t.TempDir()
	outputDir := t.TempDir()

	// Create test file for verification
	setupFilePath := filepath.Join(tempDir, "setup.sh")
	setupScript := `#!/bin/sh
echo "Setting up environment"
mkdir -p /app/data
echo "APP_VERSION=1.0.0" > /app/data/config
echo "Setup complete"
`
	if err := os.WriteFile(setupFilePath, []byte(setupScript), 0755); err != nil {
		t.Fatalf("Failed to create setup script: %v", err)
	}

	// Create config
	config := platform.NewDefaultDockerConfig(sourceDir, tempDir, outputDir)
	config.DockerImage = "alpine:latest" // Use a small image for testing

	// Add initialization commands for a more complex setup
	config.InitCommands = [][]string{
		// Update package repositories
		{"apk", "update"},

		// Install some development tools
		{"apk", "add", "python3", "py3-pip", "make", "git"},

		// Create application directories
		{"mkdir", "-p", "/app/bin", "/app/lib", "/app/config"},

		// Set up permissions
		{"chmod", "755", "/app"},

		// Run the setup script
		{"/bin/sh", "/tmp/setup.sh"},

		// Create and activate a virtual environment for Python packages
		{"python3", "-m", "venv", "/app/venv"},

		// Install packages in the virtual environment
		{"/bin/sh", "-c", "source /app/venv/bin/activate && pip install requests && deactivate"},
	}

	// Create adapter
	adapter, err := NewDockerAdapter(config)
	if err != nil {
		t.Fatalf("Failed to create Docker adapter: %v", err)
	}
	defer adapter.Cleanup()

	// Verify Python was installed
	output, err := adapter.ExecuteCommand([]string{"python3", "--version"})
	if err != nil {
		t.Fatalf("Failed to verify Python installation: %v", err)
	}
	if !strings.Contains(output, "Python 3") {
		t.Errorf("Python was not installed correctly, output: %s", output)
	}

	// Verify directory structure was created
	output, err = adapter.ExecuteCommand([]string{"ls", "-la", "/app"})
	if err != nil {
		t.Fatalf("Failed to verify directory structure: %v", err)
	}
	for _, dir := range []string{"bin", "lib", "config", "data", "venv"} {
		if !strings.Contains(output, dir) {
			t.Errorf("Expected directory /app/%s was not created", dir)
		}
	}

	// Verify config file was created by the script
	output, err = adapter.ExecuteCommand([]string{"cat", "/app/data/config"})
	if err != nil {
		t.Fatalf("Failed to read config file: %v", err)
	}
	if !strings.Contains(output, "APP_VERSION=1.0.0") {
		t.Errorf("Config file doesn't contain expected content, got: %s", output)
	}

	// Verify Python package was installed in the virtual environment
	output, err = adapter.ExecuteCommand([]string{"/bin/sh", "-c", "source /app/venv/bin/activate && pip list"})
	if err != nil {
		t.Fatalf("Failed to list Python packages: %v", err)
	}
	if !strings.Contains(output, "requests") {
		t.Errorf("Python package 'requests' was not installed in venv, output: %s", output)
	}

	// Test running a Python script that uses the installed package
	pythonScript := `
import requests
print("Successfully imported requests module")
print(f"Requests version: {requests.__version__}")
`
	// Write Python script to container
	err = adapter.CopyFileToContainer(writeFile(t, tempDir, "test.py", pythonScript), "/tmp/test.py")
	if err != nil {
		t.Fatalf("Failed to copy Python script to container: %v", err)
	}

	// Run the Python script with the virtual environment
	output, err = adapter.ExecuteCommand([]string{"/bin/sh", "-c", "source /app/venv/bin/activate && python3 /tmp/test.py"})
	if err != nil {
		t.Fatalf("Failed to run Python script: %v", err)
	}
	if !strings.Contains(output, "Successfully imported requests module") {
		t.Errorf("Python script didn't run correctly, output: %s", output)
	}
}

// Helper function to write a temporary file
func writeFile(t *testing.T, dir, name, content string) string {
	path := filepath.Join(dir, name)
	err := os.WriteFile(path, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to write file %s: %v", name, err)
	}
	return path
}
