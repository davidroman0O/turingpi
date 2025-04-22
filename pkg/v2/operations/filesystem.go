package operations

import (
	"context"
	"encoding/base64"
	"fmt"
	"io/fs"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
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

	// Ensure parent directory exists - using MkdirAll for reliability
	dirPath := filepath.Dir(fullPath)
	if _, err := f.executor.Execute(context.Background(), "mkdir", "-p", dirPath); err != nil {
		return fmt.Errorf("failed to create parent directory: %w", err)
	}

	// Create a temporary file with the content
	tempFile := filepath.Join(dirPath, fmt.Sprintf("temp-%d", time.Now().UnixNano()))

	// Write content directly to the temp file using base64 encoding to avoid shell escaping issues
	encodedContent := base64.StdEncoding.EncodeToString(content)
	if _, err := f.executor.Execute(context.Background(), "bash", "-c",
		fmt.Sprintf("echo '%s' | base64 -d > '%s'", encodedContent, tempFile)); err != nil {
		return fmt.Errorf("failed to write file content: %w", err)
	}

	// Move the temp file to the final destination (atomic operation)
	if _, err := f.executor.Execute(context.Background(), "mv", tempFile, fullPath); err != nil {
		return fmt.Errorf("failed to move temp file to destination: %w", err)
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

	// First check if file exists
	if _, err := f.executor.Execute(context.Background(), "test", "-f", fullPath); err != nil {
		return nil, fmt.Errorf("file does not exist: %w", err)
	}

	// Use base64 to read the file to avoid binary data issues and newline handling
	output, err := f.executor.Execute(context.Background(), "bash", "-c",
		fmt.Sprintf("cat '%s' | base64", fullPath))
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	// Decode the base64 content
	content, err := base64.StdEncoding.DecodeString(string(output))
	if err != nil {
		return nil, fmt.Errorf("failed to decode file content: %w", err)
	}

	return content, nil
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

	// Full path to destination
	fullDestPath := filepath.Join(mountDir, destPath)

	// Create a temporary destination to ensure atomic copy
	tempDest := fmt.Sprintf("%s.tmp.%d", fullDestPath, time.Now().UnixNano())

	// Copy the file to the temp location
	output, err := f.executor.Execute(ctx, "cp", "-f", sourcePath, tempDest)
	if err != nil {
		return fmt.Errorf("failed to copy file: %w, output: %s", err, string(output))
	}

	// Move the temp file to final destination (atomic)
	if _, err := f.executor.Execute(ctx, "mv", tempDest, fullDestPath); err != nil {
		// Try to cleanup temp file
		f.executor.Execute(ctx, "rm", "-f", tempDest)
		return fmt.Errorf("failed to finalize file copy: %w", err)
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

// FileInfo represents information about a file or directory
type FileInfo struct {
	Name        string      // Base name of the file
	Size        int64       // Size in bytes
	IsDir       bool        // Is this a directory
	Mode        fs.FileMode // File mode and permission bits
	ModTime     time.Time   // Modification time
	SymlinkPath string      // Target path if this is a symlink, empty otherwise
}

// ListFiles lists files at a given location, similar to the ls command
func (f *FilesystemOperations) ListFiles(ctx context.Context, dir string) ([]FileInfo, error) {
	// Use a more basic ls format that works in BusyBox
	// -l: long format
	// -a: show hidden files
	output, err := f.executor.Execute(ctx, "ls", "-la", dir)
	if err != nil {
		return nil, fmt.Errorf("failed to list files: %w", err)
	}

	lines := strings.Split(string(output), "\n")
	var files []FileInfo

	fmt.Println("Listing files in", dir)

	// Skip the first line, which is the total count
	for i := 1; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}

		// Parse line in the format:
		// <permissions> <links> <user> <group> <size> <month> <day> <time/year> <filename>
		// Example: -rw-r--r-- 1 user group 12345 Jan 1 12:34 file.txt
		fields := strings.Fields(line)
		if len(fields) < 8 {
			continue // Skip lines with unexpected format
		}

		// Extract information
		permissions := fields[0]
		size, _ := strconv.ParseInt(fields[4], 10, 64)

		// Filename might contain spaces and is at the end after timestamp
		// Find the index where filename starts (after timestamp)
		nameIndex := 7
		if len(fields) > nameIndex {
			name := strings.Join(fields[nameIndex:], " ")

			// Check if it's a directory or a symlink
			isDir := strings.HasPrefix(permissions, "d")
			isSymlink := strings.HasPrefix(permissions, "l")

			// Simplified modTime parsing - just current time since format varies
			modTime := time.Now()

			// Parse permissions
			var mode fs.FileMode
			if isDir {
				mode |= fs.ModeDir
			}
			if isSymlink {
				mode |= fs.ModeSymlink
			}

			// Parse permissions (simplified)
			if strings.Contains(permissions[1:4], "r") {
				mode |= 0400
			}
			if strings.Contains(permissions[1:4], "w") {
				mode |= 0200
			}
			if strings.Contains(permissions[1:4], "x") {
				mode |= 0100
			}
			if strings.Contains(permissions[4:7], "r") {
				mode |= 0040
			}
			if strings.Contains(permissions[4:7], "w") {
				mode |= 0020
			}
			if strings.Contains(permissions[4:7], "x") {
				mode |= 0010
			}
			if strings.Contains(permissions[7:10], "r") {
				mode |= 0004
			}
			if strings.Contains(permissions[7:10], "w") {
				mode |= 0002
			}
			if strings.Contains(permissions[7:10], "x") {
				mode |= 0001
			}

			// Handle symlinks
			var symlinkPath string
			if isSymlink && strings.Contains(name, " -> ") {
				parts := strings.Split(name, " -> ")
				name = parts[0]
				symlinkPath = parts[1]
			}

			// Add file to results
			files = append(files, FileInfo{
				Name:        name,
				Size:        size,
				IsDir:       isDir,
				Mode:        mode,
				ModTime:     modTime,
				SymlinkPath: symlinkPath,
			})
		}
	}

	return files, nil
}

// ListFilesBasic lists files at a given location and returns just the filenames
func (f *FilesystemOperations) ListFilesBasic(ctx context.Context, dir string) ([]string, error) {
	// Log the actual directory we're listing for debugging purposes
	fmt.Printf("ListFilesBasic: Listing files in directory: %s\n", dir)

	output, err := f.executor.Execute(ctx, "ls", "-la", dir)
	if err != nil {
		return nil, fmt.Errorf("failed to list files: %w", err)
	}

	fmt.Println("================")
	fmt.Println(string(output))
	fmt.Println("================")

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	var files []string

	for _, line := range lines {
		if line != "" {
			files = append(files, line)
		}
	}

	output, err = f.executor.Execute(ctx, "pwd")
	if err != nil {
		return nil, fmt.Errorf("failed to get current directory: %w", err)
	}

	return files, nil
}
