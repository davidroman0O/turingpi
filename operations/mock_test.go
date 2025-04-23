package operations

import (
	"context"
	"errors"
	"testing"
)

func TestMockExecutor(t *testing.T) {
	ctx := context.Background()

	// Create a new mock executor
	mock := NewMockExecutor()

	// Set up mock responses
	mock.MockResponses["ls -la"] = struct {
		Output []byte
		Err    error
	}{
		Output: []byte("total 0\ndrwxr-xr-x 2 root root 40 Jan 1 00:00 ."),
		Err:    nil,
	}

	mock.MockResponses["cat file.txt"] = struct {
		Output []byte
		Err    error
	}{
		Output: []byte("file content"),
		Err:    nil,
	}

	mock.MockResponses["rm file.txt"] = struct {
		Output []byte
		Err    error
	}{
		Output: []byte(""),
		Err:    errors.New("permission denied"),
	}

	// Test Execute with known command
	output, err := mock.Execute(ctx, "ls", "-la")
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if string(output) != "total 0\ndrwxr-xr-x 2 root root 40 Jan 1 00:00 ." {
		t.Errorf("Expected directory listing output, got %s", string(output))
	}

	// Test Execute with another known command
	output, err = mock.Execute(ctx, "cat", "file.txt")
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if string(output) != "file content" {
		t.Errorf("Expected file content, got %s", string(output))
	}

	// Test Execute with command that returns error
	output, err = mock.Execute(ctx, "rm", "file.txt")
	if err == nil {
		t.Errorf("Expected error, got nil")
	}
	if err.Error() != "permission denied" {
		t.Errorf("Expected 'permission denied' error, got %v", err)
	}

	// Test Execute with unknown command (should return empty output)
	output, err = mock.Execute(ctx, "unknown", "command")
	if err != nil {
		t.Errorf("Expected no error for unknown command, got %v", err)
	}
	if string(output) != "" {
		t.Errorf("Expected empty output for unknown command, got %s", string(output))
	}

	// Test ExecuteWithInput
	input := "hello world"
	output, err = mock.ExecuteWithInput(ctx, input, "echo", "-n")
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// Test ExecuteInPath
	output, err = mock.ExecuteInPath(ctx, "/tmp", "ls", "-la")
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// Verify calls were recorded
	if len(mock.Calls) != 6 {
		t.Errorf("Expected 6 calls, got %d", len(mock.Calls))
	}

	// Verify first call
	if mock.Calls[0].Name != "ls" {
		t.Errorf("Expected first call name to be 'ls', got '%s'", mock.Calls[0].Name)
	}
	if len(mock.Calls[0].Args) != 1 || mock.Calls[0].Args[0] != "-la" {
		t.Errorf("Expected first call args to be ['-la'], got %v", mock.Calls[0].Args)
	}
}
