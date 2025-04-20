package operations

import (
	"context"
	"fmt"
	"io/fs"
	"os/exec"
	"path/filepath"
	"strings"
)

// FilesystemOperations provides filesystem operations that can be executed
// either directly on a Linux host or inside a container on non-Linux systems
type FilesystemOperations struct {
	executor CommandExecutor
}

// NewFilesystemOperations creates a new FilesystemOperations instance
func NewFilesystemOperations(executor CommandExecutor) *FilesystemOperations {
	return &FilesystemOperations{
		executor: executor,
	}
}

// IsPartitionMounted checks if a partition is mounted
func (f *FilesystemOperations) IsPartitionMounted(ctx context.Context, partition string) (bool, string, error) {
	output, err := f.executor.Execute(ctx, "findmnt", "-n", "-o", "TARGET", partition)
	if err != nil {
		if strings.Contains(string(output), "not found") || strings.Contains(string(output), "not a block device") {
			return false, "", nil
		}
		// Exit status 1 means not mounted
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return false, "", nil
		}
		return false, "", fmt.Errorf("failed to check if partition is mounted: %w", err)
	}

	mountPoint := strings.TrimSpace(string(output))
	return mountPoint != "", mountPoint, nil
}

// GetFilesystemType gets the filesystem type of a partition
func (f *FilesystemOperations) GetFilesystemType(ctx context.Context, partition string) (string, error) {
	output, err := f.executor.Execute(ctx, "blkid", "-o", "value", "-s", "TYPE", partition)
	if err != nil {
		return "", fmt.Errorf("failed to get filesystem type: %w", err)
	}

	return strings.TrimSpace(string(output)), nil
}

// MapPartitions maps partitions in a disk image using kpartx
func (f *FilesystemOperations) MapPartitions(ctx context.Context, imgPathAbs string) (string, error) {
	output, err := f.executor.Execute(ctx, "kpartx", "-av", imgPathAbs)
	if err != nil {
		return "", fmt.Errorf("failed to map partitions: %w, output: %s", err, string(output))
	}

	// Parse kpartx output to get the root partition device
	lines := strings.Split(string(output), "\n")
	if len(lines) < 2 {
		return "", fmt.Errorf("unexpected kpartx output format (less than 2 lines)")
	}

	// Check second line for root partition (assuming first is boot, second is root)
	rootLine := lines[1]
	parts := strings.Fields(rootLine)
	if len(parts) < 3 || !strings.HasPrefix(parts[0], "add") {
		return "", fmt.Errorf("unexpected kpartx output format: %s", rootLine)
	}

	return fmt.Sprintf("/dev/mapper/%s", parts[2]), nil
}

// UnmapPartitions unmaps partitions that were mapped with kpartx
func (f *FilesystemOperations) UnmapPartitions(ctx context.Context, imgPathAbs string) error {
	output, err := f.executor.Execute(ctx, "kpartx", "-d", imgPathAbs)
	if err != nil {
		return fmt.Errorf("failed to unmap partitions: %w, output: %s", err, string(output))
	}
	return nil
}

// Mount mounts a filesystem to a specified directory
func (f *FilesystemOperations) Mount(ctx context.Context, device, mountPoint, fsType string, options []string) error {
	// Create mount point directory
	if _, err := f.executor.Execute(ctx, "mkdir", "-p", mountPoint); err != nil {
		return fmt.Errorf("failed to create mount point directory: %w", err)
	}

	args := []string{device, mountPoint}
	if fsType != "" {
		args = append(args, "-t", fsType)
	}
	if len(options) > 0 {
		args = append(args, "-o", strings.Join(options, ","))
	}

	output, err := f.executor.Execute(ctx, "mount", args...)
	if err != nil {
		return fmt.Errorf("mount failed: %s: %w", string(output), err)
	}

	return nil
}

// Unmount unmounts a filesystem
func (f *FilesystemOperations) Unmount(ctx context.Context, mountPoint string) error {
	output, err := f.executor.Execute(ctx, "umount", mountPoint)
	if err != nil {
		return fmt.Errorf("unmount failed: %s: %w", string(output), err)
	}

	return nil
}

// Format formats a partition with a specified filesystem
func (f *FilesystemOperations) Format(ctx context.Context, device, fsType string, label string) error {
	var cmdName string
	var args []string

	switch fsType {
	case "ext4":
		cmdName = "mkfs.ext4"
		args = []string{device}
		if label != "" {
			args = append(args, "-L", label)
		}
	case "fat32", "vfat":
		cmdName = "mkfs.vfat"
		args = []string{device}
		if label != "" {
			args = append(args, "-n", label)
		}
	default:
		return fmt.Errorf("unsupported filesystem type: %s", fsType)
	}

	output, err := f.executor.Execute(ctx, cmdName, args...)
	if err != nil {
		return fmt.Errorf("format failed: %s: %w", string(output), err)
	}

	return nil
}

