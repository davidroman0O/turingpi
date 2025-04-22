package platform

import (
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"
)

// OSType represents the operating system type
type OSType string

const (
	// Linux represents Linux operating systems
	Linux OSType = "linux"
	// Darwin represents macOS
	Darwin OSType = "darwin"
	// Windows represents Windows operating systems
	Windows OSType = "windows"
	// Unknown represents an unknown operating system
	Unknown OSType = "unknown"
)

// GetOSType returns the current operating system type
func GetOSType() OSType {
	switch runtime.GOOS {
	case "linux":
		return Linux
	case "darwin":
		return Darwin
	case "windows":
		return Windows
	default:
		return Unknown
	}
}

// IsLinux returns true if the current OS is Linux
func IsLinux() bool {
	return GetOSType() == Linux
}

// CanExecuteLinuxTools returns true if the system can directly execute Linux tools like kpartx
func CanExecuteLinuxTools() bool {
	return IsLinux()
}

// DockerAvailable checks if Docker is available and running
func DockerAvailable() bool {
	_, err := exec.LookPath("docker")
	if err != nil {
		return false
	}

	// Check if Docker daemon is running
	cmd := exec.Command("docker", "info")
	return cmd.Run() == nil
}

// DockerExecutionConfig holds configuration for Docker execution
type DockerExecutionConfig struct {
	// DockerImage is the Docker image to use (default: ubuntu:22.04)
	DockerImage string
	// SourceDir is the directory containing source images
	SourceDir string
	// TempDir is the temporary processing directory
	TempDir string
	// OutputDir is the output directory for prepared images
	OutputDir string
	// AdditionalMounts contains additional volume mounts (host:container)
	AdditionalMounts map[string]string
	// ContainerName is the name to assign to the container for references (e.g., file copying)
	ContainerName string
	// UseUniqueContainerName determines whether to generate a unique name for each container
	UseUniqueContainerName bool
	// InitCommands are commands to run after container startup
	// Each entry is a complete command with arguments
	InitCommands [][]string
}

// NewDefaultDockerConfig creates a default Docker execution configuration
func NewDefaultDockerConfig(sourceDir, tempDir, outputDir string) *DockerExecutionConfig {
	// Generate a unique identifier based on timestamp and random component
	uniqueID := fmt.Sprintf("%d-%x", time.Now().UnixNano(), rand.Intn(0x10000))

	return &DockerExecutionConfig{
		DockerImage:            "ubuntu:22.04",
		SourceDir:              sourceDir,
		TempDir:                tempDir,
		OutputDir:              outputDir,
		AdditionalMounts:       map[string]string{},
		ContainerName:          fmt.Sprintf("turingpi-image-builder-%s", uniqueID),
		UseUniqueContainerName: true,
	}
}

// ExecuteLinuxCommand executes a Linux command directly on Linux or via Docker on other platforms
// On Linux, it executes the command directly
// On non-Linux, it executes the command in a Docker container
func ExecuteLinuxCommand(config *DockerExecutionConfig, workDir string, command string, args ...string) error {
	if IsLinux() {
		// On Linux, execute command directly
		cmd := exec.Command(command, args...)
		cmd.Dir = workDir
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}

	if !DockerAvailable() {
		return fmt.Errorf("cannot execute Linux tools: Docker is not available on this non-Linux system. Please install Docker to use this feature")
	}

	// On non-Linux, use Docker
	return executeInDocker(config, workDir, command, args...)
}

// ExecuteLinuxCommandWithOutput executes a Linux command and returns its output
// On Linux, it executes the command directly
// On non-Linux, it executes the command in a Docker container
func ExecuteLinuxCommandWithOutput(config *DockerExecutionConfig, workDir string, command string, args ...string) ([]byte, error) {
	if IsLinux() {
		// On Linux, execute command directly
		cmd := exec.Command(command, args...)
		cmd.Dir = workDir
		return cmd.CombinedOutput()
	}

	if !DockerAvailable() {
		return nil, fmt.Errorf("cannot execute Linux tools: Docker is not available on this non-Linux system. Please install Docker to use this feature")
	}

	// On non-Linux, use Docker
	return executeInDockerWithOutput(config, workDir, command, args...)
}

