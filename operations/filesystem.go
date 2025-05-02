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
	// Ensure the image file exists
	if _, err := ExecuteCommand(f.executor, ctx, "test", "-f", imgPathAbs); err != nil {
		return "", NewOperationError("image validation", imgPathAbs, err)
	}

	// Execute kpartx to map partitions
	output, err := ExecuteCommand(f.executor, ctx, "kpartx", "-av", imgPathAbs)
	if err != nil {
		// Check if kpartx is installed
		_, checkErr := ExecuteCommand(f.executor, ctx, "which", "kpartx")
		if checkErr != nil {
			return "", fmt.Errorf("kpartx command not found. Please install kpartx: %v", checkErr)
		}

		// If kpartx is installed but failed, provide more context
		return "", NewOperationError("partition mapping", imgPathAbs, err)
	}

	// Parse kpartx output to get the root partition device
	rootDevice, err := f.parseKpartxOutput(string(output))
	if err != nil {
		return "", NewOperationError("parsing kpartx output", string(output), err)
	}

	rootDevPath := fmt.Sprintf("/dev/mapper/%s", rootDevice)

	// Wait for the device to become available
	if err := f.waitForDevice(ctx, rootDevPath, 10); err != nil {
		// Try to get more info about the device
		deviceListOutput, _ := ExecuteCommand(f.executor, ctx, "ls", "-la", "/dev/mapper")
		return "", fmt.Errorf("device not available after mapping: %w (ls -la /dev/mapper: %s)",
			err, string(deviceListOutput))
	}

	return rootDevPath, nil
}

// parseKpartxOutput parses kpartx output to extract root partition device path
func (f *FilesystemOperations) parseKpartxOutput(output string) (string, error) {
	// Example output:
	// add map loop1p1 (253:1): 0 524288 linear 7:1 8192
	// add map loop1p2 (253:2): 0 32768000 linear 7:1 532480
	//
	// For simplicity, we assume the first line is boot and second is root
	// This could be improved to detect partitions by examining sizes or file systems

	lines := strings.Split(output, "\n")

	// Filter out empty lines and log what we found
	var validLines []string
	for _, line := range lines {
		if strings.TrimSpace(line) != "" && strings.HasPrefix(line, "add") {
			validLines = append(validLines, line)
		}
	}

	if len(validLines) == 0 {
		return "", fmt.Errorf("no valid partition maps found in kpartx output")
	}

	// For single partition images, use the first line
	// For multi-partition images, assume second partition is root (common practice)
	var rootLine string
	if len(validLines) == 1 {
		rootLine = validLines[0]
	} else {
		rootLine = validLines[1] // Second partition is typically root
	}

	parts := strings.Fields(rootLine)
	if len(parts) < 3 || !strings.HasPrefix(parts[0], "add") {
		return "", fmt.Errorf("unexpected kpartx output format: %s", rootLine)
	}

	// Extract the device name (3rd field)
	return parts[2], nil
}

// waitForDevice waits for a device to become available, with a specified timeout in seconds
func (f *FilesystemOperations) waitForDevice(ctx context.Context, devicePath string, timeoutSeconds int) error {
	// Try for the specified number of seconds
	for i := 0; i < timeoutSeconds; i++ {
		// Check if device exists
		_, err := f.executor.Execute(ctx, "test", "-e", devicePath)
		if err == nil {
			// Device exists, make an additional check for block device
			_, err = f.executor.Execute(ctx, "test", "-b", devicePath)
			if err == nil {
				return nil // Device is available and is a block device
			}
		}

		// Wait 1 second before trying again
		time.Sleep(1 * time.Second)
	}

	return fmt.Errorf("timeout waiting for device to become available: %s", devicePath)
}

