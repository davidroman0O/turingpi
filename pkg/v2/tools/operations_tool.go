package tools

import (
	"context"
	"fmt"
	"io/fs"
	"os"

	"github.com/davidroman0O/turingpi/pkg/v2/container"
	"github.com/davidroman0O/turingpi/pkg/v2/operations"
)

// OperationsToolImpl is the implementation of the OperationsTool interface
// using the operations package
type OperationsToolImpl struct {
	imageOps       *operations.ImageOperations
	filesystemOps  *operations.FilesystemOperations
	networkOps     *operations.NetworkOperations
	compressionOps *operations.CompressionOperations
	executor       operations.CommandExecutor
}

// NewOperationsTool creates a new OperationsTool
func NewOperationsTool(containerTool ContainerTool) (OperationsTool, error) {
	ctx := context.Background()
	var containerInstance container.Container
	var err error

	// First try to get an existing container
	containerInstance, err = containerTool.GetContainer(ctx, "turingpi-operations")

	// If container doesn't exist, create it
	if err != nil {
		// Get current working directory
		pwd, err := os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("failed to get current working directory: %w", err)
		}

		// Ensure the directory exists up the hierarchy to prevent mounting issues
		os.MkdirAll(pwd, 0755)

		// Create a new container for operations with proper volume mounts
		containerConfig := container.ContainerConfig{
			Name:       "turingpi-operations",
			Image:      "ubuntu:latest",                     // Use a basic Linux image
			Command:    []string{"tail", "-f", "/dev/null"}, // Keep container running indefinitely
			Privileged: true,                                // Enable privileged mode for full filesystem access
			Capabilities: []string{
				"SYS_ADMIN", // Required for mount operations
				"NET_ADMIN", // Additional permissions that might be needed
				"MKNOD",     // Required for device operations
			},
			// Mount volumes with correct binding modes - read/write access
			Mounts: map[string]string{
				pwd: pwd, // Mount current working directory as read-write
			},
			// Set working directory to match host
			WorkDir: pwd,
		}

		// Create the container
		containerInstance, err = containerTool.CreateContainer(ctx, containerConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to create operations container: %w", err)
		}

		// Start the container
		if err := containerTool.StartContainer(ctx, containerInstance.ID()); err != nil {
			// Clean up if start fails
			containerTool.RemoveContainer(ctx, containerInstance.ID())
			return nil, fmt.Errorf("failed to start operations container: %w", err)
		}

		// Verify container is working by checking if the volume mount works
		// This ensures that if Docker can't properly mount volumes, we fail fast
		output, err := containerTool.RunCommand(ctx, containerInstance.ID(), []string{"ls", "-la", pwd})
		if err != nil {
			// Cleanup on failure
			containerTool.RemoveContainer(ctx, containerInstance.ID())
			return nil, fmt.Errorf("container volume mount verification failed: %w", err)
		}
		if output == "" {
			// Cleanup on failure
			containerTool.RemoveContainer(ctx, containerInstance.ID())
			return nil, fmt.Errorf("container mount appears empty, volume mounting may have failed")
		}
	}

	// Create the appropriate executor (automatically handles platform differences)
	executor := operations.NewExecutor(containerInstance)

	return &OperationsToolImpl{
		imageOps:       operations.NewImageOperations(executor),
		filesystemOps:  operations.NewFilesystemOperations(executor),
		networkOps:     operations.NewNetworkOperations(executor),
		compressionOps: operations.NewCompressionOperations(executor),
		executor:       executor,
	}, nil
}

// MapPartitions maps partitions in a disk image
func (t *OperationsToolImpl) MapPartitions(ctx context.Context, imgPath string) (string, error) {
	return t.filesystemOps.MapPartitions(ctx, imgPath)
}

// UnmapPartitions unmaps partitions in a disk image
func (t *OperationsToolImpl) UnmapPartitions(ctx context.Context, imgPath string) error {
	return t.filesystemOps.UnmapPartitions(ctx, imgPath)
}

// MountFilesystem mounts a filesystem
func (t *OperationsToolImpl) MountFilesystem(ctx context.Context, device, mountDir string) error {
	return t.filesystemOps.Mount(ctx, device, mountDir, "", nil)
}

