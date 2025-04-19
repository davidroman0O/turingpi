package imageops

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/davidroman0O/turingpi/pkg/tpi/platform"
)

// MapPartitions implements ImageOpsAdapter.MapPartitions
func (a *imageOpsAdapter) MapPartitions(imgPathAbs string) (string, error) {
	// For non-Linux platforms, use Docker
	if !platform.IsLinux() && a.isDockerInitialized() {
		fmt.Println("Using Docker for partition mapping...")

		// Check if the image file exists in the container
		// First, copy the image to the Docker container if it's not already there
		fmt.Printf("DEBUG: Checking image path: %s\n", imgPathAbs)

		// The image should be in the /tmp directory inside the container
		// Get the basename of the image file
		imgBaseName := filepath.Base(imgPathAbs)
		containerImgPath := fmt.Sprintf("/tmp/%s", imgBaseName)

		// Check if the image exists inside the container
		checkCmd := fmt.Sprintf("ls -la %s", containerImgPath)
		checkOutput, err := a.executeDockerCommand(checkCmd)
		if err != nil {
			fmt.Printf("DEBUG: Image not found in container at %s\n", containerImgPath)
			fmt.Printf("DEBUG: Will copy image to container\n")

			// Copy image to container
			copyCmd := exec.Command("docker", "cp",
				imgPathAbs,
				fmt.Sprintf("%s:%s", a.dockerAdapter.Container.ContainerID, containerImgPath))

			copyOutput, err := runCommand(copyCmd)
			if err != nil {
				return "", fmt.Errorf("failed to copy image to container: %w, output: %s", err, string(copyOutput))
			}

			fmt.Printf("DEBUG: Copied image to container: %s -> %s\n", imgPathAbs, containerImgPath)
		} else {
			fmt.Printf("DEBUG: Image already exists in container: %s\n", checkOutput)
		}

		// Debug: Check if we have necessary tools and permissions
		debugCmds := []string{
			"id",                               // Check current user
			"ls -la /dev/loop*",                // Check loop devices
			"ls -la /dev/mapper",               // Check device mapper
			"sudo losetup -a",                  // Check existing loop devices
			"lsmod | grep loop",                // Check if loop module is loaded
			"sudo modprobe loop",               // Try to load loop module
			"sudo losetup -f",                  // Find first available loop device
			"file " + containerImgPath,         // Check file type
			"sudo file -s " + containerImgPath, // Check filesystem type
		}

		for _, cmd := range debugCmds {
			fmt.Printf("DEBUG: Running: %s\n", cmd)
			output, err := a.executeDockerCommand(cmd)
			fmt.Printf("DEBUG: Output: %s\n", output)
			if err != nil {
				fmt.Printf("DEBUG: Error: %v\n", err)
			}
		}

		// Execute kpartx in Docker with sudo
		dockerCmd := fmt.Sprintf("sudo kpartx -av %s", containerImgPath)
		fmt.Printf("DEBUG: Running in container: %s\n", dockerCmd)

		output, err := a.executeDockerCommand(dockerCmd)
		if err != nil {
			return "", fmt.Errorf("Docker partition mapping failed: %w", err)
		}

		fmt.Printf("DEBUG: kpartx output: %s\n", output)

		// Parse kpartx output to get the root partition device
		rootPartition, err := internalParseKpartxOutput(output)
		if err != nil {
			return "", fmt.Errorf("failed to parse Docker kpartx output: %w", err)
		}

		// Return the full path to the root partition device
		return fmt.Sprintf("/dev/mapper/%s", rootPartition), nil
	}

	// Native Linux approach
	cmd := exec.Command("kpartx", "-av", imgPathAbs)
	output, err := runCommand(cmd)
	if err != nil {
		return "", fmt.Errorf("failed to map partitions: %w", err)
	}

	// Parse kpartx output to get the root partition device
	rootPartition, err := internalParseKpartxOutput(string(output))
	if err != nil {
		return "", fmt.Errorf("failed to parse kpartx output: %w", err)
	}

	// Return the full path to the root partition device
	return fmt.Sprintf("/dev/mapper/%s", rootPartition), nil
}

// internalParseKpartxOutput parses the output of kpartx to get the root partition device
func internalParseKpartxOutput(output string) (string, error) {
	// Example output:
	// add map loop1p1 (253:1): 0 524288 linear 7:1 8192
	// add map loop1p2 (253:2): 0 32768000 linear 7:1 532480
	//
	// For simplicity, we assume the first line is boot and second is root
	// This could be improved to detect partitions by examining sizes or file systems

	lines := strings.Split(output, "\n")
	if len(lines) < 2 {
		return "", fmt.Errorf("unexpected kpartx output format (less than 2 lines)")
	}

	// Check second line for root partition
	rootLine := lines[1]
	parts := strings.Fields(rootLine)
	if len(parts) < 3 || !strings.HasPrefix(parts[0], "add") || !strings.HasPrefix(parts[2], "loop") {
		return "", fmt.Errorf("unexpected kpartx output format: %s", rootLine)
	}

	return parts[2], nil
}

