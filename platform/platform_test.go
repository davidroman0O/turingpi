package platform

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestPlatformChecks(t *testing.T) {
	// Test IsLinux
	if IsLinux() != (runtime.GOOS == "linux") {
		t.Errorf("IsLinux() = %v, want %v", IsLinux(), runtime.GOOS == "linux")
	}

	// Test IsDarwin
	if IsDarwin() != (runtime.GOOS == "darwin") {
		t.Errorf("IsDarwin() = %v, want %v", IsDarwin(), runtime.GOOS == "darwin")
	}

	// Test IsWindows
	if IsWindows() != (runtime.GOOS == "windows") {
		t.Errorf("IsWindows() = %v, want %v", IsWindows(), runtime.GOOS == "windows")
	}
}

func TestDockerAvailable(t *testing.T) {
	// This test is informational and should not fail if Docker is not available
	available := DockerAvailable()
	t.Logf("Docker available: %v", available)
}

func TestGetHomeDir(t *testing.T) {
	homeDir, err := GetHomeDir()
	if err != nil {
		t.Fatalf("GetHomeDir() error = %v", err)
	}
	if homeDir == "" {
		t.Error("GetHomeDir() returned empty string")
	}

	// Verify the directory exists
	if _, err := os.Stat(homeDir); os.IsNotExist(err) {
		t.Errorf("Home directory %s does not exist", homeDir)
	}
}

func TestGetTempDir(t *testing.T) {
	tempDir := GetTempDir()
	if tempDir == "" {
		t.Error("GetTempDir() returned empty string")
	}

	// Verify the directory exists
	if _, err := os.Stat(tempDir); os.IsNotExist(err) {
		t.Errorf("Temp directory %s does not exist", tempDir)
	}

	// Test we can create a file in the temp directory
	testFile := filepath.Join(tempDir, "platform_test")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Errorf("Failed to write to temp directory: %v", err)
	}
	os.Remove(testFile) // Clean up
}

func TestGetWorkingDir(t *testing.T) {
	// Get initial working directory
	initialDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get initial working directory: %v", err)
	}

	// Create a temporary directory
	tempDir, err := os.MkdirTemp("", "platform_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Change to temp directory
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("Failed to change directory: %v", err)
	}
	defer os.Chdir(initialDir) // Restore initial directory

	// Test GetWorkingDir
	workingDir, err := GetWorkingDir()
	if err != nil {
		t.Fatalf("GetWorkingDir() error = %v", err)
	}
	if workingDir != tempDir {
		t.Errorf("GetWorkingDir() = %v, want %v", workingDir, tempDir)
	}
}

func TestGetOSInfo(t *testing.T) {
	info := GetOSInfo()

	// Test OS
	if info.OS != runtime.GOOS {
		t.Errorf("OSInfo.OS = %v, want %v", info.OS, runtime.GOOS)
	}

	// Test Version
	if info.Version != runtime.Version() {
		t.Errorf("OSInfo.Version = %v, want %v", info.Version, runtime.Version())
	}

	// Test Architecture
	if info.Arch != runtime.GOARCH {
		t.Errorf("OSInfo.Arch = %v, want %v", info.Arch, runtime.GOARCH)
	}

	// Test string representation
	t.Logf("OS Info: %+v", info)
}
