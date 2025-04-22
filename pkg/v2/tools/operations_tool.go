package tools

import (
	"context"
	"io/fs"

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
	// Adapt the ContainerTool to container.Registry
	registryAdapter := NewContainerToolAdapter(containerTool)

	// Create a container executor that creates temporary containers on demand
	// instead of a persistent container
	executor := operations.NewTemporaryContainerExecutor(registryAdapter)

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

// IsPartitionMounted checks if a partition is mounted
func (t *OperationsToolImpl) IsPartitionMounted(ctx context.Context, partition string) (bool, string, error) {
	return t.filesystemOps.IsPartitionMounted(ctx, partition)
}

// GetFilesystemType gets the filesystem type of a partition
func (t *OperationsToolImpl) GetFilesystemType(ctx context.Context, partition string) (string, error) {
	return t.filesystemOps.GetFilesystemType(ctx, partition)
}

// Mount mounts a filesystem to a specified directory with options
func (t *OperationsToolImpl) Mount(ctx context.Context, device, mountPoint, fsType string, options []string) error {
	return t.filesystemOps.Mount(ctx, device, mountPoint, fsType, options)
}

// Unmount unmounts a filesystem
func (t *OperationsToolImpl) Unmount(ctx context.Context, mountPoint string) error {
	return t.filesystemOps.Unmount(ctx, mountPoint)
}

// Format formats a partition with a specified filesystem
func (t *OperationsToolImpl) Format(ctx context.Context, device, fsType, label string) error {
	return t.filesystemOps.Format(ctx, device, fsType, label)
}

// ResizeFilesystem resizes a filesystem to fill its partition
func (t *OperationsToolImpl) ResizeFilesystem(ctx context.Context, device string) error {
	return t.filesystemOps.ResizeFilesystem(ctx, device)
}

// CopyDirectory recursively copies a directory to another location
func (t *OperationsToolImpl) CopyDirectory(ctx context.Context, src, dst string) error {
	return t.filesystemOps.CopyDirectory(ctx, src, dst)
}

// FileExists checks if a file exists
func (t *OperationsToolImpl) FileExists(ctx context.Context, path, relativePath string) (bool, error) {
	// Just delegate to the filesystem operations
	// FilesystemOperations.FileExists doesn't return an error, it returns a bool
	exists := t.filesystemOps.FileExists(path, relativePath)
	return exists, nil
}

// IsDirectory checks if a path is a directory
func (t *OperationsToolImpl) IsDirectory(ctx context.Context, path, relativePath string) (bool, error) {
	return t.filesystemOps.IsDirectory(path, relativePath), nil
}

// MakeDirectory creates a directory with specified permissions
func (t *OperationsToolImpl) MakeDirectory(ctx context.Context, mountDir, path string, perm fs.FileMode) error {
	return t.filesystemOps.MakeDirectory(mountDir, path, perm)
}

// ChangePermissions changes the permissions of a file or directory
func (t *OperationsToolImpl) ChangePermissions(ctx context.Context, mountDir, path string, perm fs.FileMode) error {
	return t.filesystemOps.ChangePermissions(mountDir, path, perm)
}
