package operations

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/davidroman0O/turingpi/pkg/v2/container"
	"github.com/davidroman0O/turingpi/pkg/v2/platform"
)

// skipIfNoDocker skips the test if Docker is not available
func skipIfNoDocker(t *testing.T) {
	if !platform.DockerAvailable() {
		t.Skip("Skipping test as Docker is not available")
	}
}

// TestUnifiedExecutorNativeMode tests the UnifiedExecutor in native mode
func TestUnifiedExecutorNativeMode(t *testing.T) {
	// Skip if we're on Windows as it behaves differently
	if runtime.GOOS == "windows" {
		t.Skip("Skipping test on Windows")
	}

	ctx := context.Background()

	// Create executor with native mode
	executor := NewUnifiedExecutor(UnifiedExecutorOptions{
		Mode: ExecuteNative,
	})

	// Test simple echo command
	output, err := executor.Execute(ctx, "echo", "Hello, world!")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// On Unix systems, echo adds a newline
	expectedOutput := "Hello, world!\n"
	if string(output) != expectedOutput {
		t.Errorf("Expected output %q, got %q", expectedOutput, string(output))
	}

	// Test with input
	input := "test input"
	output, err = executor.ExecuteWithInput(ctx, input, "cat")
	if err != nil {
		t.Fatalf("ExecuteWithInput failed: %v", err)
	}

	if string(output) != input {
		t.Errorf("Expected input to be echoed: %q, got %q", input, string(output))
	}

	// Test ExecuteInPath
	tempDir, err := os.MkdirTemp("", "unified-executor-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	output, err = executor.ExecuteInPath(ctx, tempDir, "pwd")
	if err != nil {
		t.Fatalf("ExecuteInPath failed: %v", err)
	}

	// The output should contain the temp dir path (may have symlinks resolved)
	if !strings.Contains(string(output), "unified-executor-test-") {
		t.Errorf("ExecuteInPath output should contain the temp dir name, got: %q", string(output))
	}
}

// TestUnifiedExecutorContainerMode tests the UnifiedExecutor in container mode
func TestUnifiedExecutorContainerMode(t *testing.T) {
	skipIfNoDocker(t)

	ctx := context.Background()

	// Create a real Docker registry
	registry, err := container.NewDockerRegistry()
	if err != nil {
		t.Fatalf("Failed to create Docker registry: %v", err)
	}
	defer registry.Close()

	// Create a temporary directory for testing file operations
	tempDir, err := os.MkdirTemp("", "container-executor-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a test file
	testFilePath := filepath.Join(tempDir, "test.txt")
	testContent := "Hello from Docker container!"
	if err := os.WriteFile(testFilePath, []byte(testContent), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	// Create a unique container name for this test
	containerName := fmt.Sprintf("unified-executor-test-%d", time.Now().Unix())

	// Create executor with container mode and real Docker registry
	executor := NewUnifiedExecutor(UnifiedExecutorOptions{
		Mode:     ExecuteContainer,
		Registry: registry,
		ContainerConfig: container.ContainerConfig{
			Image:   "ubuntu:latest",
			Name:    containerName,
			Command: []string{"sleep", "infinity"},
			Mounts:  map[string]string{tempDir: "/testdir"},
		},
		UsePersistentContainer: true,
	})
	defer executor.Close() // Ensure container cleanup

	// Test 1: Simple echo command
	output, err := executor.Execute(ctx, "echo", "Hello from container!")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if !strings.Contains(string(output), "Hello from container!") {
		t.Errorf("Expected output to contain 'Hello from container!', got %q", string(output))
	}

	// Test 2: File operations via the container - cat the test file we created
	output, err = executor.Execute(ctx, "cat", "/testdir/test.txt")
	if err != nil {
		t.Fatalf("Execute cat failed: %v", err)
	}

	if string(output) != testContent {
		t.Errorf("Expected test file content %q, got %q", testContent, string(output))
	}

	// Test 3: Write a file in the container
	newFileName := "/testdir/container-created.txt"
	newContent := "This file was created by the container"
	_, err = executor.Execute(ctx, "bash", "-c", fmt.Sprintf("echo '%s' > %s", newContent, newFileName))
	if err != nil {
		t.Fatalf("Failed to create file in container: %v", err)
	}

	// Verify the file exists and has the right content on the host
	hostFilePath := filepath.Join(tempDir, "container-created.txt")
	content, err := os.ReadFile(hostFilePath)
	if err != nil {
		t.Fatalf("Failed to read file created by container: %v", err)
	}
	if string(content) != newContent+"\n" { // echo adds a newline
		t.Errorf("Expected file content %q, got %q", newContent+"\n", string(content))
	}

	// Test 4: ExecuteInPath
	output, err = executor.ExecuteInPath(ctx, "/testdir", "pwd")
	if err != nil {
		t.Fatalf("ExecuteInPath failed: %v", err)
	}

	if !strings.Contains(string(output), "/testdir") {
		t.Errorf("Expected pwd output to contain '/testdir', got %q", string(output))
	}
}

// TestUnifiedExecutorAutoMode tests the UnifiedExecutor in auto mode
func TestUnifiedExecutorAutoMode(t *testing.T) {
	if runtime.GOOS != "linux" {
		skipIfNoDocker(t)
	}

	ctx := context.Background()

	// Create a real Docker registry for non-Linux platforms
	registry, err := container.NewDockerRegistry()
	if err != nil {
		t.Fatalf("Failed to create Docker registry: %v", err)
	}
	defer registry.Close()

	// Create a temporary directory for testing file operations
	tempDir, err := os.MkdirTemp("", "auto-executor-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a test file
	testFilePath := filepath.Join(tempDir, "auto-test.txt")
	testContent := "Testing auto mode!"
	if err := os.WriteFile(testFilePath, []byte(testContent), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	// Create a unique container name for this test
	containerName := fmt.Sprintf("auto-executor-test-%d", time.Now().Unix())

	// Create executor with auto mode
	executor := NewUnifiedExecutor(UnifiedExecutorOptions{
		Mode:     ExecuteAuto,
		Registry: registry,
		ContainerConfig: container.ContainerConfig{
			Image:   "ubuntu:latest",
			Name:    containerName,
			Command: []string{"sleep", "infinity"},
			Mounts:  map[string]string{tempDir: "/autotest"},
		},
		UsePersistentContainer: true,
	})
	defer executor.Close() // Ensure container cleanup if one was created

	// Test simple echo command
	output, err := executor.Execute(ctx, "echo", "Auto mode test")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// Verify expected behavior based on platform
	if runtime.GOOS == "linux" {
		// On Linux, should execute natively and include newline
		expectedOutput := "Auto mode test\n"
		if string(output) != expectedOutput {
			t.Errorf("Expected native output %q, got %q", expectedOutput, string(output))
		}

		// Try to read our test file with local path - should work if native
		fileBytes, err := os.ReadFile(testFilePath)
		if err != nil {
			t.Fatalf("Failed to read test file: %v", err)
		}
		if string(fileBytes) != testContent {
			t.Errorf("Expected file content %q, got %q", testContent, string(fileBytes))
		}
	} else {
		// On non-Linux, should execute in container
		if !strings.Contains(string(output), "Auto mode test") {
			t.Errorf("Expected container output to contain 'Auto mode test', got %q", string(output))
		}

		// Check if container can see the mounted file
		output, err = executor.Execute(ctx, "cat", "/autotest/auto-test.txt")
		if err != nil {
			t.Fatalf("Failed to read test file in container: %v", err)
		}
		if string(output) != testContent {
			t.Errorf("Expected test file content %q in container, got %q", testContent, string(output))
		}
	}
}

// TestUnifiedExecutorTempContainer tests temporary container creation
func TestUnifiedExecutorTempContainer(t *testing.T) {
	skipIfNoDocker(t)

	ctx := context.Background()

	// Create a real Docker registry
	registry, err := container.NewDockerRegistry()
	if err != nil {
		t.Fatalf("Failed to create Docker registry: %v", err)
	}
	defer registry.Close()

	// Create executor with container mode but no persistent container
	executor := NewUnifiedExecutor(UnifiedExecutorOptions{
		Mode:     ExecuteContainer,
		Registry: registry,
		ContainerConfig: container.ContainerConfig{
			Image:   "ubuntu:latest",
			Name:    "temp-container-test",
			Command: []string{"sleep", "infinity"},
		},
		UsePersistentContainer: false,
	})
	defer executor.Close() // Just in case

	// Execute multiple commands
	startTime := time.Now()
	for i := 0; i < 3; i++ {
		output, err := executor.Execute(ctx, "echo", fmt.Sprintf("Command %d", i))
		if err != nil {
			t.Fatalf("Execute %d failed: %v", i, err)
		}
		if !strings.Contains(string(output), fmt.Sprintf("Command %d", i)) {
			t.Errorf("Expected output to contain 'Command %d', got %q", i, string(output))
		}
	}
	execDuration := time.Since(startTime)

	// Output the execution time for diagnostics
	t.Logf("Executed 3 commands with temporary containers in %v", execDuration)

	// Verify no containers remain
	// This is hard to test definitively with the real Docker API
	// Instead, we'll check for error when trying to close - should be nil
	// since nothing needs to be closed in non-persistent mode
	if err := executor.Close(); err != nil {
		t.Errorf("Executor Close() should return nil in temporary container mode: %v", err)
	}
}
