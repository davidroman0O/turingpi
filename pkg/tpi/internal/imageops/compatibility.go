package imageops

import (
	"fmt"
	"io/fs"
)

// Global adapter instance for backward compatibility
var globalAdapter ImageOpsAdapter

// This file provides backward compatibility with existing code
// that may be using the old API. These functions delegate to the
// adapter methods.

// Compatibility functions that delegate to the adapter methods
// NOTE: These functions should eventually be deprecated and clients
// should be updated to use the adapter interface directly.

// InitDockerConfigCompat initializes the Docker configuration for backward compatibility
func InitDockerConfigCompat(sourceDir, tempDir, outputDir string) error {
	// Clean up existing adapter if any
	if globalAdapter != nil {
		globalAdapter.Cleanup()
		globalAdapter = nil
	}

	var err error
	globalAdapter, err = NewImageOpsAdapter(sourceDir, tempDir, outputDir)
	if err != nil {
		return fmt.Errorf("failed to initialize adapter: %w", err)
	}

	return nil
}

// PrepareImageCompat decompresses a disk image, modifies it with network settings, and recompresses it
func PrepareImageCompat(opts PrepareImageOptions) (string, error) {
	if globalAdapter == nil {
		return "", fmt.Errorf("adapter not initialized, call InitDockerConfig first")
	}
	return globalAdapter.PrepareImage(opts)
}

// DockerAdapterCompat returns the current Docker adapter instance
func DockerAdapterCompat() interface{} {
	// Return the internal adapter instance as an interface{}
	// so it can be type asserted by code that knows about the internal structure
	return globalAdapter
}

// PrepareImageSimpleCompat is a simplified version that follows the prep.go approach
func PrepareImageSimpleCompat(opts PrepareImageOptions) (string, error) {
	if globalAdapter == nil {
		return "", fmt.Errorf("adapter not initialized, call InitDockerConfig first")
	}
	return globalAdapter.PrepareImage(opts)
}

// DecompressImageXZCompat decompresses an XZ-compressed disk image.
func DecompressImageXZCompat(sourceImgXZAbs, tmpDir string) (string, error) {
	if globalAdapter == nil {
		return "", fmt.Errorf("adapter not initialized, call InitDockerConfig first")
	}
	return globalAdapter.DecompressImageXZ(sourceImgXZAbs, tmpDir)
}

// MapPartitionsCompat uses kpartx to map disk partitions
func MapPartitionsCompat(imgPathAbs string) (string, error) {
	if globalAdapter == nil {
		return "", fmt.Errorf("adapter not initialized, call InitDockerConfig first")
	}
	return globalAdapter.MapPartitions(imgPathAbs)
}

// CleanupPartitionsCompat unmaps partitions
func CleanupPartitionsCompat(imgPathAbs string) error {
	if globalAdapter == nil {
		return fmt.Errorf("adapter not initialized, call InitDockerConfig first")
	}
	return globalAdapter.CleanupPartitions(imgPathAbs)
}

// MountFilesystemCompat mounts a filesystem
func MountFilesystemCompat(partitionDevice, mountDir string) error {
	if globalAdapter == nil {
		return fmt.Errorf("adapter not initialized, call InitDockerConfig first")
	}
	return globalAdapter.MountFilesystem(partitionDevice, mountDir)
}

// UnmountFilesystemCompat unmounts a filesystem
func UnmountFilesystemCompat(mountDir string) error {
	if globalAdapter == nil {
		return fmt.Errorf("adapter not initialized, call InitDockerConfig first")
	}
	return globalAdapter.UnmountFilesystem(mountDir)
}

// ApplyNetworkConfigCompat applies network config to the mounted filesystem
func ApplyNetworkConfigCompat(mountDir string, hostname string, ipCIDR string, gateway string, dnsServers []string) error {
	if globalAdapter == nil {
		return fmt.Errorf("adapter not initialized, call InitDockerConfig first")
	}
	return globalAdapter.ApplyNetworkConfig(mountDir, hostname, ipCIDR, gateway, dnsServers)
}

// RecompressImageXZCompat compresses a disk image with XZ
func RecompressImageXZCompat(modifiedImgPath, finalXZPath string) error {
	if globalAdapter == nil {
		return fmt.Errorf("adapter not initialized, call InitDockerConfig first")
	}
	return globalAdapter.RecompressImageXZ(modifiedImgPath, finalXZPath)
}

// ExecuteFileOperationsCompat executes a batch of file operations
func ExecuteFileOperationsCompat(params ExecuteFileOperationsParams) error {
	if globalAdapter == nil {
		return fmt.Errorf("adapter not initialized, call InitDockerConfig first")
	}
	return globalAdapter.ExecuteFileOperations(params)
}

// WriteToImageFileCompat writes content to a file within the mounted image
func WriteToImageFileCompat(mountDir, relativePath string, content []byte, perm fs.FileMode) error {
	if globalAdapter == nil {
		return fmt.Errorf("adapter not initialized, call InitDockerConfig first")
	}
	return globalAdapter.WriteToImageFile(mountDir, relativePath, content, perm)
}

// CopyFileToImageCompat copies a local file into the mounted image
func CopyFileToImageCompat(mountDir, localSourcePath, relativeDestPath string) error {
	if globalAdapter == nil {
		return fmt.Errorf("adapter not initialized, call InitDockerConfig first")
	}
	return globalAdapter.CopyFileToImage(mountDir, localSourcePath, relativeDestPath)
}

// MkdirInImageCompat creates a directory within the mounted image
func MkdirInImageCompat(mountDir, relativePath string, perm fs.FileMode) error {
	if globalAdapter == nil {
		return fmt.Errorf("adapter not initialized, call InitDockerConfig first")
	}
	return globalAdapter.MkdirInImage(mountDir, relativePath, perm)
}

// ChmodInImageCompat changes permissions of a file/directory within the mounted image
func ChmodInImageCompat(mountDir, relativePath string, perm fs.FileMode) error {
	if globalAdapter == nil {
		return fmt.Errorf("adapter not initialized, call InitDockerConfig first")
	}
	return globalAdapter.ChmodInImage(mountDir, relativePath, perm)
}
