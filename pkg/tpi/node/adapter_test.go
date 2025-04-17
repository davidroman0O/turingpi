package node

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// getTestConfig returns a standard test configuration for the real Turing Pi hardware
func getTestConfig() SSHConfig {
	return SSHConfig{
		Host:     "192.168.1.90",
		User:     "root",
		Password: "turing",
		Timeout:  10 * time.Second,
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

	// Test that CopyFile manages SFTP connections
	content := []byte("test content")
	tmpfile, err := os.CreateTemp("", "node-test-*.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name())
	if _, err := tmpfile.Write(content); err != nil {
		t.Fatal(err)
	}
	tmpfile.Close()

	remotePath := filepath.Join("/tmp", filepath.Base(tmpfile.Name()))
	err = adapter.CopyFile(tmpfile.Name(), remotePath, true)
	if err != nil {
		t.Fatalf("CopyFile failed: %v", err)
	}

	// Verify that SFTP client is cleaned up
	if len(a.sftp) != 0 {
		t.Errorf("Expected 0 SFTP clients after copy, got %d", len(a.sftp))
	}

	// Clean up remote file
	if _, _, err := adapter.ExecuteCommand("rm " + remotePath); err != nil {
		t.Logf("Warning: Failed to clean up remote file: %v", err)
	}

	// Close everything
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

func TestNodeAdapter_CopyFile(t *testing.T) {
	adapter := NewNodeAdapter(getTestConfig())
	defer adapter.Close()

	// Create a temporary test file
	content := []byte("test content from Turing Pi node adapter test")
	tmpfile, err := os.CreateTemp("", "node-test-*.txt")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpfile.Name())

	if _, err := tmpfile.Write(content); err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}
	if err := tmpfile.Close(); err != nil {
		t.Fatalf("Failed to close temp file: %v", err)
	}

	// Upload the file
	remotePath := filepath.Join("/tmp", filepath.Base(tmpfile.Name()))
	err = adapter.CopyFile(tmpfile.Name(), remotePath, true)
	if err != nil {
		t.Fatalf("CopyFile (upload) failed: %v", err)
	}

	// Verify file exists and content matches
	stdout, stderr, err := adapter.ExecuteCommand("cat " + remotePath)
	if err != nil {
		t.Errorf("Failed to read uploaded file: %v\nStderr: %s", err, stderr)
	}
	if stdout != string(content) {
		t.Errorf("File content mismatch.\nExpected: %s\nGot: %s", content, stdout)
	}

	// Download the file to a new location
	downloadPath := tmpfile.Name() + ".downloaded"
	defer os.Remove(downloadPath)

	err = adapter.CopyFile(downloadPath, remotePath, false)
	if err != nil {
		t.Fatalf("CopyFile (download) failed: %v", err)
	}

	// Verify downloaded content
	downloadedContent, err := os.ReadFile(downloadPath)
	if err != nil {
		t.Fatalf("Failed to read downloaded file: %v", err)
	}
	if string(downloadedContent) != string(content) {
		t.Errorf("Downloaded content mismatch.\nExpected: %s\nGot: %s", content, downloadedContent)
	}

	// Clean up remote file
	_, _, err = adapter.ExecuteCommand("rm " + remotePath)
	if err != nil {
		t.Logf("Warning: Failed to clean up remote file: %v", err)
	}
}

func TestNodeAdapter_RealHardwareCommands(t *testing.T) {
	adapter := NewNodeAdapter(getTestConfig())
	defer adapter.Close()

	// Test getting system information
	stdout, stderr, err := adapter.ExecuteCommand("uname -a")
	if err != nil {
		t.Errorf("Failed to get system info: %v\nStderr: %s", err, stderr)
	}
	if stdout == "" {
		t.Error("Expected non-empty output from 'uname -a'")
	}

	// Test disk space information
	stdout, stderr, err = adapter.ExecuteCommand("df -h /")
	if err != nil {
		t.Errorf("Failed to get disk space info: %v\nStderr: %s", err, stderr)
	}
	if stdout == "" {
		t.Error("Expected non-empty output from 'df -h /'")
	}

	// Test memory information
	stdout, stderr, err = adapter.ExecuteCommand("free -h")
	if err != nil {
		t.Errorf("Failed to get memory info: %v\nStderr: %s", err, stderr)
	}
	if stdout == "" {
		t.Error("Expected non-empty output from 'free -h'")
	}
}
