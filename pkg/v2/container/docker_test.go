package container

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDockerContainer(t *testing.T) {
	ctx := context.Background()
	registry, err := NewDockerRegistry()
	require.NoError(t, err)
	defer func() {
		err := registry.Close()
		require.NoError(t, err)
	}()

	t.Run("container creation", func(t *testing.T) {
		config := ContainerConfig{
			Image:   "alpine:latest",
			Command: []string{"echo", "hello"},
		}

		container, err := registry.Create(ctx, config)
		require.NoError(t, err)
		assert.NotEmpty(t, container.ID())

		defer func() {
			err := registry.Remove(ctx, container.ID())
			require.NoError(t, err)
		}()
	})

	t.Run("container lifecycle", func(t *testing.T) {
		config := ContainerConfig{
			Image:   "alpine:latest",
			Command: []string{"sleep", "10"},
		}

		container, err := registry.Create(ctx, config)
		require.NoError(t, err)
		defer func() {
			err := registry.Remove(ctx, container.ID())
			require.NoError(t, err)
		}()

		// Start container
		err = container.Start(ctx)
		require.NoError(t, err)

		// Pause container
		err = container.Pause(ctx)
		require.NoError(t, err)

		// Unpause container
		err = container.Unpause(ctx)
		require.NoError(t, err)

		// Stop container
		err = container.Stop(ctx)
		require.NoError(t, err)
	})

	t.Run("command execution", func(t *testing.T) {
		config := ContainerConfig{
			Image:   "alpine:latest",
			Command: []string{"sleep", "10"},
		}

		container, err := registry.Create(ctx, config)
		require.NoError(t, err)
		defer func() {
			err := registry.Remove(ctx, container.ID())
			require.NoError(t, err)
		}()

		err = container.Start(ctx)
		require.NoError(t, err)

		output, err := container.Exec(ctx, []string{"echo", "test"})
		require.NoError(t, err)
		assert.Contains(t, string(output), "test")
	})

	t.Run("file operations", func(t *testing.T) {
		config := ContainerConfig{
			Image:   "alpine:latest",
			Command: []string{"sleep", "10"},
		}

		container, err := registry.Create(ctx, config)
		require.NoError(t, err)
		defer func() {
			err := registry.Remove(ctx, container.ID())
			require.NoError(t, err)
		}()

		err = container.Start(ctx)
		require.NoError(t, err)

		// Create a temporary file
		content := []byte("test content")
		tmpfile, err := os.CreateTemp("", "test")
		require.NoError(t, err)
		defer os.Remove(tmpfile.Name())

		_, err = tmpfile.Write(content)
		require.NoError(t, err)
		err = tmpfile.Close()
		require.NoError(t, err)

		// Copy file to container
		err = container.CopyTo(ctx, tmpfile.Name(), "/tmp/test")
		require.NoError(t, err)

		// Copy file back from container
		destPath := filepath.Join(os.TempDir(), "test_copy")
		defer os.Remove(destPath)

		err = container.CopyFrom(ctx, "/tmp/test", destPath)
		require.NoError(t, err)

		// Verify content
		copiedContent, err := os.ReadFile(destPath)
		require.NoError(t, err)
		assert.Equal(t, content, copiedContent)
	})

	t.Run("container logs", func(t *testing.T) {
		config := ContainerConfig{
			Image:   "alpine:latest",
			Command: []string{"echo", "test log"},
		}

		container, err := registry.Create(ctx, config)
		require.NoError(t, err)
		defer func() {
			err := registry.Remove(ctx, container.ID())
			require.NoError(t, err)
		}()

		err = container.Start(ctx)
		require.NoError(t, err)

		// Wait for container to finish
		exitCode, err := container.Wait(ctx)
		require.NoError(t, err)
		require.Zero(t, exitCode)

		// Get logs
		logs, err := container.Logs(ctx)
		require.NoError(t, err)
		defer logs.Close()

		output, err := io.ReadAll(logs)
		require.NoError(t, err)
		assert.Contains(t, string(output), "test log")
	})

	t.Run("container stats", func(t *testing.T) {
		config := ContainerConfig{
			Image:   "alpine:latest",
			Command: []string{"sleep", "1"},
		}

		container, err := registry.Create(ctx, config)
		require.NoError(t, err)
		defer func() {
			err := registry.Remove(ctx, container.ID())
			require.NoError(t, err)
		}()

		err = container.Start(ctx)
		require.NoError(t, err)

		stats, err := registry.Stats(ctx, container.ID())
		require.NoError(t, err)
		assert.NotNil(t, stats)
		assert.NotEmpty(t, stats.ID)
		assert.NotZero(t, stats.Created)
	})
}

