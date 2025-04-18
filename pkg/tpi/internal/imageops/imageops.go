package imageops

/*
This file is being deprecated in favor of the adapter pattern implementation.
It's kept for backward compatibility until all calling code is updated.
New code should use the adapter interface directly.
*/

import (
	"io/fs"

	"github.com/davidroman0O/turingpi/pkg/tpi/docker"
	"github.com/davidroman0O/turingpi/pkg/tpi/platform"
)

// DockerConfig holds the Docker execution configuration used for platform-independent image operations
// Deprecated: Use the adapter interface instead
var DockerConfig *platform.DockerExecutionConfig

// DockerContainerID stores the ID of the persistent container we create for operations
// Deprecated: Use the adapter interface instead
var DockerContainerID string

// dockerAdapter for performing operations - will be initialized for non-Linux platforms
// Deprecated: Use the adapter interface instead
var dockerAdapter *docker.DockerAdapter

// InitDockerConfig initializes the Docker configuration for cross-platform operations
// Deprecated: Use NewImageOpsAdapter instead
func InitDockerConfig(sourceDir, tempDir, outputDir string) error {
	return InitDockerConfigCompat(sourceDir, tempDir, outputDir)
}

// PrepareImage decompresses a disk image, modifies it with network settings, and recompresses it
// Deprecated: Use the adapter interface instead
func PrepareImage(opts PrepareImageOptions) (string, error) {
	return PrepareImageCompat(opts)
}

// DecompressImageXZ decompresses an XZ-compressed disk image
// Deprecated: Use the adapter interface instead
func DecompressImageXZ(sourceImgXZAbs, tmpDir string) (string, error) {
	return DecompressImageXZCompat(sourceImgXZAbs, tmpDir)
}

// MapPartitions uses kpartx to map disk partitions
// Deprecated: Use the adapter interface instead
func MapPartitions(imgPathAbs string) (string, error) {
	return MapPartitionsCompat(imgPathAbs)
}

// CleanupPartitions unmaps partitions
// Deprecated: Use the adapter interface instead
func CleanupPartitions(imgPathAbs string) error {
	return CleanupPartitionsCompat(imgPathAbs)
}

// MountFilesystem mounts a filesystem
// Deprecated: Use the adapter interface instead
func MountFilesystem(partitionDevice, mountDir string) error {
	return MountFilesystemCompat(partitionDevice, mountDir)
}

// UnmountFilesystem unmounts a filesystem
// Deprecated: Use the adapter interface instead
func UnmountFilesystem(mountDir string) error {
	return UnmountFilesystemCompat(mountDir)
}

// WriteToImageFile writes content to a file within the mounted image
// Deprecated: Use the adapter interface instead
func WriteToImageFile(mountDir, relativePath string, content []byte, perm fs.FileMode) error {
	return WriteToImageFileCompat(mountDir, relativePath, content, perm)
}

// CopyFileToImage copies a local file into the mounted image
// Deprecated: Use the adapter interface instead
func CopyFileToImage(mountDir, localSourcePath, relativeDestPath string) error {
	return CopyFileToImageCompat(mountDir, localSourcePath, relativeDestPath)
}

// MkdirInImage creates a directory within the mounted image
// Deprecated: Use the adapter interface instead
func MkdirInImage(mountDir, relativePath string, perm fs.FileMode) error {
	return MkdirInImageCompat(mountDir, relativePath, perm)
}

// ChmodInImage changes permissions of a file/directory within the mounted image
// Deprecated: Use the adapter interface instead
func ChmodInImage(mountDir, relativePath string, perm fs.FileMode) error {
	return ChmodInImageCompat(mountDir, relativePath, perm)
}

// ApplyNetworkConfig applies network config to the mounted filesystem
// Deprecated: Use the adapter interface instead
func ApplyNetworkConfig(mountDir string, hostname string, ipCIDR string, gateway string, dnsServers []string) error {
	return ApplyNetworkConfigCompat(mountDir, hostname, ipCIDR, gateway, dnsServers)
}

// RecompressImageXZ compresses a disk image with XZ
// Deprecated: Use the adapter interface instead
func RecompressImageXZ(modifiedImgPath, finalXZPath string) error {
	return RecompressImageXZCompat(modifiedImgPath, finalXZPath)
}

// ExecuteFileOperations executes a batch of file operations
// Deprecated: Use the adapter interface instead
func ExecuteFileOperations(params ExecuteFileOperationsParams) error {
	return ExecuteFileOperationsCompat(params)
}

// PrepareImageSimple is a simplified version that follows the prep.go approach
// Deprecated: Use the adapter interface instead
func PrepareImageSimple(opts PrepareImageOptions) (string, error) {
	return PrepareImageSimpleCompat(opts)
}

// DockerAdapter returns the current Docker adapter instance
// Deprecated: Use the adapter interface instead
func DockerAdapter() *docker.DockerAdapter {
	return dockerAdapter
}
