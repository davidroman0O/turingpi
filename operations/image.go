package operations

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
)

// ImageOperations provides operations for handling Linux disk images
// that can be executed either directly on a Linux host or inside a container on non-Linux systems
type ImageOperations struct {
	executor CommandExecutor
	fs       *FilesystemOperations
}

// NewImageOperations creates a new ImageOperations instance
func NewImageOperations(executor CommandExecutor) *ImageOperations {
	return &ImageOperations{
		executor: executor,
		fs:       NewFilesystemOperations(executor),
	}
}

// CopyToDevice copies an image to a device
func (i *ImageOperations) CopyToDevice(ctx context.Context, imagePath, device string) error {
	// Check if image file exists first
	_, err := i.executor.Execute(ctx, "test", "-f", imagePath)
	if err != nil {
		return fmt.Errorf("image file does not exist: %s", imagePath)
	}

	// Use dd to copy the image to the device
	output, err := i.executor.Execute(ctx, "dd", "if="+imagePath, "of="+device, "bs=4M", "status=progress")
	if err != nil {
		return fmt.Errorf("failed to copy image to device: %s: %w", string(output), err)
	}

	// Sync to ensure all data is written
	_, err = i.executor.Execute(ctx, "sync")
	if err != nil {
		return fmt.Errorf("sync failed after copying image: %w", err)
	}

	return nil
}

// ResizePartition resizes the last partition of an image to fill available space
func (i *ImageOperations) ResizePartition(ctx context.Context, device string) error {
	// Get device info
	output, err := ExecuteCommand(i.executor, ctx, "fdisk", "-l", device)
	if err != nil {
		return NewOperationError("getting device info", device, err)
	}

	// Find last partition number
	lines := strings.Split(string(output), "\n")
	var lastPartNum string
	var foundPartition bool

	for _, line := range lines {
		if strings.Contains(line, device) && !strings.Contains(line, "Disk") {
			// Line contains a partition entry
			fields := strings.Fields(line)
			if len(fields) > 0 {
				// Extract partition number from device name (e.g., /dev/sda1 -> 1)
				partName := fields[0]
				lastPartNum = partName[len(partName)-1:]
				foundPartition = true
			}
		}
	}

	if !foundPartition || lastPartNum == "" {
		// Include the fdisk output in the error to help diagnose the issue
		return fmt.Errorf("no partitions found on device %s\nfdisk output:\n%s",
			device, string(output))
	}

	// Use growpart to expand the partition to fill available space
	output, err = ExecuteCommand(i.executor, ctx, "growpart", device, lastPartNum)
	if err != nil {
		// If the error is because the partition is already at maximum size, this is not a failure
		if strings.Contains(string(output), "NOCHANGE") {
			return nil
		}

		// Try to check if growpart is installed
		_, checkErr := ExecuteCommand(i.executor, ctx, "which", "growpart")
		if checkErr != nil {
			return fmt.Errorf("growpart command not found. Please install growpart (usually part of cloud-utils package): %v", checkErr)
		}

		return NewOperationError(fmt.Sprintf("resizing partition %s on device", lastPartNum), device, err)
	}

	// Get the last partition device
	var lastPart string
	if strings.Contains(device, "loop") || strings.Contains(device, "nvme") || strings.Contains(device, "mmcblk") {
		lastPart = fmt.Sprintf("%sp%s", device, lastPartNum)
	} else {
		lastPart = fmt.Sprintf("%s%s", device, lastPartNum)
	}

	// Resize the filesystem on the partition
	resizeErr := i.fs.ResizeFilesystem(ctx, lastPart)
	if resizeErr != nil {
		// Get more info about the filesystem before reporting the error
		fsTypeOutput, _ := ExecuteCommand(i.executor, ctx, "blkid", lastPart)
		return fmt.Errorf("partition resized but filesystem resize failed: %w\nFilesystem info: %s",
			resizeErr, string(fsTypeOutput))
	}

	return nil
}

// ValidateImage validates that an image file exists and is a valid disk image
func (i *ImageOperations) ValidateImage(ctx context.Context, imagePath string) error {
	// Check if file exists
	_, err := i.executor.Execute(ctx, "test", "-f", imagePath)
	if err != nil {
		return fmt.Errorf("image file does not exist: %s", imagePath)
	}

	// Check if it's a valid image by attempting to get file info with fdisk
	output, err := i.executor.Execute(ctx, "fdisk", "-l", imagePath)
	if err != nil {
		return fmt.Errorf("not a valid disk image: %s: %w", string(output), err)
	}

	return nil
}

