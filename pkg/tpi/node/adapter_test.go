package node

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"

	tpicache "github.com/davidroman0O/turingpi/pkg/tpi/cache"
)

// getTestConfig returns a standard test configuration for the real Turing Pi hardware
func getTestConfig() SSHConfig {
	return SSHConfig{
		Host:           "192.168.1.90",
		User:           "root",
		Password:       "turing",
		Timeout:        10 * time.Second,
		MaxRetries:     3,
		RetryDelay:     1 * time.Second,
		RetryIncrement: 2 * time.Second,
	}
}

func TestNewNodeAdapter(t *testing.T) {
	config := getTestConfig()
	adapter := NewNodeAdapter(config)
	if adapter == nil {
		t.Fatal("Expected non-nil adapter")
	}

	// Type assertion to check interface implementation
	_, ok := adapter.(NodeAdapter)
	if !ok {
		t.Error("Adapter does not implement NodeAdapter interface")
	}

	// Type assertion to access internal state for testing
	a, ok := adapter.(*nodeAdapter)
	if !ok {
		t.Fatal("Expected adapter to be *nodeAdapter")
	}

	// Verify initial state
	if len(a.clients) != 0 || len(a.sessions) != 0 || len(a.sftp) != 0 {
		t.Error("New adapter should have no active connections")
	}

	if err := adapter.Close(); err != nil {
		t.Errorf("Close failed: %v", err)
	}
}

func TestNodeAdapter_ConnectionManagement(t *testing.T) {
	adapter := NewNodeAdapter(getTestConfig())
	a, ok := adapter.(*nodeAdapter)
	if !ok {
		t.Fatal("Expected adapter to be *nodeAdapter")
	}

	// Test that ExecuteCommand creates and cleans up connections
	stdout, stderr, err := adapter.ExecuteCommand("echo 'test'")
	if err != nil {
		t.Fatalf("ExecuteCommand failed: %v", err)
	}
	if stdout != "test\n" {
		t.Errorf("Expected stdout 'test\\n', got '%s'", stdout)
	}
	if stderr != "" {
		t.Errorf("Expected empty stderr, got '%s'", stderr)
	}

	// Verify that client is tracked but session is cleaned up
	if len(a.clients) != 1 {
		t.Errorf("Expected 1 client, got %d", len(a.clients))
	}
	if len(a.sessions) != 0 {
		t.Errorf("Expected 0 sessions after command, got %d", len(a.sessions))
	}

	// Test file operations
	cacheObj := adapter.FileOperations()
	if cacheObj == nil {
		t.Fatal("Expected non-nil cache from FileOperations")
	}

	// Test putting a file
	content := "test content"
	meta := tpicache.Metadata{
		Key:         "test/file",
		Filename:    "test.txt",
		ContentType: "text/plain",
		Size:        int64(len(content)),
		ModTime:     time.Now(),
		Tags:        map[string]string{"type": "test"},
		OSType:      "linux",
		OSVersion:   "5.10",
	}
	_, err = cacheObj.Put(context.Background(), "test/file", meta, strings.NewReader(content))
	if err != nil {
		t.Fatalf("Failed to put file: %v", err)
	}

	// Test getting the file
	getMeta, reader, err := cacheObj.Get(context.Background(), "test/file", true)
	if err != nil {
		t.Fatalf("Failed to get file: %v", err)
	}
	defer reader.Close()

	gotContent, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("Failed to read content: %v", err)
	}
	if string(gotContent) != content {
		t.Errorf("Content mismatch. Got %q, want %q", string(gotContent), content)
	}
	if getMeta.Filename != meta.Filename {
		t.Errorf("Metadata filename mismatch. Got %q, want %q", getMeta.Filename, meta.Filename)
	}

	// Clean up everything
	if err := adapter.Close(); err != nil {
		t.Errorf("Close failed: %v", err)
	}

	// Verify all connections are cleaned up
	if len(a.clients) != 0 || len(a.sessions) != 0 || len(a.sftp) != 0 {
		t.Error("Close should remove all connections")
	}

	// Verify that operations after Close fail appropriately
	if _, _, err := adapter.ExecuteCommand("echo 'test'"); err == nil {
		t.Error("Expected error when using closed adapter")
	}
}