// internalWaitForDevice waits for a device to become available
func internalWaitForDevice(devicePath string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(devicePath); err == nil {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("timeout waiting for device %s", devicePath)
}

// CleanupPartitions implements ImageOpsAdapter.CleanupPartitions
func (a *imageOpsAdapter) CleanupPartitions(imgPathAbs string) error {
	// For non-Linux platforms, use Docker
	if !platform.IsLinux() && a.isDockerInitialized() {
		fmt.Println("Using Docker for partition cleanup...")

		// Execute kpartx cleanup in Docker with sudo
		dockerCmd := fmt.Sprintf("sudo kpartx -d /tmp/%s", filepath.Base(imgPathAbs))

		_, err := a.executeDockerCommand(dockerCmd)
		if err != nil {
			return fmt.Errorf("Docker partition cleanup failed: %w", err)
		}

		fmt.Println("Docker partition cleanup completed")
		return nil
	}

	// Native Linux approach
	cmd := exec.Command("kpartx", "-d", imgPathAbs)
	_, err := runCommand(cmd)
	if err != nil {
		return fmt.Errorf("failed to unmap partitions: %w", err)
	}
	return nil
}

// MountFilesystem implements ImageOpsAdapter.MountFilesystem
func (a *imageOpsAdapter) MountFilesystem(partitionDevice, mountDir string) error {
	// For non-Linux platforms, use Docker
	if !platform.IsLinux() && a.isDockerInitialized() {
		fmt.Println("Using Docker for filesystem mounting...")

		// Create mount directory in container
		mkdirCmd := fmt.Sprintf("sudo mkdir -p %s", mountDir)
		_, err := a.executeDockerCommand(mkdirCmd)
		if err != nil {
			return fmt.Errorf("failed to create mount directory in Docker: %w", err)
		}

		// Execute mount in Docker with sudo
		dockerCmd := fmt.Sprintf("sudo mount %s %s", partitionDevice, mountDir)
		fmt.Printf("DEBUG: Running in container: %s\n", dockerCmd)

		_, err = a.executeDockerCommand(dockerCmd)
		if err != nil {
			return fmt.Errorf("Docker filesystem mounting failed: %w", err)
		}

		fmt.Printf("DEBUG: Mounted %s to %s in Docker container\n", partitionDevice, mountDir)
		return nil
	}

	// Native Linux approach
	// Create mount directory if it doesn't exist
	if err := os.MkdirAll(mountDir, 0755); err != nil {
		return fmt.Errorf("failed to create mount directory: %w", err)
	}

	// Wait for device to be available
	if err := internalWaitForDevice(partitionDevice, 10*time.Second); err != nil {
		return fmt.Errorf("device not available: %w", err)
	}

	cmd := exec.Command("mount", partitionDevice, mountDir)
	_, err := runCommand(cmd)
	if err != nil {
		return fmt.Errorf("mount failed: %w", err)
	}

	return nil
}

// internalIsMounted checks if a path is mounted
func internalIsMounted(path string) bool {
	// Only for Linux - for non-Linux platforms, this is handled in the Docker container
	cmd := exec.Command("mountpoint", "-q", path)
	return cmd.Run() == nil
}

// UnmountFilesystem implements ImageOpsAdapter.UnmountFilesystem
func (a *imageOpsAdapter) UnmountFilesystem(mountDir string) error {
	// For non-Linux platforms, use Docker
	if !platform.IsLinux() && a.isDockerInitialized() {
		fmt.Println("Using Docker for filesystem unmounting...")

		// Execute umount in Docker with sudo
		dockerCmd := fmt.Sprintf("sudo umount %s", mountDir)

		_, err := a.executeDockerCommand(dockerCmd)
		if err != nil {
			return fmt.Errorf("Docker filesystem unmounting failed: %w", err)
		}

		fmt.Println("Docker unmounted filesystem")
		return nil
	}

	// Native Linux approach
	// Check if actually mounted first
	if !internalIsMounted(mountDir) {
		return nil // Already unmounted
	}

	cmd := exec.Command("umount", mountDir)
	_, err := runCommand(cmd)
	if err != nil {
		return fmt.Errorf("umount failed: %w", err)
	}

	return nil
}
