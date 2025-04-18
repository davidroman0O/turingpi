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
				fmt.Sprintf("%s:%s", a.dockerAdapter.GetContainerName(), containerImgPath))

			copyOutput, err := runCommand(copyCmd)
			if err != nil {
				return "", fmt.Errorf("failed to copy image to container: %w, output: %s", err, string(copyOutput))
			}

			fmt.Printf("DEBUG: Copied image to container: %s -> %s\n", imgPathAbs, containerImgPath)
		} else {
			fmt.Printf("DEBUG: Image already exists in container: %s\n", checkOutput)
		}

		// Execute kpartx in Docker
		dockerCmd := fmt.Sprintf("kpartx -av %s", containerImgPath)
		fmt.Printf("DEBUG: Running in container: %s\n", dockerCmd)

		output, err := a.executeDockerCommand(dockerCmd)
		if err != nil {
			return "", fmt.Errorf("Docker partition mapping failed: %w", err)
		}

		fmt.Printf("DEBUG: kpartx output: %s\n", output)

		// Parse output using the same helper as native Linux approach for consistency
		rootDevice, err := internalParseKpartxOutput(output)
		if err != nil {
			return "", fmt.Errorf("failed to parse Docker kpartx output: %w", err)
		}

		rootDevPath := fmt.Sprintf("/dev/mapper/%s", rootDevice)
		fmt.Printf("Docker mapped root partition: %s\n", rootDevPath)

		// Check if the device was actually created
		checkDevCmd := fmt.Sprintf("ls -la %s", rootDevPath)
		checkDevOutput, err := a.executeDockerCommand(checkDevCmd)
		if err != nil {
			return "", fmt.Errorf("mapped device not found in container: %s", rootDevPath)
		}
		fmt.Printf("DEBUG: Device exists: %s\n", checkDevOutput)

		return rootDevPath, nil
	}

	// Native Linux approach
	cmd := exec.Command("kpartx", "-av", imgPathAbs)
	output, err := runCommand(cmd)
	if err != nil {
		return "", fmt.Errorf("kpartx failed: %w", err)
	}

	// Parse output to get device name
	rootDevice, err := internalParseKpartxOutput(string(output))
	if err != nil {
		return "", fmt.Errorf("failed to parse kpartx output: %w", err)
	}

	// Wait for device to become available
	rootDevPath := fmt.Sprintf("/dev/mapper/%s", rootDevice)
	if err := internalWaitForDevice(rootDevPath, 5*time.Second); err != nil {
		return "", fmt.Errorf("device not available: %w", err)
	}

	return rootDevPath, nil
}

// internalParseKpartxOutput parses kpartx output to extract root partition device path
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

		// Execute kpartx cleanup in Docker
		dockerCmd := fmt.Sprintf("kpartx -d /tmp/%s", filepath.Base(imgPathAbs))

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

		// Extract the partition name from the full path
		partName := filepath.Base(partitionDevice)

		// Make sure the mount directory exists in the container
		prepareCmd := "mkdir -p /mnt"
		_, err := a.executeDockerCommand(prepareCmd)
		if err != nil {
			return fmt.Errorf("Docker failed to create mount directory: %w", err)
		}

		// Check if /dev/mapper exists and has the device
		checkMapperCmd := "ls -la /dev/mapper/"
		mapperOutput, err := a.executeDockerCommand(checkMapperCmd)
		fmt.Printf("DEBUG: Mapper directory contents: \n%s\n", mapperOutput)

		// Execute mount in Docker - use the full path to the device
		dockerCmd := fmt.Sprintf("mount %s /mnt", partitionDevice)
		fmt.Printf("DEBUG: Running in container: %s\n", dockerCmd)

		output, err := a.executeDockerCommand(dockerCmd)
		if err != nil {
			return fmt.Errorf("Docker filesystem mounting failed: %w\nOutput: %s", err, output)
		}

		// Verify the mount worked
		verifyCmd := "mount | grep /mnt"
		verifyOutput, err := a.executeDockerCommand(verifyCmd)
		if err != nil {
			return fmt.Errorf("mount verification failed: %w", err)
		}
		fmt.Printf("DEBUG: Mount verification: %s\n", verifyOutput)

		fmt.Printf("Docker mounted %s to /mnt\n", partName)
		return nil
	}

	// Native Linux approach
	cmd := exec.Command("mount", partitionDevice, mountDir)
	_, err := runCommand(cmd)
	if err != nil {
		return fmt.Errorf("mount failed: %w", err)
	}

	// Verify it's actually mounted
	if !internalIsMounted(mountDir) {
		return fmt.Errorf("filesystem not mounted at %s", mountDir)
	}

	return nil
}

// internalIsMounted checks if a path is a mountpoint
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

		// Execute umount in Docker
		dockerCmd := "umount /mnt"

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