// executeInDocker executes a command in a Docker container
func executeInDocker(config *DockerExecutionConfig, workDir string, command string, args ...string) error {
	// Build Docker command
	dockerArgs := []string{"run", "--rm"}

	// Add container name if specified
	if config.ContainerName != "" {
		dockerArgs = append(dockerArgs, "--name", config.ContainerName)
	}

	// Add volume mounts
	if config.SourceDir != "" {
		dockerArgs = append(dockerArgs, "-v", fmt.Sprintf("%s:/source:ro", config.SourceDir))
	}
	if config.TempDir != "" {
		dockerArgs = append(dockerArgs, "-v", fmt.Sprintf("%s:/temp", config.TempDir))
	}
	if config.OutputDir != "" {
		dockerArgs = append(dockerArgs, "-v", fmt.Sprintf("%s:/output", config.OutputDir))
	}

	// Add additional mounts
	for hostPath, containerPath := range config.AdditionalMounts {
		dockerArgs = append(dockerArgs, "-v", fmt.Sprintf("%s:%s", hostPath, containerPath))
	}

	// Set working directory in container
	containerWorkDir := "/temp"
	if workDir != "" {
		// Convert host path to container path
		if filepath.HasPrefix(workDir, config.TempDir) {
			relPath, err := filepath.Rel(config.TempDir, workDir)
			if err == nil {
				containerWorkDir = filepath.Join("/temp", relPath)
			}
		} else if filepath.HasPrefix(workDir, config.SourceDir) {
			relPath, err := filepath.Rel(config.SourceDir, workDir)
			if err == nil {
				containerWorkDir = filepath.Join("/source", relPath)
			}
		} else if filepath.HasPrefix(workDir, config.OutputDir) {
			relPath, err := filepath.Rel(config.OutputDir, workDir)
			if err == nil {
				containerWorkDir = filepath.Join("/output", relPath)
			}
		}
	}
	dockerArgs = append(dockerArgs, "-w", containerWorkDir)

	// Add Docker image
	dockerArgs = append(dockerArgs, config.DockerImage)

	// Install required packages if needed (could be optimized with a pre-built image)
	dockerArgs = append(dockerArgs, "bash", "-c",
		fmt.Sprintf("apt-get update && apt-get install -y kpartx xz-utils sudo && %s %s",
			command,
			escapeArgsForBash(args...)))

	// Execute Docker command
	cmd := exec.Command("docker", dockerArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// executeInDockerWithOutput executes a command in a Docker container and returns its output
func executeInDockerWithOutput(config *DockerExecutionConfig, workDir string, command string, args ...string) ([]byte, error) {
	// Build Docker command
	dockerArgs := []string{"run", "--rm"}

	// Add container name if specified
	if config.ContainerName != "" {
		dockerArgs = append(dockerArgs, "--name", config.ContainerName)
	}

	// Add volume mounts
	if config.SourceDir != "" {
		dockerArgs = append(dockerArgs, "-v", fmt.Sprintf("%s:/source:ro", config.SourceDir))
	}
	if config.TempDir != "" {
		dockerArgs = append(dockerArgs, "-v", fmt.Sprintf("%s:/temp", config.TempDir))
	}
	if config.OutputDir != "" {
		dockerArgs = append(dockerArgs, "-v", fmt.Sprintf("%s:/output", config.OutputDir))
	}

	// Add additional mounts
	for hostPath, containerPath := range config.AdditionalMounts {
		dockerArgs = append(dockerArgs, "-v", fmt.Sprintf("%s:%s", hostPath, containerPath))
	}

	// Set working directory in container
	containerWorkDir := "/temp"
	if workDir != "" {
		// Convert host path to container path
		if filepath.HasPrefix(workDir, config.TempDir) {
			relPath, err := filepath.Rel(config.TempDir, workDir)
			if err == nil {
				containerWorkDir = filepath.Join("/temp", relPath)
			}
		} else if filepath.HasPrefix(workDir, config.SourceDir) {
			relPath, err := filepath.Rel(config.SourceDir, workDir)
			if err == nil {
				containerWorkDir = filepath.Join("/source", relPath)
			}
		} else if filepath.HasPrefix(workDir, config.OutputDir) {
			relPath, err := filepath.Rel(config.OutputDir, workDir)
			if err == nil {
				containerWorkDir = filepath.Join("/output", relPath)
			}
		}
	}
	dockerArgs = append(dockerArgs, "-w", containerWorkDir)

	// Add Docker image
	dockerArgs = append(dockerArgs, config.DockerImage)

	// Install required packages if needed (could be optimized with a pre-built image)
	dockerArgs = append(dockerArgs, "bash", "-c",
		fmt.Sprintf("apt-get update && apt-get install -y kpartx xz-utils sudo && %s %s",
			command,
			escapeArgsForBash(args...)))

	// Execute Docker command
	cmd := exec.Command("docker", dockerArgs...)
	return cmd.CombinedOutput()
}

// escapeArgsForBash escapes command arguments for use in bash -c
func escapeArgsForBash(args ...string) string {
	var result string
	for i, arg := range args {
		if i > 0 {
			result += " "
		}
		// Simple escaping - for complex cases this would need to be more robust
		result += fmt.Sprintf("'%s'", arg)
	}
	return result
}
