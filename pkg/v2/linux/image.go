package linux

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Image provides operations for handling Linux image operations
type Image struct {
	fs *Filesystem
}

// NewImage creates a new Image operations instance
func NewImage(fs *Filesystem) *Image {
	return &Image{
		fs: fs,
	}
}

// CopyToDevice copies an image to a device
func (i *Image) CopyToDevice(ctx context.Context, imagePath, device string) error {
	if _, err := os.Stat(imagePath); os.IsNotExist(err) {
		return fmt.Errorf("image file does not exist: %s", imagePath)
	}

	// Use dd to copy the image to the device
	cmd := exec.CommandContext(ctx, "dd", "if="+imagePath, "of="+device, "bs=4M", "status=progress")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to copy image to device: %w", err)
	}

	// Sync to ensure all data is written
	syncCmd := exec.CommandContext(ctx, "sync")
	if err := syncCmd.Run(); err != nil {
		return fmt.Errorf("sync failed after copying image: %w", err)
	}

	return nil
}

// ResizePartition resizes the last partition of an image to fill available space
func (i *Image) ResizePartition(ctx context.Context, device string) error {
	// Get the number of the last partition on the device
	partitions, err := i.fs.ListPartitions(ctx, device)
	if err != nil {
		return fmt.Errorf("failed to list partitions: %w", err)
	}

	if len(partitions) == 0 {
		return fmt.Errorf("no partitions found on device %s", device)
	}

	// Get the last partition
	lastPartNum := len(partitions)
	lastPart := fmt.Sprintf("%s%d", device, lastPartNum)
	if strings.Contains(device, "loop") || strings.Contains(device, "nvme") || strings.Contains(device, "mmcblk") {
		lastPart = fmt.Sprintf("%sp%d", device, lastPartNum)
	}

	// Use growpart to expand the partition to fill available space
	cmd := exec.CommandContext(ctx, "growpart", device, fmt.Sprintf("%d", lastPartNum))
	output, err := cmd.CombinedOutput()
	if err != nil {
		// If the error is because the partition is already at maximum size, this is not a failure
		if strings.Contains(string(output), "NOCHANGE") {
			return nil
		}
		return fmt.Errorf("failed to resize partition: %s: %w", string(output), err)
	}

	// Resize the filesystem on the partition
	return i.fs.ResizeFilesystem(ctx, lastPart)
}

// ValidateImage validates that an image file exists and is a valid disk image
func (i *Image) ValidateImage(ctx context.Context, imagePath string) error {
	// Check if file exists
	_, err := os.Stat(imagePath)
	if os.IsNotExist(err) {
		return fmt.Errorf("image file does not exist: %s", imagePath)
	}

	// Check if it's a valid image by attempting to get file info with fdisk
	cmd := exec.CommandContext(ctx, "fdisk", "-l", imagePath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("not a valid disk image: %s: %w", string(output), err)
	}

	return nil
}

// ExtractBootFiles extracts kernel and initrd files from a mounted boot partition
func (i *Image) ExtractBootFiles(ctx context.Context, bootMountPoint, outputDir string) (kernel, initrd string, err error) {
	// Create output directory if it doesn't exist
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return "", "", fmt.Errorf("failed to create output directory: %w", err)
	}

	// Find kernel and initrd files in boot directory
	var kernelPath, initrdPath string

	// Common kernel file patterns
	kernelPatterns := []string{
		filepath.Join(bootMountPoint, "vmlinuz*"),
		filepath.Join(bootMountPoint, "kernel*"),
		filepath.Join(bootMountPoint, "boot/vmlinuz*"),
	}

	// Common initrd file patterns
	initrdPatterns := []string{
		filepath.Join(bootMountPoint, "initrd*"),
		filepath.Join(bootMountPoint, "initramfs*"),
		filepath.Join(bootMountPoint, "boot/initrd*"),
		filepath.Join(bootMountPoint, "boot/initramfs*"),
	}

	// Find kernel file
	for _, pattern := range kernelPatterns {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			continue
		}
		if len(matches) > 0 {
			// Use the most recent kernel if multiple exist (assuming alphabetical sort works)
			kernelPath = matches[len(matches)-1]
			break
		}
	}

	// Find initrd file
	for _, pattern := range initrdPatterns {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			continue
		}
		if len(matches) > 0 {
			// Use the most recent initrd if multiple exist
			initrdPath = matches[len(matches)-1]
			break
		}
	}

	if kernelPath == "" {
		return "", "", fmt.Errorf("kernel file not found in boot partition")
	}

	if initrdPath == "" {
		return "", "", fmt.Errorf("initrd file not found in boot partition")
	}

	// Copy files to output directory
	kernelOut := filepath.Join(outputDir, filepath.Base(kernelPath))
	initrdOut := filepath.Join(outputDir, filepath.Base(initrdPath))

	// Copy kernel
	kernelData, err := os.ReadFile(kernelPath)
	if err != nil {
		return "", "", fmt.Errorf("failed to read kernel file: %w", err)
	}
	if err := os.WriteFile(kernelOut, kernelData, 0644); err != nil {
		return "", "", fmt.Errorf("failed to write kernel file: %w", err)
	}

	// Copy initrd
	initrdData, err := os.ReadFile(initrdPath)
	if err != nil {
		return "", "", fmt.Errorf("failed to read initrd file: %w", err)
	}
	if err := os.WriteFile(initrdOut, initrdData, 0644); err != nil {
		return "", "", fmt.Errorf("failed to write initrd file: %w", err)
	}

	return kernelOut, initrdOut, nil
}

// ApplyDTBOverlay applies a device tree overlay to a mounted boot partition
func (i *Image) ApplyDTBOverlay(ctx context.Context, bootMountPoint, dtbOverlayPath string) error {
	// Check if dtb overlay file exists
	if _, err := os.Stat(dtbOverlayPath); os.IsNotExist(err) {
		return fmt.Errorf("dtb overlay file does not exist: %s", dtbOverlayPath)
	}

	// Find the overlays directory in the boot partition
	overlaysDir := filepath.Join(bootMountPoint, "overlays")
	if _, err := os.Stat(overlaysDir); os.IsNotExist(err) {
		// Try alternative directory
		overlaysDir = filepath.Join(bootMountPoint, "dtbs/overlays")
		if _, err := os.Stat(overlaysDir); os.IsNotExist(err) {
			return fmt.Errorf("overlays directory not found in boot partition")
		}
	}

	// Copy the overlay file to the overlays directory
	overlayDest := filepath.Join(overlaysDir, filepath.Base(dtbOverlayPath))

	overlayData, err := os.ReadFile(dtbOverlayPath)
	if err != nil {
		return fmt.Errorf("failed to read dtb overlay file: %w", err)
	}

	if err := os.WriteFile(overlayDest, overlayData, 0644); err != nil {
		return fmt.Errorf("failed to write dtb overlay file: %w", err)
	}

	// Modify config.txt to load the overlay if it exists
	configPath := filepath.Join(bootMountPoint, "config.txt")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return nil // No config.txt file, can't enable overlay
	}

	// Read existing config
	configData, err := os.ReadFile(configPath)
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

	// Write back config
	if err := os.WriteFile(configPath, []byte(configStr), 0644); err != nil {
		return fmt.Errorf("failed to write config.txt: %w", err)
	}

	return nil
}
