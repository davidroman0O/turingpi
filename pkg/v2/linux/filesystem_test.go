package linux

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// execFunc defines the signature for command execution functions
type execFunc func(cmd *exec.Cmd) ([]byte, error)

// mockExecCommand mocks the command execution
func mockExecCommand(t *testing.T, expectedCmd string, expectedArgs []string, output []byte, err error) execFunc {
	return func(cmd *exec.Cmd) ([]byte, error) {
		if cmd.Path != expectedCmd {
			t.Errorf("Expected command %s, got %s", expectedCmd, cmd.Path)
		}

		// Check if args match
		for i, arg := range expectedArgs {
			if i < len(cmd.Args)-1 && cmd.Args[i+1] != arg {
				t.Errorf("Expected arg %s at position %d, got %s", arg, i, cmd.Args[i+1])
			}
		}

		return output, err
	}
}

// TestIsPartitionMounted tests the IsPartitionMounted function
func TestIsPartitionMounted(t *testing.T) {
	ctx := context.Background()

	testCases := []struct {
		name          string
		partition     string
		mockOutput    []byte
		mockError     error
		expectMounted bool
		expectMountPt string
		expectError   bool
	}{
		{
			name:          "Partition is mounted",
			partition:     "/dev/sda1",
			mockOutput:    []byte("/mnt\n"),
			mockError:     nil,
			expectMounted: true,
			expectMountPt: "/mnt",
			expectError:   false,
		},
		{
			name:          "Partition is not mounted",
			partition:     "/dev/sdb1",
			mockOutput:    []byte(""),
			mockError:     &exec.ExitError{},
			expectMounted: false,
			expectMountPt: "",
			expectError:   false,
		},
		{
			name:          "Partition not found",
			partition:     "/dev/nonexistent",
			mockOutput:    []byte("findmnt: /dev/nonexistent: not found"),
			mockError:     &exec.ExitError{},
			expectMounted: false,
			expectMountPt: "",
			expectError:   false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create filesystem with mock execution function
			filesystem := NewFilesystem()
			// Override the execCommand field
			filesystem.execCommand = func(cmd *exec.Cmd) ([]byte, error) {
				return tc.mockOutput, tc.mockError
			}

			// Call the function under test
			mounted, mountPoint, err := filesystem.IsPartitionMounted(ctx, tc.partition)

			// Check results
			if tc.expectError && err == nil {
				t.Errorf("Expected error but got nil")
			}

			if !tc.expectError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}

			if mounted != tc.expectMounted {
				t.Errorf("Expected mounted=%v but got %v", tc.expectMounted, mounted)
			}

			if mountPoint != tc.expectMountPt {
				t.Errorf("Expected mount point=%q but got %q", tc.expectMountPt, mountPoint)
			}
		})
	}
}

// TestGetFilesystemType tests the GetFilesystemType function
func TestGetFilesystemType(t *testing.T) {
	ctx := context.Background()

	testCases := []struct {
		name         string
		partition    string
		mockOutput   []byte
		mockError    error
		expectFsType string
		expectError  bool
	}{
		{
			name:         "ext4 filesystem",
			partition:    "/dev/sda1",
			mockOutput:   []byte("ext4\n"),
			mockError:    nil,
			expectFsType: "ext4",
			expectError:  false,
		},
		{
			name:         "vfat filesystem",
			partition:    "/dev/sdb1",
			mockOutput:   []byte("vfat\n"),
			mockError:    nil,
			expectFsType: "vfat",
			expectError:  false,
		},
		{
			name:         "Error getting filesystem type",
			partition:    "/dev/nonexistent",
			mockOutput:   []byte(""),
			mockError:    fmt.Errorf("command failed"),
			expectFsType: "",
			expectError:  true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create filesystem with mock execution function
			filesystem := NewFilesystem()
			// Override the execCommand field
			filesystem.execCommand = func(cmd *exec.Cmd) ([]byte, error) {
				return tc.mockOutput, tc.mockError
			}

			// Call the function under test
			fsType, err := filesystem.GetFilesystemType(ctx, tc.partition)

			// Check results
			if tc.expectError && err == nil {
				t.Errorf("Expected error but got nil")
			}

			if !tc.expectError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}

			if fsType != tc.expectFsType {
				t.Errorf("Expected filesystem type=%q but got %q", tc.expectFsType, fsType)
			}
		})
	}
}

func TestFileOperations(t *testing.T) {
	// Create a new filesystem instance with the default execCommand
	filesystem := NewFilesystem()

	// Create a temporary directory to simulate a mount point
	tmpDir, err := os.MkdirTemp("", "fs_test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Test directory creation
	testDirPath := "test_dir"
	if err := filesystem.MakeDirectory(tmpDir, testDirPath, 0755); err != nil {
		t.Fatalf("MakeDirectory failed: %v", err)
	}

	// Verify directory exists
	if !filesystem.FileExists(tmpDir, testDirPath) {
		t.Errorf("Expected directory to exist, but it doesn't")
	}

	// Test file creation
	testFilePath := filepath.Join(testDirPath, "test.txt")
	testContent := []byte("test content")
	if err := filesystem.WriteFile(tmpDir, testFilePath, testContent, 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	// Verify file exists
	if !filesystem.FileExists(tmpDir, testFilePath) {
		t.Errorf("Expected file to exist, but it doesn't")
	}

	// Test reading file
	content, err := filesystem.ReadFile(tmpDir, testFilePath)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}
	if string(content) != string(testContent) {
		t.Errorf("Expected content '%s', but got '%s'", testContent, content)
	}
}

// SetExecCommand allows setting the execCommand field for testing purposes
func (fs *Filesystem) SetExecCommand(mockExec func(cmd *exec.Cmd) ([]byte, error)) {
	fs.execCommand = mockExec
}