func TestNodeAdapter_ExecuteCommand(t *testing.T) {
	adapter := NewNodeAdapter(getTestConfig())
	defer adapter.Close()

	// Test simple command
	stdout, stderr, err := adapter.ExecuteCommand("echo 'test'")
	if err != nil {
		t.Errorf("ExecuteCommand failed: %v", err)
	}
	if stdout != "test\n" {
		t.Errorf("Expected stdout 'test\\n', got '%s'", stdout)
	}
	if stderr != "" {
		t.Errorf("Expected empty stderr, got '%s'", stderr)
	}

	// Test command that should fail
	stdout, stderr, err = adapter.ExecuteCommand("nonexistentcommand")
	if err == nil {
		t.Error("Expected error for non-existent command")
	}

	// Test node-specific command
	stdout, stderr, err = adapter.ExecuteCommand("uname -a")
	if err != nil {
		t.Errorf("Failed to get uname: %v\nStderr: %s", err, stderr)
	}
	if stdout == "" {
		t.Error("Expected non-empty output from 'uname -a'")
	}
}

func TestNodeAdapter_ExpectAndSend(t *testing.T) {
	adapter := NewNodeAdapter(getTestConfig())
	defer adapter.Close()

	// Test interactive session with a simple command
	steps := []InteractionStep{
		{
			Expect: "#",                // Wait for root shell prompt
			Send:   "echo 'test done'", // Simple command that completes immediately
			LogMsg: "Running echo command",
		},
		{
			Expect: "test done", // Wait for command output
			Send:   "exit",      // Exit the shell
			LogMsg: "Exiting shell",
		},
	}

	// Use a short timeout for the interaction
	output, err := adapter.ExpectAndSend(steps, 5*time.Second)
	if err != nil {
		t.Errorf("ExpectAndSend failed: %v", err)
	}

	// Verify we got the expected output
	if !strings.Contains(output, "test done") {
		t.Errorf("Expected output containing 'test done', got: %s", output)
	}
}

func TestNodeAdapter_RetryLogic(t *testing.T) {
	// Create adapter with short retry settings for testing
	config := getTestConfig()
	config.MaxRetries = 2
	config.RetryDelay = 100 * time.Millisecond
	config.RetryIncrement = 100 * time.Millisecond
	adapter := NewNodeAdapter(config)
	defer adapter.Close()

	// Test retry on connection failure (use invalid host)
	badConfig := config
	badConfig.Host = "nonexistent.host"
	badAdapter := NewNodeAdapter(badConfig)
	defer badAdapter.Close()

	start := time.Now()
	_, _, err := badAdapter.ExecuteCommand("echo test")
	duration := time.Since(start)

	if err == nil {
		t.Error("Expected error for non-existent host")
	}

	// Should have tried 3 times (initial + 2 retries)
	expectedMinDuration := 300 * time.Millisecond // Initial + 2 retries with 100ms delay each
	if duration < expectedMinDuration {
		t.Errorf("Retry duration too short. Got %v, expected at least %v", duration, expectedMinDuration)
	}
}

func TestNodeAdapter_BMCCommands(t *testing.T) {
	adapter := NewNodeAdapter(getTestConfig())
	defer adapter.Close()

	tests := []struct {
		name          string
		command       string
		expectError   bool
		expectOutput  string
		expectInError string
	}{
		{
			name:         "power status",
			command:      "tpi power status --node 1",
			expectError:  false,
			expectOutput: "node1: On",
		},
		{
			name:          "invalid command",
			command:       "tpi invalid command",
			expectError:   true,
			expectInError: "failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdout, stderr, err := adapter.ExecuteBMCCommand(tt.command)

			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if tt.expectOutput != "" && !strings.Contains(stdout, tt.expectOutput) {
				t.Errorf("Expected stdout to contain %q, got %q", tt.expectOutput, stdout)
			}

			if tt.expectInError != "" && err != nil && !strings.Contains(err.Error(), tt.expectInError) {
				t.Errorf("Expected error to contain %q, got %q", tt.expectInError, err.Error())
			}

			if stderr != "" {
				t.Logf("Command stderr: %s", stderr)
			}
		})
	}
}