func TestDockerContainer_ExecDetached(t *testing.T) {
	// Create registry instead of direct client access
	registry, err := NewDockerRegistry()
	if err != nil {
		t.Fatalf("Failed to create Docker registry: %v", err)
	}
	defer registry.Close()

	// Create a container via the registry
	ctx := context.Background()
	name := fmt.Sprintf("test-exec-detached-%d", time.Now().UnixNano())

	// Create container
	config := ContainerConfig{
		Image:   "alpine:latest",
		Name:    name,
		Command: []string{"sleep", "30"},
	}

	// Create and start container
	container, err := registry.Create(ctx, config)
	if err != nil {
		t.Fatalf("Failed to create container: %v", err)
	}
	defer registry.Remove(ctx, container.ID())

	if err := container.Start(ctx); err != nil {
		t.Fatalf("Failed to start container: %v", err)
	}

	// Test ExecDetached
	err = container.ExecDetached(ctx, []string{"touch", "/tmp/detached-test-file"})
	if err != nil {
		t.Fatalf("Failed to execute detached command: %v", err)
	}

	// Wait a bit for the command to execute
	time.Sleep(1 * time.Second)

	// Verify the file was created by running another command to check
	output, err := container.Exec(ctx, []string{"ls", "-la", "/tmp/detached-test-file"})
	if err != nil {
		t.Fatalf("Failed to verify file creation: %v", err)
	}
	if !strings.Contains(output, "detached-test-file") {
		t.Errorf("Expected file to be created by detached command, but it wasn't found: %s", output)
	}
}

func TestDockerContainer_Integration(t *testing.T) {
	// Create Docker client
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		t.Fatalf("Failed to create Docker client: %v", err)
	}
	defer cli.Close()

	// Create registry
	registry, err := NewDockerRegistry()
	if err != nil {
		t.Fatalf("Failed to create Docker registry: %v", err)
	}
	defer registry.Close()

	ctx := context.Background()

	// Create a unique container name
	containerName := fmt.Sprintf("test-docker-container-%d", time.Now().UnixNano())

	// Create container config
	config := ContainerConfig{
		Image:   "alpine:latest",
		Name:    containerName,
		Command: []string{"sleep", "60"},
	}

	// Create container
	container, err := registry.Create(ctx, config)
	if err != nil {
		t.Fatalf("Failed to create container: %v", err)
	}
	defer registry.Remove(ctx, container.ID())

	// Start container
	if err := container.Start(ctx); err != nil {
		t.Fatalf("Failed to start container: %v", err)
	}

	// Test exec with output
	output, err := container.Exec(ctx, []string{"echo", "Hello from Docker Container"})
	if err != nil {
		t.Fatalf("Failed to execute command: %v", err)
	}

	if !strings.Contains(output, "Hello from Docker Container") {
		t.Errorf("Expected output to contain greeting, got: %s", output)
	}

	// Test exec detached
	err = container.ExecDetached(ctx, []string{"touch", "/tmp/test-detached"})
	if err != nil {
		t.Fatalf("Failed to execute detached command: %v", err)
	}

	// Wait a bit for the detached command to complete
	time.Sleep(1 * time.Second)

	// Verify detached command result
	output, err = container.Exec(ctx, []string{"ls", "-la", "/tmp/test-detached"})
	if err != nil {
		t.Fatalf("Failed to verify detached execution: %v", err)
	}
	if !strings.Contains(output, "test-detached") {
		t.Errorf("Detached command didn't work as expected, output: %s", output)
	}

	// Test copy to container
	tempFile := filepath.Join(os.TempDir(), "test-copy-to.txt")
	content := "Hello from host"
	if err := os.WriteFile(tempFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tempFile)

	if err := container.CopyTo(ctx, tempFile, "/tmp/test-copy-to.txt"); err != nil {
		t.Fatalf("Failed to copy to container: %v", err)
	}

	// Verify copy to container worked
	output, err = container.Exec(ctx, []string{"cat", "/tmp/test-copy-to.txt"})
	if err != nil {
		t.Fatalf("Failed to verify copied file: %v", err)
	}
	if !strings.Contains(output, content) {
		t.Errorf("Expected copied file to contain '%s', got: %s", content, output)
	}

	// Test copy from container
	tempHostFile := filepath.Join(os.TempDir(), "test-copy-from.txt")
	defer os.Remove(tempHostFile)

	// Create a file in the container first
	_, err = container.Exec(ctx, []string{"sh", "-c", "echo 'Hello from container' > /tmp/test-copy-from.txt"})
	if err != nil {
		t.Fatalf("Failed to create file in container: %v", err)
	}

	// Copy from container
	if err := container.CopyFrom(ctx, "/tmp/test-copy-from.txt", tempHostFile); err != nil {
		t.Fatalf("Failed to copy from container: %v", err)
	}

	// Verify the copy worked
	hostContent, err := os.ReadFile(tempHostFile)
	if err != nil {
		t.Fatalf("Failed to read copied file: %v", err)
	}
	if !strings.Contains(string(hostContent), "Hello from container") {
		t.Errorf("Expected copied file to contain greeting, got: %s", string(hostContent))
	}

	// Test stop and pause/unpause
	if err := container.Pause(ctx); err != nil {
		t.Fatalf("Failed to pause container: %v", err)
	}

	if err := container.Unpause(ctx); err != nil {
		t.Fatalf("Failed to unpause container: %v", err)
	}

	if err := container.Stop(ctx); err != nil {
		t.Fatalf("Failed to stop container: %v", err)
	}

	// Test container restarting
	if err := container.Start(ctx); err != nil {
		t.Fatalf("Failed to restart container: %v", err)
	}

	// Final confirmation the container is working
	output, err = container.Exec(ctx, []string{"echo", "Container is back"})
	if err != nil {
		t.Fatalf("Failed to execute command after restart: %v", err)
	}
	if !strings.Contains(output, "Container is back") {
		t.Errorf("Expected output after restart, got: %s", output)
	}
}