// ResizeFilesystem resizes a filesystem to fill its partition
func (f *FilesystemOperations) ResizeFilesystem(ctx context.Context, device string) error {
	// Get filesystem type
	output, err := f.executor.Execute(ctx, "blkid", "-o", "value", "-s", "TYPE", device)
	if err != nil {
		return fmt.Errorf("failed to determine filesystem type: %w", err)
	}

	fsType := strings.TrimSpace(string(output))

	// Resize based on filesystem type
	switch fsType {
	case "ext4":
		output, err = f.executor.Execute(ctx, "resize2fs", device)
	case "vfat", "fat32":
		// FAT filesystems generally don't need explicit resizing after partition table update
		return nil
	default:
		return fmt.Errorf("unsupported filesystem type for resize: %s", fsType)
	}

	if err != nil {
		return fmt.Errorf("resize failed: %s: %w", string(output), err)
	}

	return nil
}

// CopyDirectory recursively copies a directory to another location
func (f *FilesystemOperations) CopyDirectory(ctx context.Context, src, dst string) error {
	// Create dst directory if it doesn't exist
	if _, err := f.executor.Execute(ctx, "mkdir", "-p", dst); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	// Use rsync for efficient directory copying
	output, err := f.executor.Execute(ctx, "rsync", "-av", src+"/", dst+"/")
	if err != nil {
		return fmt.Errorf("rsync failed: %s: %w", string(output), err)
	}

	return nil
}

// WriteFile writes content to a file
func (f *FilesystemOperations) WriteFile(mountDir, path string, content []byte, perm fs.FileMode) error {
	fullPath := filepath.Join(mountDir, path)

	// Ensure parent directory exists
	dirPath := filepath.Dir(fullPath)
	if _, err := f.executor.Execute(context.Background(), "mkdir", "-p", dirPath); err != nil {
		return fmt.Errorf("failed to create parent directory: %w", err)
	}

	// Write content directly to the file
	escapedContent := strings.ReplaceAll(string(content), "'", "'\\''")
	cmd := fmt.Sprintf("echo -n '%s' > '%s'", escapedContent, fullPath)
	if _, err := f.executor.Execute(context.Background(), "bash", "-c", cmd); err != nil {
		return fmt.Errorf("failed to write file content: %w", err)
	}

	// Set permissions
	if _, err := f.executor.Execute(context.Background(), "chmod", fmt.Sprintf("%o", perm), fullPath); err != nil {
		return fmt.Errorf("failed to set file permissions: %w", err)
	}

	return nil
}

// ReadFile reads a file from the mounted filesystem
func (f *FilesystemOperations) ReadFile(mountDir, relativePath string) ([]byte, error) {
	fullPath := filepath.Join(mountDir, relativePath)
	output, err := f.executor.Execute(context.Background(), "cat", fullPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}
	return output, nil
}

// FileExists checks if a file exists in the mounted filesystem
func (f *FilesystemOperations) FileExists(mountDir, relativePath string) bool {
	fullPath := filepath.Join(mountDir, relativePath)
	_, err := f.executor.Execute(context.Background(), "test", "-e", fullPath)
	return err == nil
}

// CopyFile copies a file to a mounted filesystem
func (f *FilesystemOperations) CopyFile(ctx context.Context, mountDir, sourcePath, destPath string) error {
	// Ensure source file exists
	if _, err := f.executor.Execute(ctx, "test", "-f", sourcePath); err != nil {
		return fmt.Errorf("source file does not exist: %s", sourcePath)
	}

	// Ensure the destination directory exists
	destDir := filepath.Dir(filepath.Join(mountDir, destPath))
	if _, err := f.executor.Execute(ctx, "mkdir", "-p", destDir); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	// Copy the file
	fullDestPath := filepath.Join(mountDir, destPath)
	output, err := f.executor.Execute(ctx, "cp", sourcePath, fullDestPath)
	if err != nil {
		return fmt.Errorf("failed to copy file: %w, output: %s", err, string(output))
	}

	return nil
}

// IsDirectory checks if a path is a directory
func (f *FilesystemOperations) IsDirectory(mountDir, relativePath string) bool {
	fullPath := filepath.Join(mountDir, relativePath)
	_, err := f.executor.Execute(context.Background(), "test", "-d", fullPath)
	return err == nil
}

// MakeDirectory creates a directory at the specified path
func (f *FilesystemOperations) MakeDirectory(mountDir, path string, perm fs.FileMode) error {
	fullPath := filepath.Join(mountDir, path)
	_, err := f.executor.Execute(context.Background(), "mkdir", "-p", fullPath)
	if err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Set permissions
	_, err = f.executor.Execute(context.Background(), "chmod", fmt.Sprintf("%o", perm), fullPath)
	if err != nil {
		return fmt.Errorf("failed to set directory permissions: %w", err)
	}

	return nil
}

// ChangePermissions changes the permissions of a file or directory
func (f *FilesystemOperations) ChangePermissions(mountDir, path string, perm fs.FileMode) error {
	fullPath := filepath.Join(mountDir, path)
	_, err := f.executor.Execute(context.Background(), "chmod", fmt.Sprintf("%o", perm), fullPath)
	if err != nil {
		return fmt.Errorf("failed to change permissions: %w", err)
	}
	return nil
}
