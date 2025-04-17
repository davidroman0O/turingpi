package imageops

import (
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// MapPartitions maps the partitions of an image file and returns the root partition device
func MapPartitions(imgPathAbs string) (string, error) {
	cmd := exec.Command("kpartx", "-av", imgPathAbs)
	output, err := runCommand(cmd)
	if err != nil {
		return "", fmt.Errorf("failed to map partitions: %w", err)
	}

	rootPartitionDev, err := parseKpartxOutput(string(output))
	if err != nil {
		return "", fmt.Errorf("failed to parse kpartx output: %w", err)
	}

	// Wait for device to be ready
	if err := waitForDevice(rootPartitionDev, 10*time.Second); err != nil {
		return "", fmt.Errorf("device not ready: %w", err)
	}

	return rootPartitionDev, nil
}

// CleanupPartitions unmaps all partitions of an image file
func CleanupPartitions(imgPathAbs string) error {
	cmd := exec.Command("kpartx", "-d", imgPathAbs)
	if _, err := runCommand(cmd); err != nil {
		return fmt.Errorf("failed to unmap partitions: %w", err)
	}
	return nil
}

// MountFilesystem mounts a partition device to a directory
func MountFilesystem(partitionDevice, mountDir string) error {
	if err := os.MkdirAll(mountDir, 0755); err != nil {
		return fmt.Errorf("failed to create mount directory: %w", err)
	}

	cmd := exec.Command("mount", partitionDevice, mountDir)
	if _, err := runCommand(cmd); err != nil {
		return fmt.Errorf("failed to mount filesystem: %w", err)
	}

	return nil
}

// UnmountFilesystem unmounts a filesystem
func UnmountFilesystem(mountDir string) error {
	if !isMounted(mountDir) {
		return nil
	}

	cmd := exec.Command("umount", mountDir)
	if _, err := runCommand(cmd); err != nil {
		return fmt.Errorf("failed to unmount filesystem: %w", err)
	}

	return nil
}

// isMounted checks if a path is mounted
func isMounted(path string) bool {
	cmd := exec.Command("mountpoint", "-q", path)
	return cmd.Run() == nil
}

// writeToFileAsRoot writes content to a file with root permissions
func writeToFileAsRoot(filePath string, content []byte, perm fs.FileMode) error {
	// Create parent directories if they don't exist
	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		return fmt.Errorf("failed to create parent directories: %w", err)
	}

	// Write content to file
	if err := os.WriteFile(filePath, content, perm); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

// runCommand executes a command and returns its output
func runCommand(cmd *exec.Cmd) ([]byte, error) {
	var stderr strings.Builder
	cmd.Stderr = &stderr

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("command failed: %w. stderr: %s", err, stderr.String())
	}

	return output, nil
}

// parseKpartxOutput parses kpartx output to get the root partition device
func parseKpartxOutput(output string) (string, error) {
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.Contains(line, "add map") {
			fields := strings.Fields(line)
			if len(fields) > 2 {
				return "/dev/mapper/" + fields[2], nil
			}
		}
	}
	return "", fmt.Errorf("root partition device not found in kpartx output")
}

// waitForDevice waits for a device to be ready
func waitForDevice(devicePath string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(devicePath); err == nil {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("timeout waiting for device %s", devicePath)
}
