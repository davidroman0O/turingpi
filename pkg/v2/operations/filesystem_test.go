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

// TestListFiles tests the ListFiles function
func TestListFiles(t *testing.T) {
	// Create a real executor for integration testing
	executor := &NativeExecutor{}
	fsOps := NewFilesystemOperations(executor)

	// Create a temporary directory for our test
	tempDir, err := os.MkdirTemp("", "listfiles-test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create various test files and directories
	fileData := []struct {
		name    string
		content string
		isDir   bool
		mode    os.FileMode
	}{
		{"file1.txt", "File 1 content", false, 0644},
		{"file2.txt", "File 2 content", false, 0600},
		{"subdir", "", true, 0755},
		{".hidden", "Hidden file", false, 0644},
	}

	for _, fd := range fileData {
		path := filepath.Join(tempDir, fd.name)
		if fd.isDir {
			if err := os.MkdirAll(path, fd.mode); err != nil {
				t.Fatalf("Failed to create directory %s: %v", path, err)
			}
		} else {
			if err := os.WriteFile(path, []byte(fd.content), fd.mode); err != nil {
				t.Fatalf("Failed to create file %s: %v", path, err)
			}
		}
	}

	// Test ListFiles
	ctx := context.Background()
	files, err := fsOps.ListFiles(ctx, tempDir)
	if err != nil {
		t.Fatalf("ListFiles failed: %v", err)
	}

	// Verify the expected files were found
	fileMap := make(map[string]bool)
	for _, file := range files {
		fileMap[file.Name] = true
		// Check attributes of known files
		for _, fd := range fileData {
			if file.Name == fd.name {
				if file.IsDir != fd.isDir {
					t.Errorf("File %s: expected IsDir=%v, got %v", fd.name, fd.isDir, file.IsDir)
				}
				break
			}
		}
	}

	// We should have at least the files we created, plus . and ..
	expectedFiles := len(fileData) + 2
	if len(files) < expectedFiles {
		t.Errorf("Expected at least %d files, got %d", expectedFiles, len(files))
	}

	// Check if all created files are present
	for _, fd := range fileData {
		if !fileMap[fd.name] {
			t.Errorf("File %s not found in results", fd.name)
		}
	}

	// Test ListFilesBasic
	basicFiles, err := fsOps.ListFilesBasic(ctx, tempDir)
	if err != nil {
		t.Fatalf("ListFilesBasic failed: %v", err)
	}

	// Verify the expected files were found
	basicFileMap := make(map[string]bool)
	for _, fileName := range basicFiles {
		basicFileMap[fileName] = true
	}

	// Check if all created files are present
	for _, fd := range fileData {
		if !basicFileMap[fd.name] {
			t.Errorf("File %s not found in basic results", fd.name)
		}
	}

	// Test error case with non-existent directory
	nonExistentDir := filepath.Join(tempDir, "non-existent-dir")
	_, err = fsOps.ListFiles(ctx, nonExistentDir)
	if err == nil {
		t.Fatalf("Expected error for non-existent directory, got nil")
	}

	_, err = fsOps.ListFilesBasic(ctx, nonExistentDir)
	if err == nil {
		t.Fatalf("Expected error for non-existent directory in basic list, got nil")
	}
}

// TestListFilesMock tests the ListFiles function using a mock executor
func TestListFilesMock(t *testing.T) {
	ctx := context.Background()
	mockExec := NewMockExecutor()

	// Set up mock response for ls command
	lsOutput := `total 20
drwxr-xr-x 2 user group 4096 2023-05-01 12:00 .
drwxr-xr-x 3 user group 4096 2023-05-01 11:00 ..
-rw-r--r-- 1 user group  123 2023-05-01 13:00 file1.txt
-rw------- 1 user group  456 2023-05-01 14:00 file2.txt
drwxr-xr-x 2 user group 4096 2023-05-01 15:00 subdir
lrwxrwxrwx 1 user group    8 2023-05-01 16:00 link.txt -> file1.txt
`
	mockExec.MockResponses["ls -la --time-style=iso /test/dir"] = struct {
		Output []byte
		Err    error
	}{
		Output: []byte(lsOutput),
		Err:    nil,
	}

	// Set up mock response for basic ls command
	lsBasicOutput := `.
..
file1.txt
file2.txt
subdir
link.txt
`
	mockExec.MockResponses["ls -a /test/dir"] = struct {
		Output []byte
		Err    error
	}{
		Output: []byte(lsBasicOutput),
		Err:    nil,
	}

	// Create filesystem with mock executor
	fs := NewFilesystemOperations(mockExec)

	// Test ListFiles
	files, err := fs.ListFiles(ctx, "/test/dir")
	if err != nil {
		t.Fatalf("ListFiles failed: %v", err)
	}

	// Verify the expected files were found
	if len(files) != 6 {
		t.Errorf("Expected 6 files (including . and ..), got %d", len(files))
	}

	// Check for specific files
	fileMap := make(map[string]FileInfo)
	for _, file := range files {
		fileMap[file.Name] = file
	}

	// Check file1.txt
	if file, ok := fileMap["file1.txt"]; ok {
		if file.IsDir {
			t.Errorf("file1.txt should not be a directory")
		}
		if file.Size != 123 {
			t.Errorf("file1.txt: expected size 123, got %d", file.Size)
		}
	} else {
		t.Errorf("file1.txt not found in results")
	}

	// Check subdir
	if file, ok := fileMap["subdir"]; ok {
		if !file.IsDir {
			t.Errorf("subdir should be a directory")
		}
	} else {
		t.Errorf("subdir not found in results")
	}

	// Check symlink
	if file, ok := fileMap["link.txt"]; ok {
		if file.SymlinkPath != "file1.txt" {
			t.Errorf("link.txt: expected symlink to 'file1.txt', got '%s'", file.SymlinkPath)
		}
	} else {
		t.Errorf("link.txt not found in results")
	}

	// Test ListFilesBasic
	basicFiles, err := fs.ListFilesBasic(ctx, "/test/dir")
	if err != nil {
		t.Fatalf("ListFilesBasic failed: %v", err)
	}

	// Verify the expected files were found
	if len(basicFiles) != 6 {
		t.Errorf("Expected 6 files in basic list, got %d", len(basicFiles))
	}

	// Check for specific files
	for _, expectedFile := range []string{".", "..", "file1.txt", "file2.txt", "subdir", "link.txt"} {
		found := false
		for _, fileName := range basicFiles {
			if fileName == expectedFile {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("File %s not found in basic results", expectedFile)
		}
	}
}
