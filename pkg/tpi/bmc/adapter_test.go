package bmc

import (
	"os"
	"path/filepath"
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

func TestNewBMCAdapter(t *testing.T) {
	config := getTestConfig()
	adapter := NewBMCAdapter(config)
	if adapter == nil {
		t.Fatal("Expected non-nil adapter")
	}

	// Type assertion to check interface implementation
	_, ok := adapter.(BMCAdapter)
	if !ok {
		t.Error("Adapter does not implement BMCAdapter interface")
	}
}

func TestBMCAdapter_CheckFileExists(t *testing.T) {
	adapter := NewBMCAdapter(getTestConfig())

	// Test with a file that should exist
	exists, err := adapter.CheckFileExists("/etc/passwd")
	if err != nil {
		t.Errorf("CheckFileExists failed for existing file: %v", err)
	}
	if !exists {
		t.Error("Expected /etc/passwd to exist")
	}

	// Test with a file that should not exist
	exists, err = adapter.CheckFileExists("/path/to/nonexistent/file")
	if err != nil {
		t.Errorf("CheckFileExists failed for non-existent file: %v", err)
	}
	if exists {
		t.Error("Expected non-existent file to return false")
	}
}

func TestBMCAdapter_UploadFile(t *testing.T) {
	adapter := NewBMCAdapter(getTestConfig())

	// Create a temporary test file
	content := []byte("test content from Turing Pi BMC adapter test")
	tmpfile, err := os.CreateTemp("", "bmc-test-*.txt")
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
	err = adapter.UploadFile(tmpfile.Name(), remotePath)
	if err != nil {
		t.Fatalf("UploadFile failed: %v", err)
	}

	// Verify the file exists
	exists, err := adapter.CheckFileExists(remotePath)
	if err != nil {
		t.Errorf("Failed to check uploaded file: %v", err)
	}
	if !exists {
		t.Error("Uploaded file not found on remote system")
	}

	// Verify file content
	stdout, stderr, err := adapter.ExecuteCommand("cat " + remotePath)
	if err != nil {
		t.Errorf("Failed to read uploaded file: %v\nStderr: %s", err, stderr)
	}
	if stdout != string(content) {
		t.Errorf("File content mismatch.\nExpected: %s\nGot: %s", content, stdout)
	}

	// Clean up remote file
	_, _, err = adapter.ExecuteCommand("rm " + remotePath)
	if err != nil {
		t.Logf("Warning: Failed to clean up remote file: %v", err)
	}
}

func TestBMCAdapter_ExecuteCommand(t *testing.T) {
	adapter := NewBMCAdapter(getTestConfig())

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

	// Test BMC-specific command
	stdout, stderr, err = adapter.ExecuteCommand("tpi --version")
	if err != nil {
		t.Errorf("Failed to get TPI version: %v\nStderr: %s", err, stderr)
	}
	if stdout == "" {
		t.Error("Expected non-empty output from 'tpi --version'")
	}
}

func TestBMCAdapter_RealHardwareCommands(t *testing.T) {
	adapter := NewBMCAdapter(getTestConfig())

	// Test getting node information using power status
	stdout, stderr, err := adapter.ExecuteCommand("tpi power status")
	if err != nil {
		t.Errorf("Failed to get power status: %v\nStderr: %s", err, stderr)
	}
	if stdout == "" {
		t.Error("Expected non-empty output from 'tpi power status'")
	}

	// Test getting BMC information
	stdout, stderr, err = adapter.ExecuteCommand("tpi info")
	if err != nil {
		t.Errorf("Failed to get BMC info: %v\nStderr: %s", err, stderr)
	}
	if stdout == "" {
		t.Error("Expected non-empty output from 'tpi info'")
	}
}
