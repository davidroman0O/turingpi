package operations

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// MockExecutor implements CommandExecutor for testing
type MockExecutor struct {
	// Map of command+args to mock output and error
	MockResponses map[string]struct {
		Output []byte
		Err    error
	}
	// Records calls for verification
	Calls []struct {
		Name string
		Args []string
	}
}

// Execute implements CommandExecutor.Execute for testing
func (m *MockExecutor) Execute(ctx context.Context, name string, args ...string) ([]byte, error) {
	// Record the call
	m.Calls = append(m.Calls, struct {
		Name string
		Args []string
	}{
		Name: name,
		Args: args,
	})

	// Create a key for lookup
	key := name
	for _, arg := range args {
		key += " " + arg
	}

	// Lookup the response
	response, ok := m.MockResponses[key]
	if !ok {
		// Default response if not found
		return []byte(""), nil
	}

	return response.Output, response.Err
}

// ExecuteWithInput implements CommandExecutor.ExecuteWithInput for testing
func (m *MockExecutor) ExecuteWithInput(ctx context.Context, input string, name string, args ...string) ([]byte, error) {
	// For simplicity, ignore input in tests
	return m.Execute(ctx, name, args...)
}

// ExecuteInPath implements CommandExecutor.ExecuteInPath for testing
func (m *MockExecutor) ExecuteInPath(ctx context.Context, dir string, name string, args ...string) ([]byte, error) {
	// Add directory to args for testing
	return m.Execute(ctx, name, append([]string{"cd", dir, "&&"}, args...)...)
}

// NewMockExecutor creates a new MockExecutor
func NewMockExecutor() *MockExecutor {
	return &MockExecutor{
		MockResponses: make(map[string]struct {
			Output []byte
			Err    error
		}),
	}
}

// TestIsPartitionMounted tests the IsPartitionMounted method
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
			mockError:     nil,
			expectMounted: false,
			expectMountPt: "",
			expectError:   false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create mock executor
			mockExec := NewMockExecutor()

			// Set up mock response
			cmdKey := "findmnt -n -o TARGET " + tc.partition
			mockExec.MockResponses[cmdKey] = struct {
				Output []byte
				Err    error
			}{
				Output: tc.mockOutput,
				Err:    tc.mockError,
			}

			// Create filesystem with mock executor
			fs := NewFilesystemOperations(mockExec)

			// Call the function under test
			mounted, mountPoint, err := fs.IsPartitionMounted(ctx, tc.partition)

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

			// Verify the call was made
			if len(mockExec.Calls) != 1 {
				t.Errorf("Expected 1 call, got %d", len(mockExec.Calls))
			} else if mockExec.Calls[0].Name != "findmnt" {
				t.Errorf("Expected call to 'findmnt', got '%s'", mockExec.Calls[0].Name)
			}
		})
	}
}

// TestGetFilesystemType tests the GetFilesystemType method
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
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create mock executor
			mockExec := NewMockExecutor()

			// Set up mock response
			cmdKey := "blkid -o value -s TYPE " + tc.partition
			mockExec.MockResponses[cmdKey] = struct {
				Output []byte
				Err    error
			}{
				Output: tc.mockOutput,
				Err:    tc.mockError,
			}

			// Create filesystem with mock executor
			fs := NewFilesystemOperations(mockExec)

			// Call the function under test
			fsType, err := fs.GetFilesystemType(ctx, tc.partition)

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

			// Verify the call was made
			if len(mockExec.Calls) != 1 {
				t.Errorf("Expected 1 call, got %d", len(mockExec.Calls))
			} else if mockExec.Calls[0].Name != "blkid" {
				t.Errorf("Expected call to 'blkid', got '%s'", mockExec.Calls[0].Name)
			}
		})
	}
}

func TestCopyFile(t *testing.T) {
	// Create a real executor for integration testing
	executor := &NativeExecutor{}
	fsOps := NewFilesystemOperations(executor)

	// Create a temporary directory for our test files
	tempDir, err := os.MkdirTemp("", "filesystem-test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create source test file
	sourceContent := []byte("This is a test file for copying")
	sourceFile := filepath.Join(tempDir, "source.txt")
	if err := os.WriteFile(sourceFile, sourceContent, 0644); err != nil {
		t.Fatalf("Failed to create source file: %v", err)
	}

	// Create a "mount" directory
	mountDir := filepath.Join(tempDir, "mount")
	if err := os.MkdirAll(mountDir, 0755); err != nil {
		t.Fatalf("Failed to create mount directory: %v", err)
	}

	// Test copying the file
	ctx := context.Background()
	destPath := "subdir/dest.txt"
	err = fsOps.CopyFile(ctx, mountDir, sourceFile, destPath)
	if err != nil {
		t.Fatalf("CopyFile failed: %v", err)
	}

	// Verify the file was copied correctly
	copiedFile := filepath.Join(mountDir, destPath)
	if _, err := os.Stat(copiedFile); os.IsNotExist(err) {
		t.Fatalf("Copied file was not created at %s", copiedFile)
	}

	// Read the copied file and verify its contents
	copiedContent, err := os.ReadFile(copiedFile)
	if err != nil {
		t.Fatalf("Failed to read copied file: %v", err)
	}

	if string(copiedContent) != string(sourceContent) {
		t.Fatalf("Copied content does not match source. Expected %q, got %q", sourceContent, copiedContent)
	}

	// Test error case with non-existent source file
	nonExistentFile := filepath.Join(tempDir, "non-existent.txt")
	err = fsOps.CopyFile(ctx, mountDir, nonExistentFile, "error.txt")
	if err == nil || !strings.Contains(err.Error(), "source file does not exist") {
		t.Fatalf("Expected 'source file does not exist' error, got: %v", err)
	}
}