// UnmapPartitions unmaps partitions that were mapped with kpartx
func (f *FilesystemOperations) UnmapPartitions(ctx context.Context, imgPathAbs string) error {
	// Ensure the image file exists
	if _, err := ExecuteCommand(f.executor, ctx, "test", "-f", imgPathAbs); err != nil {
		return NewOperationError("image validation", imgPathAbs, err)
	}

	// Execute kpartx with -d flag to unmap partitions
	fmt.Printf("Unmapping partitions for image: %s\n", imgPathAbs)

	// First, get a list of mapped devices for this image to verify cleanup
	// We use losetup to find which loop device is associated with our image
	loopDevices := []string{}
	losetupOutput, err := ExecuteCommand(f.executor, ctx, "losetup", "-j", imgPathAbs)
	if err == nil && len(losetupOutput) > 0 {
		// Parse out the loop device name from output like "/dev/loop0: []: (/path/to/image)"
		loopLines := strings.Split(string(losetupOutput), "\n")
		for _, line := range loopLines {
			if strings.TrimSpace(line) == "" {
				continue
			}

			loopParts := strings.Split(line, ":")
			if len(loopParts) > 0 {
				loopDev := strings.TrimSpace(loopParts[0])
				fmt.Printf("Found mapped loop device: %s\n", loopDev)
				loopDevices = append(loopDevices, loopDev)
			}
		}
	}

	// Now run kpartx -d
	output, err := ExecuteCommand(f.executor, ctx, "kpartx", "-d", imgPathAbs)
	if err != nil {
		// Check if kpartx is installed
		_, checkErr := ExecuteCommand(f.executor, ctx, "which", "kpartx")
		if checkErr != nil {
			return fmt.Errorf("kpartx command not found. Please install kpartx: %v", checkErr)
		}

		return NewOperationError("partition unmapping", imgPathAbs, err)
	}

	// If there was output, log it - it might contain important info
	if len(output) > 0 {
		fmt.Printf("Unmap output: %s\n", string(output))
	}

	// Verify that the image is no longer mapped to any loop devices
	verifyOutput, err := ExecuteCommand(f.executor, ctx, "losetup", "-j", imgPathAbs)
	if err == nil && len(verifyOutput) > 0 && strings.TrimSpace(string(verifyOutput)) != "" {
		// Still mapped, try a more aggressive approach
		fmt.Printf("Image still has loop mappings, attempting forceful cleanup\n")

		// Try the -dv (delete with verbose) option for more forceful unmapping
		forceOutput, forceErr := ExecuteCommand(f.executor, ctx, "kpartx", "-dv", imgPathAbs)
		if forceErr != nil {
			fmt.Printf("Warning: forceful unmap attempt failed: %v\nOutput: %s\n",
				forceErr, string(forceOutput))
		}

		// If we found specific loop devices earlier, try to detach them directly
		for _, loopDev := range loopDevices {
			fmt.Printf("Attempting to detach loop device: %s\n", loopDev)
			detachOutput, detachErr := ExecuteCommand(f.executor, ctx, "losetup", "-d", loopDev)
			if detachErr != nil {
				fmt.Printf("Warning: Failed to detach loop device %s: %v\nOutput: %s\n",
					loopDev, detachErr, string(detachOutput))
			} else {
				fmt.Printf("Successfully detached loop device: %s\n", loopDev)
			}
		}

		// As a last resort, try to force ALL loop devices to rescan
		fmt.Printf("Requesting all loop devices to rescan...\n")
		_, _ = ExecuteCommand(f.executor, ctx, "losetup", "-D")

		// Final verification
		finalVerifyOutput, _ := ExecuteCommand(f.executor, ctx, "losetup", "-j", imgPathAbs)
		if len(finalVerifyOutput) > 0 && strings.TrimSpace(string(finalVerifyOutput)) != "" {
			fmt.Printf("Warning: Image still has loop mappings after forceful cleanup: %s\n",
				string(finalVerifyOutput))
		}
	}

	fmt.Printf("Partition unmapping completed for: %s\n", imgPathAbs)
	return nil
}

