package linux

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// Partition represents a disk partition
type Partition struct {
	Device     string
	Number     int
	Start      string
	End        string
	Size       string
	Type       string
	Filesystem string
}

// Filesystem provides operations for handling Linux filesystem operations
type Filesystem struct {
	execCommand func(cmd *exec.Cmd) ([]byte, error)
}

// NewFilesystem creates a new Filesystem operations instance
func NewFilesystem() *Filesystem {
	return &Filesystem{
		execCommand: func(cmd *exec.Cmd) ([]byte, error) {
			return cmd.CombinedOutput()
		},
	}
}

// MapPartitions maps partitions in a disk image using kpartx
func (fs *Filesystem) MapPartitions(ctx context.Context, imgPathAbs string) (string, error) {
	cmd := exec.Command("kpartx", "-av", imgPathAbs)
	output, err := fs.execCommand(cmd)
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
func (fs *Filesystem) UnmapPartitions(ctx context.Context, imgPathAbs string) error {
	cmd := exec.Command("kpartx", "-d", imgPathAbs)
	output, err := fs.execCommand(cmd)
	if err != nil {
		return fmt.Errorf("failed to unmap partitions: %w, output: %s", err, string(output))
	}
	return nil
}

// Mount mounts a filesystem to a specified directory
func (f *Filesystem) Mount(ctx context.Context, device, mountPoint, fsType string, options []string) error {
	// Create mount point if it doesn't exist
	if err := os.MkdirAll(mountPoint, 0755); err != nil {
		return fmt.Errorf("failed to create mount point directory: %w", err)
	}

	args := []string{device, mountPoint}
	if fsType != "" {
		args = append(args, "-t", fsType)
	}
	if len(options) > 0 {
		args = append(args, "-o", strings.Join(options, ","))
	}

	cmd := exec.CommandContext(ctx, "mount", args...)
	if output, err := f.execCommand(cmd); err != nil {
		return fmt.Errorf("mount failed: %s: %w", string(output), err)
	}

	return nil
}

// Unmount unmounts a filesystem
func (f *Filesystem) Unmount(ctx context.Context, mountPoint string) error {
	cmd := exec.CommandContext(ctx, "umount", mountPoint)
	if output, err := f.execCommand(cmd); err != nil {
		return fmt.Errorf("unmount failed: %s: %w", string(output), err)
	}

	return nil
}

// Format formats a partition with a specified filesystem
func (f *Filesystem) Format(ctx context.Context, device, fsType string, label string) error {
	var cmd *exec.Cmd

	switch fsType {
	case "ext4":
		args := []string{device}
		if label != "" {
			args = append(args, "-L", label)
		}
		cmd = exec.CommandContext(ctx, "mkfs.ext4", args...)
	case "fat32", "vfat":
		args := []string{device}
		if label != "" {
			args = append(args, "-n", label)
		}
		cmd = exec.CommandContext(ctx, "mkfs.vfat", args...)
	default:
		return fmt.Errorf("unsupported filesystem type: %s", fsType)
	}

	if output, err := f.execCommand(cmd); err != nil {
		return fmt.Errorf("format failed: %s: %w", string(output), err)
	}

	return nil
}

// ResizeFilesystem resizes a filesystem to fill its partition
func (f *Filesystem) ResizeFilesystem(ctx context.Context, device string) error {
	// Get filesystem type
	cmd := exec.CommandContext(ctx, "blkid", "-o", "value", "-s", "TYPE", device)
	fsType, err := f.execCommand(cmd)
	if err != nil {
		return fmt.Errorf("failed to determine filesystem type: %w", err)
	}

	// Resize based on filesystem type
	switch strings.TrimSpace(string(fsType)) {
	case "ext4":
		cmd = exec.CommandContext(ctx, "resize2fs", device)
	case "vfat", "fat32":
		// FAT filesystems generally don't need explicit resizing after partition table update
		return nil
	default:
		return fmt.Errorf("unsupported filesystem type for resize: %s", fsType)
	}

	if output, err := f.execCommand(cmd); err != nil {
		return fmt.Errorf("resize failed: %s: %w", string(output), err)
	}

	return nil
}

// ListPartitions lists all partitions on a device
func (f *Filesystem) ListPartitions(ctx context.Context, device string) ([]Partition, error) {
	cmd := exec.CommandContext(ctx, "fdisk", "-l", device)
	output, err := f.execCommand(cmd)
	if err != nil {
		return nil, fmt.Errorf("failed to list partitions: %s: %w", string(output), err)
	}

	// Parse fdisk output to extract partition information
	partitions := []Partition{}
	partRe := regexp.MustCompile(`(?m)^(` + regexp.QuoteMeta(device) + `.*?)\s+(\d+)\s+(\d+)\s+(\d+)\s+([0-9\.]+[GMK])\s+(\w+)(\s+(.+))?$`)

	matches := partRe.FindAllStringSubmatch(string(output), -1)
	for _, match := range matches {
		if len(match) < 7 {
			continue
		}

		// Create partition entry
		partition := Partition{
			Device: match[1],
			Number: atoi(match[2]), // You'll need to implement atoi()
			Start:  match[3],
			End:    match[4],
			Size:   match[5],
			Type:   match[6],
		}

		// Add filesystem type if available
		if len(match) > 8 && match[8] != "" {
			partition.Filesystem = match[8]
		}

		partitions = append(partitions, partition)
	}

	return partitions, nil
}

// CreatePartition creates a new partition on a device
func (f *Filesystem) CreatePartition(ctx context.Context, device string, start, end int, partType string) error {
	// Create a temporary script for fdisk commands
	tempScript, err := os.CreateTemp("", "fdisk-script")
	if err != nil {
		return fmt.Errorf("failed to create temporary script: %w", err)
	}
	defer os.Remove(tempScript.Name())

	// Write fdisk commands to script
	script := []string{
		"n", // new partition
	}

	// Add partition type (p for primary, e for extended, l for logical)
	switch partType {
	case "primary":
		script = append(script, "p")
	case "extended":
		script = append(script, "e")
	case "logical":
		script = append(script, "l")
	default:
		// Default to primary
		script = append(script, "p")
	}

	// Accept default partition number
	script = append(script, "")

	// Set start sector
	if start > 0 {
		script = append(script, fmt.Sprintf("%d", start))
	} else {
		// Accept default
		script = append(script, "")
	}

	// Set end sector
	if end > 0 {
		script = append(script, fmt.Sprintf("%d", end))
	} else {
		// Accept default (end of device)
		script = append(script, "")
	}

	// Write changes
	script = append(script, "w")

	// Write script to file
	if _, err := tempScript.WriteString(strings.Join(script, "\n")); err != nil {
		return fmt.Errorf("failed to write fdisk script: %w", err)
	}
	tempScript.Close()

	// Execute fdisk with script
	cmd := exec.CommandContext(ctx, "fdisk", device)
	cmd.Stdin = strings.NewReader(strings.Join(script, "\n"))

	if output, err := f.execCommand(cmd); err != nil {
		return fmt.Errorf("fdisk failed: %s: %w", string(output), err)
	}

	return nil
}

// atoi converts a string to an integer, returning 0 if conversion fails
func atoi(s string) int {
	var n int
	_, err := fmt.Sscanf(s, "%d", &n)
	if err != nil {
		return 0
	}
	return n
}

// IsPartitionMounted checks if a partition is mounted
func (f *Filesystem) IsPartitionMounted(ctx context.Context, partition string) (bool, string, error) {
	cmd := exec.CommandContext(ctx, "findmnt", "-n", "-o", "TARGET", partition)
	output, err := f.execCommand(cmd)
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
func (f *Filesystem) GetFilesystemType(ctx context.Context, partition string) (string, error) {
	cmd := exec.CommandContext(ctx, "blkid", "-o", "value", "-s", "TYPE", partition)
	output, err := f.execCommand(cmd)
	if err != nil {
		return "", fmt.Errorf("failed to get filesystem type: %w", err)
	}

	return strings.TrimSpace(string(output)), nil
}

// CopyDirectory recursively copies a directory to another location
func (f *Filesystem) CopyDirectory(ctx context.Context, src, dst string) error {
	// Create dst directory if it doesn't exist
	if err := os.MkdirAll(dst, 0755); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	// Use rsync for efficient directory copying
	cmd := exec.CommandContext(ctx, "rsync", "-av", src+"/", dst+"/")
	if output, err := f.execCommand(cmd); err != nil {
		return fmt.Errorf("rsync failed: %s: %w", string(output), err)
	}

	return nil
}

// WriteFile writes content to a file
func (fs *Filesystem) WriteFile(mountDir, path string, content []byte, perm fs.FileMode) error {
	fullPath := filepath.Join(mountDir, path)

	// Ensure parent directory exists
	parentDir := filepath.Dir(fullPath)
	if err := os.MkdirAll(parentDir, 0755); err != nil {
		return fmt.Errorf("failed to create parent directory: %w", err)
	}

	// Write the file
	if err := os.WriteFile(fullPath, content, perm); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

// CopyFile copies a file from sourcePath to destPath
func (fs *Filesystem) CopyFile(mountDir, sourcePath, destPath string) error {
	// Open source file
	src, err := os.Open(sourcePath)
	if err != nil {
		return fmt.Errorf("failed to open source file: %w", err)
	}
	defer src.Close()

	// Get source file info for permissions
	srcInfo, err := src.Stat()
	if err != nil {
		return fmt.Errorf("failed to get source file info: %w", err)
	}

	// Create destination path
	fullDestPath := filepath.Join(mountDir, destPath)

	// Ensure parent directory exists
	parentDir := filepath.Dir(fullDestPath)
	if err := os.MkdirAll(parentDir, 0755); err != nil {
		return fmt.Errorf("failed to create parent directory: %w", err)
	}

	// Create destination file
	dst, err := os.OpenFile(fullDestPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, srcInfo.Mode())
	if err != nil {
		return fmt.Errorf("failed to create destination file: %w", err)
	}
	defer dst.Close()

	// Copy the contents
	if _, err := io.Copy(dst, src); err != nil {
		return fmt.Errorf("failed to copy file contents: %w", err)
	}

	return nil
}

// MakeDirectory creates a directory at the specified path
func (fs *Filesystem) MakeDirectory(mountDir, path string, perm fs.FileMode) error {
	fullPath := filepath.Join(mountDir, path)
	if err := os.MkdirAll(fullPath, perm); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}
	return nil
}

// ChangePermissions changes the permissions of a file or directory
func (fs *Filesystem) ChangePermissions(mountDir, path string, perm fs.FileMode) error {
	fullPath := filepath.Join(mountDir, path)
	if err := os.Chmod(fullPath, perm); err != nil {
		return fmt.Errorf("failed to change permissions: %w", err)
	}
	return nil
}

// ReadFile reads a file from the mounted filesystem
func (fs *Filesystem) ReadFile(mountDir, relativePath string) ([]byte, error) {
	fullPath := filepath.Join(mountDir, relativePath)
	return os.ReadFile(fullPath)
}

// FileExists checks if a file exists in the mounted filesystem
func (fs *Filesystem) FileExists(mountDir, relativePath string) bool {
	fullPath := filepath.Join(mountDir, relativePath)
	_, err := os.Stat(fullPath)
	return err == nil
}

// IsDirectory checks if a path is a directory
func (fs *Filesystem) IsDirectory(mountDir, relativePath string) bool {
	fullPath := filepath.Join(mountDir, relativePath)
	info, err := os.Stat(fullPath)
	if err != nil {
		return false
	}
	return info.IsDir()
}
