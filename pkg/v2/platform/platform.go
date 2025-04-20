package platform

import (
	"os"
	"os/exec"
	"runtime"
)

// IsLinux returns true if the current platform is Linux
func IsLinux() bool {
	return runtime.GOOS == "linux"
}

// IsDarwin returns true if the current platform is Darwin (macOS)
func IsDarwin() bool {
	return runtime.GOOS == "darwin"
}

// IsWindows returns true if the current platform is Windows
func IsWindows() bool {
	return runtime.GOOS == "windows"
}

// DockerAvailable checks if Docker is available and running
func DockerAvailable() bool {
	cmd := exec.Command("docker", "version")
	return cmd.Run() == nil
}

// GetHomeDir returns the user's home directory
func GetHomeDir() (string, error) {
	return os.UserHomeDir()
}

// GetTempDir returns the system's temporary directory
func GetTempDir() string {
	return os.TempDir()
}

// GetWorkingDir returns the current working directory
func GetWorkingDir() (string, error) {
	return os.Getwd()
}

// GetOSInfo returns basic information about the operating system
type OSInfo struct {
	OS      string
	Version string
	Arch    string
}

// GetOSInfo returns information about the current operating system
func GetOSInfo() OSInfo {
	return OSInfo{
		OS:      runtime.GOOS,
		Version: runtime.Version(),
		Arch:    runtime.GOARCH,
	}
}