// UnmountFilesystem unmounts a filesystem
func (t *OperationsToolImpl) UnmountFilesystem(ctx context.Context, mountDir string) error {
	return t.filesystemOps.Unmount(ctx, mountDir)
}

// DecompressImageXZ decompresses an XZ-compressed disk image
func (t *OperationsToolImpl) DecompressImageXZ(ctx context.Context, sourceXZ, targetDir string) (string, error) {
	return t.compressionOps.DecompressXZ(ctx, sourceXZ, targetDir)
}

// CompressImageXZ compresses a disk image with XZ
func (t *OperationsToolImpl) CompressImageXZ(ctx context.Context, sourceImg, targetXZ string) error {
	return t.compressionOps.CompressXZ(ctx, sourceImg, targetXZ)
}

// WriteFile writes content to a file in the mounted image
func (t *OperationsToolImpl) WriteFile(ctx context.Context, mountDir, relativePath string, content []byte, perm fs.FileMode) error {
	return t.filesystemOps.WriteFile(mountDir, relativePath, content, perm)
}

// CopyFile copies a file to the mounted image
func (t *OperationsToolImpl) CopyFile(ctx context.Context, mountDir, sourcePath, destPath string) error {
	return t.filesystemOps.CopyFile(ctx, mountDir, sourcePath, destPath)
}

// ReadFile reads a file from the mounted image
func (t *OperationsToolImpl) ReadFile(ctx context.Context, mountDir, relativePath string) ([]byte, error) {
	return t.filesystemOps.ReadFile(mountDir, relativePath)
}

// CopyToDevice copies an image to a device
func (t *OperationsToolImpl) CopyToDevice(ctx context.Context, imagePath, device string) error {
	return t.imageOps.CopyToDevice(ctx, imagePath, device)
}

// ResizePartition resizes the last partition of a device to fill available space
func (t *OperationsToolImpl) ResizePartition(ctx context.Context, device string) error {
	return t.imageOps.ResizePartition(ctx, device)
}

// ValidateImage validates that an image file exists and is a valid disk image
func (t *OperationsToolImpl) ValidateImage(ctx context.Context, imagePath string) error {
	return t.imageOps.ValidateImage(ctx, imagePath)
}

// ExtractBootFiles extracts kernel and initrd files from a mounted boot partition
func (t *OperationsToolImpl) ExtractBootFiles(ctx context.Context, bootMountPoint, outputDir string) (string, string, error) {
	return t.imageOps.ExtractBootFiles(ctx, bootMountPoint, outputDir)
}

// ApplyDTBOverlay applies a device tree overlay to a mounted boot partition
func (t *OperationsToolImpl) ApplyDTBOverlay(ctx context.Context, bootMountPoint, dtbOverlayPath string) error {
	return t.imageOps.ApplyDTBOverlay(ctx, bootMountPoint, dtbOverlayPath)
}

// ApplyNetworkConfig applies network configuration to a mounted system
func (t *OperationsToolImpl) ApplyNetworkConfig(ctx context.Context, mountDir, hostname, ipCIDR, gateway string, dnsServers []string) error {
	return t.networkOps.ApplyNetworkConfig(ctx, mountDir, hostname, ipCIDR, gateway, dnsServers)
}

// DecompressTarGZ decompresses a tar.gz archive to a directory
func (t *OperationsToolImpl) DecompressTarGZ(ctx context.Context, sourceTarGZ, outputDir string) error {
	return t.compressionOps.DecompressTarGZ(ctx, sourceTarGZ, outputDir)
}

// CompressTarGZ compresses a directory to a tar.gz archive
func (t *OperationsToolImpl) CompressTarGZ(ctx context.Context, sourceDir, outputTarGZ string) error {
	return t.compressionOps.CompressTarGZ(ctx, sourceDir, outputTarGZ)
}

// DecompressGZ decompresses a GZ-compressed file
func (t *OperationsToolImpl) DecompressGZ(ctx context.Context, sourceGZ, outputDir string) (string, error) {
	return t.compressionOps.DecompressGZ(ctx, sourceGZ, outputDir)
}

// CompressGZ compresses a file using GZ compression
func (t *OperationsToolImpl) CompressGZ(ctx context.Context, sourcePath, outputGZ string) error {
	return t.compressionOps.CompressGZ(ctx, sourcePath, outputGZ)
}