// ExtractBootFiles extracts kernel and initrd files from a mounted boot partition
func (i *ImageOperations) ExtractBootFiles(ctx context.Context, bootMountPoint, outputDir string) (kernel, initrd string, err error) {
	// Create output directory if it doesn't exist
	if _, err := i.executor.Execute(ctx, "mkdir", "-p", outputDir); err != nil {
		return "", "", fmt.Errorf("failed to create output directory: %w", err)
	}

	// Find kernel file
	kernelOutput, err := i.executor.Execute(ctx, "bash", "-c", fmt.Sprintf("find %s -name 'vmlinuz*' -o -name 'kernel*' | sort | tail -1", bootMountPoint))
	if err != nil || len(kernelOutput) == 0 {
		return "", "", fmt.Errorf("kernel file not found in boot partition")
	}
	kernelPath := strings.TrimSpace(string(kernelOutput))

	// Find initrd file
	initrdOutput, err := i.executor.Execute(ctx, "bash", "-c", fmt.Sprintf("find %s -name 'initrd*' -o -name 'initramfs*' | sort | tail -1", bootMountPoint))
	if err != nil || len(initrdOutput) == 0 {
		return "", "", fmt.Errorf("initrd file not found in boot partition")
	}
	initrdPath := strings.TrimSpace(string(initrdOutput))

	// Copy kernel
	kernelOut := filepath.Join(outputDir, filepath.Base(kernelPath))
	if _, err := i.executor.Execute(ctx, "cp", kernelPath, kernelOut); err != nil {
		return "", "", fmt.Errorf("failed to copy kernel file: %w", err)
	}

	// Copy initrd
	initrdOut := filepath.Join(outputDir, filepath.Base(initrdPath))
	if _, err := i.executor.Execute(ctx, "cp", initrdPath, initrdOut); err != nil {
		return "", "", fmt.Errorf("failed to copy initrd file: %w", err)
	}

	return kernelOut, initrdOut, nil
}

// ApplyDTBOverlay applies a device tree overlay to a mounted boot partition
func (i *ImageOperations) ApplyDTBOverlay(ctx context.Context, bootMountPoint, dtbOverlayPath string) error {
	// Check if dtb overlay file exists
	_, err := i.executor.Execute(ctx, "test", "-f", dtbOverlayPath)
	if err != nil {
		return fmt.Errorf("dtb overlay file does not exist: %s", dtbOverlayPath)
	}

	// Find the overlays directory
	overlaysDir := filepath.Join(bootMountPoint, "overlays")
	if _, err := i.executor.Execute(ctx, "test", "-d", overlaysDir); err != nil {
		// Try alternative directory
		overlaysDir = filepath.Join(bootMountPoint, "dtbs/overlays")
		if _, err := i.executor.Execute(ctx, "test", "-d", overlaysDir); err != nil {
			return fmt.Errorf("overlays directory not found in boot partition")
		}
	}

	// Copy the overlay file to the overlays directory
	overlayDest := filepath.Join(overlaysDir, filepath.Base(dtbOverlayPath))
	if _, err := i.executor.Execute(ctx, "cp", dtbOverlayPath, overlayDest); err != nil {
		return fmt.Errorf("failed to copy dtb overlay file: %w", err)
	}

	// Modify config.txt to load the overlay if it exists
	configPath := filepath.Join(bootMountPoint, "config.txt")
	if _, err := i.executor.Execute(ctx, "test", "-f", configPath); err != nil {
		return nil // No config.txt file, can't enable overlay
	}

	// Read existing config
	configData, err := i.executor.Execute(ctx, "cat", configPath)
	if err != nil {
		return fmt.Errorf("failed to read config.txt: %w", err)
	}

	configStr := string(configData)
	overlayName := strings.TrimSuffix(filepath.Base(dtbOverlayPath), ".dtbo")
	overlayLine := fmt.Sprintf("dtoverlay=%s", overlayName)

	// Check if overlay is already enabled
	if strings.Contains(configStr, overlayLine) {
		return nil // Already enabled
	}

	// Add overlay line
	configStr = configStr + "\n" + overlayLine + "\n"

	// Create a temporary file and then move it to the final location
	// This matches the approach expected by the test
	tempFile := "/tmp/config.txt.new"

	// Write new config to temp file
	escapedConfig := strings.ReplaceAll(configStr, "'", "'\\''")
	if _, err := i.executor.Execute(ctx, "bash", "-c", fmt.Sprintf("cat > %s << 'EOT'\n%s\nEOT", tempFile, escapedConfig)); err != nil {
		return fmt.Errorf("failed to create temporary config file: %w", err)
	}

	// Move temp file to destination
	if _, err := i.executor.Execute(ctx, "mv", tempFile, configPath); err != nil {
		return fmt.Errorf("failed to update config.txt: %w", err)
	}

	return nil
}