// Mount mounts a filesystem to a specified directory
func (f *FilesystemOperations) Mount(ctx context.Context, device, mountPoint, fsType string, options []string) error {
	// Create mount point directory
	if _, err := ExecuteCommand(f.executor, ctx, "mkdir", "-p", mountPoint); err != nil {
		return NewOperationError("creating mount point", mountPoint, err)
	}

	// Check if the device exists
	if _, err := ExecuteCommand(f.executor, ctx, "test", "-e", device); err != nil {
		// Try to get more information about the device
		lsOutput, _ := ExecuteCommand(f.executor, ctx, "ls", "-la", filepath.Dir(device))
		return fmt.Errorf("device does not exist: %s\nDirectory contents: %s",
			device, string(lsOutput))
	}

	// Log what we're attempting to do
	fmt.Printf("Mounting %s to %s", device, mountPoint)
	if fsType != "" {
		fmt.Printf(" with filesystem type %s", fsType)
	}
	if len(options) > 0 {
		fmt.Printf(" and options %s", strings.Join(options, ","))
	}
	fmt.Println()

	// Check if already mounted
	isMounted, existingMountPoint, err := f.IsPartitionMounted(ctx, device)
	if err == nil && isMounted {
		// If already mounted to our desired location, we're done
		if existingMountPoint == mountPoint {
			fmt.Printf("Device %s is already mounted at %s\n", device, mountPoint)
			return nil
		}
		// Mounted somewhere else - this might be an issue
		fmt.Printf("Warning: device %s is already mounted at %s\n", device, existingMountPoint)
	}

	// Build mount command arguments
	args := []string{device, mountPoint}
	if fsType != "" {
		args = append(args, "-t", fsType)
	}
	if len(options) > 0 {
		args = append(args, "-o", strings.Join(options, ","))
	}

	// Execute mount command
	_, err = ExecuteCommand(f.executor, ctx, "mount", args...)
	if err != nil {
		// Get filesystem details to help diagnose the issue
		fsTypeOutput, _ := ExecuteCommand(f.executor, ctx, "blkid", device)

		errDetails := fmt.Sprintf("mount failed for device: %s\n", device)
		errDetails += fmt.Sprintf("Filesystem details: %s\n", string(fsTypeOutput))

		// Try dmesg for more kernel-level errors
		dmesgOutput, _ := ExecuteCommand(f.executor, ctx, "dmesg", "|", "tail", "-n", "10")
		if len(dmesgOutput) > 0 {
			errDetails += fmt.Sprintf("Recent kernel messages:\n%s\n", string(dmesgOutput))
		}

		return NewOperationError("mount", errDetails, err)
	}

	// Verify mount was successful
	_, verifyErr := ExecuteCommand(f.executor, ctx, "mountpoint", "-q", mountPoint)
	if verifyErr != nil {
		// Mount command succeeded but verification failed
		// This is strange, so include extra diagnostic information
		mountsOutput, _ := ExecuteCommand(f.executor, ctx, "cat", "/proc/mounts")
		return fmt.Errorf("mount command succeeded but verification failed: %w\nCurrent mounts: %s",
			verifyErr, string(mountsOutput))
	}

	// Report success
	fmt.Printf("Successfully mounted %s at %s\n", device, mountPoint)
	return nil
}

// Unmount unmounts a filesystem
func (f *FilesystemOperations) Unmount(ctx context.Context, mountPoint string) error {
	// Check if mountPoint is actually mounted
	mounted, err := f.isMountPoint(ctx, mountPoint)
	if err != nil {
		fmt.Printf("Warning: couldn't verify if %s is mounted: %v\n", mountPoint, err)
		// Continue anyway, let the unmount command handle it
	} else if !mounted {
		fmt.Printf("Mount point %s is not mounted, nothing to unmount\n", mountPoint)
		return nil // Already unmounted, no error
	}

	fmt.Printf("Unmounting filesystem at %s\n", mountPoint)

	// Try a normal unmount first
	output, err := f.executor.Execute(ctx, "umount", mountPoint)
	if err != nil {
		// If the regular unmount fails, try with -l (lazy) option
		fmt.Printf("Regular unmount failed, attempting lazy unmount: %v\n", err)
		lazyOutput, lazyErr := f.executor.Execute(ctx, "umount", "-l", mountPoint)
		if lazyErr != nil {
			return fmt.Errorf("unmount failed (both regular and lazy): %w, output: %s",
				err, string(output)+"\n"+string(lazyOutput))
		}
		fmt.Printf("Lazy unmount of %s succeeded\n", mountPoint)
		return nil
	}

	fmt.Printf("Successfully unmounted %s\n", mountPoint)
	return nil
}

// isMountPoint checks if a path is a mount point
func (f *FilesystemOperations) isMountPoint(ctx context.Context, path string) (bool, error) {
	output, err := f.executor.Execute(ctx, "mountpoint", "-q", path)
	if err != nil {
		// Exit code 1 means it's not a mount point
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return false, nil
		}
		// Otherwise there was some other error
		return false, fmt.Errorf("error checking mountpoint: %w, output: %s", err, string(output))
	}
	return true, nil // Successfully ran, so it is a mount point
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

// Remove deletes a file or directory at the specified path
func (f *FilesystemOperations) Remove(ctx context.Context, path string, recursive bool) error {
	// Check if path exists
	if _, err := f.executor.Execute(ctx, "test", "-e", path); err != nil {
		return fmt.Errorf("path does not exist: %s", path)
	}

	// Determine args based on whether we're removing recursively
	args := []string{"-f"}
	if recursive {
		args = append(args, "-r")
	}
	args = append(args, path)

	// Execute rm command
	output, err := f.executor.Execute(ctx, "rm", args...)
	if err != nil {
		return fmt.Errorf("remove operation failed: %w, output: %s", err, string(output))
	}

	return nil
}
